package stemmer

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	got := Tokenize("Hello, 世界! Go-lang 123")
	want := []string{"hello", "世界", "go", "lang", "123"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Tokenize = %v; want %v", got, want)
	}
}

func TestFilterStopWords(t *testing.T) {
	got := FilterStopWords([]string{"the", "cat", "is", "on", "the", "mat"}, "en")
	want := []string{"cat", "mat"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilterStopWords = %v; want %v", got, want)
	}
}

func TestStemText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		lang     string
		expected string
	}{
		{
			name:     "english running with stopwords removed",
			text:     "The cats are running quickly",
			lang:     "en",
			expected: "cat run quick",
		},
		{
			name:     "spanish corriendo",
			text:     "Los gatos están corriendo",
			lang:     "es",
			expected: "gat corr",
		},
		{
			name:     "basque stemming lowercases and removes suffixes",
			text:     "Katuek Etxean Daude",
			lang:     "eu",
			expected: "katu etxean",
		},
		{
			name:     "unknown language lowercases",
			text:     "Hello WORLD",
			lang:     "fr",
			expected: "hello world",
		},
		{
			name:     "empty text",
			text:     "",
			lang:     "en",
			expected: "",
		},
		{
			name:     "punctuation stripped",
			text:     "running, quickly!!!",
			lang:     "en",
			expected: "run quick",
		},
		{
			name:     "all stopwords become empty",
			text:     "the a an is",
			lang:     "en",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StemText(tt.text, tt.lang)
			if got != tt.expected {
				t.Errorf("StemText(%q, %q) = %q; want %q", tt.text, tt.lang, got, tt.expected)
			}
		})
	}
}
