package spanish

import (
	"marrow/internal/stemmer/snowballword"
)

var step0Suffix1 = [][]rune{
	[]rune("selas"),
	[]rune("selos"),
	[]rune("sela"),
	[]rune("selo"),
	[]rune("las"),
	[]rune("les"),
	[]rune("los"),
	[]rune("nos"),
	[]rune("me"),
	[]rune("se"),
	[]rune("la"),
	[]rune("le"),
	[]rune("lo"),
}

var step0Suffix2 = [][]rune{
	[]rune("iéndo"),
	[]rune("iendo"),
	[]rune("yendo"),
	[]rune("ando"),
	[]rune("ándo"),
	[]rune("ár"),
	[]rune("ér"),
	[]rune("ír"),
	[]rune("ar"),
	[]rune("er"),
	[]rune("ir"),
}

var step0Replacements = map[string][]rune{
	"iéndo": []rune("iendo"),
	"ándo":  []rune("ando"),
	"ár":    []rune("ar"),
	"ír":    []rune("ir"),
}

// Step 0 is the removal of attached pronouns
//
func step0(word *snowballword.SnowballWord) bool {

	// Search for the longest among the following suffixes
	suffix1Runes := word.FirstSuffixInRunes(word.RVstart, len(word.RS), step0Suffix1...)

	// If the suffix empty or not in RV, we have nothing to do.
	if suffix1Runes == nil {
		return false
	}

	// We'll remove suffix1, if comes after one of the following
	suffix2Runes := word.FirstSuffixInRunes(word.RVstart, len(word.RS)-len(suffix1Runes), step0Suffix2...)
	if suffix2Runes == nil {
		// Nothing to do
		return false
	}

	switch string(suffix2Runes) {
	case "iéndo", "ándo", "ár", "ér", "ír":
		// In these cases, deletion is followed by removing
		// the acute accent (e.g., haciéndola -> haciendo).
		repl := step0Replacements[string(suffix2Runes)]
		word.RemoveLastNRunes(len(suffix1Runes))
		word.ReplaceSuffixRunes(suffix2Runes, repl, true)
		return true

	case "ando", "iendo", "ar", "er", "ir":
		word.RemoveLastNRunes(len(suffix1Runes))
		return true

	case "yendo":
		// In the case of "yendo", the "yendo" must lie in RV,
		// and be preceded by a "u" somewhere in the word.
		for i := 0; i < len(word.RS)-(len(suffix1Runes)+len(suffix2Runes)); i++ {
			// Note, the unicode code point for "u" is 117.
			if word.RS[i] == 117 {
				word.RemoveLastNRunes(len(suffix1Runes))
				return true
			}
		}
	}
	return false
}
