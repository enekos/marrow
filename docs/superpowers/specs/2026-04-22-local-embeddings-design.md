# Local Embeddings — Design

Ship a pure-Go implementation of a small pre-trained sentence encoder so that
Marrow can produce real semantic embeddings with no external service (no
Ollama, no OpenAI) and no new C runtime dependency.

## Goal

Add a new `local` embedding provider that:

- Runs entirely in-process, pure Go.
- Produces 384-dim L2-normalized vectors (matching the existing
  `sqlite-vec` schema — no DB migration).
- Matches the reference output of `sentence-transformers/all-MiniLM-L6-v2`
  within a tight tolerance on a canary set.
- Does not require CGo beyond what Marrow already uses (sqlite).

## Non-goals

- GPU acceleration.
- Training a model from scratch.
- Supporting arbitrary HuggingFace checkpoints. We target exactly one model
  family (BERT-style MiniLM variants). Other families can come later.
- Fine-tuning or on-device adaptation.

## Model choice

`sentence-transformers/all-MiniLM-L6-v2`:

- Apache-2.0.
- 384-dim output → matches existing `EmbeddingDim = 384`.
- 6 encoder layers, hidden size 384, 12 attention heads, intermediate size
  1536, max sequence length 512.
- WordPiece tokenizer, `bert-base-uncased` vocab (30522 tokens).
- Pooling: mean over token embeddings weighted by the attention mask,
  followed by L2 normalization.

Total weights ~90 MB in fp32. Acceptable to ship/download separately; not
embedded in the binary.

## Architecture

New subtree under `internal/embed/local/`:

```
internal/embed/local/
├── local.go                 # public NewLocal(dir) entry point, encoder struct
├── tokenizer/
│   ├── wordpiece.go         # BasicTokenizer + WordPiece
│   └── wordpiece_test.go
├── weights/
│   ├── safetensors.go       # header parse + tensor view
│   └── safetensors_test.go
├── model/
│   ├── embeddings.go        # token + position + type embeddings + LN
│   ├── attention.go         # multi-head self-attention
│   ├── ffn.go               # intermediate + output (GELU)
│   ├── encoder.go           # one encoder layer, stack of 6
│   └── model_test.go
└── mat/
    ├── mat.go               # matmul, softmax, layernorm, gelu, pooling
    └── mat_test.go
```

`internal/embed/provider.go` gains a new `case "local"` that constructs
`local.NewLocal(baseURL)` — where `base_url` is repurposed as the model
directory path for the `local` provider.

## Model directory layout

A model directory holds:

```
<dir>/
├── config.json              # bert config (vocab_size, hidden_size, ...)
├── tokenizer_vocab.txt      # one token per line, 30522 entries
└── model.safetensors        # fp32 weights
```

Path is `embedding.base_url` in config (repurposed) or, more explicitly, a
new `embedding.model_path` field. We'll add `ModelPath` as a first-class
field and keep `BaseURL` untouched.

## Forward pass

For input text `s`:

1. **Tokenize** → `ids`, `mask` of length `L ≤ 512`.
   - Prepend `[CLS]`, append `[SEP]`, pad with `[PAD]` to batch length.
2. **Embeddings**: `E = WE[ids] + WP[pos] + WT[0]`, then LayerNorm.
3. **For each of 6 encoder layers**:
   - Multi-head self-attention with the attention mask:
     `Q, K, V = E @ Wq, E @ Wk, E @ Wv` (split into 12 heads of 32 dims).
     `A = softmax((Q @ Kᵀ)/√dₖ + mask) @ V`
     Reassemble, project through `Wo`, residual + LayerNorm.
   - FFN: `H' = GELU(H @ W1 + b1) @ W2 + b2`, residual + LayerNorm.
4. **Mean pool**: `out = sum(H ⊙ mask) / sum(mask)` across sequence dim.
5. **L2 normalize**: `out / ‖out‖₂`.

All math in fp32. Matmul via `gonum.org/v1/gonum/blas/gonum` (pure Go with
assembly kernels for amd64/arm64). LayerNorm, softmax, GELU hand-rolled in
`mat/`. No quantization in v1.

## Parity validation

Commit a fixture `internal/embed/local/testdata/parity.json` with ~20
short/medium/long English strings plus their reference 384-dim embeddings
produced by Python `sentence-transformers`. The Go test asserts cosine
similarity > 0.999 for each entry. A small Python script
(`scripts/gen-parity-fixture.py`) regenerates the fixture.

Marrow CI does not run the Python script; the fixture is pre-generated and
checked in. Contributors regenerate only when the model changes.

## Distribution of weights

v1: user supplies the model directory. We provide a shell script
`scripts/download-minilm.sh` that downloads the three files from the
HuggingFace CDN into `~/.cache/marrow/models/minilm-l6-v2/`. `embedding.model_path`
then points there.

v2 (future, not in this spec): optional auto-download on first use when the
configured path doesn't exist.

## Config surface

```toml
[embedding]
provider   = "local"
model_path = "~/.cache/marrow/models/minilm-l6-v2"
```

`embedding.model` is ignored for `local`. `embedding.base_url` and
`embedding.api_key` are ignored.

## Performance

Target: ≤ 30 ms / encode for a 64-token input on a modern laptop (M-series
or Ryzen/Core), single-threaded. A full 512-token encode will be much
slower (attention is O(L²)); that's acceptable since chunking keeps most
inputs short. No concurrency primitives in v1; the caller (indexer, search
engine) already serializes embedding calls.

## Error handling

- Missing model files → return a descriptive error from `NewLocal` so
  `embed.Validate` fires at startup, not mid-index.
- Tokenizer input with no decodable characters → emit a single `[UNK]`
  token rather than an empty sequence. This matches reference behavior.
- Inputs longer than 512 tokens are truncated (matching HF default). A
  debug log records the truncation count.

## Phasing

1. **Tokenizer** — WordPiece with a BERT vocab. Tests against HF reference
   ids for ~50 strings.
2. **Safetensors loader** — parse header JSON, mmap body, expose
   `Tensor(name) (data []float32, shape []int)`.
3. **Forward pass** — embeddings, one encoder layer, stack to 6, pooling.
   Unit tests per block check against Python reference activations.
4. **End-to-end parity** — canary fixture, cosine-similarity gate.
5. **Provider wiring** — add `ModelPath` to `EmbeddingConfig`, register
   `local` in `NewProvider`, update Validate/docs/README/main.go warning.
6. **Perf pass** — benchmarks, profile, optimize hot spots if we're far
   from the target. Potential follow-up: int8 quantization, custom kernels.

## Risks

- **WordPiece subtleties.** BERT tokenization has edge cases around CJK,
  accents, and Unicode normalization. Mitigation: strict byte-level
  parity tests vs the HF tokenizer on a broad corpus slice.
- **Numerical drift.** Mean-pool + normalize is forgiving, but softmax +
  layernorm can drift with fp32 reorderings. Mitigation: 1e-3 per-element
  tolerance at block boundaries, 1e-4 cosine tolerance end-to-end.
- **Gonum matmul perf on ARM.** If gonum assembly is amd64-only, ARM falls
  back to pure Go loops. Mitigation: benchmark early; drop to a hand-rolled
  kernel if needed.
