package basque

import "testing"

func TestStem(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"museoak", "museo"},
		{"museoan", "museo"},
		{"musikagilea", "musi"},
		{"musikagileak", "musi"},
		{"barrutiaren", "barru"},
		{"barrutiek", "barru"},
		{"barrutien", "barru"},
		{"barrutietako", "barru"},
		{"barrutietan", "barru"},
		{"barrutik", "barrut"},
		{"barrutiko", "barru"},
		{"barrutitan", "barrutit"},
		{"basa", "basa"},
		{"basailu", "basailu"},
		{"basalto", "basal"},
		{"basamortu", "basam"},
		{"katuek", "katu"},
		{"etxean", "etxean"},
		{"etxea", "etxea"},
		{"etxeak", "etxe"},
		{"ikasleak", "ikasle"},
		{"liburua", "liburua"},
		{"egunero", "egun"},
		{"izenak", "iz"},
		{"zuhaitzak", "zuhai"},
		{"eskolara", "eskol"},
		{"urtean", "urtean"},
		{"herrian", "herri"},
		{"ikaslea", "ikaslea"},
		{"eskola", "esko"},
		{"gela", "gela"},
		{"gauza", "gau"},
		{"etorri", "etorri"},
		{"egin", "egin"},
		{"joan", "joan"},
		{"ikusi", "ikusi"},
	}

	for _, tt := range tests {
		got := Stem(tt.in, true)
		if got != tt.out {
			t.Errorf("Stem(%q) = %q; want %q", tt.in, got, tt.out)
		}
	}
}
