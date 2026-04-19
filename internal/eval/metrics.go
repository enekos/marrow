package eval

import "math"

// ComputeMetrics calculates all IR metrics for a single query given the
// ranked result paths and the set of ground-truth relevant paths.
// cutoffs must be sorted in ascending order.
func ComputeMetrics(ranked []string, relevant []string, cutoffs []int) PerQueryResult {
	relSet := make(map[string]struct{}, len(relevant))
	for _, p := range relevant {
		relSet[p] = struct{}{}
	}

	res := PerQueryResult{
		Query:       "",
		Precision:   make(map[int]float64, len(cutoffs)),
		Recall:      make(map[int]float64, len(cutoffs)),
		NDCG:        make(map[int]float64, len(cutoffs)),
		F1:          make(map[int]float64, len(cutoffs)),
		RankedPaths: ranked,
	}

	// Build relevance vector: 1 if ranked[i] is relevant, else 0.
	relVec := make([]int, len(ranked))
	for i, p := range ranked {
		if _, ok := relSet[p]; ok {
			relVec[i] = 1
		}
	}

	// Precompute cumulative relevant counts and DCG components.
	cumulativeRel := make([]int, len(ranked))
	if len(ranked) > 0 {
		cumulativeRel[0] = relVec[0]
		for i := 1; i < len(ranked); i++ {
			cumulativeRel[i] = cumulativeRel[i-1] + relVec[i]
		}
	}

	// MRR and AP require scanning for first relevant and precision-at-rank.
	firstRelevantRank := 0
	precisionsAtRelevant := []float64{}
	for i, isRel := range relVec {
		if isRel == 1 {
			if firstRelevantRank == 0 {
				firstRelevantRank = i + 1
			}
			pAtI := float64(cumulativeRel[i]) / float64(i+1)
			precisionsAtRelevant = append(precisionsAtRelevant, pAtI)
		}
	}

	if firstRelevantRank > 0 {
		res.MRR = 1.0 / float64(firstRelevantRank)
	}

	if len(precisionsAtRelevant) > 0 {
		var sum float64
		for _, p := range precisionsAtRelevant {
			sum += p
		}
		res.AP = sum / float64(len(precisionsAtRelevant))
	}

	// Compute metrics at each cutoff.
	for _, k := range cutoffs {
		if k <= 0 {
			continue
		}
		topK := min(k, len(ranked))
		relInTopK := 0
		if topK > 0 {
			relInTopK = cumulativeRel[topK-1]
		}

		p := float64(relInTopK) / float64(k)
		r := 0.0
		if len(relevant) > 0 {
			r = float64(relInTopK) / float64(len(relevant))
		}
		res.Precision[k] = p
		res.Recall[k] = r
		res.F1[k] = f1(p, r)
		res.NDCG[k] = ndcgAtK(ranked, relSet, k)
	}

	return res
}

// AggregateReport computes mean metrics across all per-query results.
func AggregateReport(results []PerQueryResult, cutoffs []int) Report {
	report := Report{
		Queries:       results,
		MeanPrecision: make(map[int]float64, len(cutoffs)),
		MeanRecall:    make(map[int]float64, len(cutoffs)),
		MeanNDCG:      make(map[int]float64, len(cutoffs)),
		MeanF1:        make(map[int]float64, len(cutoffs)),
	}

	if len(results) == 0 {
		return report
	}

	var mrrSum, mapSum float64
	for _, r := range results {
		mrrSum += r.MRR
		mapSum += r.AP
	}
	report.MRR = mrrSum / float64(len(results))
	report.MAP = mapSum / float64(len(results))

	for _, k := range cutoffs {
		var pSum, rSum, nSum, fSum float64
		for _, res := range results {
			pSum += res.Precision[k]
			rSum += res.Recall[k]
			nSum += res.NDCG[k]
			fSum += res.F1[k]
		}
		n := float64(len(results))
		report.MeanPrecision[k] = pSum / n
		report.MeanRecall[k] = rSum / n
		report.MeanNDCG[k] = nSum / n
		report.MeanF1[k] = fSum / n
	}

	return report
}

// f1 computes the harmonic mean of precision and recall.
func f1(p, r float64) float64 {
	if p+r == 0 {
		return 0
	}
	return 2 * p * r / (p + r)
}

// ndcgAtK computes Normalized Discounted Cumulative Gain at cutoff k.
func ndcgAtK(ranked []string, relSet map[string]struct{}, k int) float64 {
	topK := min(k, len(ranked))
	if topK == 0 {
		return 0
	}

	dcg := 0.0
	for i := 0; i < topK; i++ {
		if _, ok := relSet[ranked[i]]; ok {
			dcg += 1.0 / math.Log2(float64(i)+2.0)
		}
	}

	// IDCG: ideal ranking has all relevant docs at the top.
	idealRelCount := len(relSet)
	if idealRelCount > k {
		idealRelCount = k
	}
	idcg := 0.0
	for i := 0; i < idealRelCount; i++ {
		idcg += 1.0 / math.Log2(float64(i)+2.0)
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
