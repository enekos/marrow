// Package model implements the BERT encoder forward pass for
// sentence-transformers/all-MiniLM-L6-v2.
//
// Shape vocabulary (single encode):
//
//	L  — sequence length including [CLS] and [SEP]
//	H  — hidden size (384)
//	Hi — intermediate / FFN size (1536)
//	A  — number of attention heads (12)
//	Dk — per-head dimension (32 = H/A)
//
// All tensors are fp32, row-major, laid out exactly like PyTorch / NumPy.
// Linear weights live in PyTorch's `[out_features, in_features]` layout so
// projections use mat.MatMulTransposeB rather than a pre-transposed copy.
package model

import (
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/enekos/marrow/internal/embed/local/mat"
	"github.com/enekos/marrow/internal/embed/local/weights"
)

// parallelThreshold is the sequence length at or above which we use
// parallel matmul and per-head parallelism. Below this, goroutine dispatch
// costs exceed the speedup; above, parallel scales near-linearly.
const parallelThreshold = 64

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
	Wq, Bq             []float32 // query projection   [H,H], [H]
	Wk, Bk             []float32 // key projection     [H,H], [H]
	Wv, Bv             []float32 // value projection   [H,H], [H]
	WattnOut, BattnOut []float32 // attention output   [H,H], [H]
	LN1Gamma, LN1Beta  []float32 // post-attention LN  [H]
	WInter, BInter     []float32 // FFN intermediate   [Hi,H], [Hi]
	WOut, BOut         []float32 // FFN output         [H,Hi], [H]
	LN2Gamma, LN2Beta  []float32 // post-FFN LN        [H]
}

// Weights holds every tensor needed by the model.
type Weights struct {
	WordEmb, PosEmb, TypeEmb []float32 // [V,H], [Pmax,H], [T,H]
	EmbLNGamma, EmbLNBeta    []float32 // [H]
	Layers                   []Layer
}

// Load reads every required tensor from f and validates shapes against cfg.
func Load(f *weights.File, cfg Config) (*Weights, error) {
	ld := loader{f: f}
	w := &Weights{Layers: make([]Layer, cfg.Layers)}

	H, Hi := cfg.Hidden, cfg.Intermediate
	w.WordEmb = ld.get("embeddings.word_embeddings.weight", cfg.VocabSize, H)
	w.PosEmb = ld.get("embeddings.position_embeddings.weight", cfg.MaxPositions, H)
	w.TypeEmb = ld.get("embeddings.token_type_embeddings.weight", cfg.TypeVocab, H)
	w.EmbLNGamma = ld.get("embeddings.LayerNorm.weight", H)
	w.EmbLNBeta = ld.get("embeddings.LayerNorm.bias", H)

	for i := 0; i < cfg.Layers; i++ {
		p := fmt.Sprintf("encoder.layer.%d.", i)
		l := &w.Layers[i]
		l.Wq = ld.get(p+"attention.self.query.weight", H, H)
		l.Bq = ld.get(p+"attention.self.query.bias", H)
		l.Wk = ld.get(p+"attention.self.key.weight", H, H)
		l.Bk = ld.get(p+"attention.self.key.bias", H)
		l.Wv = ld.get(p+"attention.self.value.weight", H, H)
		l.Bv = ld.get(p+"attention.self.value.bias", H)
		l.WattnOut = ld.get(p+"attention.output.dense.weight", H, H)
		l.BattnOut = ld.get(p+"attention.output.dense.bias", H)
		l.LN1Gamma = ld.get(p+"attention.output.LayerNorm.weight", H)
		l.LN1Beta = ld.get(p+"attention.output.LayerNorm.bias", H)
		l.WInter = ld.get(p+"intermediate.dense.weight", Hi, H)
		l.BInter = ld.get(p+"intermediate.dense.bias", Hi)
		l.WOut = ld.get(p+"output.dense.weight", H, Hi)
		l.BOut = ld.get(p+"output.dense.bias", H)
		l.LN2Gamma = ld.get(p+"output.LayerNorm.weight", H)
		l.LN2Beta = ld.get(p+"output.LayerNorm.bias", H)
	}
	if ld.err != nil {
		return nil, ld.err
	}
	return w, nil
}

// loader accumulates the first error so Load can stay a flat sequence of
// `w.X = ld.get(...)` lines without an `if err != nil` after each.
type loader struct {
	f   *weights.File
	err error
}

func (ld *loader) get(name string, wantShape ...int) []float32 {
	if ld.err != nil {
		return nil
	}
	data, shape, err := ld.f.F32(name)
	if err != nil {
		ld.err = err
		return nil
	}
	if !shapeEq(shape, wantShape) {
		ld.err = fmt.Errorf("tensor %s: got shape %v want %v", name, shape, wantShape)
		return nil
	}
	return data
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

// Encoder drives the forward pass. A single instance is safe for concurrent
// Encode calls — each call allocates its own scratch.
type Encoder struct {
	Cfg Config
	W   *Weights
}

// NewEncoder returns an Encoder bound to cfg and weights.
func NewEncoder(cfg Config, w *Weights) *Encoder {
	return &Encoder{Cfg: cfg, W: w}
}

// Encode runs the full forward pass over the given token ids and returns a
// mean-pooled, L2-normalized hidden-state vector of length Hidden. typeIDs
// may be nil, in which case all-zeros (single-segment input) is assumed.
func (e *Encoder) Encode(ids, mask, typeIDs []int32) []float32 {
	cfg, w := e.Cfg, e.W
	L, H := len(ids), cfg.Hidden

	if typeIDs == nil {
		typeIDs = make([]int32, L)
	}

	x := embedTokens(cfg, w, ids, typeIDs)
	mat.LayerNorm(x, w.EmbLNGamma, w.EmbLNBeta, L, H, cfg.LayerNormEps)

	s := newScratch(cfg, L)
	mm := pickMatMul(L)
	for i := range w.Layers {
		s.runLayer(cfg, &w.Layers[i], x, mask, mm)
	}

	out := make([]float32, H)
	mat.MeanPoolMasked(x, mask, L, H, out)
	mat.L2Normalize(out)
	return out
}

// embedTokens computes x[i] = WordEmb[ids[i]] + PosEmb[i] + TypeEmb[typeIDs[i]]
// for each position i. Shape: [L, H].
func embedTokens(cfg Config, w *Weights, ids, typeIDs []int32) []float32 {
	L, H := len(ids), cfg.Hidden
	x := make([]float32, L*H)
	for i := 0; i < L; i++ {
		row := x[i*H : (i+1)*H]
		we := w.WordEmb[int(ids[i])*H : (int(ids[i])+1)*H]
		pe := w.PosEmb[i*H : (i+1)*H]
		te := w.TypeEmb[int(typeIDs[i])*H : (int(typeIDs[i])+1)*H]
		for j := 0; j < H; j++ {
			row[j] = we[j] + pe[j] + te[j]
		}
	}
	return x
}

// matMulFunc is the signature of serial and parallel matmul primitives.
type matMulFunc func(a, b, c []float32, M, K, N int)

// pickMatMul picks serial vs parallel matmul based on sequence length.
func pickMatMul(L int) matMulFunc {
	if L >= parallelThreshold {
		return mat.MatMulTransposeBParallel
	}
	return mat.MatMulTransposeB
}

// scratch holds the per-encode buffers that the forward pass reuses across
// all six layers. The [L, Dk] per-head buffers live in headScratch and are
// one-per-worker so attention heads can run concurrently without sharing.
type scratch struct {
	q, k, v       []float32
	attn, attnOut []float32
	inter, ffnOut []float32
	heads         []headScratch
	workers       int
	scale         float32
}

type headScratch struct {
	qh, kh, vh, attnH, scores []float32
}

func newScratch(cfg Config, L int) *scratch {
	H, Hi, A, Dk := cfg.Hidden, cfg.Intermediate, cfg.Heads, cfg.Hidden/cfg.Heads
	s := &scratch{
		q:       make([]float32, L*H),
		k:       make([]float32, L*H),
		v:       make([]float32, L*H),
		attn:    make([]float32, L*H),
		attnOut: make([]float32, L*H),
		inter:   make([]float32, L*Hi),
		ffnOut:  make([]float32, L*H),
		scale:   float32(1.0 / math.Sqrt(float64(Dk))),
	}

	// One head-scratch per worker. Short inputs get a single serial worker
	// to avoid goroutine wakeup cost.
	s.workers = 1
	if L >= parallelThreshold {
		s.workers = runtime.GOMAXPROCS(0)
		if s.workers > A {
			s.workers = A
		}
		if s.workers < 1 {
			s.workers = 1
		}
	}
	s.heads = make([]headScratch, s.workers)
	for i := range s.heads {
		s.heads[i] = headScratch{
			qh:     make([]float32, L*Dk),
			kh:     make([]float32, L*Dk),
			vh:     make([]float32, L*Dk),
			attnH:  make([]float32, L*Dk),
			scores: make([]float32, L*L),
		}
	}
	return s
}

// runLayer applies one encoder layer to x in place.
func (s *scratch) runLayer(cfg Config, l *Layer, x []float32, mask []int32, mm matMulFunc) {
	L, H, Hi := len(x)/cfg.Hidden, cfg.Hidden, cfg.Intermediate

	// Q/K/V projections.
	mm(x, l.Wq, s.q, L, H, H)
	mat.AddBias(s.q, l.Bq, L, H)
	mm(x, l.Wk, s.k, L, H, H)
	mat.AddBias(s.k, l.Bk, L, H)
	mm(x, l.Wv, s.v, L, H, H)
	mat.AddBias(s.v, l.Bv, L, H)

	// Multi-head self-attention → s.attn.
	s.multiHeadAttention(cfg, L, mask)

	// Attention output + residual + LayerNorm.
	mm(s.attn, l.WattnOut, s.attnOut, L, H, H)
	mat.AddBias(s.attnOut, l.BattnOut, L, H)
	mat.AddInPlace(x, s.attnOut)
	mat.LayerNorm(x, l.LN1Gamma, l.LN1Beta, L, H, cfg.LayerNormEps)

	// FFN: intermediate + GELU + output + residual + LayerNorm.
	mm(x, l.WInter, s.inter, L, H, Hi)
	mat.AddBias(s.inter, l.BInter, L, Hi)
	mat.GELU(s.inter)
	mm(s.inter, l.WOut, s.ffnOut, L, Hi, H)
	mat.AddBias(s.ffnOut, l.BOut, L, H)
	mat.AddInPlace(x, s.ffnOut)
	mat.LayerNorm(x, l.LN2Gamma, l.LN2Beta, L, H, cfg.LayerNormEps)
}

// multiHeadAttention runs all A attention heads over s.q/s.k/s.v and writes
// the concatenated result into s.attn. Heads are assigned round-robin to
// workers so workers make roughly equal progress even when A isn't evenly
// divisible by workers.
func (s *scratch) multiHeadAttention(cfg Config, L int, mask []int32) {
	A, Dk := cfg.Heads, cfg.Hidden/cfg.Heads

	var wg sync.WaitGroup
	wg.Add(s.workers)
	for wi := 0; wi < s.workers; wi++ {
		go func(wi int) {
			defer wg.Done()
			hs := &s.heads[wi]
			for h := wi; h < A; h += s.workers {
				s.headForward(hs, h, L, Dk, cfg.Hidden, mask)
			}
		}(wi)
	}
	wg.Wait()
}

// headForward computes one attention head.
func (s *scratch) headForward(hs *headScratch, h, L, Dk, H int, mask []int32) {
	off := h * Dk

	// Repack interleaved [L, H] slices into contiguous [L, Dk] so the
	// matmul primitives hit their fast path.
	for i := 0; i < L; i++ {
		copy(hs.qh[i*Dk:(i+1)*Dk], s.q[i*H+off:i*H+off+Dk])
		copy(hs.kh[i*Dk:(i+1)*Dk], s.k[i*H+off:i*H+off+Dk])
		copy(hs.vh[i*Dk:(i+1)*Dk], s.v[i*H+off:i*H+off+Dk])
	}

	// scores = qh @ khᵀ / √Dk, with -inf at padded columns.
	mat.MatMulTransposeB(hs.qh, hs.kh, hs.scores, L, Dk, L)
	for i := 0; i < L; i++ {
		row := hs.scores[i*L : (i+1)*L]
		for j := 0; j < L; j++ {
			if mask[j] == 0 {
				row[j] = -1e9
			} else {
				row[j] *= s.scale
			}
		}
	}
	mat.SoftmaxRows(hs.scores, L, L)
	mat.MatMul(hs.scores, hs.vh, hs.attnH, L, L, Dk)

	// Scatter per-head output back into the interleaved [L, H] attn buffer.
	for i := 0; i < L; i++ {
		copy(s.attn[i*H+off:i*H+off+Dk], hs.attnH[i*Dk:(i+1)*Dk])
	}
}
