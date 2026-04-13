package english

import "testing"

func BenchmarkStem(b *testing.B) {
	words := []string{
		"running", "generously", "aberration", "accumulations",
		"agreement", "skating", "fluently", "because", "above",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, w := range words {
			_ = Stem(w, true)
		}
	}
}
