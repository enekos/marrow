package snowballword

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantRS string
		wantR1 int
		wantR2 int
		wantRV int
	}{
		{"empty", "", "", 0, 0, 0},
		{"ascii", "hello", "hello", 5, 5, 5},
		{"unicode", "ñáéíóú", "ñáéíóú", 6, 6, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := New(tt.input)
			if got := string(w.RS); got != tt.wantRS {
				t.Errorf("RS = %q, want %q", got, tt.wantRS)
			}
			if w.R1start != tt.wantR1 {
				t.Errorf("R1start = %d, want %d", w.R1start, tt.wantR1)
			}
			if w.R2start != tt.wantR2 {
				t.Errorf("R2start = %d, want %d", w.R2start, tt.wantR2)
			}
			if w.RVstart != tt.wantRV {
				t.Errorf("RVstart = %d, want %d", w.RVstart, tt.wantRV)
			}
		})
	}
}

func TestSnowballWord_String(t *testing.T) {
	w := New("café")
	if got := w.String(); got != "café" {
		t.Errorf("String() = %q, want %q", got, "café")
	}
	w = New("")
	if got := w.String(); got != "" {
		t.Errorf("String() = %q, want %q", got, "")
	}
}

func TestSnowballWord_DebugString(t *testing.T) {
	w := New("test")
	w.R1start = 1
	w.R2start = 2
	w.RVstart = 3
	want := `{"test", 1, 2, 3}`
	if got := w.DebugString(); got != want {
		t.Errorf("DebugString() = %q, want %q", got, want)
	}
}

func TestSnowballWord_R1_R2_RV(t *testing.T) {
	w := New("beautiful")
	w.R1start = 2
	w.R2start = 6
	w.RVstart = 3

	if got := string(w.R1()); got != "autiful" {
		t.Errorf("R1() = %q, want %q", got, "autiful")
	}
	if got := w.R1String(); got != "autiful" {
		t.Errorf("R1String() = %q, want %q", got, "autiful")
	}

	if got := string(w.R2()); got != "ful" {
		t.Errorf("R2() = %q, want %q", got, "ful")
	}
	if got := w.R2String(); got != "ful" {
		t.Errorf("R2String() = %q, want %q", got, "ful")
	}

	if got := string(w.RV()); got != "utiful" {
		t.Errorf("RV() = %q, want %q", got, "utiful")
	}
	if got := w.RVString(); got != "utiful" {
		t.Errorf("RVString() = %q, want %q", got, "utiful")
	}
}

func TestSnowballWord_FitsInR1R2RV(t *testing.T) {
	w := New("abc")
	w.R1start = 1
	w.R2start = 2
	w.RVstart = 1

	// len=3, R1start=1 => R1 len=2. FitsInR1(2) => 1 <= 1 true. FitsInR1(3) => 1 <= 0 false.
	if !w.FitsInR1(2) {
		t.Error("FitsInR1(2) should be true")
	}
	if w.FitsInR1(3) {
		t.Error("FitsInR1(3) should be false")
	}

	// len=3, R2start=2 => R2 len=1. FitsInR2(1) => 2 <= 2 true. FitsInR2(2) => 2 <= 1 false.
	if !w.FitsInR2(1) {
		t.Error("FitsInR2(1) should be true")
	}
	if w.FitsInR2(2) {
		t.Error("FitsInR2(2) should be false")
	}

	// len=3, RVstart=1 => RV len=2. FitsInRV(2) => 1 <= 1 true. FitsInRV(3) => 1 <= 0 false.
	if !w.FitsInRV(2) {
		t.Error("FitsInRV(2) should be true")
	}
	if w.FitsInRV(3) {
		t.Error("FitsInRV(3) should be false")
	}
}

func TestSnowballWord_FitsInR1R2RV_Empty(t *testing.T) {
	w := New("")
	// len=0, R1start=0. FitsInR1(0) => 0 <= 0 true.
	if !w.FitsInR1(0) {
		t.Error("FitsInR1(0) on empty should be true")
	}
	if !w.FitsInR2(0) {
		t.Error("FitsInR2(0) on empty should be true")
	}
	if !w.FitsInRV(0) {
		t.Error("FitsInRV(0) on empty should be true")
	}
	// Any x > 0 should be false
	if w.FitsInR1(1) {
		t.Error("FitsInR1(1) on empty should be false")
	}
	if w.FitsInR2(1) {
		t.Error("FitsInR2(1) on empty should be false")
	}
	if w.FitsInRV(1) {
		t.Error("FitsInRV(1) on empty should be false")
	}
}

func TestSnowballWord_RemoveLastNRunes(t *testing.T) {
	w := New("running")
	w.R1start = 2
	w.R2start = 4
	w.RVstart = 1

	w.RemoveLastNRunes(3)
	if got := w.String(); got != "runn" {
		t.Errorf("after RemoveLastNRunes(3) = %q, want %q", got, "runn")
	}
	if w.R1start != 2 {
		t.Errorf("R1start = %d, want %d", w.R1start, 2)
	}
	if w.R2start != 4 {
		t.Errorf("R2start = %d, want %d", w.R2start, 4)
	}
	if w.RVstart != 1 {
		t.Errorf("RVstart = %d, want %d", w.RVstart, 1)
	}

	// removing beyond bounds should clamp R1/R2/RV
	w.R1start = 10
	w.R2start = 10
	w.RVstart = 10
	w.RemoveLastNRunes(1)
	if w.R1start != 3 {
		t.Errorf("R1start clamped = %d, want %d", w.R1start, 3)
	}
	if w.R2start != 3 {
		t.Errorf("R2start clamped = %d, want %d", w.R2start, 3)
	}
	if w.RVstart != 3 {
		t.Errorf("RVstart clamped = %d, want %d", w.RVstart, 3)
	}
}

func TestSnowballWord_RemoveLastNRunes_Unicode(t *testing.T) {
	w := New("cafés")
	w.R1start = 2
	w.RemoveLastNRunes(1)
	if got := w.String(); got != "café" {
		t.Errorf("String() = %q, want %q", got, "café")
	}
}

func TestSnowballWord_ReplaceSuffix(t *testing.T) {
	w := New("running")
	w.R1start = 2
	w.R2start = 4

	// non-force, suffix exists
	if !w.ReplaceSuffix("ing", "", false) {
		t.Error("ReplaceSuffix('ing', '', false) should return true")
	}
	if got := w.String(); got != "runn" {
		t.Errorf("String() = %q, want %q", got, "runn")
	}

	// non-force, suffix does not exist
	if w.ReplaceSuffix("xyz", "abc", false) {
		t.Error("ReplaceSuffix('xyz', 'abc', false) should return false")
	}

	// force=true, suffix doesn't need to exist
	w = New("test")
	w.R1start = 1
	if !w.ReplaceSuffix("st", "sted", true) {
		t.Error("ReplaceSuffix('st', 'sted', true) should return true")
	}
	if got := w.String(); got != "tested" {
		t.Errorf("String() = %q, want %q", got, "tested")
	}
}

func TestSnowballWord_ReplaceSuffix_EmptyReplacement(t *testing.T) {
	w := New("cats")
	if !w.ReplaceSuffix("s", "", true) {
		t.Error("ReplaceSuffix('s', '', true) should return true")
	}
	if got := w.String(); got != "cat" {
		t.Errorf("String() = %q, want %q", got, "cat")
	}
}

func TestSnowballWord_ReplaceSuffixRunes(t *testing.T) {
	w := New("running")
	w.R1start = 2
	w.R2start = 4

	if !w.ReplaceSuffixRunes([]rune("ing"), []rune("ed"), false) {
		t.Error("ReplaceSuffixRunes('ing', 'ed', false) should return true")
	}
	if got := w.String(); got != "runned" {
		t.Errorf("String() = %q, want %q", got, "runned")
	}

	if w.ReplaceSuffixRunes([]rune("xyz"), []rune("abc"), false) {
		t.Error("ReplaceSuffixRunes('xyz', 'abc', false) should return false")
	}

	w = New("test")
	if !w.ReplaceSuffixRunes([]rune("st"), []rune("sted"), true) {
		t.Error("ReplaceSuffixRunes('st', 'sted', true) should return true")
	}
	if got := w.String(); got != "tested" {
		t.Errorf("String() = %q, want %q", got, "tested")
	}
}

func TestSnowballWord_ReplaceSuffixRunes_Unicode(t *testing.T) {
	w := New("niño")
	if !w.ReplaceSuffixRunes([]rune("ño"), []rune("ña"), false) {
		t.Error("ReplaceSuffixRunes('ño', 'ña', false) should return true")
	}
	if got := w.String(); got != "niña" {
		t.Errorf("String() = %q, want %q", got, "niña")
	}
}

func TestSnowballWord_FirstPrefix(t *testing.T) {
	w := New("unhappy")
	found, foundRunes := w.FirstPrefix("re", "un", "in")
	if found != "un" {
		t.Errorf("FirstPrefix() = %q, want %q", found, "un")
	}
	if string(foundRunes) != "un" {
		t.Errorf("foundRunes = %q, want %q", string(foundRunes), "un")
	}

	found, foundRunes = w.FirstPrefix("re", "in")
	if found != "" {
		t.Errorf("FirstPrefix() = %q, want %q", found, "")
	}
	if len(foundRunes) != 0 {
		t.Errorf("foundRunes length = %d, want 0", len(foundRunes))
	}
}

func TestSnowballWord_FirstPrefix_EmptyWord(t *testing.T) {
	w := New("")
	found, foundRunes := w.FirstPrefix("a", "b")
	if found != "" {
		t.Errorf("FirstPrefix() = %q, want %q", found, "")
	}
	if len(foundRunes) != 0 {
		t.Errorf("foundRunes length = %d, want 0", len(foundRunes))
	}
}

func TestSnowballWord_FirstPrefix_LongerThanWord(t *testing.T) {
	w := New("a")
	found, _ := w.FirstPrefix("ab", "abc")
	if found != "" {
		t.Errorf("FirstPrefix() = %q, want %q", found, "")
	}
}

func TestSnowballWord_HasSuffixRunes(t *testing.T) {
	w := New("running")
	if !w.HasSuffixRunes([]rune("ing")) {
		t.Error("HasSuffixRunes('ing') should be true")
	}
	if w.HasSuffixRunes([]rune("run")) {
		t.Error("HasSuffixRunes('run') should be false")
	}
	if w.HasSuffixRunes([]rune("runningx")) {
		t.Error("HasSuffixRunes('runningx') should be false")
	}
}

func TestSnowballWord_HasSuffixRunesIn(t *testing.T) {
	w := New("running")

	// whole word ends with "ing"
	if !w.HasSuffixRunesIn(0, 7, []rune("ing")) {
		t.Error("HasSuffixRunesIn(0,7,'ing') should be true")
	}

	// substring "runn" (0,4) ends with "nn"
	if !w.HasSuffixRunesIn(0, 4, []rune("nn")) {
		t.Error("HasSuffixRunesIn(0,4,'nn') should be true")
	}

	// substring "run" (0,3) ends with "run"
	if !w.HasSuffixRunesIn(0, 3, []rune("run")) {
		t.Error("HasSuffixRunesIn(0,3,'run') should be true")
	}

	// substring "nn" (2,4) ends with "n"
	if !w.HasSuffixRunesIn(2, 4, []rune("n")) {
		t.Error("HasSuffixRunesIn(2,4,'n') should be true")
	}

	// suffix longer than region
	if w.HasSuffixRunesIn(0, 2, []rune("run")) {
		t.Error("HasSuffixRunesIn(0,2,'run') should be false")
	}
}

func TestSnowballWord_FirstSuffix(t *testing.T) {
	w := New("beautifully")
	// Order matters: "ly" matches before "fully" is checked
	// ("ful" is NOT a suffix of "beautifully", which ends in "lly")
	suffix, suffixRunes := w.FirstSuffix("ful", "ly", "fully")
	if suffix != "ly" {
		t.Errorf("FirstSuffix() = %q, want %q", suffix, "ly")
	}
	if string(suffixRunes) != "ly" {
		t.Errorf("suffixRunes = %q, want %q", string(suffixRunes), "ly")
	}
}

func TestSnowballWord_FirstSuffix_NoMatch(t *testing.T) {
	w := New("cat")
	suffix, suffixRunes := w.FirstSuffix("dog", "ing")
	if suffix != "" {
		t.Errorf("FirstSuffix() = %q, want %q", suffix, "")
	}
	if len(suffixRunes) != 0 {
		t.Errorf("suffixRunes length = %d, want 0", len(suffixRunes))
	}
}

func TestSnowballWord_FirstSuffixIfIn(t *testing.T) {
	w := New("beautifully")
	// "beautifully" ends with "fully" at position 11. startPos=4 means suffix must start at index >=4.
	// "fully" starts at index 6, so it should match.
	suffix, suffixRunes := w.FirstSuffixIfIn(4, len(w.RS), "fully", "ly")
	if suffix != "fully" {
		t.Errorf("FirstSuffixIfIn() = %q, want %q", suffix, "fully")
	}

	// startPos=6 means suffix must start at index >=6. "fully" starts at 6, so it should match.
	suffix, suffixRunes = w.FirstSuffixIfIn(6, len(w.RS), "fully", "ly")
	if suffix != "fully" {
		t.Errorf("FirstSuffixIfIn() = %q, want %q", suffix, "fully")
	}
	if string(suffixRunes) != "fully" {
		t.Errorf("suffixRunes = %q, want %q", string(suffixRunes), "fully")
	}

	// startPos=7 means suffix must start at index >=7. "fully" starts at 6, which is blocked.
	// The function returns "" immediately when the first match is blocked,
	// without checking remaining suffixes.
	suffix, suffixRunes = w.FirstSuffixIfIn(7, len(w.RS), "fully", "ly")
	if suffix != "" {
		t.Errorf("FirstSuffixIfIn() = %q, want %q", suffix, "")
	}
	if len(suffixRunes) != 0 {
		t.Errorf("suffixRunes length = %d, want 0", len(suffixRunes))
	}
}

func TestSnowballWord_FirstSuffixIn(t *testing.T) {
	w := New("beautifully")
	// search in region [4, 9) => "tiful". "ful" is a suffix of "tiful" and starts at 6 which is >=4.
	suffix, suffixRunes := w.FirstSuffixIn(4, 9, "ful", "ly")
	if suffix != "ful" {
		t.Errorf("FirstSuffixIn() = %q, want %q", suffix, "ful")
	}

	// "ly" is not a suffix of region [4,9) since region is "tiful"
	suffix, suffixRunes = w.FirstSuffixIn(4, 9, "ly")
	if suffix != "" {
		t.Errorf("FirstSuffixIn() = %q, want %q", suffix, "")
	}
	if len(suffixRunes) != 0 {
		t.Errorf("suffixRunes length = %d, want 0", len(suffixRunes))
	}
}

func TestSnowballWord_RemoveFirstSuffix(t *testing.T) {
	w := New("cats")
	suffix, suffixRunes := w.RemoveFirstSuffix("s", "ing")
	if suffix != "s" {
		t.Errorf("RemoveFirstSuffix() = %q, want %q", suffix, "s")
	}
	if got := w.String(); got != "cat" {
		t.Errorf("String() = %q, want %q", got, "cat")
	}
	if len(suffixRunes) != 1 {
		t.Errorf("suffixRunes length = %d, want 1", len(suffixRunes))
	}
}

func TestSnowballWord_RemoveFirstSuffix_NoMatch(t *testing.T) {
	w := New("cat")
	suffix, suffixRunes := w.RemoveFirstSuffix("s", "ing")
	if suffix != "" {
		t.Errorf("RemoveFirstSuffix() = %q, want %q", suffix, "")
	}
	if got := w.String(); got != "cat" {
		t.Errorf("String() = %q, want %q", got, "cat")
	}
	if len(suffixRunes) != 0 {
		t.Errorf("suffixRunes length = %d, want 0", len(suffixRunes))
	}
}

func TestSnowballWord_RemoveFirstSuffixIfIn(t *testing.T) {
	w := New("beautifully")
	// "fully" starts at 6 >= 4, so it matches and is removed
	suffix, suffixRunes := w.RemoveFirstSuffixIfIn(4, "fully", "ly")
	if suffix != "fully" {
		t.Errorf("RemoveFirstSuffixIfIn() = %q, want %q", suffix, "fully")
	}
	if got := w.String(); got != "beauti" {
		t.Errorf("String() = %q, want %q", got, "beauti")
	}
	if len(suffixRunes) != 5 {
		t.Errorf("suffixRunes length = %d, want 5", len(suffixRunes))
	}
}

func TestSnowballWord_RemoveFirstSuffixIfIn_BlockedByStartPos(t *testing.T) {
	w := New("beautifully")
	// "fully" starts at 6, but startPos=7 means it's blocked.
	// The function returns "" immediately without checking "ly".
	suffix, _ := w.RemoveFirstSuffixIfIn(7, "fully", "ly")
	if suffix != "" {
		t.Errorf("RemoveFirstSuffixIfIn() = %q, want %q", suffix, "")
	}
	if got := w.String(); got != "beautifully" {
		t.Errorf("String() = %q, want %q", got, "beautifully")
	}
}

func TestSnowballWord_RemoveFirstSuffixIn(t *testing.T) {
	w := New("beautifully")
	// RemoveFirstSuffixIn uses FirstSuffixIn(startPos, len(w.RS), suffixes...)
	// For startPos=4, it searches region RS[4:11] = "ifully".
	// "ful" is NOT a suffix of "ifully" (which ends in "lly"), but "ly" is.
	suffix, _ := w.RemoveFirstSuffixIn(4, "ful", "ly")
	if suffix != "ly" {
		t.Errorf("RemoveFirstSuffixIn() = %q, want %q", suffix, "ly")
	}
	if got := w.String(); got != "beautiful" {
		t.Errorf("String() = %q, want %q", got, "beautiful")
	}
}

// --- Rune-optimized variant tests ---

func TestSnowballWord_FirstSuffixRunes(t *testing.T) {
	w := New("running")
	suffixRunes := w.FirstSuffixRunes([]rune("ing"), []rune("run"))
	if string(suffixRunes) != "ing" {
		t.Errorf("FirstSuffixRunes() = %q, want %q", string(suffixRunes), "ing")
	}
}

func TestSnowballWord_FirstSuffixRunes_NoMatch(t *testing.T) {
	w := New("cat")
	suffixRunes := w.FirstSuffixRunes([]rune("dog"), []rune("ing"))
	if suffixRunes != nil {
		t.Error("FirstSuffixRunes() should return nil")
	}
}

func TestSnowballWord_FirstSuffixIfInRunes(t *testing.T) {
	w := New("beautifully")
	suffixRunes := w.FirstSuffixIfInRunes(4, len(w.RS), []rune("fully"), []rune("ly"))
	if string(suffixRunes) != "fully" {
		t.Errorf("FirstSuffixIfInRunes() = %q, want %q", string(suffixRunes), "fully")
	}

	// blocked by startPos for first match -> returns nil immediately
	suffixRunes = w.FirstSuffixIfInRunes(7, len(w.RS), []rune("fully"), []rune("ly"))
	if suffixRunes != nil {
		t.Error("FirstSuffixIfInRunes() should return nil")
	}

	// completely blocked
	suffixRunes = w.FirstSuffixIfInRunes(10, len(w.RS), []rune("ly"))
	if suffixRunes != nil {
		t.Error("FirstSuffixIfInRunes() should return nil")
	}
}

func TestSnowballWord_FirstSuffixInRunes(t *testing.T) {
	w := New("beautifully")
	suffixRunes := w.FirstSuffixInRunes(4, 9, []rune("ful"), []rune("ly"))
	if string(suffixRunes) != "ful" {
		t.Errorf("FirstSuffixInRunes() = %q, want %q", string(suffixRunes), "ful")
	}

	suffixRunes = w.FirstSuffixInRunes(4, 9, []rune("ly"))
	if suffixRunes != nil {
		t.Error("FirstSuffixInRunes() should return nil")
	}
}

func TestSnowballWord_RemoveFirstSuffixIfInRunes(t *testing.T) {
	w := New("beautifully")
	suffixRunes := w.RemoveFirstSuffixIfInRunes(4, []rune("fully"), []rune("ly"))
	if string(suffixRunes) != "fully" {
		t.Errorf("RemoveFirstSuffixIfInRunes() = %q, want %q", string(suffixRunes), "fully")
	}
	if got := w.String(); got != "beauti" {
		t.Errorf("String() = %q, want %q", got, "beauti")
	}
}

func TestSnowballWord_RemoveFirstSuffixInRunes(t *testing.T) {
	w := New("beautifully")
	// "ful" is NOT a suffix of region RS[4:11]="ifully", but "ly" is.
	suffixRunes := w.RemoveFirstSuffixInRunes(4, []rune("ful"), []rune("ly"))
	if string(suffixRunes) != "ly" {
		t.Errorf("RemoveFirstSuffixInRunes() = %q, want %q", string(suffixRunes), "ly")
	}
	if got := w.String(); got != "beautiful" {
		t.Errorf("String() = %q, want %q", got, "beautiful")
	}
}

func TestSnowballWord_ReplaceSuffix_AdjustsR1R2(t *testing.T) {
	w := New("runningly")
	w.R1start = 5
	w.R2start = 7
	w.RVstart = 4

	w.ReplaceSuffix("ingly", "", false)
	if got := w.String(); got != "runn" {
		t.Errorf("String() = %q, want %q", got, "runn")
	}
	if w.R1start != 4 {
		t.Errorf("R1start = %d, want %d", w.R1start, 4)
	}
	if w.R2start != 4 {
		t.Errorf("R2start = %d, want %d", w.R2start, 4)
	}
	if w.RVstart != 4 {
		t.Errorf("RVstart = %d, want %d", w.RVstart, 4)
	}
}

func TestSnowballWord_ReplaceSuffixRunes_AdjustsR1R2(t *testing.T) {
	w := New("happiness")
	w.R1start = 3
	w.R2start = 6
	w.RVstart = 2

	w.ReplaceSuffixRunes([]rune("ness"), []rune("ness"), false)
	if got := w.String(); got != "happiness" {
		t.Errorf("String() = %q, want %q", got, "happiness")
	}
	// R1start was 3, len remains 9, R1start should remain 3
	if w.R1start != 3 {
		t.Errorf("R1start = %d, want %d", w.R1start, 3)
	}
	// R2start was 6, which is <= 9, so it should remain 6
	if w.R2start != 6 {
		t.Errorf("R2start = %d, want %d", w.R2start, 6)
	}
	// RVstart was 2, which is <= 9, so it should remain 2
	if w.RVstart != 2 {
		t.Errorf("RVstart = %d, want %d", w.RVstart, 2)
	}

	// Now test with a replacement that shortens the word
	w = New("happiness")
	w.R1start = 3
	w.R2start = 8
	w.RVstart = 2
	w.ReplaceSuffixRunes([]rune("ness"), []rune("y"), false)
	if got := w.String(); got != "happiy" {
		t.Errorf("String() = %q, want %q", got, "happiy")
	}
	// R2start was 8, new len is 6, so it should be clamped to 6
	if w.R2start != 6 {
		t.Errorf("R2start = %d, want %d", w.R2start, 6)
	}
}

func TestSnowballWord_HasSuffixRunesIn_EdgeCases(t *testing.T) {
	w := New("ab")
	if !w.HasSuffixRunesIn(0, 1, []rune("a")) {
		t.Error("HasSuffixRunesIn(0,1,'a') should be true")
	}
	if w.HasSuffixRunesIn(0, 1, []rune("b")) {
		t.Error("HasSuffixRunesIn(0,1,'b') should be false")
	}
	// empty suffix
	if !w.HasSuffixRunesIn(0, 0, []rune("")) {
		t.Error("HasSuffixRunesIn(0,0,'') should be true")
	}
}

func TestSnowballWord_FirstSuffix_EmptyWord(t *testing.T) {
	w := New("")
	suffix, suffixRunes := w.FirstSuffix("a", "b")
	if suffix != "" {
		t.Errorf("FirstSuffix() = %q, want %q", suffix, "")
	}
	if len(suffixRunes) != 0 {
		t.Errorf("suffixRunes length = %d, want 0", len(suffixRunes))
	}
}

func TestSnowballWord_FirstSuffixIfIn_EmptySuffix(t *testing.T) {
	w := New("abc")
	// empty suffix should match the end of any word
	suffix, suffixRunes := w.FirstSuffixIfIn(0, len(w.RS), "")
	if suffix != "" {
		t.Errorf("FirstSuffixIfIn() = %q, want %q", suffix, "")
	}
	if len(suffixRunes) != 0 {
		t.Errorf("suffixRunes length = %d, want 0", len(suffixRunes))
	}
}

func TestSnowballWord_RemoveFirstSuffixIfInRunes_BlockedByStartPos(t *testing.T) {
	w := New("beautifully")
	// "fully" starts at 6, but startPos=7 means it's blocked.
	// The function returns nil immediately without checking "ly".
	suffixRunes := w.RemoveFirstSuffixIfInRunes(7, []rune("fully"), []rune("ly"))
	if suffixRunes != nil {
		t.Error("RemoveFirstSuffixIfInRunes() should return nil")
	}
	if got := w.String(); got != "beautifully" {
		t.Errorf("String() = %q, want %q", got, "beautifully")
	}
}

func TestSnowballWord_RemoveFirstSuffixInRunes_NoMatch(t *testing.T) {
	w := New("cat")
	suffixRunes := w.RemoveFirstSuffixInRunes(0, []rune("dog"), []rune("ing"))
	if suffixRunes != nil {
		t.Error("RemoveFirstSuffixInRunes() should return nil")
	}
	if got := w.String(); got != "cat" {
		t.Errorf("String() = %q, want %q", got, "cat")
	}
}

func TestSnowballWord_ReplaceSuffix_LongerReplacement(t *testing.T) {
	w := New("cat")
	w.R1start = 1
	w.ReplaceSuffix("", "alog", true)
	if got := w.String(); got != "catalog" {
		t.Errorf("String() = %q, want %q", got, "catalog")
	}
}

func TestSnowballWord_FirstSuffixIfIn_OrderMatters(t *testing.T) {
	w := New("running")
	// "ing" and "ng" both match; first in list wins
	suffix, _ := w.FirstSuffixIfIn(0, len(w.RS), "ing", "ng")
	if suffix != "ing" {
		t.Errorf("FirstSuffixIfIn() = %q, want %q", suffix, "ing")
	}

	suffix, _ = w.FirstSuffixIfIn(0, len(w.RS), "ng", "ing")
	if suffix != "ng" {
		t.Errorf("FirstSuffixIfIn() = %q, want %q", suffix, "ng")
	}
}

func TestSnowballWord_FirstSuffixRunes_OrderMatters(t *testing.T) {
	w := New("running")
	suffixRunes := w.FirstSuffixRunes([]rune("ing"), []rune("ng"))
	if string(suffixRunes) != "ing" {
		t.Errorf("FirstSuffixRunes() = %q, want %q", string(suffixRunes), "ing")
	}

	suffixRunes = w.FirstSuffixRunes([]rune("ng"), []rune("ing"))
	if string(suffixRunes) != "ng" {
		t.Errorf("FirstSuffixRunes() = %q, want %q", string(suffixRunes), "ng")
	}
}

func TestSnowballWord_FirstPrefix_OrderMatters(t *testing.T) {
	w := New("unhappy")
	found, _ := w.FirstPrefix("un", "unh")
	if found != "un" {
		t.Errorf("FirstPrefix() = %q, want %q", found, "un")
	}

	found, _ = w.FirstPrefix("unh", "un")
	if found != "unh" {
		t.Errorf("FirstPrefix() = %q, want %q", found, "unh")
	}
}

func TestSnowballWord_ReplaceSuffix_ForceFalseNoMatch(t *testing.T) {
	w := New("test")
	if w.ReplaceSuffix("xyz", "abc", false) {
		t.Error("ReplaceSuffix('xyz', 'abc', false) should return false")
	}
	if got := w.String(); got != "test" {
		t.Errorf("String() = %q, want %q", got, "test")
	}
}

func TestSnowballWord_ReplaceSuffixRunes_ForceFalseNoMatch(t *testing.T) {
	w := New("test")
	if w.ReplaceSuffixRunes([]rune("xyz"), []rune("abc"), false) {
		t.Error("ReplaceSuffixRunes('xyz', 'abc', false) should return false")
	}
	if got := w.String(); got != "test" {
		t.Errorf("String() = %q, want %q", got, "test")
	}
}

func TestSnowballWord_ResetR1R2_AfterRemove(t *testing.T) {
	w := New("abc")
	w.R1start = 5
	w.R2start = 6
	w.RVstart = 7
	w.RemoveLastNRunes(1)
	if w.R1start != 2 {
		t.Errorf("R1start = %d, want %d", w.R1start, 2)
	}
	if w.R2start != 2 {
		t.Errorf("R2start = %d, want %d", w.R2start, 2)
	}
	if w.RVstart != 2 {
		t.Errorf("RVstart = %d, want %d", w.RVstart, 2)
	}
}

func TestSnowballWord_HasSuffixRunes_Unicode(t *testing.T) {
	w := New("niño")
	if !w.HasSuffixRunes([]rune("ño")) {
		t.Error("HasSuffixRunes('ño') should be true")
	}
	if w.HasSuffixRunes([]rune("n")) {
		t.Error("HasSuffixRunes('n') should be false")
	}
}

func TestSnowballWord_FirstSuffix_Unicode(t *testing.T) {
	w := New("niñez")
	// Order matters: "ez" matches before "ñez" is checked
	suffix, _ := w.FirstSuffix("ez", "ñez")
	if suffix != "ez" {
		t.Errorf("FirstSuffix() = %q, want %q", suffix, "ez")
	}
}

func TestSnowballWord_RemoveFirstSuffixIfIn_ExactStartPos(t *testing.T) {
	w := New("running")
	// "ing" starts at position 4. startPos=4 means it's allowed.
	suffix, _ := w.RemoveFirstSuffixIfIn(4, "run", "ing")
	if suffix != "ing" {
		t.Errorf("RemoveFirstSuffixIfIn() = %q, want %q", suffix, "ing")
	}
	if got := w.String(); got != "runn" {
		t.Errorf("String() = %q, want %q", got, "runn")
	}
}

func TestSnowballWord_FirstSuffixIfIn_ExactBoundary(t *testing.T) {
	w := New("running")
	// "ing" starts at 4. startPos=4 means suffix must start at index >=4. "ing" starts at 4, so it matches.
	suffix, _ := w.FirstSuffixIfIn(4, len(w.RS), "ing")
	if suffix != "ing" {
		t.Errorf("FirstSuffixIfIn() = %q, want %q", suffix, "ing")
	}
}

func TestSnowballWord_RemoveFirstSuffixInRunes_RegionSearch(t *testing.T) {
	w := New("beautifully")
	// RemoveFirstSuffixInRunes uses FirstSuffixInRunes(startPos, len(w.RS), suffixes...)
	// For startPos=4, it searches region RS[4:11] = "ifully".
	// "ly" is a suffix of "ifully" and is checked first, so it matches.
	suffixRunes := w.RemoveFirstSuffixInRunes(4, []rune("ly"), []rune("ful"))
	if string(suffixRunes) != "ly" {
		t.Errorf("RemoveFirstSuffixInRunes() = %q, want %q", string(suffixRunes), "ly")
	}
	if got := w.String(); got != "beautiful" {
		t.Errorf("String() = %q, want %q", got, "beautiful")
	}
}

func TestSnowballWord_FirstSuffixIfIn_MultipleBlocked(t *testing.T) {
	w := New("running")
	// "ing" starts at 4. startPos=5 blocks it.
	// The function returns "" immediately when the first match is blocked,
	// without checking remaining suffixes. So "ng" is never checked.
	suffix, _ := w.FirstSuffixIfIn(5, len(w.RS), "ing", "ng")
	if suffix != "" {
		t.Errorf("FirstSuffixIfIn() = %q, want %q", suffix, "")
	}

	// Both blocked: "ing" at 4 < 5, "run" is not a suffix.
	suffix, _ = w.FirstSuffixIfIn(5, len(w.RS), "ing", "run")
	if suffix != "" {
		t.Errorf("FirstSuffixIfIn() = %q, want %q", suffix, "")
	}
}

func TestSnowballWord_FirstSuffixIn_PartialRegion(t *testing.T) {
	w := New("running")
	// region [2,5) => w.RS[2:5] = "nni" (indices 2='n', 3='n', 4='i')
	// "n" is NOT a suffix of "nni" (last char is 'i'). "i" IS a suffix.
	suffix, _ := w.FirstSuffixIn(2, 5, "n", "i")
	if suffix != "i" {
		t.Errorf("FirstSuffixIn() = %q, want %q", suffix, "i")
	}

	// region [2,5) => "nni". "i" comes first in the list.
	suffix, _ = w.FirstSuffixIn(2, 5, "i", "n")
	if suffix != "i" {
		t.Errorf("FirstSuffixIn() = %q, want %q", suffix, "i")
	}
}

func TestSnowballWord_FirstPrefix_EmptyPrefix(t *testing.T) {
	w := New("abc")
	found, foundRunes := w.FirstPrefix("", "a")
	// Empty prefix always matches at position 0
	if found != "" {
		t.Errorf("FirstPrefix() = %q, want %q", found, "")
	}
	if len(foundRunes) != 0 {
		t.Errorf("foundRunes length = %d, want 0", len(foundRunes))
	}
}

func TestSnowballWord_RemoveLastNRunes_Zero(t *testing.T) {
	w := New("abc")
	w.R1start = 1
	w.RemoveLastNRunes(0)
	if got := w.String(); got != "abc" {
		t.Errorf("String() = %q, want %q", got, "abc")
	}
	if w.R1start != 1 {
		t.Errorf("R1start = %d, want %d", w.R1start, 1)
	}
}

func TestSnowballWord_ReplaceSuffixRunes_EmptySuffix(t *testing.T) {
	w := New("cat")
	w.R1start = 1
	w.ReplaceSuffixRunes([]rune(""), []rune("s"), true)
	if got := w.String(); got != "cats" {
		t.Errorf("String() = %q, want %q", got, "cats")
	}
}
