package english

import (
	"github.com/enekos/marrow/internal/stemmer/snowballword"
	"strings"
)

// Stemmer implements the language-agnostic stemmer interface for English.
type Stemmer struct{}

// Stem stems a single English word.
func (Stemmer) Stem(word string, stemStopWords bool) string {
	return Stem(word, stemStopWords)
}

// Stem an English word.
func Stem(word string, stemStopWords bool) string {
	word = strings.ToLower(strings.TrimSpace(word))

	// Return small words and stop words
	if len(word) <= 2 || (!stemStopWords && isStopWord(word)) {
		return word
	}

	// Return special words immediately
	if specialVersion := stemSpecialWord(word); specialVersion != "" {
		word = specialVersion
		return word
	}

	w := snowballword.New(word)

	// Stem the word.  Note, each of these
	// steps will alter `w` in place.
	preprocess(w)
	step0(w)
	step1a(w)
	step1b(w)
	step1c(w)
	step2(w)
	step3(w)
	step4(w)
	step5(w)
	postprocess(w)

	return w.String()
}
