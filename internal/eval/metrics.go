package eval

import (
	"fmt"
	"math"
)

// ComputeMetrics calculates all IR metrics for a single query given the
// ranked result paths and the ground-truth QRel. cutoffs must be sorted
// in ascending order.
func ComputeMetrics(ranked []string, qrel QRel, cutoffs []int) PerQueryResult {
	relevant := qrel.Relevant
	relSet := make(map[string]struct{}, len(relevant))
	for _, p := range relevant {
		relSet[p] = struct{}{}
	}

	negSet := make(map[string]struct{}, len(qrel.Negative))
	for _, p := range qrel.Negative {
		negSet[p] = struct{}{}
	}

	res := PerQueryResult{
		Query:       qrel.Query,
		Category:    qrel.Category,
		Description: qrel.Description,
		Precision:   make(map[int]float64, len(cutoffs)),
		Recall:      make(map[int]float64, len(cutoffs)),
		NDCG:        make(map[int]float64, len(cutoffs)),
		F1:          make(map[int]float64, len(cutoffs)),
		HitRate:     make(map[int]float64, len(cutoffs)),
		RankedPaths: ranked,
	}

	// Build relevance vector: 1 if ranked[i] is relevant, else 0.
	relVec := make([]int, len(ranked))
	for i, p := range ranked {
		if _, ok := relSet[p]; ok {
			relVec[i] = 1
		}
	}

	// Precompute cumulative relevant counts.
	cumulativeRel := make([]int, len(ranked))
	if len(ranked) > 0 {
		cumulativeRel[0] = relVec[0]
		for i := 1; i < len(ranked); i++ {
			cumulativeRel[i] = cumulativeRel[i-1] + relVec[i]
		}
	}

	// MRR, AP, and R-Precision require scanning for first relevant and precision-at-rank.
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

	// R-Precision: precision at rank = number of relevant documents.
	rRank := len(relevant)
	if rRank > 0 && rRank <= len(ranked) {
		res.RPrecision = float64(cumulativeRel[rRank-1]) / float64(rRank)
	} else if rRank > 0 && len(ranked) > 0 {
		res.RPrecision = float64(cumulativeRel[len(ranked)-1]) / float64(rRank)
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
		res.HitRate[k] = 0.0
		if relInTopK > 0 {
			res.HitRate[k] = 1.0
		}

		if len(qrel.GradedRelevance) > 0 {
			res.NDCG[k] = ndcgAtKGraded(ranked, qrel.GradedRelevance, k)
		} else {
			res.NDCG[k] = ndcgAtK(ranked, relSet, k)
		}
	}

	// Check negative constraints.
	for i, p := range ranked {
		if _, ok := negSet[p]; ok {
			res.FailureReasons = append(res.FailureReasons,
				"negative doc "+p+" appeared at rank "+itoa(i+1))
		}
	}

	// Check per-query thresholds.
	for metricKey, minVal := range qrel.MinMetrics {
		got := resolveMetric(res, metricKey)
		if got < minVal {
			res.FailureReasons = append(res.FailureReasons,
				metricKey+" = "+ftoa(got)+", want >= "+ftoa(minVal))
		}
	}

	return res
}

// AggregateReport computes mean metrics across all per-query results.
func AggregateReport(results []PerQueryResult, cutoffs []int) Report {
	report := Report{
		Queries:           results,
		MeanPrecision:     make(map[int]float64, len(cutoffs)),
		MeanRecall:        make(map[int]float64, len(cutoffs)),
		MeanNDCG:          make(map[int]float64, len(cutoffs)),
		MeanF1:            make(map[int]float64, len(cutoffs)),
		MeanHitRate:       make(map[int]float64, len(cutoffs)),
		CategoryBreakdown: make(map[string]CategoryStats),
	}

	if len(results) == 0 {
		return report
	}

	var mrrSum, mapSum, rPrecSum float64
	for _, r := range results {
		mrrSum += r.MRR
		mapSum += r.AP
		rPrecSum += r.RPrecision
	}
	report.MeanMRR = mrrSum / float64(len(results))
	report.MeanMAP = mapSum / float64(len(results))
	report.MeanRPrecision = rPrecSum / float64(len(results))

	for _, k := range cutoffs {
		var pSum, rSum, nSum, fSum, hSum float64
		for _, res := range results {
			pSum += res.Precision[k]
			rSum += res.Recall[k]
			nSum += res.NDCG[k]
			fSum += res.F1[k]
			hSum += res.HitRate[k]
		}
		n := float64(len(results))
		report.MeanPrecision[k] = pSum / n
		report.MeanRecall[k] = rSum / n
		report.MeanNDCG[k] = nSum / n
		report.MeanF1[k] = fSum / n
		report.MeanHitRate[k] = hSum / n
	}

	// Per-category aggregation.
	catResults := make(map[string][]PerQueryResult)
	for _, r := range results {
		cat := r.Category
		if cat == "" {
			cat = "uncategorized"
		}
		catResults[cat] = append(catResults[cat], r)
	}

	for cat, cres := range catResults {
		stats := CategoryStats{
			Count:         len(cres),
			MeanPrecision: make(map[int]float64, len(cutoffs)),
			MeanRecall:    make(map[int]float64, len(cutoffs)),
			MeanNDCG:      make(map[int]float64, len(cutoffs)),
			MeanF1:        make(map[int]float64, len(cutoffs)),
			MeanHitRate:   make(map[int]float64, len(cutoffs)),
		}

		var cmrr, cmap, crprec float64
		for _, r := range cres {
			cmrr += r.MRR
			cmap += r.AP
			crprec += r.RPrecision
			if len(r.FailureReasons) == 0 {
				stats.PassCount++
			} else {
				stats.FailCount++
			}
		}
		stats.MeanMRR = cmrr / float64(len(cres))
		stats.MeanMAP = cmap / float64(len(cres))
		stats.MeanRPrecision = crprec / float64(len(cres))

		for _, k := range cutoffs {
			var ps, rs, ns, fs, hs float64
			for _, r := range cres {
				ps += r.Precision[k]
				rs += r.Recall[k]
				ns += r.NDCG[k]
				fs += r.F1[k]
				hs += r.HitRate[k]
			}
			n := float64(len(cres))
			stats.MeanPrecision[k] = ps / n
			stats.MeanRecall[k] = rs / n
			stats.MeanNDCG[k] = ns / n
			stats.MeanF1[k] = fs / n
			stats.MeanHitRate[k] = hs / n
		}

		report.CategoryBreakdown[cat] = stats
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

// ndcgAtK computes Normalized Discounted Cumulative Gain at cutoff k
// using binary relevance.
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

	idealRelCount := min(len(relSet), k)
	idcg := 0.0
	for i := 0; i < idealRelCount; i++ {
		idcg += 1.0 / math.Log2(float64(i)+2.0)
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// ndcgAtKGraded computes NDCG using graded relevance gains.
func ndcgAtKGraded(ranked []string, graded map[string]int, k int) float64 {
	topK := min(k, len(ranked))
	if topK == 0 {
		return 0
	}

	dcg := 0.0
	for i := 0; i < topK; i++ {
		if gain, ok := graded[ranked[i]]; ok && gain > 0 {
			dcg += float64(gain) / math.Log2(float64(i)+2.0)
		}
	}

	// Build ideal ranking: all graded docs sorted by gain desc.
	type pair struct {
		path string
		gain int
	}
	var ideal []pair
	for p, g := range graded {
		if g > 0 {
			ideal = append(ideal, pair{path: p, gain: g})
		}
	}
	// Simple insertion sort by gain desc (small N).
	for i := 1; i < len(ideal); i++ {
		for j := i; j > 0 && ideal[j].gain > ideal[j-1].gain; j-- {
			ideal[j], ideal[j-1] = ideal[j-1], ideal[j]
		}
	}

	idcg := 0.0
	for i := 0; i < min(len(ideal), k); i++ {
		idcg += float64(ideal[i].gain) / math.Log2(float64(i)+2.0)
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// resolveMetric extracts a metric value from a PerQueryResult by name.
// Supported names: "P@K", "R@K", "F1@K", "NDCG@K", "HR@K", "MRR", "AP", "RPrec".
func resolveMetric(r PerQueryResult, name string) float64 {
	switch name {
	case "MRR":
		return r.MRR
	case "AP":
		return r.AP
	case "RPrec", "R-Precision":
		return r.RPrecision
	}

	// Try cutoff-based metrics.
	var prefix string
	var k int
	if _, err := fmt.Sscanf(name, "P@%d", &k); err == nil {
		prefix = "P"
	} else if _, err := fmt.Sscanf(name, "R@%d", &k); err == nil {
		prefix = "R"
	} else if _, err := fmt.Sscanf(name, "F1@%d", &k); err == nil {
		prefix = "F1"
	} else if _, err := fmt.Sscanf(name, "NDCG@%d", &k); err == nil {
		prefix = "NDCG"
	} else if _, err := fmt.Sscanf(name, "HR@%d", &k); err == nil {
		prefix = "HR"
	}

	switch prefix {
	case "P":
		return r.Precision[k]
	case "R":
		return r.Recall[k]
	case "F1":
		return r.F1[k]
	case "NDCG":
		return r.NDCG[k]
	case "HR":
		return r.HitRate[k]
	}
	return 0
}

// itoa is a tiny int->string helper to avoid strconv in this package.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// ftoa is a tiny float->string helper.
func ftoa(f float64) string {
	return fmt.Sprintf("%.4f", f)
}
