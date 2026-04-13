package spanish

import (
	"marrow/internal/stemmer/snowballword"
)

var step2bSuffixes = [][]rune{
	[]rune("iésemos"),
	[]rune("iéramos"),
	[]rune("iríamos"),
	[]rune("eríamos"),
	[]rune("aríamos"),
	[]rune("ásemos"),
	[]rune("áramos"),
	[]rune("ábamos"),
	[]rune("isteis"),
	[]rune("iríais"),
	[]rune("iremos"),
	[]rune("ieseis"),
	[]rune("ierais"),
	[]rune("eríais"),
	[]rune("eremos"),
	[]rune("asteis"),
	[]rune("aríais"),
	[]rune("aremos"),
	[]rune("íamos"),
	[]rune("irías"),
	[]rune("irían"),
	[]rune("iréis"),
	[]rune("ieses"),
	[]rune("iesen"),
	[]rune("ieron"),
	[]rune("ieras"),
	[]rune("ieran"),
	[]rune("iendo"),
	[]rune("erías"),
	[]rune("erían"),
	[]rune("eréis"),
	[]rune("aseis"),
	[]rune("arías"),
	[]rune("arían"),
	[]rune("aréis"),
	[]rune("arais"),
	[]rune("abais"),
	[]rune("íais"),
	[]rune("iste"),
	[]rune("iría"),
	[]rune("irás"),
	[]rune("irán"),
	[]rune("imos"),
	[]rune("iese"),
	[]rune("iera"),
	[]rune("idos"),
	[]rune("idas"),
	[]rune("ería"),
	[]rune("erás"),
	[]rune("erán"),
	[]rune("aste"),
	[]rune("ases"),
	[]rune("asen"),
	[]rune("aría"),
	[]rune("arás"),
	[]rune("arán"),
	[]rune("aron"),
	[]rune("aras"),
	[]rune("aran"),
	[]rune("ando"),
	[]rune("amos"),
	[]rune("ados"),
	[]rune("adas"),
	[]rune("abas"),
	[]rune("aban"),
	[]rune("ías"),
	[]rune("ían"),
	[]rune("éis"),
	[]rune("áis"),
	[]rune("iré"),
	[]rune("irá"),
	[]rune("ido"),
	[]rune("ida"),
	[]rune("eré"),
	[]rune("erá"),
	[]rune("emos"),
	[]rune("ase"),
	[]rune("aré"),
	[]rune("ará"),
	[]rune("ara"),
	[]rune("ado"),
	[]rune("ada"),
	[]rune("aba"),
	[]rune("ís"),
	[]rune("ía"),
	[]rune("ió"),
	[]rune("ir"),
	[]rune("id"),
	[]rune("es"),
	[]rune("er"),
	[]rune("en"),
	[]rune("ed"),
	[]rune("as"),
	[]rune("ar"),
	[]rune("an"),
	[]rune("ad"),
}

var runeGU = []rune("gu")

// Step 2b is the removal of verb suffixes beginning y,
// Search for the longest among the following suffixes
// in RV, and if found, delete if preceded by u.
//
func step2b(word *snowballword.SnowballWord) bool {
	suffixRunes := word.FirstSuffixInRunes(word.RVstart, len(word.RS), step2bSuffixes...)
	if suffixRunes == nil {
		return false
	}

	switch string(suffixRunes) {
	case "en", "es", "éis", "emos":
		// Delete, and if preceded by gu delete the u (the gu need not be in RV)
		word.RemoveLastNRunes(len(suffixRunes))
		if word.HasSuffixRunes(runeGU) {
			word.RemoveLastNRunes(1)
		}

	default:
		// Delete
		word.RemoveLastNRunes(len(suffixRunes))
	}
	return true
}
