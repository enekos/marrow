package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/enekos/marrow/internal/embed/local"
)

// NewBatchProvider returns a BatchFunc for the given provider. Providers
// with native batching (OpenAI) get a true batched implementation; others
// fall back to N sequential Func calls.
func NewBatchProvider(provider, model, baseURL, apiKey string) (BatchFunc, error) {
	switch strings.ToLower(provider) {
	case "openai":
		if apiKey == "" {
			return nil, fmt.Errorf("openai provider requires api_key")
		}
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		if model == "" {
			model = "text-embedding-3-small"
		}
		o := &OpenAI{
			BaseURL: strings.TrimSuffix(baseURL, "/"),
			APIKey:  apiKey,
			Model:   model,
			Client:  &http.Client{Timeout: 2 * time.Minute},
		}
		return o.EmbedBatch, nil
	default:
		f, err := NewProvider(provider, model, baseURL, apiKey)
		if err != nil {
			return nil, err
		}
		return FallbackBatch(f), nil
	}
}

// Options holds the configuration for constructing a provider. Prefer
// NewProviderWithOptions for provider-specific knobs such as ModelPath.
type Options struct {
	Provider  string
	Model     string
	BaseURL   string
	APIKey    string
	ModelPath string // used by "local"
}

// NewProvider creates an embedding Func based on the provider name.
// An empty provider is a configuration error; callers must choose explicitly
// so mocks are never used in production by accident.
func NewProvider(provider, model, baseURL, apiKey string) (Func, error) {
	return NewProviderWithOptions(Options{
		Provider: provider,
		Model:    model,
		BaseURL:  baseURL,
		APIKey:   apiKey,
	})
}

// NewProviderWithOptions is the fully-parameterized constructor.
func NewProviderWithOptions(opts Options) (Func, error) {
	provider := opts.Provider
	model := opts.Model
	baseURL := opts.BaseURL
	apiKey := opts.APIKey
	switch strings.ToLower(provider) {
	case "":
		return nil, fmt.Errorf("embedding.provider not configured; set it to one of: mock, local, ollama, openai")
	case "mock":
		return NewMock(), nil
	case "local":
		if opts.ModelPath == "" {
			return nil, fmt.Errorf("local provider requires embedding.model_path")
		}
		enc, err := local.New(opts.ModelPath)
		if err != nil {
			return nil, fmt.Errorf("load local model: %w", err)
		}
		return enc.Embed, nil
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		if model == "" {
			model = "nomic-embed-text"
		}
		return NewOllama(baseURL, model), nil
	case "openai":
		if apiKey == "" {
			return nil, fmt.Errorf("openai provider requires api_key")
		}
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		if model == "" {
			model = "text-embedding-3-small"
		}
		return NewOpenAI(baseURL, apiKey, model), nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", provider)
	}
}

// Ollama embedder talks to the Ollama API.
type Ollama struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewOllama(baseURL, model string) Func {
	o := &Ollama{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		Model:   model,
		Client:  &http.Client{Timeout: 2 * time.Minute},
	}
	return o.Embed
}

type ollamaReq struct {
	Model string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaResp struct {
	Embedding []float32 `json:"embedding"`
}

func (o *Ollama) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(ollamaReq{Model: o.Model, Prompt: text})
	if err != nil {
		return nil, fmt.Errorf("marshal ollama request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama %s: %s", resp.Status, string(b))
	}
	var r ollamaResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return normalize(r.Embedding), nil
}

// OpenAI embedder talks to the OpenAI-compatible API.
type OpenAI struct {
	BaseURL string
	APIKey  string
	Model   string
	Client  *http.Client
}

func NewOpenAI(baseURL, apiKey, model string) Func {
	o := &OpenAI{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		Client:  &http.Client{Timeout: 2 * time.Minute},
	}
	return o.Embed
}

type openaiReq struct {
	Model string `json:"model"`
	Input any    `json:"input"` // string or []string
}

type openaiResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// OpenAIBatchSize caps how many inputs are sent per HTTP request. OpenAI's
// documented limit is 2048 inputs; we use a conservative 96 to keep single
// request bodies under ~1 MB for 1500-char chunks.
const OpenAIBatchSize = 96

// EmbedBatch sends multiple inputs in a single request. Results are returned
// in the same order as the input texts. Inputs longer than the model's
// context are the caller's responsibility (use the chunker first).
func (o *OpenAI) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([][]float32, len(texts))
	for start := 0; start < len(texts); start += OpenAIBatchSize {
		end := start + OpenAIBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		body, err := json.Marshal(openaiReq{Model: o.Model, Input: texts[start:end]})
		if err != nil {
			return nil, fmt.Errorf("marshal openai batch: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/v1/embeddings", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
		resp, err := o.Client.Do(req)
		if err != nil {
			return nil, err
		}
		var r openaiResp
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("openai %s: %s", resp.Status, string(b))
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()
		if len(r.Data) != end-start {
			return nil, fmt.Errorf("openai returned %d embeddings for %d inputs", len(r.Data), end-start)
		}
		// Trust the `index` field so responses reordered by the server still
		// align with request order.
		for _, item := range r.Data {
			if item.Index < 0 || item.Index >= end-start {
				return nil, fmt.Errorf("openai returned out-of-range index %d", item.Index)
			}
			out[start+item.Index] = normalize(item.Embedding)
		}
	}
	return out, nil
}

func (o *OpenAI) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(openaiReq{Model: o.Model, Input: text})
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai %s: %s", resp.Status, string(b))
	}
	var r openaiResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	if len(r.Data) == 0 {
		return nil, fmt.Errorf("openai returned no embeddings")
	}
	return normalize(r.Data[0].Embedding), nil
}

func normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	norm := float32(math.Sqrt(sum))
	for i := range v {
		v[i] /= norm
	}
	return v
}
