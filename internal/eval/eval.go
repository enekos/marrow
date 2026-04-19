// Package eval provides a strict, precise retrieval evaluation framework for
// measuring search quality against ground-truth relevance judgments.
package eval

// QRel is a single query with its ground-truth relevant document paths.
type QRel struct {
	Query    string   `json:"query"`
	Lang     string   `json:"lang,omitempty"`
	Relevant []string `json:"relevant"` // exact document paths
}

// PerQueryResult holds all metrics computed for one query.
type PerQueryResult struct {
	Query       string
	Precision   map[int]float64 // key = K
	Recall      map[int]float64
	NDCG        map[int]float64
	F1          map[int]float64
	MRR         float64
	AP          float64
	RankedPaths []string // full ranking returned by the engine
}

// Report aggregates metrics across all queries.
type Report struct {
	Queries       []PerQueryResult
	MeanPrecision map[int]float64
	MeanRecall    map[int]float64
	MeanNDCG      map[int]float64
	MeanF1        map[int]float64
	MRR           float64
	MAP           float64
}
