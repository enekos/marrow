package local

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
)

// TestEmbed_ConcurrentSafety runs many goroutines encoding the same inputs
// and asserts the outputs are bit-identical to a single-threaded baseline.
// Catches any accidental sharing of scratch buffers introduced while tuning.
func TestEmbed_ConcurrentSafety(t *testing.T) {
	if os.Getenv("MARROW_MINILM_DIR") == "" {
		t.Skip("set MARROW_MINILM_DIR to run concurrency test")
	}
	enc := loadEncoder(t)
	ctx := context.Background()

	inputs := []string{
		"The quick brown fox jumps over the lazy dog.",
		"hello world",
		"Marrow is a local-first search engine.",
		"Another passage for embedding concurrency testing.",
		"Short",
	}

	// Single-threaded baseline.
	want := make([][]float32, len(inputs))
	for i, s := range inputs {
		v, err := enc.Embed(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		want[i] = v
	}

	const workers = 8
	const iters = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := 0; it < iters; it++ {
				for i, s := range inputs {
					got, err := enc.Embed(ctx, s)
					if err != nil {
						errs <- err
						return
					}
					if len(got) != len(want[i]) {
						errs <- errDim(len(got), len(want[i]))
						return
					}
					for k := range got {
						if got[k] != want[i][k] {
							errs <- errValue(i, k, got[k], want[i][k])
							return
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func errDim(got, want int) error {
	return fmt.Errorf("dim mismatch: got %d, want %d", got, want)
}

func errValue(i, k int, got, want float32) error {
	return fmt.Errorf("input %d idx %d: got %v, want %v", i, k, got, want)
}
