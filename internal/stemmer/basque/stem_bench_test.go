package basque

import "testing"

func BenchmarkStem(b *testing.B) {
	words := []string{
		"museoak", "musikagilea", "barrutiaren", "barrutietako",
		"basamortu", "katuek", "etxean", "ikasleak", "liburua",
		"egunero", "izenak", "zuhaitzak", "eskolara", "urtean",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, w := range words {
			_ = Stem(w, true)
		}
	}
}
