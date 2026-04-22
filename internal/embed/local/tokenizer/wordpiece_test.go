package tokenizer

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeVocab writes a vocab file with the given tokens (one per line) and
// returns its path.
func writeVocab(t *testing.T, tokens []string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "vocab.txt")
	if err := os.WriteFile(p, []byte(strings.Join(tokens, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

// miniVocab gives us the five special tokens plus a handful of real ones in
// bert-base-uncased order (PAD=0, UNK=100, CLS=101, SEP=102, MASK=103). The
// tests use id lookups, not positions in this slice; only the vocab map
// matters. We just need the specials at well-known offsets.
func miniVocab(extra []string) []string {
	v := make([]string, 104)
	v[PadID] = PadToken
	v[UnkID] = UnkToken
	v[CLSID] = CLSToken
	v[SEPID] = SEPToken
	v[MaskID] = MaskToken
	// Fill unused positions with dummies so the file parses cleanly.
	for i := range v {
		if v[i] == "" {
			v[i] = "__unused" + itoa(i)
		}
	}
	return append(v, extra...)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[n:])
}

func TestEncode_SpecialsAndLowercase(t *testing.T) {
	vocab := miniVocab([]string{
		"hello", "world", "!",
	})
	p := writeVocab(t, vocab)
	tk, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	got := tk.Encode("Hello, World!")
	// Expect: [CLS] hello , world ! [SEP]
	//         101   104  UNK 105  106  102
	want := []int32{int32(tk.clsID), 104, int32(tk.unkID), 105, 106, int32(tk.sepID)}
	if !reflect.DeepEqual(got.IDs, want) {
		t.Fatalf("ids mismatch:\n got: %v\nwant: %v", got.IDs, want)
	}
	if len(got.AttentionMask) != len(got.IDs) {
		t.Fatalf("mask len %d != ids len %d", len(got.AttentionMask), len(got.IDs))
	}
	for i, m := range got.AttentionMask {
		if m != 1 {
			t.Fatalf("mask[%d] = %d, want 1", i, m)
		}
	}
}

func TestWordPiece_GreedyLongestMatch(t *testing.T) {
	// "playing" → play + ##ing (greedy longest match should prefer "play"
	// over "p").
	vocab := miniVocab([]string{"p", "play", "##ing"})
	p := writeVocab(t, vocab)
	tk, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	got := tk.wordpiece("playing")
	want := []string{"play", "##ing"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestWordPiece_UnknownReturnsUnk(t *testing.T) {
	vocab := miniVocab([]string{"hello"})
	p := writeVocab(t, vocab)
	tk, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	got := tk.wordpiece("zzqxzz")
	if len(got) != 1 || got[0] != UnkToken {
		t.Fatalf("got %v, want [[UNK]]", got)
	}
}

func TestBasicTokenize_SplitsPunct(t *testing.T) {
	vocab := miniVocab(nil)
	p := writeVocab(t, vocab)
	tk, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	got := tk.basicTokenize("don't stop!")
	want := []string{"don", "'", "t", "stop", "!"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBasicTokenize_StripsAccents(t *testing.T) {
	vocab := miniVocab(nil)
	p := writeVocab(t, vocab)
	tk, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	got := tk.basicTokenize("Café résumé")
	want := []string{"cafe", "resume"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBasicTokenize_CJKEachCharOwnToken(t *testing.T) {
	vocab := miniVocab(nil)
	p := writeVocab(t, vocab)
	tk, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	got := tk.basicTokenize("hello 世界")
	want := []string{"hello", "世", "界"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestEncode_TruncatesToMaxInput(t *testing.T) {
	vocab := miniVocab([]string{"x"})
	p := writeVocab(t, vocab)
	tk, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	// 600 single-char tokens, each maps to the same "x" → UNK id doesn't
	// matter, we just care about the length.
	in := strings.Repeat("x ", 600)
	got := tk.Encode(in)
	if len(got.IDs) != 512 {
		t.Fatalf("len=%d, want 512", len(got.IDs))
	}
	if got.IDs[0] != int32(tk.clsID) || got.IDs[511] != int32(tk.sepID) {
		t.Fatalf("expected CLS/SEP at boundaries, got %d/%d", got.IDs[0], got.IDs[511])
	}
}
