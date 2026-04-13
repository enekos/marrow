package spanish

import "testing"

func BenchmarkStem(b *testing.B) {
	words := []string{
		"corriendo", "retrospectiva", "emperador", "instalaciones",
		"finiquitación", "definitivamente", "turísticas", "puntualizaciones",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, w := range words {
			_ = Stem(w, true)
		}
	}
}
