package eval

import (
	"math"
	"testing"
)

func TestComputeMetrics_PerfectRanking(t *testing.T) {
	ranked := []string{"/a", "/b", "/c", "/d", "/e"}
	qrel := QRel{Query: "test", Relevant: []string{"/a", "/b", "/c"}}
	cutoffs := []int{1, 3, 5}

	m := ComputeMetrics(ranked, qrel, cutoffs)

	if m.Precision[1] != 1.0 {
		t.Errorf("P@1 = %f, want 1.0", m.Precision[1])
	}
	if m.Recall[1] != 1.0/3.0 {
		t.Errorf("R@1 = %f, want 0.333", m.Recall[1])
	}
	if m.Precision[3] != 1.0 {
		t.Errorf("P@3 = %f, want 1.0", m.Precision[3])
	}
	if m.Recall[3] != 1.0 {
		t.Errorf("R@3 = %f, want 1.0", m.Recall[3])
	}
	if m.MRR != 1.0 {
		t.Errorf("MRR = %f, want 1.0", m.MRR)
	}
	if m.AP != 1.0 {
		t.Errorf("AP = %f, want 1.0", m.AP)
	}
	if m.NDCG[3] != 1.0 {
		t.Errorf("NDCG@3 = %f, want 1.0", m.NDCG[3])
	}
	if m.RPrecision != 1.0 {
		t.Errorf("RPrecision = %f, want 1.0", m.RPrecision)
	}
	if m.HitRate[1] != 1.0 {
		t.Errorf("HitRate@1 = %f, want 1.0", m.HitRate[1])
	}
}

func TestComputeMetrics_NoRelevant(t *testing.T) {
	ranked := []string{"/x", "/y", "/z"}
	qrel := QRel{Query: "test", Relevant: []string{"/a", "/b"}}
	cutoffs := []int{1, 3}

	m := ComputeMetrics(ranked, qrel, cutoffs)

	for _, k := range cutoffs {
		if m.Precision[k] != 0 {
			t.Errorf("P@%d = %f, want 0", k, m.Precision[k])
		}
		if m.Recall[k] != 0 {
			t.Errorf("R@%d = %f, want 0", k, m.Recall[k])
		}
		if m.HitRate[k] != 0 {
			t.Errorf("HitRate@%d = %f, want 0", k, m.HitRate[k])
		}
	}
	if m.MRR != 0 {
		t.Errorf("MRR = %f, want 0", m.MRR)
	}
	if m.AP != 0 {
		t.Errorf("AP = %f, want 0", m.AP)
	}
	if m.RPrecision != 0 {
		t.Errorf("RPrecision = %f, want 0", m.RPrecision)
	}
}

func TestComputeMetrics_FirstRelevantAtRank3(t *testing.T) {
	ranked := []string{"/x", "/y", "/a", "/b", "/c"}
	qrel := QRel{Query: "test", Relevant: []string{"/a", "/b"}}
	cutoffs := []int{1, 3, 5}

	m := ComputeMetrics(ranked, qrel, cutoffs)

	if m.MRR != 1.0/3.0 {
		t.Errorf("MRR = %f, want 0.333", m.MRR)
	}
	// P@1 = 0, P@3 = 1/3, P@5 = 2/5
	if m.Precision[1] != 0 {
		t.Errorf("P@1 = %f, want 0", m.Precision[1])
	}
	if m.Precision[3] != 1.0/3.0 {
		t.Errorf("P@3 = %f, want 0.333", m.Precision[3])
	}
	if m.Precision[5] != 0.4 {
		t.Errorf("P@5 = %f, want 0.4", m.Precision[5])
	}
	// R@3 = 1/2, R@5 = 1
	if m.Recall[3] != 0.5 {
		t.Errorf("R@3 = %f, want 0.5", m.Recall[3])
	}
	if m.Recall[5] != 1.0 {
		t.Errorf("R@5 = %f, want 1.0", m.Recall[5])
	}
	// RPrecision = P@2 = 0
	if m.RPrecision != 0 {
		t.Errorf("RPrecision = %f, want 0", m.RPrecision)
	}
}

func TestComputeMetrics_AP(t *testing.T) {
	// Classic AP example: relevant at ranks 1 and 3.
	ranked := []string{"/a", "/x", "/b", "/y", "/z"}
	qrel := QRel{Query: "test", Relevant: []string{"/a", "/b"}}
	cutoffs := []int{5}

	m := ComputeMetrics(ranked, qrel, cutoffs)

	// P@1 = 1.0, P@3 = 2/3
	// AP = (1.0 + 2/3) / 2 = 5/6 ≈ 0.8333
	wantAP := 5.0 / 6.0
	if math.Abs(m.AP-wantAP) > 1e-9 {
		t.Errorf("AP = %f, want %f", m.AP, wantAP)
	}
}

func TestComputeMetrics_NDCG(t *testing.T) {
	// Relevant at ranks 2 and 4 (0-indexed: 1 and 3).
	ranked := []string{"/x", "/a", "/y", "/b", "/z"}
	qrel := QRel{Query: "test", Relevant: []string{"/a", "/b"}}
	cutoffs := []int{3, 5}

	m := ComputeMetrics(ranked, qrel, cutoffs)

	// DCG@3: rel at pos 1 → 1/log2(3) ≈ 0.6309
	// IDCG@3: both relevant at top → 1/log2(2) + 1/log2(3) ≈ 1 + 0.6309 = 1.6309
	// NDCG@3 ≈ 0.6309 / 1.6309 ≈ 0.3868
	wantNDCG3 := (1.0 / math.Log2(3.0)) / (1.0 + 1.0/math.Log2(3.0))
	if math.Abs(m.NDCG[3]-wantNDCG3) > 1e-9 {
		t.Errorf("NDCG@3 = %f, want %f", m.NDCG[3], wantNDCG3)
	}

	// DCG@5: add rel at pos 3 → 1/log2(5) ≈ 0.4307
	// DCG@5 ≈ 0.6309 + 0.4307 = 1.0616
	// IDCG@5: same as IDCG@3 since only 2 relevant docs → 1.6309
	wantNDCG5 := (1.0/math.Log2(3.0) + 1.0/math.Log2(5.0)) / (1.0 + 1.0/math.Log2(3.0))
	if math.Abs(m.NDCG[5]-wantNDCG5) > 1e-9 {
		t.Errorf("NDCG@5 = %f, want %f", m.NDCG[5], wantNDCG5)
	}
}

func TestComputeMetrics_GradedNDCG(t *testing.T) {
	ranked := []string{"/a", "/b", "/c", "/d"}
	qrel := QRel{
		Query: "test",
		GradedRelevance: map[string]int{
			"/a": 3,
			"/b": 2,
			"/c": 0,
		},
	}
	cutoffs := []int{2, 4}

	m := ComputeMetrics(ranked, qrel, cutoffs)

	// DCG@2: 3/log2(2) + 2/log2(3) = 3 + 1.26186 = 4.26186
	// IDCG@2: 3/log2(2) + 2/log2(3) = same = 4.26186
	if m.NDCG[2] != 1.0 {
		t.Errorf("NDCG@2 = %f, want 1.0", m.NDCG[2])
	}

	// DCG@4: add 0 at pos 2 and 0 at pos 3 → same as DCG@2
	// IDCG@4: 3/log2(2) + 2/log2(3) + 0 + 0 = same
	if m.NDCG[4] != 1.0 {
		t.Errorf("NDCG@4 = %f, want 1.0", m.NDCG[4])
	}
}

func TestComputeMetrics_NegativeConstraint(t *testing.T) {
	ranked := []string{"/a", "/b", "/bad"}
	qrel := QRel{
		Query:    "test",
		Relevant: []string{"/a", "/b"},
		Negative: []string{"/bad"},
	}
	cutoffs := []int{3}

	m := ComputeMetrics(ranked, qrel, cutoffs)
	if len(m.FailureReasons) == 0 {
		t.Error("expected failure reason for negative doc /bad")
	}
}

func TestComputeMetrics_MinMetrics(t *testing.T) {
	ranked := []string{"/x", "/a", "/x", "/x", "/x"}
	qrel := QRel{
		Query:      "test",
		Relevant:   []string{"/a", "/b"},
		MinMetrics: map[string]float64{"P@5": 0.5, "MRR": 0.9},
	}
	cutoffs := []int{1, 5}

	m := ComputeMetrics(ranked, qrel, cutoffs)
	if len(m.FailureReasons) == 0 {
		t.Error("expected failure reasons for missed thresholds")
	}
	foundP5 := false
	foundMRR := false
	for _, fr := range m.FailureReasons {
		if len(fr) >= 4 && fr[:4] == "P@5 " {
			foundP5 = true
		}
		if len(fr) >= 4 && fr[:4] == "MRR " {
			foundMRR = true
		}
	}
	if !foundP5 {
		t.Error("expected P@5 failure reason")
	}
	if !foundMRR {
		t.Error("expected MRR failure reason")
	}
}

func TestAggregateReport(t *testing.T) {
	results := []PerQueryResult{
		{Precision: map[int]float64{1: 1.0, 5: 0.6}, Recall: map[int]float64{1: 0.5, 5: 1.0}, MRR: 1.0, AP: 1.0, HitRate: map[int]float64{1: 1.0, 5: 1.0}},
		{Precision: map[int]float64{1: 0.0, 5: 0.4}, Recall: map[int]float64{1: 0.0, 5: 0.5}, MRR: 0.5, AP: 0.5, HitRate: map[int]float64{1: 0.0, 5: 1.0}},
	}
	cutoffs := []int{1, 5}

	r := AggregateReport(results, cutoffs)

	if r.MeanMRR != 0.75 {
		t.Errorf("MRR = %f, want 0.75", r.MeanMRR)
	}
	if r.MeanMAP != 0.75 {
		t.Errorf("MAP = %f, want 0.75", r.MeanMAP)
	}
	if r.MeanPrecision[1] != 0.5 {
		t.Errorf("MeanP@1 = %f, want 0.5", r.MeanPrecision[1])
	}
	if r.MeanPrecision[5] != 0.5 {
		t.Errorf("MeanP@5 = %f, want 0.5", r.MeanPrecision[5])
	}
	if r.MeanRecall[1] != 0.25 {
		t.Errorf("MeanR@1 = %f, want 0.25", r.MeanRecall[1])
	}
	if r.MeanRecall[5] != 0.75 {
		t.Errorf("MeanR@5 = %f, want 0.75", r.MeanRecall[5])
	}
	if r.MeanHitRate[1] != 0.5 {
		t.Errorf("MeanHitRate@1 = %f, want 0.5", r.MeanHitRate[1])
	}
	if r.MeanHitRate[5] != 1.0 {
		t.Errorf("MeanHitRate@5 = %f, want 1.0", r.MeanHitRate[5])
	}
}

func TestAggregateReport_CategoryBreakdown(t *testing.T) {
	results := []PerQueryResult{
		{Category: "A", Precision: map[int]float64{1: 1.0}, MRR: 1.0, AP: 1.0},
		{Category: "A", Precision: map[int]float64{1: 0.0}, MRR: 0.0, AP: 0.0},
		{Category: "B", Precision: map[int]float64{1: 0.5}, MRR: 0.5, AP: 0.5},
	}
	cutoffs := []int{1}

	r := AggregateReport(results, cutoffs)

	if len(r.CategoryBreakdown) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(r.CategoryBreakdown))
	}
	if r.CategoryBreakdown["A"].MeanPrecision[1] != 0.5 {
		t.Errorf("cat A MeanP@1 = %f, want 0.5", r.CategoryBreakdown["A"].MeanPrecision[1])
	}
	if r.CategoryBreakdown["A"].Count != 2 {
		t.Errorf("cat A count = %d, want 2", r.CategoryBreakdown["A"].Count)
	}
	if r.CategoryBreakdown["B"].MeanPrecision[1] != 0.5 {
		t.Errorf("cat B MeanP@1 = %f, want 0.5", r.CategoryBreakdown["B"].MeanPrecision[1])
	}
	if r.CategoryBreakdown["B"].Count != 1 {
		t.Errorf("cat B count = %d, want 1", r.CategoryBreakdown["B"].Count)
	}
}

func TestF1(t *testing.T) {
	if f1(0, 0) != 0 {
		t.Errorf("f1(0,0) = %f, want 0", f1(0, 0))
	}
	if f1(1.0, 1.0) != 1.0 {
		t.Errorf("f1(1,1) = %f, want 1", f1(1.0, 1.0))
	}
	got := f1(0.5, 0.5)
	if got != 0.5 {
		t.Errorf("f1(0.5,0.5) = %f, want 0.5", got)
	}
}
