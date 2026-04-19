package eval

import (
	"math"
	"testing"
)

func TestComputeMetrics_PerfectRanking(t *testing.T) {
	ranked := []string{"/a", "/b", "/c", "/d", "/e"}
	relevant := []string{"/a", "/b", "/c"}
	cutoffs := []int{1, 3, 5}

	m := ComputeMetrics(ranked, relevant, cutoffs)

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
}

func TestComputeMetrics_NoRelevant(t *testing.T) {
	ranked := []string{"/x", "/y", "/z"}
	relevant := []string{"/a", "/b"}
	cutoffs := []int{1, 3}

	m := ComputeMetrics(ranked, relevant, cutoffs)

	for _, k := range cutoffs {
		if m.Precision[k] != 0 {
			t.Errorf("P@%d = %f, want 0", k, m.Precision[k])
		}
		if m.Recall[k] != 0 {
			t.Errorf("R@%d = %f, want 0", k, m.Recall[k])
		}
	}
	if m.MRR != 0 {
		t.Errorf("MRR = %f, want 0", m.MRR)
	}
	if m.AP != 0 {
		t.Errorf("AP = %f, want 0", m.AP)
	}
}

func TestComputeMetrics_FirstRelevantAtRank3(t *testing.T) {
	ranked := []string{"/x", "/y", "/a", "/b", "/c"}
	relevant := []string{"/a", "/b"}
	cutoffs := []int{1, 3, 5}

	m := ComputeMetrics(ranked, relevant, cutoffs)

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
}

func TestComputeMetrics_AP(t *testing.T) {
	// Classic AP example: relevant at ranks 1 and 3.
	ranked := []string{"/a", "/x", "/b", "/y", "/z"}
	relevant := []string{"/a", "/b"}
	cutoffs := []int{5}

	m := ComputeMetrics(ranked, relevant, cutoffs)

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
	relevant := []string{"/a", "/b"}
	cutoffs := []int{3, 5}

	m := ComputeMetrics(ranked, relevant, cutoffs)

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

func TestAggregateReport(t *testing.T) {
	results := []PerQueryResult{
		{Precision: map[int]float64{1: 1.0, 5: 0.6}, Recall: map[int]float64{1: 0.5, 5: 1.0}, MRR: 1.0, AP: 1.0},
		{Precision: map[int]float64{1: 0.0, 5: 0.4}, Recall: map[int]float64{1: 0.0, 5: 0.5}, MRR: 0.5, AP: 0.5},
	}
	cutoffs := []int{1, 5}

	r := AggregateReport(results, cutoffs)

	if r.MRR != 0.75 {
		t.Errorf("MRR = %f, want 0.75", r.MRR)
	}
	if r.MAP != 0.75 {
		t.Errorf("MAP = %f, want 0.75", r.MAP)
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
