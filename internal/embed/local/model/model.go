// Package model implements the BERT encoder forward pass for
// sentence-transformers/all-MiniLM-L6-v2.
//
// Shapes, named by their dimensions, are (for a single encode):
//
//	L  — sequence length including [CLS] and [SEP]
//	H  — hidden size (384)
//	Hi — intermediate / FFN size (1536)
//	A  — number of attention heads (12)
//	Dk — per-head dimension (32 = H/A)
//
// All tensors are fp32, row-major, laid out exactly like PyTorch / NumPy.
//
// Weights come from a standard `BertModel` safetensors export with names
// like `embeddings.word_embeddings.weight` and
// `encoder.layer.3.attention.self.query.weight`. Linear weights are stored
// in PyTorch's `[out_features, in_features]` layout; we therefore multiply
// by Wᵀ using `mat.MatMulTransposeB`.
package model

import (
	"fmt"
	"math"
	"runtime"
	"sync"

	"marrow/internal/embed/local/mat"
	"marrow/internal/embed/local/weights"
)

// Config captures the structural hyperparameters needed to drive the
// forward pass. We do not read a JSON config file: these constants are
// fixed for MiniLM-L6-v2 and are verified at load time against the actual
// weight shapes.
type Config struct {
	Hidden       int
	Layers       int
	Heads        int
	Intermediate int
	MaxPositions int
	TypeVocab    int
	VocabSize    int
	LayerNormEps float32
}

// MiniLML6V2 is the canonical config for sentence-transformers/all-MiniLM-L6-v2.
func MiniLML6V2() Config {
	return Config{
		Hidden:       384,
		Layers:       6,
		Heads:        12,
		Intermediate: 1536,
		MaxPositions: 512,
		TypeVocab:    2,
		VocabSize:    30522,
		LayerNormEps: 1e-12,
	}
}

// Layer holds the weight tensors for a single encoder layer.
type Layer struct {
	Wq, Bq []float32 // query projection [H,H], [H]
	Wk, Bk []float32
	Wv, Bv []float32

	WattnOut, BattnOut []float32 // output projection [H,H], [H]
	LN1Gamma, LN1Beta  []float32 // post-attention LayerNorm [H]

	WInter, BInter []float32 // intermediate [Hi,H], [Hi]
	WOut, BOut     []float32 // output [H,Hi], [H]
	LN2Gamma, LN2Beta []float32 // post-FFN LayerNorm [H]
}

// Weights holds all the tensors needed by the model.
type Weights struct {
	WordEmb, PosEmb, TypeEmb []float32 // [V,H], [Pmax,H], [T,H]
	EmbLNGamma, EmbLNBeta    []float32 // [H]

	Layers []Layer
}

// Load reads every required tensor from f and validates shapes against cfg.
func Load(f *weights.File, cfg Config) (*Weights, error) {
	w := &Weights{Layers: make([]Layer, cfg.Layers)}

	get := func(name string, wantShape []int) ([]float32, error) {
		data, shape, err := f.F32(name)
		if err != nil {
			return nil, err
		}
		if !shapeEq(shape, wantShape) {
			return nil, fmt.Errorf("tensor %s: got shape %v want %v", name, shape, wantShape)
		}
		return data, nil
	}

	var err error
	if w.WordEmb, err = get("embeddings.word_embeddings.weight",
		[]int{cfg.VocabSize, cfg.Hidden}); err != nil {
		return nil, err
	}
	if w.PosEmb, err = get("embeddings.position_embeddings.weight",
		[]int{cfg.MaxPositions, cfg.Hidden}); err != nil {
		return nil, err
	}
	if w.TypeEmb, err = get("embeddings.token_type_embeddings.weight",
		[]int{cfg.TypeVocab, cfg.Hidden}); err != nil {
		return nil, err
	}
	if w.EmbLNGamma, err = get("embeddings.LayerNorm.weight",
		[]int{cfg.Hidden}); err != nil {
		return nil, err
	}
	if w.EmbLNBeta, err = get("embeddings.LayerNorm.bias",
		[]int{cfg.Hidden}); err != nil {
		return nil, err
	}

	for i := 0; i < cfg.Layers; i++ {
		prefix := fmt.Sprintf("encoder.layer.%d.", i)
		l := &w.Layers[i]
		get2 := func(short string, shape []int) []float32 {
			if err != nil {
				return nil
			}
			var d []float32
			d, err = get(prefix+short, shape)
			return d
		}
		H := cfg.Hidden
		Hi := cfg.Intermediate
		l.Wq = get2("attention.self.query.weight", []int{H, H})
		l.Bq = get2("attention.self.query.bias", []int{H})
		l.Wk = get2("attention.self.key.weight", []int{H, H})
		l.Bk = get2("attention.self.key.bias", []int{H})
		l.Wv = get2("attention.self.value.weight", []int{H, H})
		l.Bv = get2("attention.self.value.bias", []int{H})
		l.WattnOut = get2("attention.output.dense.weight", []int{H, H})
		l.BattnOut = get2("attention.output.dense.bias", []int{H})
		l.LN1Gamma = get2("attention.output.LayerNorm.weight", []int{H})
		l.LN1Beta = get2("attention.output.LayerNorm.bias", []int{H})
		l.WInter = get2("intermediate.dense.weight", []int{Hi, H})
		l.BInter = get2("intermediate.dense.bias", []int{Hi})
		l.WOut = get2("output.dense.weight", []int{H, Hi})
		l.BOut = get2("output.dense.bias", []int{H})
		l.LN2Gamma = get2("output.LayerNorm.weight", []int{H})
		l.LN2Beta = get2("output.LayerNorm.bias", []int{H})
		if err != nil {
			return nil, err
		}
	}
	return w, nil
}

// pickMatMul returns the serial or parallel matmul based on input length.
// Below ~64 tokens the goroutine overhead exceeds the win; above, the row
// split gives near-linear scaling.
func pickMatMul(L int) func(a, b, c []float32, M, K, N int) {
	if L >= 64 {
		return mat.MatMulTransposeBParallel
	}
	return mat.MatMulTransposeB
}

func shapeEq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Encoder is a stateless forward-pass driver bound to a config + weights.
// A single Encoder instance is safe for concurrent Encode calls only if
// callers allocate their own scratch space — the convenience Encode method
// allocates per call, which keeps things simple at the cost of GC pressure.
type Encoder struct {
	Cfg Config
	W   *Weights
}

func NewEncoder(cfg Config, w *Weights) *Encoder {
	return &Encoder{Cfg: cfg, W: w}
}

// Encode runs the full forward pass over the given token ids and returns
// a mean-pooled, L2-normalized hidden-state vector of length Hidden.
// typeIDs may be nil, in which case all-zeros is assumed.
func (e *Encoder) Encode(ids []int32, mask []int32, typeIDs []int32) []float32 {
	cfg := e.Cfg
	w := e.W
	L := len(ids)
	H := cfg.Hidden

	if typeIDs == nil {
		typeIDs = make([]int32, L)
	}

	// ---- 1. Embeddings ----
	x := make([]float32, L*H)
	for i := 0; i < L; i++ {
		tok := int(ids[i])
		pos := i
		typ := int(typeIDs[i])
		row := x[i*H : (i+1)*H]
		we := w.WordEmb[tok*H : (tok+1)*H]
		pe := w.PosEmb[pos*H : (pos+1)*H]
		te := w.TypeEmb[typ*H : (typ+1)*H]
		for j := 0; j < H; j++ {
			row[j] = we[j] + pe[j] + te[j]
		}
	}
	mat.LayerNorm(x, w.EmbLNGamma, w.EmbLNBeta, L, H, cfg.LayerNormEps)

	// ---- 2. Encoder layers ----
	Hi := cfg.Intermediate
	A := cfg.Heads
	Dk := H / A
	scale := float32(1.0 / math.Sqrt(float64(Dk)))

	q := make([]float32, L*H)
	k := make([]float32, L*H)
	v := make([]float32, L*H)
	attn := make([]float32, L*H)
	attnOut := make([]float32, L*H)
	inter := make([]float32, L*Hi)
	ffnOut := make([]float32, L*H)

	// Per-head scratch in contiguous [L, Dk] layout so we can feed it
	// directly to the vectorized matmul primitives. Repacking once per
	// head costs O(L·Dk) — cheap next to the O(L²·Dk) attention itself.
	//
	// With parallel head execution, each worker needs its own scratch so
	// they don't tread on each other. We gate on L: below ~64 tokens the
	// per-head work is small enough that goroutine wakeup costs (measured
	// as ~25% of an encode on short inputs) dominate the speedup.
	workers := 1
	if L >= 64 {
		workers = runtime.GOMAXPROCS(0)
		if workers > A {
			workers = A
		}
		if workers < 1 {
			workers = 1
		}
	}
	type headScratch struct {
		qh, kh, vh []float32
		attnH      []float32
		scores     []float32
	}
	scratch := make([]headScratch, workers)
	for wi := range scratch {
		scratch[wi] = headScratch{
			qh:     make([]float32, L*Dk),
			kh:     make([]float32, L*Dk),
			vh:     make([]float32, L*Dk),
			attnH:  make([]float32, L*Dk),
			scores: make([]float32, L*L),
		}
	}

	for li := 0; li < cfg.Layers; li++ {
		layer := &w.Layers[li]

		// Q/K/V projections: x [L,H] @ W.T + b  where W is [H,H]. For long
		// sequences the row-split parallel path wins; for very short inputs
		// the goroutine dispatch dwarfs the work so we stay serial.
		mm := pickMatMul(L)
		mm(x, layer.Wq, q, L, H, H)
		mat.AddBias(q, layer.Bq, L, H)
		mm(x, layer.Wk, k, L, H, H)
		mat.AddBias(k, layer.Bk, L, H)
		mm(x, layer.Wv, v, L, H, H)
		mat.AddBias(v, layer.Bv, L, H)

		// Multi-head attention: one job per head, executed in parallel.
		// Each worker owns a private scratch; we assign heads round-robin
		// so worker 0 handles heads 0, 0+workers, ... and so on.
		var wg sync.WaitGroup
		for wi := 0; wi < workers; wi++ {
			wg.Add(1)
			go func(wi int) {
				defer wg.Done()
				sc := &scratch[wi]
				for h := wi; h < A; h += workers {
					off := h * Dk
					for i := 0; i < L; i++ {
						copy(sc.qh[i*Dk:(i+1)*Dk], q[i*H+off:i*H+off+Dk])
						copy(sc.kh[i*Dk:(i+1)*Dk], k[i*H+off:i*H+off+Dk])
						copy(sc.vh[i*Dk:(i+1)*Dk], v[i*H+off:i*H+off+Dk])
					}

					// scores = qh @ kh^T  — shape [L,L].
					mat.MatMulTransposeB(sc.qh, sc.kh, sc.scores, L, Dk, L)
					for i := 0; i < L; i++ {
						row := sc.scores[i*L : (i+1)*L]
						for j := 0; j < L; j++ {
							if mask[j] == 0 {
								row[j] = -1e9
							} else {
								row[j] *= scale
							}
						}
					}
					mat.SoftmaxRows(sc.scores, L, L)
					mat.MatMul(sc.scores, sc.vh, sc.attnH, L, L, Dk)
					for i := 0; i < L; i++ {
						copy(attn[i*H+off:i*H+off+Dk], sc.attnH[i*Dk:(i+1)*Dk])
					}
				}
			}(wi)
		}
		wg.Wait()

		// Attention output projection + residual + LayerNorm.
		mm(attn, layer.WattnOut, attnOut, L, H, H)
		mat.AddBias(attnOut, layer.BattnOut, L, H)
		mat.AddInPlace(x, attnOut)
		mat.LayerNorm(x, layer.LN1Gamma, layer.LN1Beta, L, H, cfg.LayerNormEps)

		// FFN: intermediate + GELU + output + residual + LayerNorm.
		mm(x, layer.WInter, inter, L, H, Hi)
		mat.AddBias(inter, layer.BInter, L, Hi)
		mat.GELU(inter)
		mm(inter, layer.WOut, ffnOut, L, Hi, H)
		mat.AddBias(ffnOut, layer.BOut, L, H)
		mat.AddInPlace(x, ffnOut)
		mat.LayerNorm(x, layer.LN2Gamma, layer.LN2Beta, L, H, cfg.LayerNormEps)
	}

	// ---- 3. Mean-pool + L2 normalize ----
	out := make([]float32, H)
	mat.MeanPoolMasked(x, mask, L, H, out)
	mat.L2Normalize(out)
	return out
}
