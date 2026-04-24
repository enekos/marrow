package english

import (
	"github.com/enekos/marrow/internal/stemmer/snowballword"
)

var step3Suffixes = [][]rune{
	[]rune("ational"),
	[]rune("tional"),
	[]rune("alize"),
	[]rune("icate"),
	[]rune("ative"),
	[]rune("iciti"),
	[]rune("ical"),
	[]rune("ful"),
	[]rune("ness"),
}

var step3Replacements = map[string][]rune{
	"ational": []rune("ate"),
	"tional":  []rune("tion"),
	"alize":   []rune("al"),
	"icate":   []rune("ic"),
	"iciti":   []rune("ic"),
	"ical":    []rune("ic"),
	"ful":     nil,
	"ness":    nil,
}

// Step 3 is the stemming of various longer sufficies
// found in R1.
//
func step3(w *snowballword.SnowballWord) bool {

	suffixRunes := w.FirstSuffixRunes(step3Suffixes...)

	// If it is not in R1, do nothing
	if suffixRunes == nil || len(suffixRunes) > len(w.RS)-w.R1start {
		return false
	}

	// Handle special cases where we're not just going to
	// replace the suffix with another suffix: there are
	// other things we need to do.
	//
	if string(suffixRunes) == "ative" {

		// If in R2, delete.
		//
		if len(w.RS)-w.R2start >= 5 {
			w.RemoveLastNRunes(len(suffixRunes))
			return true
		}
		return false
	}

	// Handle a suffix that was found, which is going
	// to be replaced with a different suffix.
	//
	repl := step3Replacements[string(suffixRunes)]
	w.ReplaceSuffixRunes(suffixRunes, repl, true)
	return true

}
