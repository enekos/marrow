package english

import (
	"github.com/enekos/marrow/internal/stemmer/snowballword"
)

var step2Suffixes = [][]rune{
	[]rune("ational"),
	[]rune("fulness"),
	[]rune("iveness"),
	[]rune("ization"),
	[]rune("ousness"),
	[]rune("biliti"),
	[]rune("lessli"),
	[]rune("tional"),
	[]rune("alism"),
	[]rune("aliti"),
	[]rune("ation"),
	[]rune("entli"),
	[]rune("fulli"),
	[]rune("iviti"),
	[]rune("ousli"),
	[]rune("anci"),
	[]rune("abli"),
	[]rune("alli"),
	[]rune("ator"),
	[]rune("enci"),
	[]rune("izer"),
	[]rune("bli"),
	[]rune("ogi"),
	[]rune("li"),
}

var step2Replacements = map[string][]rune{
	"tional": []rune("tion"),
	"enci":   []rune("ence"),
	"anci":   []rune("ance"),
	"abli":   []rune("able"),
	"entli":  []rune("ent"),
	"izer":   []rune("ize"),
	"ization":[]rune("ize"),
	"ational":[]rune("ate"),
	"ation":  []rune("ate"),
	"ator":   []rune("ate"),
	"alism":  []rune("al"),
	"aliti":  []rune("al"),
	"alli":   []rune("al"),
	"fulness":[]rune("ful"),
	"ousli":  []rune("ous"),
	"ousness":[]rune("ous"),
	"iveness":[]rune("ive"),
	"iviti":  []rune("ive"),
	"biliti": []rune("ble"),
	"bli":    []rune("ble"),
	"fulli":  []rune("ful"),
	"lessli": []rune("less"),
}

var (
	runeOG = []rune("og")
)

// Step 2 is the stemming of various endings found in
// R1 including "al", "ness", and "li".
//
func step2(w *snowballword.SnowballWord) bool {

	// Possible sufficies for this step, longest first.
	suffixRunes := w.FirstSuffixRunes(step2Suffixes...)

	// If it is not in R1, do nothing
	if suffixRunes == nil || len(suffixRunes) > len(w.RS)-w.R1start {
		return false
	}

	// Handle special cases where we're not just going to
	// replace the suffix with another suffix: there are
	// other things we need to do.
	//
	switch string(suffixRunes) {
	case "li":
		// Delete if preceded by a valid li-ending. Valid li-endings inlude the
		// following charaters: cdeghkmnrt. (Note, the unicode code points for
		// these characters are, respectively, as follows:
		// 99 100 101 103 104 107 109 110 114 116)
		//
		rsLen := len(w.RS)
		if rsLen >= 3 {
			switch w.RS[rsLen-3] {
			case 99, 100, 101, 103, 104, 107, 109, 110, 114, 116:
				w.RemoveLastNRunes(len(suffixRunes))
				return true
			}
		}
		return false

	case "ogi":
		// Replace by og if preceded by l.
		// (Note, the unicode code point for l is 108)
		//
		rsLen := len(w.RS)
		if rsLen >= 4 && w.RS[rsLen-4] == 108 {
			w.ReplaceSuffixRunes(suffixRunes, runeOG, true)
		}
		return true
	}

	// Handle a suffix that was found, which is going
	// to be replaced with a different suffix.
	//
	repl := step2Replacements[string(suffixRunes)]
	if repl != nil {
		w.ReplaceSuffixRunes(suffixRunes, repl, true)
	}
	return true
}
