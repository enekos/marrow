package spanish

import (
	"marrow/internal/stemmer/snowballword"
)

var step2aSuffixes = [][]rune{
	[]rune("yeron"),
	[]rune("yendo"),
	[]rune("yan"),
	[]rune("yen"),
	[]rune("yais"),
	[]rune("yamos"),
	[]rune("ya"),
	[]rune("ye"),
	[]rune("yo"),
	[]rune("yó"),
	[]rune("yas"),
	[]rune("yes"),
}

// Step 2a is the removal of verb suffixes beginning y,
// Search for the longest among the following suffixes
// in RV, and if found, delete if preceded by u.
//
func step2a(word *snowballword.SnowballWord) bool {
	suffixRunes := word.FirstSuffixInRunes(word.RVstart, len(word.RS), step2aSuffixes...)
	if suffixRunes != nil {
		idx := len(word.RS) - len(suffixRunes) - 1
		if idx >= 0 && word.RS[idx] == 117 {
			word.RemoveLastNRunes(len(suffixRunes))
			return true
		}
	}
	return false
}
