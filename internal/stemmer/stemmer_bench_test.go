package stemmer

import "testing"

func BenchmarkStemTextEnglish(b *testing.B) {
	text := "The cats are running quickly through the beautiful gardens"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StemText(text, "en")
	}
}

func BenchmarkStemTextSpanish(b *testing.B) {
	text := "Los gatos están corriendo rápidamente por los hermosos jardines"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StemText(text, "es")
	}
}

func BenchmarkStemTextBasque(b *testing.B) {
	text := "Katuek etxean daude eta musika entzuten ari dira gaur"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StemText(text, "eu")
	}
}

func BenchmarkTokenize(b *testing.B) {
	text := "Hello, 世界! Go-lang 123 running quickly"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Tokenize(text)
	}
}
