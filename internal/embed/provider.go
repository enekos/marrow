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
)

// NewProvider creates an embedding Func based on the provider name.
func NewProvider(provider, model, baseURL, apiKey string) (Func, error) {
	switch strings.ToLower(provider) {
	case "mock", "":
		return NewMock(), nil
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
	body, _ := json.Marshal(ollamaReq{Model: o.Model, Prompt: text})
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
	Input string `json:"input"`
}

type openaiResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (o *OpenAI) Embed(ctx context.Context, text string) ([]float32, error) {
	body, _ := json.Marshal(openaiReq{Model: o.Model, Input: text})
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
