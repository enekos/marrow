package eval

import (
	"context"
	"fmt"
)

// SearchFunc is the minimal contract needed to run evaluation.
// It must return the ranked document paths for the given query.
type SearchFunc func(ctx context.Context, query, lang string, limit int) ([]string, error)

// Runner executes evaluation queries against a search function.
type Runner struct {
	Search  SearchFunc
	Cutoffs []int
	Limit   int
}

// NewRunner creates an evaluation runner with sensible defaults.
// The caller must provide a SearchFunc (typically a thin wrapper around
// search.Engine.Search that extracts result paths).
func NewRunner(search SearchFunc) *Runner {
	return &Runner{
		Search:  search,
		Cutoffs: []int{1, 3, 5, 10},
		Limit:   10,
	}
}

// Run evaluates a single query.
func (r *Runner) Run(ctx context.Context, qrel QRel) (PerQueryResult, error) {
	paths, err := r.Search(ctx, qrel.Query, qrel.Lang, r.Limit)
	if err != nil {
		return PerQueryResult{}, fmt.Errorf("search %q: %w", qrel.Query, err)
	}

	m := ComputeMetrics(paths, qrel.Relevant, r.Cutoffs)
	m.Query = qrel.Query
	return m, nil
}

// RunAll evaluates all queries and returns an aggregated report.
func (r *Runner) RunAll(ctx context.Context, qrels []QRel) (Report, error) {
	perQuery := make([]PerQueryResult, 0, len(qrels))
	for _, q := range qrels {
		m, err := r.Run(ctx, q)
		if err != nil {
			return Report{}, err
		}
		perQuery = append(perQuery, m)
	}
	return AggregateReport(perQuery, r.Cutoffs), nil
}
