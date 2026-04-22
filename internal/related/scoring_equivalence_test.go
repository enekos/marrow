package related

import (
	"log/slog"
	"sort"
	"testing"
)

// TestScoreAll_PostingsMatchAllPairs verifies that the inverted-index path
// (scoreViaPostings) produces the same candidate set and scores as the
// exhaustive scoreAllPairs path in IgnoreSemantic mode. Any divergence is
// a correctness bug in the posting enumeration.
func TestScoreAll_PostingsMatchAllPairs(t *testing.T) {
	// Build a corpus with mixed signal origins: some pairs share only a tag,
	// some only a salient term, some only links, some co-cite, some overlap.
	docs := []*docRecord{
		{id: 1, path: "a.md", taxonomy: []string{"x", "y"},
			salientTerms: setOf("alpha", "beta"), titleStems: setOf("a")},
		{id: 2, path: "b.md", taxonomy: []string{"y"},
			salientTerms: setOf("beta", "gamma"), titleStems: setOf("b")},
		{id: 3, path: "c.md", taxonomy: []string{"z"},
			salientTerms: setOf("gamma"), titleStems: setOf("c")},
		{id: 4, path: "d.md", taxonomy: []string{"x"},
			salientTerms: setOf("delta"), titleStems: setOf("d")},
		{id: 5, path: "e.md", taxonomy: []string{"w"}, // only reachable via links
			salientTerms: setOf("epsilon"), titleStems: setOf("e")},
		{id: 6, path: "f.md", taxonomy: []string{"w"}, // co-cited with src via shared outbound
			salientTerms: setOf("zeta"), titleStems: setOf("f")},
		{id: 7, path: "g.md", taxonomy: []string{"unrelated"}, // should be unreachable → score 0
			salientTerms: setOf("nothing"), titleStems: setOf("g")},
	}
	// src is docs[0] with id=1. Link graph: 1 -> 5, 6 -> 5 (so 1 and 6 co-cite 5).
	b := NewBuilder(Config{
		Limit: 10, WSem: 0, WLex: 0.2, WLink: 0.15, WCat: 0.1,
		IgnoreSemantic: true, CandidatePoolMultiplier: 6, MMRLambda: 1.0,
	}, slog.Default())
	b.docs = docs
	b.byPath = map[string]*docRecord{}
	b.byID = map[int64]int{}
	for i, d := range docs {
		b.byPath[d.path] = d
		b.byID[d.id] = i
	}
	b.linksFw = map[int64]map[int64]struct{}{
		1: {5: {}},
		6: {5: {}},
	}
	b.linksBw = map[int64]map[int64]struct{}{
		5: {1: {}, 6: {}},
	}
	b.computeTagIDF()
	b.buildInvertedIndex()

	src := docs[0]
	scratch := newScoreScratch(len(docs))

	got := b.scoreViaPostings(src, scratch)
	want := b.scoreAllPairs(src)

	normalize := func(cs []candidate) map[int64]candidate {
		m := make(map[int64]candidate, len(cs))
		for _, c := range cs {
			m[c.doc.id] = c
		}
		return m
	}
	gm, wm := normalize(got), normalize(want)

	if len(gm) != len(wm) {
		t.Errorf("candidate count mismatch: postings=%d pairs=%d", len(gm), len(wm))
	}
	allIDs := map[int64]struct{}{}
	for k := range gm {
		allIDs[k] = struct{}{}
	}
	for k := range wm {
		allIDs[k] = struct{}{}
	}
	var ids []int64
	for k := range allIDs {
		ids = append(ids, k)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		g, gok := gm[id]
		w, wok := wm[id]
		switch {
		case gok && !wok:
			t.Errorf("doc %d: in postings (score=%.4f) but not in pairs", id, g.score)
		case wok && !gok:
			t.Errorf("doc %d: in pairs (score=%.4f) but not in postings", id, w.score)
		case g.score != w.score:
			t.Errorf("doc %d: score mismatch postings=%.6f pairs=%.6f (sem=%v/%v lex=%v/%v link=%v/%v cat=%v/%v)",
				id, g.score, w.score, g.sem, w.sem, g.lex, w.lex, g.link, w.link, g.cat, w.cat)
		}
	}
}

func setOf(terms ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(terms))
	for _, t := range terms {
		m[t] = struct{}{}
	}
	return m
}
