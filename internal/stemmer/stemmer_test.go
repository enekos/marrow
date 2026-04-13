package stemmer

import (
	"reflect"
	"strings"
	"testing"

	"marrow/internal/testutil"
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
	cases := []struct {
		name string
		text string
		lang string
	}{
		{
			name: "english running with stopwords removed",
			text: "The cats are running quickly",
			lang: "en",
		},
		{
			name: "spanish corriendo",
			text: "Los gatos están corriendo",
			lang: "es",
		},
		{
			name: "basque stemming lowercases and removes suffixes",
			text: "Katuek Etxean Daude",
			lang: "eu",
		},
		{
			name: "unknown language lowercases",
			text: "Hello WORLD",
			lang: "fr",
		},
		{
			name: "empty text",
			text: "",
			lang: "en",
		},
		{
			name: "punctuation stripped",
			text: "running, quickly!!!",
			lang: "en",
		},
		{
			name: "all stopwords become empty",
			text: "the a an is",
			lang: "en",
		},
	}

	var sb strings.Builder
	for _, tt := range cases {
		sb.WriteString(tt.name)
		sb.WriteString(": ")
		sb.WriteString(StemText(tt.text, tt.lang))
		sb.WriteByte('\n')
	}

	testutil.VerifyApprovedString(t, sb.String())
}
