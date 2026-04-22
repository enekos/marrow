package related

import (
	"hash/fnv"
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
		// Deterministic tiebreak by term so two runs on the same corpus pick
		// the same TopKSalient terms when TF-IDF values collide.
		sort.Slice(scores, func(i, j int) bool {
			if scores[i].value != scores[j].value {
				return scores[i].value > scores[j].value
			}
			return scores[i].term < scores[j].term
		})
		k := b.cfg.TopKSalient
		if k > len(scores) {
			k = len(scores)
		}
		set := make(map[string]struct{}, k)
		hashes := make([]uint32, 0, k)
		for i := 0; i < k; i++ {
			set[scores[i].term] = struct{}{}
			hashes = append(hashes, fnvHash32(scores[i].term))
		}
		sort.Slice(hashes, func(i, j int) bool { return hashes[i] < hashes[j] })
		d.salientTerms = set
		d.salientHashes = hashes
	}
}

// fnvHash32 returns a 32-bit FNV-1a hash of s. Used to encode salient terms
// as integers for fast O(n+m) Jaccard intersection on sorted slices.
func fnvHash32(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// buildInvertedIndex populates termPostings and tagPostings. Each posting is
// a sorted list of doc indices (into b.docs) that contain the term/tag.
// Used by scoreAll to enumerate candidates without scanning the whole corpus.
func (b *Builder) buildInvertedIndex() {
	b.termPostings = make(map[string][]int32, 4096)
	b.tagPostings = make(map[string][]int32, 512)
	if b.byID == nil {
		b.byID = make(map[int64]int, len(b.docs))
	}
	for i, d := range b.docs {
		idx := int32(i)
		if _, ok := b.byID[d.id]; !ok {
			b.byID[d.id] = i
		}
		for t := range d.salientTerms {
			b.termPostings[t] = append(b.termPostings[t], idx)
		}
		for _, t := range d.taxonomy {
			b.tagPostings[t] = append(b.tagPostings[t], idx)
		}
		if len(d.taxonomy) > 0 {
			h := make([]uint32, len(d.taxonomy))
			for j, t := range d.taxonomy {
				h[j] = fnvHash32(t)
			}
			sort.Slice(h, func(i, j int) bool { return h[i] < h[j] })
			d.taxonomyHashes = h
		}
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

// jaccardSalient computes Jaccard overlap of two salient-term sets. When
// both sides carry precomputed sorted hashes we walk them in O(n+m) with
// integer compares. Falls back to the string-map implementation for tests
// that construct docs by hand without precomputed hashes.
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

// jaccardHashes is the hot-path form used by scorePair: two sorted uint32
// slices, linear merge. Collisions would slightly over-count intersection
// but for 32-bit FNV with ~24 terms per doc the probability is negligible.
func jaccardHashes(a, b []uint32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var i, j, inter int
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			inter++
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
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
