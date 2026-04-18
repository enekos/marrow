package embed

import (
	"context"
	"math"
	"reflect"
	"testing"
)

func TestNewMock_Determinism(t *testing.T) {
	mock := NewMock()
	ctx := context.Background()
	text := "hello world"

	v1, err := mock(ctx, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v2, err := mock(ctx, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(v1, v2) {
		t.Error("expected same text to produce identical vectors")
	}
}

func TestNewMock_EmptyString(t *testing.T) {
	mock := NewMock()
	ctx := context.Background()

	vec, err := mock(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != EmbeddingDim {
		t.Fatalf("expected length %d, got %d", EmbeddingDim, len(vec))
	}
	for i, v := range vec {
		if v != 0 {
			t.Fatalf("expected zero vector, got non-zero at index %d: %f", i, v)
		}
	}
}

func TestNewMock_Normalization(t *testing.T) {
	mock := NewMock()
	ctx := context.Background()

	vec, err := mock(ctx, "some text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != EmbeddingDim {
		t.Fatalf("expected length %d, got %d", EmbeddingDim, len(vec))
	}

	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	norm := math.Sqrt(sum)
	if math.Abs(norm-1.0) > 1e-5 {
		t.Fatalf("expected unit norm, got %f", norm)
	}
}

