package basque

import (
	"strings"
	"testing"

	"marrow/internal/testutil"
)

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
