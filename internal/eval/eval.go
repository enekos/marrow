// Package eval provides a strict, precise retrieval evaluation framework for
// measuring search quality against ground-truth relevance judgments.
package eval

// QRel is a single query with its ground-truth relevant document paths.
type QRel struct {
	Query    string   `json:"query"`
	Lang     string   `json:"lang,omitempty"`
	Relevant []string `json:"relevant"` // exact document paths (binary relevance)

	// Category groups queries for aggregate reporting (e.g. "language-specific", "cross-language", "edge-case").
	Category string `json:"category,omitempty"`

	// Description is a human-readable explanation of what this query is testing.
	Description string `json:"description,omitempty"`

	// GradedRelevance maps document paths to a relevance grade (0–3).
	// When provided, it is used for graded NDCG instead of binary Relevant.
	// 0 = irrelevant, 1 = somewhat relevant, 2 = relevant, 3 = highly relevant.
	GradedRelevance map[string]int `json:"graded_relevance,omitempty"`

	// Negative lists documents that must NOT appear in the top-N results.
	Negative []string `json:"negative,omitempty"`

	// Variants are semantically equivalent phrasings of the same query intent.
	// They are expanded into separate QRel entries by ExpandVariants.
	Variants []string `json:"variants,omitempty"`

	// MinMetrics defines per-query pass/fail thresholds.
	// Keys are metric names: "P@5", "R@3", "MRR", "AP", etc.
	// A missing key means no threshold for that metric.
	MinMetrics map[string]float64 `json:"min_metrics,omitempty"`
}

// PerQueryResult holds all metrics computed for one query.
type PerQueryResult struct {
	Query       string
	Category    string
	Description string

	Precision   map[int]float64 // key = K
	Recall      map[int]float64
	NDCG        map[int]float64
	F1          map[int]float64
	MRR         float64
	AP          float64
	RPrecision  float64
	HitRate     map[int]float64 // 1.0 if ≥1 relevant doc in top K, else 0.0

	RankedPaths    []string // full ranking returned by the engine
	FailureReasons []string // populated when thresholds or negative constraints are violated
}

// Report aggregates metrics across all queries.
type Report struct {
	Queries       []PerQueryResult
	MeanPrecision map[int]float64
	MeanRecall    map[int]float64
	MeanNDCG      map[int]float64
	MeanF1        map[int]float64
	MeanMRR       float64
	MeanMAP       float64
	MeanRPrecision float64
	MeanHitRate   map[int]float64

	// CategoryBreakdown holds per-category aggregates.
	CategoryBreakdown map[string]CategoryStats
}

// CategoryStats aggregates metrics for a single category.
type CategoryStats struct {
	Count          int
	MeanPrecision  map[int]float64
	MeanRecall     map[int]float64
	MeanNDCG       map[int]float64
	MeanF1         map[int]float64
	MeanMRR        float64
	MeanMAP        float64
	MeanRPrecision float64
	MeanHitRate    map[int]float64
	PassCount      int
	FailCount      int
}

// ExpandVariants duplicates QRel entries that have Variants so each variant
// becomes its own query. The original query is preserved; variants are appended.
func ExpandVariants(qrels []QRel) []QRel {
	out := make([]QRel, 0, len(qrels))
	for _, q := range qrels {
		out = append(out, q)
		for _, v := range q.Variants {
			clone := q
			clone.Query = v
			clone.Variants = nil // prevent recursive expansion
			out = append(out, clone)
		}
	}
	return out
}
