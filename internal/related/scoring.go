package related

import (
	"math"
	"sort"
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
func (b *Builder) relatedFor(src *docRecord) []RelatedDoc {
	pool := b.scoreAll(src)
	if len(pool) == 0 {
		return nil
	}

	// Pool cap.
	poolSize := b.cfg.Limit * b.cfg.CandidatePoolMultiplier
	if poolSize > len(pool) {
		poolSize = len(pool)
	}
	sort.Slice(pool, func(i, j int) bool { return pool[i].score > pool[j].score })
	pool = pool[:poolSize]

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

// scoreAll ranks every non-self doc against src.
func (b *Builder) scoreAll(src *docRecord) []candidate {
	out := make([]candidate, 0, len(b.docs)-1)
	for _, d := range b.docs {
		if d.id == src.id {
			continue
		}
		c := candidate{doc: d, reasonLabel: b.cfg.TaxonomyReasonLabel}

		if !b.cfg.IgnoreSemantic && len(src.vec) > 0 && len(d.vec) > 0 {
			c.sem = cosine(src.vec, d.vec)
			if c.sem < 0 {
				c.sem = 0
			}
		}
		c.lex = jaccardSalient(src.salientTerms, d.salientTerms)
		c.link = b.linkScore(src, d)
		if b.cfg.UseCatIDF {
			c.cat = taxonomyOverlapIDF(src.taxonomy, d.taxonomy, b.tagIDF, src.srcIDFSum)
		} else {
			c.cat = taxonomyOverlap(src.taxonomy, d.taxonomy)
		}
		c.titleOv = titleOverlap(src.titleStems, d.titleStems)

		score := b.cfg.WSem*c.sem +
			b.cfg.WLex*c.lex +
			b.cfg.WLink*c.link +
			b.cfg.WCat*c.cat

		// Trivial-title demotion: candidate shares most title stems with
		// source but lacks deeper topical signal. This is the exact failure
		// mode we saw with "txiki"-matching before.
		if c.titleOv >= 0.5 && c.sem < b.cfg.TrivialTitleThreshold && c.link == 0 && c.cat == 0 {
			score *= 0.35
		}

		// Direct link bypass: always float directly-linked docs to a
		// competitive score so at least one shows up.
		if c.link >= 0.999 {
			if score < 0.5 {
				score = 0.5
			}
		}

		c.score = score
		if c.score <= 0 {
			continue
		}
		out = append(out, c)
	}
	return out
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
// when available; otherwise falls back to salience Jaccard.
func similarity(a, b *docRecord, ignoreSemantic bool) float64 {
	if !ignoreSemantic && len(a.vec) > 0 && len(b.vec) > 0 {
		s := cosine(a.vec, b.vec)
		if s < 0 {
			s = 0
		}
		return s
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
