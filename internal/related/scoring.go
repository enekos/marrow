package related

import (
	"cmp"
	"math"
	"slices"
)

// candidate is one potential related doc with its decomposed signals.
type candidate struct {
	doc         *docRecord
	sem         float64
	lex         float64
	link        float64
	cat         float64
	titleOv     float64
	score       float64
	reasonLabel string
}

// relatedFor computes the related-doc list for one source document.
//
// Steps:
//  1. Score every other doc across the four signals (cheap thanks to
//     L2-normalised vectors and precomputed salience sets).
//  2. Collect a candidate pool (CandidatePoolMultiplier × Limit) by raw score.
//  3. Apply MMR: greedy pick that maximises λ·relevance − (1−λ)·maxSim to
//     already-picked candidates. The "sim" between candidates is the
//     semantic cosine (falling back to salience Jaccard if semantic is
//     disabled), giving genuine topical diversity.
//  4. Emit reasons per picked candidate.
func (b *Builder) relatedFor(src *docRecord, scratch *scoreScratch) []RelatedDoc {
	pool := b.scoreAll(src, scratch)
	if len(pool) == 0 {
		return nil
	}

	// Pool cap. Always sort (even when within cap) so MMR sees a
	// deterministic order — salient-term and link-graph maps iterate in
	// random order upstream so pool contents arrive shuffled, and MMR's
	// tie-breaker is "first encountered in pool".
	poolSize := b.cfg.Limit * b.cfg.CandidatePoolMultiplier
	if poolSize > len(pool) {
		poolSize = len(pool)
	}
	scratch.idxs = scratch.idxs[:0]
	for i := range pool {
		scratch.idxs = append(scratch.idxs, int32(i))
	}
	slices.SortFunc(scratch.idxs, func(a, b int32) int {
		ca, cb := &pool[a], &pool[b]
		if ca.score != cb.score {
			return cmp.Compare(cb.score, ca.score)
		}
		return cmp.Compare(ca.doc.id, cb.doc.id)
	})
	scratch.ranked = scratch.ranked[:0]
	for _, i := range scratch.idxs[:poolSize] {
		scratch.ranked = append(scratch.ranked, pool[i])
	}
	pool = scratch.ranked

	// MMR selection.
	picked := b.mmr(pool, b.cfg.Limit)

	// Convert to output shape.
	out := make([]RelatedDoc, 0, len(picked))
	for _, c := range picked {
		out = append(out, RelatedDoc{
			Path:    c.doc.path,
			Title:   c.doc.title,
			Slug:    c.doc.slug,
			Score:   roundScore(c.score),
			Reasons: b.buildReasons(src, c),
		})
	}
	return out
}

// scoreAll ranks non-self docs against src.
//
// When semantic is disabled (mock embeddings), only docs that share at least
// one salient term, taxonomy tag, or direct link with src can possibly score
// above zero — so we enumerate the candidate set from the inverted index and
// skip the full-corpus scan. When semantic is enabled, cosine similarity can
// produce signal for any pair, so we fall back to the O(n) scan.
//
// scratch is a per-worker reusable buffer to avoid allocating a visited map
// on every call. Caller is responsible for resetting it between sources.
func (b *Builder) scoreAll(src *docRecord, scratch *scoreScratch) []candidate {
	if b.cfg.IgnoreSemantic || len(src.vec) == 0 {
		return b.scoreViaPostings(src, scratch)
	}
	return b.scoreAllPairs(src)
}

// scoreScratch holds per-worker reusable buffers for candidate enumeration.
type scoreScratch struct {
	// seen is a densely-indexed "visited" marker keyed by doc index.
	// We use a generation counter to reset in O(1): a doc i is considered
	// visited in this pass iff seen[i] == gen.
	seen []uint32
	gen  uint32
	// pool is a reusable candidate slice; each source truncates it to 0
	// and appends into it, so we amortize the slice header allocation.
	pool []candidate
	// idxs holds sort indices into pool; reused per source to avoid
	// allocating a freshly sized index slice every call.
	idxs []int32
	// ranked is the rank-ordered, pool-capped candidate slice handed to
	// MMR, also reused to skip one allocation per source.
	ranked []candidate
}

func newScoreScratch(n int) *scoreScratch {
	return &scoreScratch{
		seen:   make([]uint32, n),
		pool:   make([]candidate, 0, 128),
		idxs:   make([]int32, 0, 128),
		ranked: make([]candidate, 0, 64),
	}
}

func (s *scoreScratch) reset() {
	s.gen++
	// Wrap: on overflow, zero the slice so stale markers don't look current.
	if s.gen == 0 {
		for i := range s.seen {
			s.seen[i] = 0
		}
		s.gen = 1
	}
	s.pool = s.pool[:0]
}

func (s *scoreScratch) mark(i int32) bool {
	if s.seen[i] == s.gen {
		return false
	}
	s.seen[i] = s.gen
	return true
}

// scoreViaPostings enumerates candidates via the inverted indexes and scores
// each. Called when semantic is disabled (no cosine signal) so unrelated docs
// are guaranteed to score zero and can be skipped wholesale.
func (b *Builder) scoreViaPostings(src *docRecord, scratch *scoreScratch) []candidate {
	scratch.reset()
	selfIdx, _ := b.byID[src.id]

	visit := func(idx int32) {
		if int(idx) == selfIdx {
			return
		}
		if !scratch.mark(idx) {
			return
		}
		d := b.docs[idx]
		c := b.scorePair(src, d)
		if c.score > 0 {
			scratch.pool = append(scratch.pool, c)
		}
	}

	// Salient-term overlap → lex signal > 0.
	for t := range src.salientTerms {
		for _, idx := range b.termPostings[t] {
			visit(idx)
		}
	}
	// Taxonomy overlap → cat signal > 0.
	for _, t := range src.taxonomy {
		for _, idx := range b.tagPostings[t] {
			visit(idx)
		}
	}
	// Direct outgoing links → linkScore == 1.
	if fw, ok := b.linksFw[src.id]; ok {
		for id := range fw {
			if j, ok := b.byID[id]; ok {
				visit(int32(j))
			}
		}
	}
	// Direct incoming links → linkScore == 0.7.
	if bw, ok := b.linksBw[src.id]; ok {
		for id := range bw {
			if j, ok := b.byID[id]; ok {
				visit(int32(j))
			}
		}
	}
	// Co-citation: docs that share an out-neighbor with src (both point at
	// the same doc). Enumerate via the destination's in-neighbors.
	if fw, ok := b.linksFw[src.id]; ok {
		for dst := range fw {
			for coSrc := range b.linksBw[dst] {
				if j, ok := b.byID[coSrc]; ok {
					visit(int32(j))
				}
			}
		}
	}
	// Co-citation (incoming): docs that are pointed to by the same doc as src.
	if bw, ok := b.linksBw[src.id]; ok {
		for srcOf := range bw {
			for sibling := range b.linksFw[srcOf] {
				if j, ok := b.byID[sibling]; ok {
					visit(int32(j))
				}
			}
		}
	}
	return scratch.pool
}

// scoreAllPairs is the original O(n) scoring loop, retained for the semantic
// path where cosine similarity can produce signal for any pair.
func (b *Builder) scoreAllPairs(src *docRecord) []candidate {
	out := make([]candidate, 0, len(b.docs)-1)
	for _, d := range b.docs {
		if d.id == src.id {
			continue
		}
		c := b.scorePair(src, d)
		if c.score > 0 {
			out = append(out, c)
		}
	}
	return out
}

// scorePair computes the decomposed signals and final score for one
// (src, d) pair. Factored out so both the posting-driven and all-pairs
// paths share identical scoring semantics.
func (b *Builder) scorePair(src, d *docRecord) candidate {
	c := candidate{doc: d, reasonLabel: b.cfg.TaxonomyReasonLabel}

	if !b.cfg.IgnoreSemantic && len(src.vec) > 0 && len(d.vec) > 0 {
		c.sem = cosine(src.vec, d.vec)
		if c.sem < 0 {
			c.sem = 0
		}
	}
	if src.salientHashes != nil && d.salientHashes != nil {
		c.lex = jaccardHashes(src.salientHashes, d.salientHashes)
	} else {
		c.lex = jaccardSalient(src.salientTerms, d.salientTerms)
	}
	c.link = b.linkScore(src, d)
	if b.cfg.UseCatIDF {
		c.cat = taxonomyOverlapIDF(src.taxonomy, d.taxonomy, b.tagIDF, src.srcIDFSum)
	} else if src.taxonomyHashes != nil && d.taxonomyHashes != nil {
		c.cat = taxonomyOverlapHashes(src.taxonomyHashes, d.taxonomyHashes)
	} else {
		c.cat = taxonomyOverlap(src.taxonomy, d.taxonomy)
	}
	c.titleOv = titleOverlap(src.titleStems, d.titleStems)

	score := b.cfg.WSem*c.sem +
		b.cfg.WLex*c.lex +
		b.cfg.WLink*c.link +
		b.cfg.WCat*c.cat

	// Trivial-title demotion: candidate shares most title stems with
	// source but lacks deeper topical signal.
	if c.titleOv >= 0.5 && c.sem < b.cfg.TrivialTitleThreshold && c.link == 0 && c.cat == 0 {
		score *= 0.35
	}

	// Direct link bypass.
	if c.link >= 0.999 && score < 0.5 {
		score = 0.5
	}
	c.score = score
	return c
}

// mmr runs a greedy Maximal Marginal Relevance pass over the scored pool.
func (b *Builder) mmr(pool []candidate, k int) []candidate {
	if k >= len(pool) {
		return pool
	}
	picked := make([]candidate, 0, k)
	chosen := make([]bool, len(pool))

	lambda := b.cfg.MMRLambda

	for len(picked) < k {
		bestIdx := -1
		bestVal := -math.MaxFloat64
		for i, c := range pool {
			if chosen[i] {
				continue
			}
			var maxSim float64
			for _, p := range picked {
				s := similarity(c.doc, p.doc, b.cfg.IgnoreSemantic)
				if s > maxSim {
					maxSim = s
				}
			}
			val := lambda*c.score - (1.0-lambda)*maxSim
			if val > bestVal {
				bestVal = val
				bestIdx = i
			}
		}
		if bestIdx < 0 {
			break
		}
		chosen[bestIdx] = true
		picked = append(picked, pool[bestIdx])
	}
	return picked
}

// similarity between two candidate docs for MMR diversity. Prefers semantic
// when available; otherwise falls back to salience Jaccard (hash-based when
// precomputed).
func similarity(a, b *docRecord, ignoreSemantic bool) float64 {
	if !ignoreSemantic && len(a.vec) > 0 && len(b.vec) > 0 {
		s := cosine(a.vec, b.vec)
		if s < 0 {
			s = 0
		}
		return s
	}
	if a.salientHashes != nil && b.salientHashes != nil {
		return jaccardHashes(a.salientHashes, b.salientHashes)
	}
	return jaccardSalient(a.salientTerms, b.salientTerms)
}

// buildReasons produces a compact, human-meaningful list of explanations
// for why a candidate was selected. When UseCatIDF is active, the taxonomy
// reason names the rarest (most informative) shared tag; otherwise it falls
// back to the first-shared tag for backward compatibility.
func (b *Builder) buildReasons(src *docRecord, c candidate) []string {
	var reasons []string
	if c.link >= 0.999 {
		reasons = append(reasons, "linked")
	} else if c.link >= 0.65 {
		reasons = append(reasons, "back-linked")
	} else if c.link > 0 {
		reasons = append(reasons, "co-cited")
	}
	if c.cat > 0 {
		label := "category"
		if c.reasonLabel != "" {
			label = c.reasonLabel
		}
		var term string
		if b.cfg.UseCatIDF {
			term = rarestShared(src.taxonomy, c.doc.taxonomy, b.tagDF)
		} else {
			term = firstShared(src.taxonomy, c.doc.taxonomy)
		}
		if term != "" {
			reasons = append(reasons, label+":"+term)
		} else {
			reasons = append(reasons, label)
		}
	}
	if c.sem >= 0.55 {
		reasons = append(reasons, "semantic")
	}
	if c.lex >= 0.12 {
		reasons = append(reasons, "shared-terms")
	}
	return reasons
}

func firstShared(a, b []string) string {
	aset := make(map[string]struct{}, len(a))
	for _, s := range a {
		aset[s] = struct{}{}
	}
	for _, s := range b {
		if _, ok := aset[s]; ok {
			return s
		}
	}
	return ""
}

// rarestShared returns the tag that is shared between a and b and has the
// lowest document frequency (highest IDF, i.e. most informative). Ties are
// broken lexicographically so the output is deterministic. Tags missing from
// df are treated as df=0 (maximally rare). Returns "" when the sets are
// disjoint.
func rarestShared(a, b []string, df map[string]int) string {
	aset := make(map[string]struct{}, len(a))
	for _, s := range a {
		aset[s] = struct{}{}
	}
	best := ""
	bestDF := 0
	haveBest := false
	for _, s := range b {
		if _, ok := aset[s]; !ok {
			continue
		}
		d := df[s] // missing -> 0
		if !haveBest || d < bestDF || (d == bestDF && s < best) {
			best = s
			bestDF = d
			haveBest = true
		}
	}
	return best
}

// cosine assumes both vectors are L2-normalised; returns dot product.
func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return float64(sum)
}

// l2Normalise normalises v in place.
func l2Normalise(v []float32) {
	var sq float64
	for _, x := range v {
		sq += float64(x) * float64(x)
	}
	if sq <= 0 {
		return
	}
	inv := float32(1.0 / math.Sqrt(sq))
	for i := range v {
		v[i] *= inv
	}
}

func roundScore(s float64) float64 {
	return math.Round(s*10000) / 10000
}
