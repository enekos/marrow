package basque

import (
	"strings"

	"marrow/internal/stemmer/romance"
	"marrow/internal/stemmer/snowballword"
)

// Stemmer implements the language-agnostic stemmer interface for Basque.
type Stemmer struct{}

// Stem stems a single Basque word.
func (Stemmer) Stem(word string, stemStopWords bool) string {
	return Stem(word, stemStopWords)
}

var stopWords = map[string]struct{}{
	"a": {}, "al": {}, "an": {}, "andi": {}, "arabera": {}, "arekin": {}, "arengan": {}, "arengatik": {},
	"arentzat": {}, "ari": {}, "arren": {}, "arte": {}, "artean": {}, "at": {}, "aurretik": {},
	"bada": {}, "baditu": {}, "baino": {}, "bai": {}, "baita": {}, "barru": {}, "bat": {}, "batean": {},
	"batek": {}, "batekiko": {}, "baten": {}, "batere": {}, "batera": {}, "baterantz": {},
	"bezala": {}, "bi": {}, "bitan": {}, "bizi": {}, "da": {}, "dago": {}, "daude": {}, "dela": {},
	"den": {}, "dena": {}, "denak": {}, "denean": {}, "ditu": {}, "du": {}, "dute": {}, "edo": {},
	"egin": {}, "ere": {}, "esku": {}, "eta": {}, "eurak": {}, "ez": {}, "ezker": {}, "gainera": {},
	"gainerako": {}, "gara": {}, "gaude": {}, "gero": {}, "gisa": {}, "gu": {}, "gutxi": {},
	"guzti": {}, "haiek": {}, "haietan": {}, "hainbeste": {}, "hala": {}, "han": {}, "handik": {},
	"hango": {}, "hara": {}, "hari": {}, "hark": {}, "hartan": {}, "hau": {}, "hauek": {}, "hauekin": {},
	"hauen": {}, "hauetan": {}, "hemen": {}, "hemendik": {}, "hemengo": {}, "hi": {}, "hona": {},
	"honek": {}, "honela": {}, "honetan": {}, "hontzat": {}, "hor": {}, "hori": {}, "horiek": {},
	"horien": {}, "horko": {}, "hortik": {}, "hortxe": {}, "hura": {}, "izan": {}, "ni": {},
	"noiz": {}, "nola": {}, "non": {}, "nondik": {}, "nongo": {}, "nor": {}, "nora": {}, "nork": {},
	"nori": {}, "nortzuk": {}, "nortzu": {}, "nun": {}, "oraingo": {}, "oro": {}, "oro har": {},
	"oro harretik": {}, "ostean": {}, "oso": {}, "ta": {}, "te": {}, "ti": {}, "tun": {}, "tu": {},
	"zergatik": {}, "zer": {}, "zion": {}, "zu": {}, "zuek": {}, "zuen": {},
}

func isStopWord(word string) bool {
	_, ok := stopWords[word]
	return ok
}

func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

func markRegions(w *snowballword.SnowballWord) (p1, p2, pV int) {
	rs := w.RS
	limit := len(rs)
	pV = limit
	p1 = limit
	p2 = limit

	cursor := 0

	if tryPVOptionA(rs, limit, &cursor) {
		pV = cursor
	} else if tryPVOptionB(rs, limit, &cursor) {
		pV = cursor
	}

	p1 = romance.VnvSuffix(w, isVowel, 0)
	p2 = romance.VnvSuffix(w, isVowel, p1)
	return
}

func tryPVOptionA(rs []rune, limit int, cursor *int) bool {
	c := *cursor
	if c >= limit || !isVowel(rs[c]) {
		return false
	}
	c++

	v3 := c
	if c < limit && !isVowel(rs[c]) {
		c++
		for c < limit && !isVowel(rs[c]) {
			c++
		}
		if c < limit {
			*cursor = c + 1
			return true
		}
	}

	c = v3
	if c < limit && isVowel(rs[c]) {
		c++
		for c < limit && isVowel(rs[c]) {
			c++
		}
		if c < limit {
			*cursor = c + 1
			return true
		}
	}
	return false
}

func tryPVOptionB(rs []rune, limit int, cursor *int) bool {
	c := *cursor
	if c >= limit || isVowel(rs[c]) {
		return false
	}
	c++

	v4 := c
	if c < limit && !isVowel(rs[c]) {
		c++
		for c < limit && !isVowel(rs[c]) {
			c++
		}
		if c < limit {
			*cursor = c + 1
			return true
		}
	}

	c = v4
	if c < limit && isVowel(rs[c]) {
		c++
		if c < limit {
			*cursor = c + 1
			return true
		}
	}
	return false
}

func aditzak(w *snowballword.SnowballWord) bool {
	sa, ok := findSuffix(w, aditzakSuffixes)
	if !ok {
		return false
	}
	rsLen := len(w.RS)
	suffixStart := rsLen - len(sa.suffixRunes)

	switch sa.action {
	case 1:
		if suffixStart < w.RVstart {
			return false
		}
		w.RemoveLastNRunes(len(sa.suffixRunes))
		return true
	case 2:
		if suffixStart < w.R2start {
			return false
		}
		w.RemoveLastNRunes(len(sa.suffixRunes))
		return true
	case -1:
		return false
	}
	return false
}

func izenak(w *snowballword.SnowballWord) bool {
	sa, ok := findSuffix(w, izenakSuffixes)
	if !ok {
		return false
	}
	rsLen := len(w.RS)
	suffixStart := rsLen - len(sa.suffixRunes)

	switch sa.action {
	case 1:
		if suffixStart < w.RVstart {
			return false
		}
		w.RemoveLastNRunes(len(sa.suffixRunes))
		return true
	case 2:
		if suffixStart < w.R2start {
			return false
		}
		w.RemoveLastNRunes(len(sa.suffixRunes))
		return true
	case 3:
		w.ReplaceSuffixRunes(sa.suffixRunes, []rune("jok"), true)
		return true
	case 4:
		if suffixStart < w.R1start {
			return false
		}
		w.RemoveLastNRunes(len(sa.suffixRunes))
		return true
	case 5:
		w.ReplaceSuffixRunes(sa.suffixRunes, []rune("tra"), true)
		return true
	case 6:
		w.ReplaceSuffixRunes(sa.suffixRunes, []rune("minutu"), true)
		return true
	case -1:
		return false
	}
	return false
}

func adjetiboak(w *snowballword.SnowballWord) bool {
	sa, ok := findSuffix(w, adjetiboakSuffixes)
	if !ok {
		return false
	}
	rsLen := len(w.RS)
	suffixStart := rsLen - len(sa.suffixRunes)

	switch sa.action {
	case 1:
		if suffixStart < w.RVstart {
			return false
		}
		w.RemoveLastNRunes(len(sa.suffixRunes))
		return true
	case 2:
		w.ReplaceSuffixRunes(sa.suffixRunes, []rune("z"), true)
		return true
	}
	return false
}

// Stem a Basque word.
func Stem(word string, stemStopWords bool) string {
	word = strings.ToLower(strings.TrimSpace(word))

	if len(word) <= 2 || (!stemStopWords && isStopWord(word)) {
		return word
	}

	w := snowballword.New(word)

	p1, p2, pV := markRegions(w)
	w.R1start = p1
	w.R2start = p2
	w.RVstart = pV

	for aditzak(w) {
	}
	for izenak(w) {
	}
	adjetiboak(w)

	return w.String()
}
