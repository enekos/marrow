package english

import (
	"marrow/internal/stemmer/snowballword"
)

var step0Suffixes = [][]rune{
	[]rune("'s'"),
	[]rune("'s"),
	[]rune("'"),
}

// Step 0 is to strip off apostrophes and "s".
//
func step0(w *snowballword.SnowballWord) bool {
	suffixRunes := w.FirstSuffixRunes(step0Suffixes...)
	if suffixRunes == nil {
		return false
	}
	w.RemoveLastNRunes(len(suffixRunes))
	return true
}
