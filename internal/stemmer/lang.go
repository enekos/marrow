package stemmer

import "strings"

// DetectLanguage guesses the language of a query based on character-level
// and word-level heuristics. Supported languages are en, es, and eu.
func DetectLanguage(query string) string {
	lower := strings.ToLower(query)
	tokens := Tokenize(query)

	const (
		idxEn = 0
		idxEs = 1
		idxEu = 2
	)
	var scores [3]int

	// --- Character-level signals ------------------------------------------
	for _, r := range lower {
		switch r {
		case 'ñ', '¿', '¡':
			scores[idxEs] += 10
		case 'á', 'é', 'í', 'ó', 'ú', 'ü':
			scores[idxEs] += 5
		}
	}
	// tx is extremely rare in English/Spanish; tz is also strongly Basque.
	if strings.Contains(lower, "tx") {
		scores[idxEu] += 5
	}
	if strings.Contains(lower, "tz") {
		scores[idxEu] += 3
	}

	// --- Word-level signals -----------------------------------------------
	spanishWords := DetectWords["es"]
	basqueWords := DetectWords["eu"]
	englishWords := DetectWords["en"]

	for _, tok := range tokens {
		if _, ok := spanishWords[tok]; ok {
			scores[idxEs] += 2
		}
		if _, ok := basqueWords[tok]; ok {
			scores[idxEu] += 2
		}
		if _, ok := englishWords[tok]; ok {
			scores[idxEn] += 2
		}
	}

	// --- Resolve winner ---------------------------------------------------
	maxScore := scores[idxEn]
	lang := "en"
	if scores[idxEs] > maxScore {
		maxScore = scores[idxEs]
		lang = "es"
	}
	if scores[idxEu] > maxScore {
		lang = "eu"
	}
	return lang
}
