package romance

import (
	"testing"

	"marrow/internal/stemmer/snowballword"
)

// isVowelASCII reports whether r is one of a, e, i, o, u (lowercase or uppercase).
func isVowelASCII(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u',
		'A', 'E', 'I', 'O', 'U':
		return true
	}
	return false
}

// isVowelUnicode reports whether r is an ASCII vowel or a selection of
// Latin-1 and extended vowels (é, è, ê, ï, etc.).
func isVowelUnicode(r rune) bool {
	if isVowelASCII(r) {
		return true
	}
	switch r {
	case 'á', 'é', 'í', 'ó', 'ú',
		'à', 'è', 'ì', 'ò', 'ù',
		'â', 'ê', 'î', 'ô', 'û',
		'ä', 'ë', 'ï', 'ö', 'ü',
		'Á', 'É', 'Í', 'Ó', 'Ú',
		'À', 'È', 'Ì', 'Ò', 'Ù',
		'Â', 'Ê', 'Î', 'Ô', 'Û',
		'Ä', 'Ë', 'Ï', 'Ö', 'Ü':
		return true
	}
	return false
}

func TestVnvSuffix(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		start    int
		isVowel  isVowelFunc
		expected int
	}{
		// Basic vowel patterns
		{
			name:     "consonant-vowel-consonant",
			word:     "hello",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 3, // hel|lo  (e->l transition)
		},
		{
			name:     "all-vowels",
			word:     "aeiou",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 5, // no V->NV transition
		},
		{
			name:     "all-consonants",
			word:     "bcdfg",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 5, // no V->NV transition
		},
		{
			name:     "single-vowel",
			word:     "a",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 1, // no V->NV transition
		},
		{
			name:     "single-consonant",
			word:     "x",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 1, // no V->NV transition
		},
		{
			name:     "two-runes-vowel-consonant",
			word:     "at",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 2, // a->t
		},
		{
			name:     "two-runes-consonant-vowel",
			word:     "to",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 2, // no V->NV transition
		},
		{
			name:     "vowel-at-start",
			word:     "apple",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 2, // a->p
		},
		{
			name:     "vowels-at-end",
			word:     "sea",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 3, // s->e->a, no V->NV
		},

		// Words starting with consonant clusters then vowel-consonant
		{
			name:     "string",
			word:     "string",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 5, // str|ing  (i->n)
		},
		{
			name:     "plural",
			word:     "plural",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 3, // pl|ural  (u->r)
		},
		{
			name:     "street",
			word:     "street",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 3, // st|reet  (r->e is vowel->vowel, e->e is vowel->vowel... wait)
			// Actually for "street": s t r e e t
			// i=1: s->t (N->N) no
			// i=2: t->r (N->N) no
			// i=3: r->e (N->V) no
			// i=4: e->e (V->V) no
			// i=5: e->t (V->NV) yes -> return 6
		},
		{
			name:     "school",
			word:     "school",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 4, // sch|ool  (c->h is N->N, h->o is N->V, o->o V->V, o->l V->NV) -> return 5? Wait...
			// "school": s c h o o l
			// i=1: s->c (N->N) no
			// i=2: c->h (N->N) no
			// i=3: h->o (N->V) no
			// i=4: o->o (V->V) no
			// i=5: o->l (V->NV) yes -> return 6
		},
		{
			name:     "rhythm",
			word:     "rhythm",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 6, // no V->NV (y not counted as vowel here)
		},

		// Different start positions
		{
			name:     "banana-start-0",
			word:     "banana",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 3, // ba|nana (a->n)
		},
		{
			name:     "banana-start-2",
			word:     "banana",
			start:    2,
			isVowel:  isVowelASCII,
			expected: 5, // n|a->n->a, first V->NV at index 4 (a->n), return 5
		},
		{
			name:     "banana-start-3",
			word:     "banana",
			start:    3,
			isVowel:  isVowelASCII,
			expected: 5, // ana: a->n V->NV at index 4, return 5
		},
		{
			name:     "banana-start-4",
			word:     "banana",
			start:    4,
			isVowel:  isVowelASCII,
			expected: 6, // na: n->a N->V, no V->NV
		},
		{
			name:     "start-at-last-rune",
			word:     "hello",
			start:    4,
			isVowel:  isVowelASCII,
			expected: 5, // only "o" remains, loop doesn't run
		},
		{
			name:     "start-beyond-length",
			word:     "hi",
			start:    5,
			isVowel:  isVowelASCII,
			expected: 2, // word.RS[start:] is empty, len=0, loop doesn't run
		},
		{
			name:     "start-at-transition",
			word:     "avocado",
			start:    1,
			isVowel:  isVowelASCII,
			expected: 3, // from "vocado": v->o N->V, o->c V->NV at j=3, return 4? Wait...
			// "avocado": a v o c a d o
			// start=1, i=1 -> j=2, RS[1]=v(N), RS[2]=o(V) -> no
			// i=2 -> j=3, RS[2]=o(V), RS[3]=c(NV) -> yes, return 4
		},

		// Unicode vowels
		{
			name:     "cafe-acute",
			word:     "café",
			start:    0,
			isVowel:  isVowelUnicode,
			expected: 3, // caf|é -> a->f V->NV at j=2, return 3
		},
		{
			name:     "naive-dieresis",
			word:     "naïve",
			start:    0,
			isVowel:  isVowelUnicode,
			expected: 2, // n|aïve -> n->a N->V, a->ï V->V, ï->v V->NV at j=3, return 4? Wait...
			// "naïve": n a ï v e
			// i=1: n->a (N->V) no
			// i=2: a->ï (V->V) no
			// i=3: ï->v (V->NV) yes -> return 4
		},
		{
			name:     "resume-grave",
			word:     "résumé",
			start:    0,
			isVowel:  isVowelUnicode,
			expected: 2, // r|ésumé -> r->é N->V, é->s V->NV at j=2, return 3? Wait...
			// "résumé": r é s u m é
			// i=1: r->é (N->V) no
			// i=2: é->s (V->NV) yes -> return 3
		},
		{
			name:     "all-unicode-vowels",
			word:     "éèà",
			start:    0,
			isVowel:  isVowelUnicode,
			expected: 3, // no V->NV transition
		},

		// Multiple VNV patterns
		{
			name:     "avocado-first-vnv",
			word:     "avocado",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 2, // a->v at j=1, return 2 (first occurrence wins)
		},
		{
			name:     "elephant",
			word:     "elephant",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 2, // e->l at j=1, return 2 (first occurrence wins)
		},
		{
			name:     "abacate",
			word:     "abacate",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 2, // a->b at j=1
		},
		{
			name:     "abacate-start-2",
			word:     "abacate",
			start:    2,
			isVowel:  isVowelASCII,
			expected: 4, // a->c at j=3, return 4 (second occurrence when skipped)
		},
		{
			name:     "mississippi",
			word:     "mississippi",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 4, // m->i N->V, i->s V->NV at j=3, return 4
		},
		{
			name:     "mississippi-start-4",
			word:     "mississippi",
			start:    4,
			isVowel:  isVowelASCII,
			expected: 8, // issippi: i->s V->NV at j=5, return 6... wait let me recount
			// m i s s i s s i p p i
			// 0 1 2 3 4 5 6 7 8 9 10
			// start=4, RS[4:] = "issippi"
			// i=1 -> j=5, RS[4]=i(V), RS[5]=s(NV) -> yes, return 6
		},

		// Edge cases
		{
			name:     "empty-string",
			word:     "",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 0,
		},
		{
			name:     "only-vowels",
			word:     "euouae",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 6,
		},
		{
			name:     "alternating-vc",
			word:     "ababab",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 2, // a->b at j=1
		},
		{
			name:     "double-consonant-after-vowel",
			word:     "act",
			start:    0,
			isVowel:  isVowelASCII,
			expected: 2, // a->c at j=1, return 2 (only first non-vowel matters)
		},
	}

	// Fix expected values for a few test cases that I computed inline above
	for i := range tests {
		switch tests[i].name {
		case "plural":
			tests[i].expected = 4
		case "street":
			tests[i].expected = 6
		case "school":
			tests[i].expected = 6
		case "start-at-transition":
			tests[i].expected = 4
		case "naive-dieresis":
			tests[i].expected = 4
		case "resume-grave":
			tests[i].expected = 3
		case "mississippi":
			tests[i].expected = 3
		case "mississippi-start-4":
			tests[i].expected = 6
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			word := snowballword.New(tt.word)
			got := VnvSuffix(word, tt.isVowel, tt.start)
			if got != tt.expected {
				t.Errorf("VnvSuffix(%q, start=%d) = %d; want %d",
					tt.word, tt.start, got, tt.expected)
			}
		})
	}
}

func TestVnvSuffixMultipleCallsSameWord(t *testing.T) {
	// Ensure the function does not mutate the word.
	word := snowballword.New("banana")

	first := VnvSuffix(word, isVowelASCII, 0)
	if first != 3 {
		t.Fatalf("first call: expected 3, got %d", first)
	}

	second := VnvSuffix(word, isVowelASCII, 2)
	if second != 5 {
		t.Fatalf("second call: expected 5, got %d", second)
	}

	third := VnvSuffix(word, isVowelASCII, 0)
	if third != 3 {
		t.Fatalf("third call: expected 3, got %d", third)
	}

	if got := len(word.RS); got != 6 {
		t.Fatalf("word length mutated: expected 6, got %d", got)
	}
}
