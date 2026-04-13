package stemmer

import (
	"regexp"
	"strings"

	"marrow/internal/stemmer/basque"
	"marrow/internal/stemmer/english"
	"marrow/internal/stemmer/spanish"
)

var wordSplitter = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// StopWords for supported languages.
var StopWords = map[string]map[string]struct{}{
	"en": {
		"the": {}, "a": {}, "an": {}, "is": {}, "are": {}, "was": {}, "were": {},
		"be": {}, "been": {}, "being": {}, "have": {}, "has": {}, "had": {}, "do": {},
		"does": {}, "did": {}, "will": {}, "would": {}, "shall": {}, "should": {},
		"can": {}, "could": {}, "may": {}, "might": {}, "must": {}, "ought": {},
		"i": {}, "you": {}, "he": {}, "she": {}, "it": {}, "we": {}, "they": {},
		"me": {}, "him": {}, "her": {}, "us": {}, "them": {}, "my": {}, "your": {},
		"his": {}, "its": {}, "our": {}, "their": {}, "this": {}, "that": {},
		"these": {}, "those": {}, "of": {}, "in": {}, "to": {}, "for": {}, "with": {},
		"on": {}, "at": {}, "by": {}, "from": {}, "as": {}, "into": {}, "through": {},
		"during": {}, "before": {}, "after": {}, "above": {}, "below": {}, "between": {},
		"and": {}, "but": {}, "or": {}, "yet": {}, "so": {}, "if": {}, "because": {},
		"although": {}, "though": {}, "while": {}, "where": {}, "when": {},
		"which": {}, "who": {}, "whom": {}, "whose": {}, "what": {}, "whatever": {},
	},
	"es": {
		"el": {}, "la": {}, "los": {}, "las": {}, "un": {}, "una": {}, "unos": {}, "unas": {},
		"de": {}, "del": {}, "al": {}, "y": {}, "o": {}, "pero": {}, "sin": {}, "con": {},
		"por": {}, "para": {}, "en": {}, "a": {}, "ante": {}, "bajo": {}, "desde": {},
		"entre": {}, "hacia": {}, "hasta": {}, "mediante": {}, "según": {}, "sobre": {},
		"tras": {}, "durante": {}, "excepto": {}, "salvo": {}, "como": {}, "lo": {}, "le": {},
		"les": {}, "me": {}, "te": {}, "se": {}, "nos": {}, "os": {}, "que": {}, "quien": {},
		"cual": {}, "cuales": {}, "cuyo": {}, "cuya": {}, "cuyos": {}, "cuyas": {},
		"donde": {}, "cuando": {}, "cuanto": {}, "cuanta": {}, "es": {}, "son": {},
		"está": {}, "están": {}, "fue": {}, "fueron": {}, "ha": {}, "han": {}, "había": {},
	},
	"eu": {
		"eta": {}, "edo": {}, "baina": {}, "ez": {}, "bai": {}, "ere": {}, "bestela": {},
		"gainera": {}, "beraz": {}, "ala": {}, "bada": {}, "hura": {}, "hau": {}, "berori": {},
		"nor": {}, "zer": {}, "nork": {}, "nori": {}, "noren": {}, "non": {}, "nola": {},
		"noiz": {}, "zergatik": {}, "zenbat": {}, "ni": {}, "zu": {}, "gu": {}, "zuek": {},
		"haiek": {}, "nire": {}, "zure": {}, "haren": {}, "gure": {}, "zuen": {}, "haien": {},
		"da": {}, "dago": {}, "daude": {}, "du": {}, "ditu": {}, "dute": {}, "izan": {},
		"egin": {}, "bat": {}, "bi": {}, "guzti": {}, "gutxi": {}, "oso": {}, "inoiz": {},
		"beti": {}, "hemen": {}, "han": {}, "hortxe": {}, "honela": {}, "hori": {},
	},
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

	var stemFn func(string) string
	switch lang {
	case "es":
		stemFn = func(w string) string { return spanish.Stem(w, true) }
	case "en":
		stemFn = func(w string) string { return english.Stem(w, true) }
	case "eu":
		stemFn = func(w string) string { return basque.Stem(w, true) }
	default:
		stemFn = func(w string) string { return w }
	}

	for i, t := range tokens {
		tokens[i] = stemFn(t)
	}
	return strings.Join(tokens, " ")
}
