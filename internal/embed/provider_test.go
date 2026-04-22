package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestNewProvider_Mock(t *testing.T) {
	f, err := NewProvider("mock", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil Func")
	}
}

func TestNewProvider_Empty(t *testing.T) {
	_, err := NewProvider("", "", "", "")
	if err == nil {
		t.Fatal("expected error for unconfigured provider")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not-configured error, got: %v", err)
	}
}

func TestNewProvider_Ollama_Defaults(t *testing.T) {
	f, err := NewProvider("ollama", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil Func")
	}
}

func TestNewProvider_OpenAI_MissingKey(t *testing.T) {
	_, err := NewProvider("openai", "", "", "")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("expected api_key error, got: %v", err)
	}
}

func TestNewProvider_OpenAI_Defaults(t *testing.T) {
	f, err := NewProvider("openai", "", "", "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil Func")
	}
}

func TestNewProvider_Unknown(t *testing.T) {
	_, err := NewProvider("unknown", "", "", "")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown embedding provider") {
		t.Fatalf("expected unknown provider error, got: %v", err)
	}
}

func TestOllama_Embed_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("unexpected content-type: %s", ct)
		}

		var req ollamaReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("unexpected model: %s", req.Model)
		}
		if req.Prompt != "hello" {
			t.Errorf("unexpected prompt: %s", req.Prompt)
		}

		resp := ollamaResp{Embedding: []float32{3, 4}}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ollama := &Ollama{
		BaseURL: server.URL,
		Model:   "test-model",
		Client:  server.Client(),
	}

	vec, err := ollama.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []float32{0.6, 0.8}
	if !reflect.DeepEqual(vec, expected) {
		t.Fatalf("expected %v, got %v", expected, vec)
	}
}

func TestOllama_Embed_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	ollama := &Ollama{
		BaseURL: server.URL,
		Model:   "test-model",
		Client:  server.Client(),
	}

	_, err := ollama.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "ollama") {
		t.Fatalf("expected ollama error prefix, got: %v", err)
	}
}

func TestOllama_Embed_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	ollama := &Ollama{
		BaseURL: server.URL,
		Model:   "test-model",
		Client:  server.Client(),
	}

	_, err := ollama.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestOpenAI_Embed_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("unexpected content-type: %s", ct)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("unexpected authorization: %s", auth)
		}

		var req openaiReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("unexpected model: %s", req.Model)
		}
		if req.Input != "hello" {
			t.Errorf("unexpected input: %s", req.Input)
		}

		resp := openaiResp{Data: []struct{ Embedding []float32 `json:"embedding"`; Index int `json:"index"` }{{Embedding: []float32{3, 4}}}}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	openai := &OpenAI{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Client:  server.Client(),
	}

	vec, err := openai.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []float32{0.6, 0.8}
	if !reflect.DeepEqual(vec, expected) {
		t.Fatalf("expected %v, got %v", expected, vec)
	}
}

func TestOpenAI_Embed_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	openai := &OpenAI{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Client:  server.Client(),
	}

	_, err := openai.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "openai") {
		t.Fatalf("expected openai error prefix, got: %v", err)
	}
}

func TestOpenAI_Embed_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	openai := &OpenAI{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Client:  server.Client(),
	}

	_, err := openai.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestOpenAI_Embed_EmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiResp{Data: []struct{ Embedding []float32 `json:"embedding"`; Index int `json:"index"` }{}}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	openai := &OpenAI{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Client:  server.Client(),
	}

	_, err := openai.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for empty data")
	}
	if !strings.Contains(err.Error(), "no embeddings") {
		t.Fatalf("expected no embeddings error, got: %v", err)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    []float32
		expected []float32
	}{
		{
			name:     "basic",
			input:    []float32{3, 4},
			expected: []float32{0.6, 0.8},
		},
		{
			name:     "zero vector",
			input:    []float32{0, 0, 0},
			expected: []float32{0, 0, 0},
		},
		{
			name:     "single element",
			input:    []float32{5},
			expected: []float32{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// copy input because normalize mutates in place
			input := append([]float32(nil), tt.input...)
			result := normalize(input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
