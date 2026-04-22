// Package local is Marrow's pure-Go embedding provider. It loads a
// sentence-transformers/all-MiniLM-L6-v2 checkpoint from disk and runs
// inference in-process — no Ollama, no OpenAI, no CGo.
//
// A model directory must contain:
//
//	tokenizer_vocab.txt     bert-base-uncased vocab, one token per line
//	model.safetensors       fp32 weights from BertModel.save_pretrained
//
// The config is fixed to MiniLM-L6-v2. A future revision can read a
// `config.json` to support other BERT-family checkpoints.
package local

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"marrow/internal/embed/local/model"
	"marrow/internal/embed/local/tokenizer"
	"marrow/internal/embed/local/weights"
)

// Encoder is a reusable embedder. Safe for concurrent use — Encode takes no
// shared mutable state except the read-only weights and tokenizer.
type Encoder struct {
	tk  *tokenizer.Tokenizer
	enc *model.Encoder
	mu  sync.Mutex // serializes Encode to avoid GC pressure from concurrent scratch allocs
}

// New loads the model directory and returns an Encoder.
func New(modelDir string) (*Encoder, error) {
	vocabPath := filepath.Join(modelDir, "tokenizer_vocab.txt")
	weightsPath := filepath.Join(modelDir, "model.safetensors")

	tk, err := tokenizer.Load(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}
	wf, err := weights.Open(weightsPath)
	if err != nil {
		return nil, fmt.Errorf("open weights: %w", err)
	}
	cfg := model.MiniLML6V2()
	w, err := model.Load(wf, cfg)
	if err != nil {
		return nil, fmt.Errorf("load weights: %w", err)
	}
	return &Encoder{tk: tk, enc: model.NewEncoder(cfg, w)}, nil
}

// Embed tokenizes the text, runs the encoder, and returns the 384-dim
// mean-pooled L2-normalized embedding.
func (e *Encoder) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	enc := e.tk.Encode(text)
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enc.Encode(enc.IDs, enc.AttentionMask, enc.TypeIDs), nil
}
