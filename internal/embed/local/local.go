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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"marrow/internal/embed/local/model"
	"marrow/internal/embed/local/tokenizer"
	"marrow/internal/embed/local/weights"
)

// Encoder is a reusable embedder. Safe for concurrent use: the tokenizer
// and weights are read-only after load, and model.Encoder.Encode allocates
// fresh per-call scratch so concurrent calls share no mutable state.
type Encoder struct {
	tk  *tokenizer.Tokenizer
	enc *model.Encoder
}

// New loads the model directory and returns an Encoder.
func New(modelDir string) (*Encoder, error) {
	vocabPath := filepath.Join(modelDir, "tokenizer_vocab.txt")
	weightsPath := filepath.Join(modelDir, "model.safetensors")

	tk, err := tokenizer.Load(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}
	if max, ok := readSentenceBertMaxSeq(modelDir); ok {
		tk.SetMaxInput(max)
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
	return e.enc.Encode(enc.IDs, enc.AttentionMask, enc.TypeIDs), nil
}

// readSentenceBertMaxSeq reads sentence_bert_config.json (an optional
// sibling of the model weights) and returns the max_seq_length. Missing,
// unparseable, or zero-valued files return ok=false so the tokenizer keeps
// its built-in default.
func readSentenceBertMaxSeq(modelDir string) (int, bool) {
	path := filepath.Join(modelDir, "sentence_bert_config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var cfg struct {
		MaxSeqLength int `json:"max_seq_length"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return 0, false
	}
	if cfg.MaxSeqLength <= 0 {
		return 0, false
	}
	return cfg.MaxSeqLength, true
}
