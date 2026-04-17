package stemmer

import (
	"regexp"
	"strings"

	"marrow/internal/stemmer/basque"
	"marrow/internal/stemmer/english"
	"marrow/internal/stemmer/spanish"
)

var wordSplitter = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// stemmerRegistry maps language codes to their stemmer implementations.
var stemmerRegistry = map[string]LanguageStemmer{
	"en": english.Stemmer{},
	"es": spanish.Stemmer{},
	"eu": basque.Stemmer{},
}

// SupportedLanguages returns the list of language codes with registered stemmers.
func SupportedLanguages() []string {
	langs := make([]string, 0, len(stemmerRegistry))
	for lang := range stemmerRegistry {
		langs = append(langs, lang)
	}
	return langs
}

// RegisterStemmer registers a stemmer for a language code. It can be used to
// override built-in stemmers or add new languages at runtime.
func RegisterStemmer(lang string, s LanguageStemmer) {
	stemmerRegistry[lang] = s
}

// Tokenize splits text into lowercased word tokens using Unicode boundaries.
func Tokenize(text string) []string {
	raw := wordSplitter.Split(strings.ToLower(text), -1)
	tokens := make([]string, 0, len(raw))
	for _, t := range raw {
		if t != "" {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

// FilterStopWords removes common stop words for the given language.
func FilterStopWords(tokens []string, lang string) []string {
	sw, ok := StopWords[lang]
	if !ok {
		return tokens
	}
	filtered := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, found := sw[t]; !found {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// StemText tokenizes the input, removes stop words, and stems each token.
// Supported langs: "en", "es", "eu".
func StemText(text, lang string) string {
	tokens := Tokenize(text)
	if len(tokens) == 0 {
		return ""
	}
	tokens = FilterStopWords(tokens, lang)
	if len(tokens) == 0 {
		return ""
	}

	stemmer, ok := stemmerRegistry[lang]
	if !ok {
		return strings.Join(tokens, " ")
	}

	for i, t := range tokens {
		tokens[i] = stemmer.Stem(t, true)
	}
	return strings.Join(tokens, " ")
}
