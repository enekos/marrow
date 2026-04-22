// Package tokenizer implements the BERT WordPiece tokenizer in pure Go.
//
// It targets bert-base-uncased and derivatives (including
// sentence-transformers/all-MiniLM-L6-v2). The goal is byte-for-byte parity
// with the reference HuggingFace `BertTokenizer` for English text. CJK and
// other non-Latin scripts follow the same rules the reference applies.
package tokenizer

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	PadID  = 0
	UnkID  = 100
	CLSID  = 101
	SEPID  = 102
	MaskID = 103

	PadToken  = "[PAD]"
	UnkToken  = "[UNK]"
	CLSToken  = "[CLS]"
	SEPToken  = "[SEP]"
	MaskToken = "[MASK]"

	MaxInputCharsPerWord = 100
)

// Tokenizer is a BERT-style WordPiece tokenizer. Safe for concurrent use
// after construction — no internal state is mutated after Load.
type Tokenizer struct {
	vocab    map[string]int
	doLower  bool
	unkID    int
	clsID    int
	sepID    int
	padID    int
	maxInput int // max model input length including [CLS] and [SEP]
}

// Load reads a vocab file (one token per line; line number is the token id)
// and returns a Tokenizer configured for lowercase BERT.
func Load(vocabPath string) (*Tokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("open vocab: %w", err)
	}
	defer f.Close()

	vocab := make(map[string]int, 30522)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	i := 0
	for sc.Scan() {
		tok := sc.Text()
		vocab[tok] = i
		i++
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read vocab: %w", err)
	}
	if len(vocab) == 0 {
		return nil, fmt.Errorf("empty vocab file: %s", vocabPath)
	}

	t := &Tokenizer{
		vocab:    vocab,
		doLower:  true,
		unkID:    idOr(vocab, UnkToken, UnkID),
		clsID:    idOr(vocab, CLSToken, CLSID),
		sepID:    idOr(vocab, SEPToken, SEPID),
		padID:    idOr(vocab, PadToken, PadID),
		// BERT's architectural cap. Individual sentence-transformer models
		// truncate tighter — e.g. all-MiniLM-L6-v2 sets max_seq_length=256
		// in sentence_bert_config.json. Callers load that file and call
		// SetMaxInput when present; otherwise we default to 512.
		maxInput: 512,
	}
	return t, nil
}

func idOr(v map[string]int, tok string, fallback int) int {
	if id, ok := v[tok]; ok {
		return id
	}
	return fallback
}

// VocabSize returns the number of entries in the vocab.
func (t *Tokenizer) VocabSize() int { return len(t.vocab) }

// SetMaxInput overrides the maximum model input length (including [CLS] and
// [SEP]). Used when loading configuration files like sentence_bert_config.json.
// Values ≤ 2 are ignored so the tokenizer always has room for both special
// tokens. Values above BERT's 512 position-embedding limit are clamped.
func (t *Tokenizer) SetMaxInput(n int) {
	if n <= 2 {
		return
	}
	if n > 512 {
		n = 512
	}
	t.maxInput = n
}

// Encoded is the output of Encode, suitable to feed directly to the model.
type Encoded struct {
	IDs          []int32
	AttentionMask []int32
	TypeIDs      []int32 // all zeros for single-segment input
}

// Encode tokenizes a single string and produces model-ready tensors.
// Adds [CLS] at the start and [SEP] at the end; truncates the interior to
// fit within maxInput. AttentionMask is 1 for real tokens, 0 for pads —
// but Encode does not pad: callers that want a fixed length must pad
// themselves. For our encoder, we pack variable-length inputs one at a time
// and attention masking covers only the real tokens, which is all we need.
func (t *Tokenizer) Encode(text string) Encoded {
	pieces := t.tokenize(text)

	// Truncate to leave room for [CLS] and [SEP].
	maxBody := t.maxInput - 2
	if len(pieces) > maxBody {
		pieces = pieces[:maxBody]
	}

	ids := make([]int32, 0, len(pieces)+2)
	ids = append(ids, int32(t.clsID))
	for _, p := range pieces {
		if id, ok := t.vocab[p]; ok {
			ids = append(ids, int32(id))
		} else {
			ids = append(ids, int32(t.unkID))
		}
	}
	ids = append(ids, int32(t.sepID))

	mask := make([]int32, len(ids))
	types := make([]int32, len(ids))
	for i := range mask {
		mask[i] = 1
	}
	return Encoded{IDs: ids, AttentionMask: mask, TypeIDs: types}
}

// tokenize runs basic tokenization then WordPiece over each basic token.
func (t *Tokenizer) tokenize(text string) []string {
	basic := t.basicTokenize(text)
	out := make([]string, 0, len(basic)*2)
	for _, w := range basic {
		out = append(out, t.wordpiece(w)...)
	}
	return out
}

// basicTokenize implements BertBasicTokenizer: clean, CJK pad, whitespace
// split, then for each token: lowercase, strip accents, split on punctuation.
func (t *Tokenizer) basicTokenize(text string) []string {
	text = cleanText(text)
	text = padCJK(text)

	var out []string
	for _, tok := range strings.Fields(text) {
		if t.doLower {
			tok = strings.ToLower(tok)
			tok = stripAccents(tok)
		}
		out = append(out, splitOnPunct(tok)...)
	}
	return out
}

// cleanText removes invalid/control characters and normalizes whitespace.
func cleanText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == 0 || r == 0xFFFD || isControl(r) {
			continue
		}
		if isWhitespace(r) {
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// padCJK surrounds CJK characters with spaces so they become their own tokens.
func padCJK(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		if isCJK(r) {
			b.WriteByte(' ')
			b.WriteRune(r)
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// stripAccents applies NFD then drops combining marks.
func stripAccents(s string) string {
	decomposed := norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// splitOnPunct splits a token on punctuation runs; each punct char becomes
// its own token. "don't" -> ["don", "'", "t"].
func splitOnPunct(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	var cur strings.Builder
	for _, r := range s {
		if isPunct(r) {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			out = append(out, string(r))
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// wordpiece applies greedy-longest-match WordPiece to a single basic token.
func (t *Tokenizer) wordpiece(word string) []string {
	if len(word) == 0 {
		return nil
	}
	// Length in bytes — WordPiece operates on Unicode chars, but the BERT
	// tokenizer counts codepoints for the max-char cutoff and byte-walks
	// for substring lookup. We convert to a rune slice for codepoint work.
	runes := []rune(word)
	if len(runes) > MaxInputCharsPerWord {
		return []string{UnkToken}
	}

	var out []string
	start := 0
	for start < len(runes) {
		end := len(runes)
		var cur string
		matched := false
		for end > start {
			sub := string(runes[start:end])
			if start > 0 {
				sub = "##" + sub
			}
			if _, ok := t.vocab[sub]; ok {
				cur = sub
				matched = true
				break
			}
			end--
		}
		if !matched {
			return []string{UnkToken}
		}
		out = append(out, cur)
		start = end
	}
	return out
}

// --- Unicode classification helpers (mirroring BertBasicTokenizer) ---

func isWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r':
		return true
	}
	// Reference also treats any unicode Zs as whitespace.
	return unicode.Is(unicode.Zs, r)
}

func isControl(r rune) bool {
	switch r {
	case '\t', '\n', '\r':
		return false
	}
	if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
		return true
	}
	return false
}

// isPunct matches BERT's punctuation rule: any ASCII punct-ish byte in
// !"#$%&'()*+,-./:;<=>?@[\]^_`{|}~ or any Unicode P* category.
func isPunct(r rune) bool {
	if (r >= 33 && r <= 47) || (r >= 58 && r <= 64) ||
		(r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
		return true
	}
	return unicode.In(r, unicode.P)
}

// isCJK mirrors the BERT `_is_chinese_char` ranges.
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2B73F) ||
		(r >= 0x2B740 && r <= 0x2B81F) ||
		(r >= 0x2B820 && r <= 0x2CEAF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x2F800 && r <= 0x2FA1F)
}
