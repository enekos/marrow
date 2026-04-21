package related

import (
	"math"
	"testing"
)

func floatEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestTaxonomyOverlapIDF(t *testing.T) {
	// Corpus of 100 docs. Tag "common" appears in 80, "rare" in 4.
	idf := map[string]float64{
		"common": math.Log(1 + 100.0/80.0), // ≈ 0.8109
		"rare":   math.Log(1 + 100.0/4.0),  // ≈ 3.2581
		"mid":    math.Log(1 + 100.0/20.0), // ≈ 1.7918
	}

	t.Run("identical sets score 1.0", func(t *testing.T) {
		srcTags := []string{"common", "rare"}
		srcIDFSum := idf["common"] + idf["rare"]
		got := taxonomyOverlapIDF(srcTags, []string{"common", "rare"}, idf, srcIDFSum)
		if !floatEq(got, 1.0) {
			t.Fatalf("got %v, want 1.0", got)
		}
	})

	t.Run("disjoint sets score 0", func(t *testing.T) {
		srcTags := []string{"common"}
		srcIDFSum := idf["common"]
		got := taxonomyOverlapIDF(srcTags, []string{"rare"}, idf, srcIDFSum)
		if got != 0 {
			t.Fatalf("got %v, want 0", got)
		}
	})

	t.Run("rare match outscores common match", func(t *testing.T) {
		srcTags := []string{"common", "rare"}
		srcIDFSum := idf["common"] + idf["rare"]
		commonOnly := taxonomyOverlapIDF(srcTags, []string{"common"}, idf, srcIDFSum)
		rareOnly := taxonomyOverlapIDF(srcTags, []string{"rare"}, idf, srcIDFSum)
		if !(rareOnly > commonOnly) {
			t.Fatalf("rareOnly=%v should exceed commonOnly=%v", rareOnly, commonOnly)
		}
	})

	t.Run("empty source tags returns 0", func(t *testing.T) {
		got := taxonomyOverlapIDF(nil, []string{"common"}, idf, 0)
		if got != 0 {
			t.Fatalf("got %v, want 0", got)
		}
	})

	t.Run("empty candidate tags returns 0", func(t *testing.T) {
		got := taxonomyOverlapIDF([]string{"common"}, nil, idf, idf["common"])
		if got != 0 {
			t.Fatalf("got %v, want 0", got)
		}
	})

	t.Run("missing idf entry contributes 0 without panic", func(t *testing.T) {
		// Defensive: a tag on the candidate that's somehow absent from idf.
		srcTags := []string{"common"}
		srcIDFSum := idf["common"]
		got := taxonomyOverlapIDF(srcTags, []string{"common", "unknown"}, idf, srcIDFSum)
		if !floatEq(got, 1.0) {
			t.Fatalf("got %v, want 1.0 (unknown tag ignored)", got)
		}
	})
}

func TestRarestShared(t *testing.T) {
	// df lower = rarer = preferred.
	df := map[string]int{
		"common": 80,
		"mid":    20,
		"rare":   4,
	}

	t.Run("picks the rarest shared tag", func(t *testing.T) {
		got := rarestShared(
			[]string{"common", "mid", "rare"},
			[]string{"common", "rare"},
			df,
		)
		if got != "rare" {
			t.Fatalf("got %q, want %q", got, "rare")
		}
	})

	t.Run("ties broken lexicographically", func(t *testing.T) {
		dfTie := map[string]int{"alpha": 5, "beta": 5, "common": 80}
		got := rarestShared(
			[]string{"alpha", "beta", "common"},
			[]string{"alpha", "beta", "common"},
			dfTie,
		)
		if got != "alpha" {
			t.Fatalf("got %q, want %q", got, "alpha")
		}
	})

	t.Run("no shared tags returns empty string", func(t *testing.T) {
		got := rarestShared([]string{"a"}, []string{"b"}, df)
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("missing df entry treated as rarest (df=0)", func(t *testing.T) {
		// A tag not in df defaults to df=0, which should win over any known tag.
		got := rarestShared(
			[]string{"common", "unknown"},
			[]string{"common", "unknown"},
			df,
		)
		if got != "unknown" {
			t.Fatalf("got %q, want %q", got, "unknown")
		}
	})
}
