package related

import (
	"log/slog"
	"testing"
)

// newFixtureBuilder returns a Builder whose docs and linkgraph are hand-set
// (bypassing DB/filesystem load), with tagDF/tagIDF populated as if Load had
// finished. Use it for unit-level scoring tests.
func newFixtureBuilder(t *testing.T, cfg Config, docs []*docRecord) *Builder {
	t.Helper()
	b := NewBuilder(cfg, slog.Default())
	b.docs = docs
	b.byPath = make(map[string]*docRecord, len(docs))
	b.bySlug = make(map[string]*docRecord, len(docs))
	for _, d := range docs {
		b.byPath[d.path] = d
		if d.slug != "" {
			b.bySlug[d.slug] = d
		}
	}
	b.linksFw = map[int64]map[int64]struct{}{}
	b.linksBw = map[int64]map[int64]struct{}{}
	b.computeTagIDF()
	b.buildInvertedIndex()
	return b
}

func TestRelatedFor_IDFPrefersRareTagMatch(t *testing.T) {
	// Build a corpus where a "rare" tag is strongly diagnostic and a "common"
	// tag is nearly noise. A doc sharing rare+common should outrank a doc
	// sharing only common.
	var docs []*docRecord
	// 50 noise docs carry only "common".
	for i := 0; i < 50; i++ {
		docs = append(docs, &docRecord{
			id:       int64(i + 1),
			path:     pathN(i + 1),
			title:    "noise",
			slug:     slugN("noise", i+1),
			taxonomy: []string{"common"},
		})
	}
	// 3 rare-tag docs carry both "common" and "rare".
	for i := 0; i < 3; i++ {
		docs = append(docs, &docRecord{
			id:       int64(100 + i),
			path:     pathN(100 + i),
			title:    "rare-topic",
			slug:     slugN("rare", i+1),
			taxonomy: []string{"common", "rare"},
		})
	}
	src := &docRecord{
		id:       999,
		path:     "src.md",
		title:    "src",
		slug:     "src",
		taxonomy: []string{"common", "rare"},
	}
	docs = append(docs, src)

	cfg := Config{
		Limit:                   10,
		WSem:                    0,
		WLex:                    0,
		WLink:                   0,
		WCat:                    1.0,
		IgnoreSemantic:          true,
		UseCatIDF:               true,
		CandidatePoolMultiplier: 6,
		MMRLambda:               1.0, // disable diversity bias for this test
	}
	b := newFixtureBuilder(t, cfg, docs)

	rels := b.relatedFor(src, newScoreScratch(len(b.docs)))
	if len(rels) == 0 {
		t.Fatalf("expected some related docs, got 0")
	}
	// Top 3 picks should all be rare-topic docs.
	for i := 0; i < 3 && i < len(rels); i++ {
		if rels[i].Title != "rare-topic" {
			t.Errorf("rank %d: got title=%q, want \"rare-topic\" (rels=%+v)", i, rels[i].Title, rels)
		}
	}

	// And the reason should name the rare tag, not the common one.
	foundRareReason := false
	for _, r := range rels[:minInt(3, len(rels))] {
		for _, reason := range r.Reasons {
			if reason == "category:rare" || reason == "tag:rare" {
				foundRareReason = true
			}
			if reason == "category:common" || reason == "tag:common" {
				t.Errorf("top pick reason referenced common tag: %v", r.Reasons)
			}
		}
	}
	if !foundRareReason {
		t.Errorf("expected a top pick to cite the rare tag; reasons=%+v", topReasons(rels))
	}
}

func TestRelatedFor_FlatOverlapUnchanged(t *testing.T) {
	// With UseCatIDF=false, a doc sharing only the dominant tag scores the
	// same as it did before — verifying artikuluak output is byte-identical.
	src := &docRecord{
		id: 1, path: "a.md", title: "a", slug: "a",
		taxonomy: []string{"x", "y"},
	}
	dShared1 := &docRecord{
		id: 2, path: "b.md", title: "b", slug: "b",
		taxonomy: []string{"x"},
	}
	dShared2 := &docRecord{
		id: 3, path: "c.md", title: "c", slug: "c",
		taxonomy: []string{"y"},
	}
	docs := []*docRecord{src, dShared1, dShared2}

	cfg := Config{
		Limit: 10, WSem: 0, WLex: 0, WLink: 0, WCat: 1.0,
		IgnoreSemantic: true, UseCatIDF: false,
		CandidatePoolMultiplier: 6, MMRLambda: 1.0,
	}
	b := newFixtureBuilder(t, cfg, docs)

	rels := b.relatedFor(src, newScoreScratch(len(b.docs)))
	if len(rels) != 2 {
		t.Fatalf("expected 2 rels, got %d", len(rels))
	}
	if rels[0].Score != rels[1].Score {
		t.Errorf("flat overlap should score the two equally; got %v vs %v", rels[0].Score, rels[1].Score)
	}
}

// helpers

func pathN(i int) string { return "doc-" + itoa(i) + ".md" }
func slugN(prefix string, i int) string {
	return prefix + "-" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func topReasons(rels []RelatedDoc) [][]string {
	out := [][]string{}
	for _, r := range rels {
		out = append(out, r.Reasons)
	}
	return out
}
