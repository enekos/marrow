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

// DetectWords contains expanded word lists used for query-language detection.
var DetectWords = map[string]map[string]struct{}{
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
		"not": {}, "no": {}, "yes": {}, "there": {}, "then": {}, "than": {},
		"here": {}, "how": {}, "why": {}, "all": {}, "any": {}, "both": {},
		"each": {}, "few": {}, "more": {}, "most": {}, "other": {}, "some": {},
		"such": {}, "only": {}, "own": {}, "same": {}, "too": {}, "very": {},
		"just": {}, "now": {}, "also": {}, "back": {}, "still": {}, "already": {},
		"even": {}, "once": {}, "twice": {}, "again": {}, "always": {}, "never": {},
		"often": {}, "sometimes": {}, "usually": {}, "really": {}, "actually": {},
		"probably": {}, "maybe": {}, "perhaps": {}, "sure": {}, "well": {},
		"good": {}, "bad": {}, "new": {}, "old": {}, "first": {}, "last": {},
		"long": {}, "great": {}, "little": {}, "right": {}, "left": {}, "big": {},
		"small": {}, "large": {}, "next": {}, "early": {}, "young": {},
		"important": {}, "different": {}, "following": {}, "public": {}, "able": {},
	},
	"es": {
		"el": {}, "la": {}, "los": {}, "las": {},
		"un": {}, "una": {}, "unos": {}, "unas": {},
		"de": {}, "del": {}, "al": {}, "y": {}, "o": {}, "pero": {},
		"sin": {}, "con": {}, "por": {}, "para": {}, "en": {}, "a": {},
		"ante": {}, "bajo": {}, "desde": {}, "entre": {}, "hacia": {},
		"hasta": {}, "mediante": {}, "según": {}, "sobre": {}, "tras": {},
		"durante": {}, "excepto": {}, "salvo": {}, "como": {},
		"lo": {}, "le": {}, "les": {}, "me": {}, "te": {}, "se": {},
		"nos": {}, "os": {}, "que": {}, "qué": {}, "quien": {}, "quién": {},
		"cual": {}, "cuál": {}, "cuales": {}, "cuáles": {}, "cuyo": {},
		"cuya": {}, "cuyos": {}, "cuyas": {},
		"donde": {}, "dónde": {}, "cuando": {}, "cuándo": {},
		"cuanto": {}, "cuánto": {}, "cuanta": {}, "cuánta": {},
		"es": {}, "son": {}, "está": {}, "están": {}, "estoy": {},
		"estamos": {}, "estáis": {}, "fue": {}, "fueron": {},
		"ha": {}, "han": {}, "había": {}, "hay": {},
		"tengo": {}, "tiene": {}, "tienen": {}, "tenemos": {}, "tenéis": {},
		"hago": {}, "hace": {}, "hacen": {}, "hacemos": {}, "hacéis": {},
		"más": {}, "muy": {}, "mucho": {}, "mucha": {}, "muchos": {}, "muchas": {},
		"poco": {}, "poca": {}, "pocos": {}, "pocas": {},
		"todo": {}, "todos": {}, "toda": {}, "todas": {},
		"este": {}, "esta": {}, "estos": {}, "estas": {},
		"ese": {}, "esa": {}, "esos": {}, "esas": {},
		"aquel": {}, "aquella": {}, "aquellos": {}, "aquellas": {},
		"mi": {}, "mis": {}, "tu": {}, "tus": {}, "su": {}, "sus": {},
		"nuestro": {}, "nuestra": {}, "nuestros": {}, "nuestras": {},
		"vuestro": {}, "vuestra": {}, "vuestros": {}, "vuestras": {},
		"si": {}, "sino": {}, "también": {}, "ya": {}, "aún": {},
		"todavía": {}, "siempre": {}, "nunca": {}, "jamás": {},
		"aquí": {}, "ahí": {}, "allí": {}, "acá": {}, "allá": {},
		"ahora": {}, "antes": {}, "después": {}, "luego": {},
		"pronto": {}, "tarde": {}, "temprano": {},
		"bien": {}, "mal": {}, "mejor": {}, "peor": {},
		"cómo": {}, "porqué": {}, "porque": {}, "pues": {},
		"sí": {}, "no": {},
	},
	"eu": {
		"eta": {}, "edo": {}, "baina": {}, "ez": {}, "bai": {}, "ere": {},
		"bestela": {}, "gainera": {}, "beraz": {}, "ala": {}, "bada": {},
		"hura": {}, "hau": {}, "berori": {}, "hori": {},
		"nor": {}, "zer": {}, "nork": {}, "nori": {}, "noren": {},
		"non": {}, "nola": {}, "noiz": {}, "zergatik": {}, "zenbat": {},
		"ni": {}, "zu": {}, "gu": {}, "zuek": {}, "haiek": {},
		"nire": {}, "zure": {}, "haren": {}, "gure": {}, "zuen": {}, "haien": {},
		"da": {}, "dago": {}, "daude": {}, "du": {}, "ditu": {}, "dute": {},
		"izan": {}, "egin": {}, "bat": {}, "bi": {}, "guzti": {}, "gutxi": {},
		"donostia": {}, "kalean": {}, "kale": {}, "kaleak": {},
		"oso": {}, "inoiz": {}, "beti": {}, "hemen": {}, "han": {}, "hortxe": {},
		"honela": {}, "euskal": {}, "euskara": {}, "herri": {}, "etxe": {},
		"etxea": {}, "izena": {}, "urte": {}, "egun": {}, "baietz": {}, "ezetz": {},
		"alde": {}, "arte": {}, "aurka": {}, "bezala": {}, "gisa": {},
		"kontra": {}, "ondo": {}, "zai": {}, "aurre": {}, "bitartean": {},
		"tartean": {}, "barruan": {}, "kanpoan": {}, "gainean": {},
		"azpian": {}, "aurrean": {}, "atzean": {}, "ondoren": {},
		"lehen": {}, "gero": {}, "orduan": {}, "orain": {},
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
