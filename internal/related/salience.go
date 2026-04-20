package related

import (
	"math"
	"sort"
	"strings"
)

// computeSalience derives a per-document set of "salient" stemmed terms
// using TF-IDF. Top cfg.TopKSalient terms are retained per document and
// used for the lex-overlap signal.
func (b *Builder) computeSalience() {
	if len(b.docs) == 0 {
		return
	}

	// Document frequency per stem.
	df := make(map[string]int, 8192)
	for _, d := range b.docs {
		seen := make(map[string]struct{}, len(d.stems))
		for _, s := range d.stems {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			df[s]++
		}
	}
	N := float64(len(b.docs))

	for _, d := range b.docs {
		if len(d.stems) == 0 {
			d.salientTerms = map[string]struct{}{}
			continue
		}
		tf := make(map[string]int, len(d.stems))
		for _, s := range d.stems {
			tf[s]++
		}
		type scored struct {
			term  string
			value float64
		}
		scores := make([]scored, 0, len(tf))
		for term, f := range tf {
			// Skip ultra-short stems (usually noise) and terms that appear
			// in nearly every document (degenerate IDF).
			if len(term) < 3 {
				continue
			}
			n := float64(df[term])
			if n >= N*0.5 {
				continue
			}
			idf := math.Log((N + 1.0) / (n + 1.0))
			// Sub-linear TF damps repeated-word spam.
			tfn := 1.0 + math.Log(float64(f))
			scores = append(scores, scored{term: term, value: tfn * idf})
		}
		sort.Slice(scores, func(i, j int) bool { return scores[i].value > scores[j].value })
		k := b.cfg.TopKSalient
		if k > len(scores) {
			k = len(scores)
		}
		set := make(map[string]struct{}, k)
		for i := 0; i < k; i++ {
			set[scores[i].term] = struct{}{}
		}
		d.salientTerms = set
	}
}

// uniqueFields returns the fields of s deduped, preserving first occurrence.
func uniqueFields(s string) []string {
	parts := strings.Fields(s)
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// jaccardSalient computes Jaccard overlap of two salient-term sets.
func jaccardSalient(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	small, large := a, b
	if len(b) < len(a) {
		small, large = b, a
	}
	for t := range small {
		if _, ok := large[t]; ok {
			inter++
		}
	}
	if inter == 0 {
		return 0
	}
	union := len(a) + len(b) - inter
	return float64(inter) / float64(union)
}

// titleOverlap returns the Jaccard overlap between two title-stem sets.
func titleOverlap(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for t := range a {
		if _, ok := b[t]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
