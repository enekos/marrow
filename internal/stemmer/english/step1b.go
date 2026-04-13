package english

import (
	"marrow/internal/stemmer/snowballword"
)

var step1bSuffixes = [][]rune{
	[]rune("eedly"),
	[]rune("ingly"),
	[]rune("edly"),
	[]rune("ing"),
	[]rune("eed"),
	[]rune("ed"),
}

var step1bInnerSuffixes = [][]rune{
	[]rune("at"),
	[]rune("bl"),
	[]rune("iz"),
	[]rune("bb"),
	[]rune("dd"),
	[]rune("ff"),
	[]rune("gg"),
	[]rune("mm"),
	[]rune("nn"),
	[]rune("pp"),
	[]rune("rr"),
	[]rune("tt"),
}

var (
	runeEE = []rune("ee")
	runeE  = []rune("e")
)

// Step 1b is the normalization of various "ly" and "ed" sufficies.
//
func step1b(w *snowballword.SnowballWord) bool {

	suffixRunes := w.FirstSuffixRunes(step1bSuffixes...)
	if suffixRunes == nil {
		return false
	}

	switch string(suffixRunes) {
	case "eed", "eedly":

		// Replace by ee if in R1
		if len(suffixRunes) <= len(w.RS)-w.R1start {
			w.ReplaceSuffixRunes(suffixRunes, runeEE, true)
		}
		return true

	case "ed", "edly", "ing", "ingly":
		hasLowerVowel := false
		for i := 0; i < len(w.RS)-len(suffixRunes); i++ {
			if isLowerVowel(w.RS[i]) {
				hasLowerVowel = true
				break
			}
		}
		if hasLowerVowel {

			// This case requires a two-step transformation and, due
			// to the way we've implemented the `ReplaceSuffix` method
			// here, information about R1 and R2 would be lost between
			// the two.  Therefore, we need to keep track of the
			// original R1 & R2, so that we may set them below, at the
			// end of this case.
			//
			originalR1start := w.R1start
			originalR2start := w.R2start

			// Delete if the preceding word part contains a vowel
			w.RemoveLastNRunes(len(suffixRunes))

			// ...and after the deletion...

			newSuffixRunes := w.FirstSuffixRunes(step1bInnerSuffixes...)
			if newSuffixRunes != nil {
				switch string(newSuffixRunes) {
				case "at", "bl", "iz":
					// If the word ends "at", "bl" or "iz" add "e"
					repl := append(newSuffixRunes, 'e')
					w.ReplaceSuffixRunes(newSuffixRunes, repl, true)

				case "bb", "dd", "ff", "gg", "mm", "nn", "pp", "rr", "tt":
					// If the word ends with a double remove the last letter.
					// Note that, "double" does not include all possible doubles,
					// just those shown above.
					//
					w.RemoveLastNRunes(1)
				}
			} else {
				// If the word is short, add "e"
				if isShortWord(w) {

					// By definition, r1 and r2 are the empty string for
					// short words.
					w.RS = append(w.RS, 'e')
					w.R1start = len(w.RS)
					w.R2start = len(w.RS)
					return true
				}
			}

			// Because we did a double replacement, we need to fix
			// R1 and R2 manually. This is just becase of how we've
			// implemented the `ReplaceSuffix` method.
			//
			rsLen := len(w.RS)
			if originalR1start < rsLen {
				w.R1start = originalR1start
			} else {
				w.R1start = rsLen
			}
			if originalR2start < rsLen {
				w.R2start = originalR2start
			} else {
				w.R2start = rsLen
			}

			return true
		}

	}

	return false
}
