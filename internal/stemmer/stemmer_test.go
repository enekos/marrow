package stemmer

import (
	"reflect"
	"strings"
	"testing"

	"github.com/enekos/marrow/internal/testutil"
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

func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()
	if len(langs) != 3 {
		t.Errorf("SupportedLanguages() len = %d; want 3", len(langs))
	}
	m := make(map[string]struct{}, len(langs))
	for _, l := range langs {
		m[l] = struct{}{}
	}
	for _, want := range []string{"en", "es", "eu"} {
		if _, ok := m[want]; !ok {
			t.Errorf("SupportedLanguages() missing %q", want)
		}
	}
}

type mockStemmer struct{}

func (mockStemmer) Stem(word string, _ bool) string { return word }

func TestRegisterStemmer(t *testing.T) {
	RegisterStemmer("xx", mockStemmer{})
	got := StemText("hello", "xx")
	if got != "hello" {
		t.Errorf("StemText with custom stemmer = %q; want %q", got, "hello")
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
		{
			name: "english longer sentence",
			text: "The beautiful organizations were running through the national gardens",
			lang: "en",
		},
		{
			name: "english mixed punctuation and numbers",
			text: "Hello... world!!! 123 test-running (fast)",
			lang: "en",
		},
		{
			name: "spanish longer sentence",
			text: "Los estudiantes estaban estudiando en la universidad",
			lang: "es",
		},
		{
			name: "spanish with accents",
			text: "La música española es fantástica",
			lang: "es",
		},
		{
			name: "basque longer sentence",
			text: "Ikasleak eskolara joan ziren eta liburuak irakurri zituzten",
			lang: "eu",
		},
		{
			name: "basque with punctuation",
			text: "Katuek, etxean, daude! Zure etxea da?",
			lang: "eu",
		},
		{
			name: "english possessives",
			text: "The cat's tail and the dogs' bones",
			lang: "en",
		},
		{
			name: "spanish question words",
			text: "¿Dónde están? ¿Quién viene?",
			lang: "es",
		},
		{
			name: "english uppercase with stopwords",
			text: "THE QUICK BROWN FOX JUMPS OVER THE LAZY DOG",
			lang: "en",
		},
		{
			name: "spanish all stopwords",
			text: "el la los las de y en",
			lang: "es",
		},
		{
			name: "basque all stopwords",
			text: "eta ez bai da dago",
			lang: "eu",
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
