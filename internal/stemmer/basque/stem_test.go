package basque

import (
	"strings"
	"testing"

	"marrow/internal/testutil"
)

func TestIsStopWord(t *testing.T) {
	stopWords := []string{"eta", "edo", "baino", "ez", "bai"}
	for _, w := range stopWords {
		if !isStopWord(w) {
			t.Errorf("isStopWord(%q) = false, want true", w)
		}
	}
}

func TestIsStopWordFalse(t *testing.T) {
	nonStopWords := []string{"etxea", "korrika"}
	for _, w := range nonStopWords {
		if isStopWord(w) {
			t.Errorf("isStopWord(%q) = true, want false", w)
		}
	}
}

func TestStemPreservesStopWords(t *testing.T) {
	stopWords := []string{"eta", "edo", "baino", "ez", "bai"}
	for _, w := range stopWords {
		got := Stem(w, false)
		if got != w {
			t.Errorf("Stem(%q, false) = %q, want %q", w, got, w)
		}
	}
}

func TestStemStemsStopWords(t *testing.T) {
	// "baino" is a stop word that also has a removable suffix,
	// so it should be stemmed when stemStopWords is true.
	got := Stem("baino", true)
	if got == "baino" {
		t.Errorf("Stem(%q, true) = %q, expected a stemmed form", "baino", got)
	}
}

func TestStem(t *testing.T) {
	inputs := []string{
		"museoak",
		"museoan",
		"musikagilea",
		"musikagileak",
		"barrutiaren",
		"barrutiek",
		"barrutien",
		"barrutietako",
		"barrutietan",
		"barrutik",
		"barrutiko",
		"barrutitan",
		"basa",
		"basailu",
		"basalto",
		"basamortu",
		"katuek",
		"etxean",
		"etxea",
		"etxeak",
		"ikasleak",
		"liburua",
		"egunero",
		"izenak",
		"zuhaitzak",
		"eskolara",
		"urtean",
		"herrian",
		"ikaslea",
		"eskola",
		"gela",
		"gauza",
		"etorri",
		"egin",
		"joan",
		"ikusi",
	}

	var sb strings.Builder
	for _, in := range inputs {
		sb.WriteString(in)
		sb.WriteString(" -> ")
		sb.WriteString(Stem(in, true))
		sb.WriteByte('\n')
	}

	testutil.VerifyApprovedString(t, sb.String())
}
