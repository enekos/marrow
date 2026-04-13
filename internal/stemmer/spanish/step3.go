package spanish

import (
	"marrow/internal/stemmer/snowballword"
)

var step3Suffixes = [][]rune{
	[]rune("os"),
	[]rune("a"),
	[]rune("o"),
	[]rune("á"),
	[]rune("í"),
	[]rune("ó"),
	[]rune("e"),
	[]rune("é"),
}

// Step 3 is the removal of residual suffixes.
//
func step3(word *snowballword.SnowballWord) bool {
	suffixRunes := word.FirstSuffixIfInRunes(word.RVstart, len(word.RS), step3Suffixes...)

	// No suffix found, nothing to do.
	//
	if suffixRunes == nil {
		return false
	}

	// Remove all these suffixes
	word.RemoveLastNRunes(len(suffixRunes))

	s := string(suffixRunes)
	if s == "e" || s == "é" {

		// If preceded by gu with the u in RV delete the u
		//
		if word.HasSuffixRunes(runeGU) {
			word.RemoveLastNRunes(1)
		}
	}
	return true
}
