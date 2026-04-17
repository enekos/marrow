package spanish

import (
	"marrow/internal/stemmer/snowballword"
	"strings"
)

// Stemmer implements the language-agnostic stemmer interface for Spanish.
type Stemmer struct{}

// Stem stems a single Spanish word.
func (Stemmer) Stem(word string, stemStopWords bool) string {
	return Stem(word, stemStopWords)
}

// Stem a Spanish word.
func Stem(word string, stemStopWords bool) string {
	word = strings.ToLower(strings.TrimSpace(word))

	// Return small words and stop words
	if len(word) <= 2 || (!stemStopWords && isStopWord(word)) {
		return word
	}

	w := snowballword.New(word)

	// Stem the word.  Note, each of these
	// steps will alter `w` in place.
	preprocess(w)
	step0(w)
	changeInStep1 := step1(w)
	if changeInStep1 == false {
		changeInStep2a := step2a(w)
		if changeInStep2a == false {
			step2b(w)
		}
	}
	step3(w)
	postprocess(w)

	return w.String()
}
