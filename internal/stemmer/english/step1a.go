package english

import (
	"github.com/enekos/marrow/internal/stemmer/snowballword"
)

var step1aSuffixes = [][]rune{
	[]rune("sses"),
	[]rune("ied"),
	[]rune("ies"),
	[]rune("us"),
	[]rune("ss"),
	[]rune("s"),
}

var (
	runeI  = []rune("i")
	runeIE = []rune("ie")
	runeSS = []rune("ss")
)

// Step 1a is normalization of various special "s"-endings.
//
func step1a(w *snowballword.SnowballWord) bool {

	suffixRunes := w.FirstSuffixRunes(step1aSuffixes...)
	if suffixRunes == nil {
		return false
	}

	switch len(suffixRunes) {
	case 4: // "sses"
		// Replace by ss
		w.ReplaceSuffixRunes(suffixRunes, runeSS, true)
		return true

	case 3: // "ied" or "ies"
		// Replace by i if preceded by more than one letter,
		// otherwise by ie (so ties -> tie, cries -> cri).
		var repl []rune
		if len(w.RS) > 4 {
			repl = runeI
		} else {
			repl = runeIE
		}
		w.ReplaceSuffixRunes(suffixRunes, repl, true)
		return true

	case 2: // "us" or "ss"
		// Do nothing
		return false

	case 1: // "s"
		// Delete if the preceding word part contains a vowel
		// not immediately before the s (so gas and this retain
		// the s, gaps and kiwis lose it)
		//
		for i := 0; i < len(w.RS)-2; i++ {
			if isLowerVowel(w.RS[i]) {
				w.RemoveLastNRunes(len(suffixRunes))
				return true
			}
		}
	}
	return false
}
