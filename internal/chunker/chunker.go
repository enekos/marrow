// Package chunker splits document text into bounded paragraph-aware chunks
// for per-chunk embedding. Embedding a long document as a single vector
// dilutes local signal; splitting into smaller chunks lets similarity search
// pinpoint the relevant passage.
package chunker

import "strings"

// DefaultMaxChars is the target chunk size. Roughly 1500 characters is
// ~250-400 tokens for most embedding models — a commonly-tuned window for
// retrieval that preserves enough context to disambiguate a passage.
const DefaultMaxChars = 1500

// Chunk splits text into paragraph-bounded chunks of at most maxChars.
// Paragraphs are detected via blank-line boundaries. A paragraph larger than
// maxChars is hard-wrapped at the nearest whitespace to avoid mid-word splits.
// Returns nil for empty input.
func Chunk(text string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = DefaultMaxChars
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= maxChars {
		return []string{text}
	}

	var chunks []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}

	for _, p := range strings.Split(text, "\n\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if len(p) > maxChars {
			flush()
			chunks = append(chunks, splitOversized(p, maxChars)...)
			continue
		}

		if cur.Len()+len(p)+2 > maxChars {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
	}
	flush()
	return chunks
}

// splitOversized breaks a paragraph longer than maxChars at whitespace. It
// walks back from the maxChars boundary to the nearest space so words stay
// intact; if no space is found in the top half we fall back to a hard cut.
func splitOversized(p string, maxChars int) []string {
	var out []string
	for len(p) > maxChars {
		cut := maxChars
		for cut > maxChars/2 && p[cut] != ' ' && p[cut] != '\n' && p[cut] != '\t' {
			cut--
		}
		if cut <= maxChars/2 {
			cut = maxChars
		}
		out = append(out, strings.TrimSpace(p[:cut]))
		p = strings.TrimSpace(p[cut:])
	}
	if p != "" {
		out = append(out, p)
	}
	return out
}
