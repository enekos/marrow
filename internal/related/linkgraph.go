package related

import (
	"bytes"
	"math"
	"regexp"
	"strings"
)


// splitFrontMatter returns (frontmatter, body). Frontmatter is the YAML
// between two `---` fences; body is the rest. If there is no frontmatter
// the returned frontmatter slice is empty.
func splitFrontMatter(data []byte) (fm, body []byte) {
	trim := bytes.TrimLeft(data, " \t\r\n")
	if !bytes.HasPrefix(trim, []byte("---")) {
		return nil, data
	}
	rest := trim[3:]
	// Advance past the optional trailing newline so our closing fence match
	// stays on a fresh line.
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}
	closeIdx := bytes.Index(rest, []byte("\n---"))
	if closeIdx < 0 {
		return nil, data
	}
	fm = rest[:closeIdx]
	body = rest[closeIdx+len("\n---"):]
	// Trim the newline immediately after the closing fence.
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
		body = body[2:]
	}
	return fm, body
}

// extractTaxonomyTerms pulls discrete taxonomy terms out of Hugo front matter
// for one or more configured field names. Each field value may be either:
//   - a list of strings (e.g. Hugo's `categories: [politika, ekonomia]` or
//     preprocessed `tag_pairs: ["es|padre", "kat|izen"]`)
//   - a list of maps with `slug`, `value`, or `name` keys (e.g.
//     `categories_meta: [{slug: politika, name: Politika}]` or the raw
//     `tags: [{name: es, value: padre}]` shape — in which case the term is
//     rendered as "name|value" so it matches the preprocessed `tag_pairs`).
//
// Missing fields are silently ignored, making the same signal reusable
// across content types with different front-matter shapes.
func extractTaxonomyTerms(meta map[string]any, fields []string) []string {
	if meta == nil || len(fields) == 0 {
		return nil
	}
	set := map[string]struct{}{}
	for _, field := range fields {
		v, ok := meta[field]
		if !ok {
			continue
		}
		lst, ok := v.([]any)
		if !ok {
			continue
		}
		for _, item := range lst {
			switch it := item.(type) {
			case string:
				if it != "" {
					set[it] = struct{}{}
				}
			case map[string]any:
				if s, ok := it["slug"].(string); ok && s != "" {
					set[s] = struct{}{}
					continue
				}
				name, _ := it["name"].(string)
				value, _ := it["value"].(string)
				switch {
				case name != "" && value != "":
					set[name+"|"+value] = struct{}{}
				case value != "":
					set[value] = struct{}{}
				case name != "":
					set[name] = struct{}{}
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	return out
}

// taxonomyOverlap returns the size of the intersection of two taxonomy-term
// lists normalised by the smaller set's size.
func taxonomyOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	aset := make(map[string]struct{}, len(a))
	for _, s := range a {
		aset[s] = struct{}{}
	}
	inter := 0
	for _, s := range b {
		if _, ok := aset[s]; ok {
			inter++
		}
	}
	if inter == 0 {
		return 0
	}
	denom := len(a)
	if len(b) < denom {
		denom = len(b)
	}
	return float64(inter) / float64(denom)
}

// linkRe matches markdown inline links of the form [text](target). We pull
// out the target for the caller to parse.
var linkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)`)

// extractInternalLinkSlugs returns the set of internal-article slugs linked
// from the given markdown body. We accept both `/slug/` and
// `/kategoria/slug/` shapes, and always take the last non-empty path
// segment as the candidate slug — callers resolve it against the slug map
// and silently drop unknown slugs.
func extractInternalLinkSlugs(body []byte) []string {
	matches := linkRe.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		target := string(m[1])
		if target == "" {
			continue
		}
		low := strings.ToLower(target)
		if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") ||
			strings.HasPrefix(low, "mailto:") || strings.HasPrefix(target, "#") {
			continue
		}
		// Strip query + anchor.
		if i := strings.IndexAny(target, "#?"); i >= 0 {
			target = target[:i]
		}
		// Normalise relative paths like "foo-bar" (no leading slash) to "/foo-bar".
		if !strings.HasPrefix(target, "/") {
			target = "/" + target
		}
		// Trim trailing slash.
		trimmed := strings.TrimSuffix(target, "/")
		if trimmed == "" {
			continue
		}
		last := trimmed
		if i := strings.LastIndex(trimmed, "/"); i >= 0 {
			last = trimmed[i+1:]
		}
		// `/kategoria/foo` (category pages) uses the final slug segment which
		// won't match a document slug, so callers filter unknowns anyway.
		if last == "" {
			continue
		}
		if _, ok := seen[last]; ok {
			continue
		}
		seen[last] = struct{}{}
		out = append(out, last)
	}
	return out
}

// linkScore returns a [0,1] score for the linkage strength between two
// documents, combining direct linkage (either direction) and co-citation.
//
//	direct outgoing:  src -> dst      => 1.0
//	direct incoming:  dst -> src      => 0.7
//	co-cited (share ancestor / descendant): up to 0.4 scaled by overlap.
func (b *Builder) linkScore(src, dst *docRecord) float64 {
	var best float64
	if out, ok := b.linksFw[src.id]; ok {
		if _, hit := out[dst.id]; hit {
			best = 1.0
		}
	}
	if in, ok := b.linksBw[src.id]; ok {
		if _, hit := in[dst.id]; hit {
			if 0.7 > best {
				best = 0.7
			}
		}
	}
	if best == 1.0 {
		return best
	}

	// Co-citation via shared out-neighbour or shared in-neighbour sets.
	srcOut := b.linksFw[src.id]
	dstOut := b.linksFw[dst.id]
	srcIn := b.linksBw[src.id]
	dstIn := b.linksBw[dst.id]

	co := coOverlap(srcOut, dstOut) // both point to the same thing
	bi := coOverlap(srcIn, dstIn)   // both pointed at by the same thing
	combined := 0.4 * math.Max(co, bi)
	if combined > best {
		best = combined
	}
	return best
}

func coOverlap(a, b map[int64]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	small, large := a, b
	if len(b) < len(a) {
		small, large = b, a
	}
	inter := 0
	for k := range small {
		if _, ok := large[k]; ok {
			inter++
		}
	}
	if inter == 0 {
		return 0
	}
	// Jaccard.
	union := len(a) + len(b) - inter
	return float64(inter) / float64(union)
}
