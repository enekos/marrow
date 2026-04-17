package stemmer

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
