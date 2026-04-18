package chunker

import (
	"strings"
	"testing"
)

func TestChunk_EmptyReturnsNil(t *testing.T) {
	if got := Chunk("", 100); got != nil {
		t.Fatalf("empty input → %v, want nil", got)
	}
	if got := Chunk("   \n\n  ", 100); got != nil {
		t.Fatalf("whitespace-only → %v, want nil", got)
	}
}

func TestChunk_FitsInSingleChunk(t *testing.T) {
	chunks := Chunk("hello world", 100)
	if len(chunks) != 1 || chunks[0] != "hello world" {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
}

func TestChunk_ParagraphBoundaries(t *testing.T) {
	text := "para one\n\npara two\n\npara three"
	// maxChars small enough to force split between paragraphs.
	chunks := Chunk(text, 12)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
	for i, want := range []string{"para one", "para two", "para three"} {
		if chunks[i] != want {
			t.Errorf("chunk %d = %q, want %q", i, chunks[i], want)
		}
	}
}

func TestChunk_MergesSmallParagraphs(t *testing.T) {
	text := "a\n\nb\n\nc\n\nd"
	chunks := Chunk(text, 100)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 merged chunk, got %d: %v", len(chunks), chunks)
	}
	if !strings.Contains(chunks[0], "a") || !strings.Contains(chunks[0], "d") {
		t.Errorf("merged chunk missing content: %q", chunks[0])
	}
}

func TestChunk_SplitsOversizedParagraph(t *testing.T) {
	words := strings.Repeat("word ", 200)
	chunks := Chunk(strings.TrimSpace(words), 100)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > 120 {
			t.Errorf("chunk %d is %d chars, exceeds bound + slack", i, len(c))
		}
	}
}

func TestChunk_NoMidWordSplits(t *testing.T) {
	// Single long paragraph of 8-char words separated by spaces.
	words := strings.Repeat("friendly ", 40)
	chunks := Chunk(strings.TrimSpace(words), 50)
	for i, c := range chunks {
		if strings.Contains(c, "frien ") || strings.HasSuffix(c, "frien") {
			t.Errorf("chunk %d split mid-word: %q", i, c)
		}
	}
}

func TestChunk_AllChunksNonEmpty(t *testing.T) {
	text := "first\n\n\n\nsecond\n\n\n\nthird"
	for _, c := range Chunk(text, 8) {
		if strings.TrimSpace(c) == "" {
			t.Errorf("got empty chunk in output")
		}
	}
}
