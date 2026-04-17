package stemmer

// LanguageStemmer is the common interface implemented by all language-specific stemmers.
type LanguageStemmer interface {
	// Stem returns the stemmed form of a single word.
	// If stemStopWords is false, common stop words are returned unchanged.
	Stem(word string, stemStopWords bool) string
}
