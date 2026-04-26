package eval

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// TextOptions controls the verbosity and formatting of text reports.
type TextOptions struct {
	// ShowDetail includes per-query ranking diffs for every query, not just failures.
	ShowDetail bool
	// MaxQueryWidth caps the query column width (0 = no limit).
	MaxQueryWidth int
	// ShowCategories emits per-category breakdown tables.
	ShowCategories bool
	// Cutoffs determines which K values to display in tables.
	Cutoffs []int
}

// DefaultTextOptions returns sensible defaults.
func DefaultTextOptions() TextOptions {
	return TextOptions{
		ShowDetail:     false,
		MaxQueryWidth:  28,
		ShowCategories: true,
		Cutoffs:        []int{1, 3, 5, 10},
	}
}

// WriteText writes a human-readable evaluation report to w.
func WriteText(w io.Writer, report Report, opts TextOptions) {
	r := &textReporter{w: w, opts: opts}
	r.writeHeader(report)

	if opts.ShowCategories {
		r.writeCategoryTables(report)
	} else {
		r.writeFlatTable(report)
	}

	r.writeFailures(report)
	r.writeAggregate(report)
}

// WriteMarkdown writes a Markdown evaluation report to w.
func WriteMarkdown(w io.Writer, report Report, opts TextOptions) {
	r := &mdReporter{w: w, opts: opts}
	r.write(report)
}

type textReporter struct {
	w    io.Writer
	opts TextOptions
}

func (r *textReporter) writeHeader(report Report) {
	total := len(report.Queries)
	cats := len(report.CategoryBreakdown)
	fmt.Fprintf(r.w, "=== Retrieval Evaluation Report ===\n")
	fmt.Fprintf(r.w, "%d queries evaluated across %d categories\n\n", total, cats)
}

func (r *textReporter) writeCategoryTables(report Report) {
	cats := make([]string, 0, len(report.CategoryBreakdown))
	for cat := range report.CategoryBreakdown {
		cats = append(cats, cat)
	}
	sort.Strings(cats)
	for _, cat := range cats {
		r.writeCategoryTable(cat, report.CategoryBreakdown[cat], report.Queries)
	}
}

func (r *textReporter) writeCategoryTable(cat string, stats CategoryStats, queries []PerQueryResult) {
	// Collect queries in this category.
	var qs []PerQueryResult
	for _, q := range queries {
		c := q.Category
		if c == "" {
			c = "uncategorized"
		}
		if c == cat {
			qs = append(qs, q)
		}
	}
	if len(qs) == 0 {
		return
	}

	// Header line with category name.
	title := fmt.Sprintf("Category: %s (%d queries)", cat, stats.Count)
	fmt.Fprintf(r.w, "--- %s ---\n", title)

	// Build tabwriter for aligned columns.
	tw := tabwriter.NewWriter(r.w, 0, 0, 2, ' ', 0)

	// Column headers.
	header := "Query"
	for _, k := range r.opts.Cutoffs {
		header += fmt.Sprintf("\tP@%d", k)
	}
	header += "\tMRR\tAP\t✓/✗"
	fmt.Fprintln(tw, header)

	// Separator-ish.
	sep := strings.Repeat("─", r.opts.MaxQueryWidth)
	for range r.opts.Cutoffs {
		sep += "\t────"
	}
	sep += "\t────\t────\t───"
	fmt.Fprintln(tw, sep)

	// Rows.
	for _, q := range qs {
		name := r.truncate(q.Query)
		row := name
		for _, k := range r.opts.Cutoffs {
			row += fmt.Sprintf("\t%.2f", q.Precision[k])
		}
		row += fmt.Sprintf("\t%.2f\t%.2f\t%s", q.MRR, q.AP, r.passFail(q))
		fmt.Fprintln(tw, row)

		if r.opts.ShowDetail || len(q.FailureReasons) > 0 {
			r.writeQueryDetail(tw, q)
		}
	}

	// Category mean row.
	row := "Category mean"
	for _, k := range r.opts.Cutoffs {
		row += fmt.Sprintf("\t%.2f", stats.MeanPrecision[k])
	}
	row += fmt.Sprintf("\t%.2f\t%.2f", stats.MeanMRR, stats.MeanMAP)
	fmt.Fprintln(tw, row)
	fmt.Fprintln(tw)
	tw.Flush()
}

func (r *textReporter) writeFlatTable(report Report) {
	tw := tabwriter.NewWriter(r.w, 0, 0, 2, ' ', 0)

	header := "Query"
	for _, k := range r.opts.Cutoffs {
		header += fmt.Sprintf("\tP@%d", k)
	}
	header += "\tMRR\tAP\t✓/✗"
	fmt.Fprintln(tw, header)

	for _, q := range report.Queries {
		name := r.truncate(q.Query)
		row := name
		for _, k := range r.opts.Cutoffs {
			row += fmt.Sprintf("\t%.2f", q.Precision[k])
		}
		row += fmt.Sprintf("\t%.2f\t%.2f\t%s", q.MRR, q.AP, r.passFail(q))
		fmt.Fprintln(tw, row)

		if r.opts.ShowDetail || len(q.FailureReasons) > 0 {
			r.writeQueryDetail(tw, q)
		}
	}
	fmt.Fprintln(tw)
	tw.Flush()
}

func (r *textReporter) writeQueryDetail(tw *tabwriter.Writer, q PerQueryResult) {
	if q.Description != "" {
		fmt.Fprintf(tw, "  \t\t\t\t\t  %s\n", q.Description)
	}
	if len(q.FailureReasons) > 0 {
		for _, reason := range q.FailureReasons {
			fmt.Fprintf(tw, "  \t\t\t\t\t  ⚠ %s\n", reason)
		}
	}
	if r.opts.ShowDetail || len(q.FailureReasons) > 0 {
		r.writeRankingDiff(tw, q)
	}
}

func (r *textReporter) writeRankingDiff(tw *tabwriter.Writer, q PerQueryResult) {
	relSet := make(map[string]struct{}, len(q.RankedPaths))
	// We don't have direct access to qrel.Relevant here, but we can infer
	// from the FailureReasons or we can add it to PerQueryResult.
	// For now, skip the detailed diff in textReporter since PerQueryResult
	// doesn't store the original relevant/negative sets. We only show
	// failure reasons which already contain the key info.
	// TODO: enrich PerQueryResult with Relevant and Negative if we want full diffs.
	_ = tw
	_ = relSet
}

func (r *textReporter) writeFailures(report Report) {
	var failed []PerQueryResult
	for _, q := range report.Queries {
		if len(q.FailureReasons) > 0 {
			failed = append(failed, q)
		}
	}
	if len(failed) == 0 {
		fmt.Fprintln(r.w, "✓ All queries passed their thresholds and constraints.")
		fmt.Fprintln(r.w)
		return
	}

	fmt.Fprintf(r.w, "Failed thresholds and constraints (%d queries):\n", len(failed))
	for _, q := range failed {
		fmt.Fprintf(r.w, "  • \"%s\" (%s)\n", q.Query, q.Category)
		for _, reason := range q.FailureReasons {
			fmt.Fprintf(r.w, "    ⚠ %s\n", reason)
		}
		if len(q.RankedPaths) > 0 {
			fmt.Fprintf(r.w, "    returned top-%d: %v\n", len(q.RankedPaths), q.RankedPaths)
		}
	}
	fmt.Fprintln(r.w)
}

func (r *textReporter) writeAggregate(report Report) {
	fmt.Fprintln(r.w, "Aggregate:")
	for _, k := range r.opts.Cutoffs {
		fmt.Fprintf(r.w, "  Mean P@%d: %.3f  R@%d: %.3f  F1@%d: %.3f  NDCG@%d: %.3f  HR@%d: %.3f\n",
			k, report.MeanPrecision[k],
			k, report.MeanRecall[k],
			k, report.MeanF1[k],
			k, report.MeanNDCG[k],
			k, report.MeanHitRate[k])
	}
	fmt.Fprintf(r.w, "  MRR:  %.4f\n", report.MeanMRR)
	fmt.Fprintf(r.w, "  MAP:  %.4f\n", report.MeanMAP)
	fmt.Fprintf(r.w, "  RPrec:%.4f\n", report.MeanRPrecision)
	fmt.Fprintln(r.w)

	// Per-category summary lines.
	if len(report.CategoryBreakdown) > 1 {
		fmt.Fprintln(r.w, "Per-category summary:")
		cats := make([]string, 0, len(report.CategoryBreakdown))
		for cat := range report.CategoryBreakdown {
			cats = append(cats, cat)
		}
		sort.Strings(cats)
		for _, cat := range cats {
			stats := report.CategoryBreakdown[cat]
			status := "✓"
			if stats.FailCount > 0 {
				status = "✗"
			}
			fmt.Fprintf(r.w, "  %-22s  MAP: %.3f  MRR: %.3f  pass: %d/%d  %s\n",
				cat, stats.MeanMAP, stats.MeanMRR, stats.PassCount, stats.Count, status)
		}
		fmt.Fprintln(r.w)
	}
}

func (r *textReporter) truncate(s string) string {
	max := r.opts.MaxQueryWidth
	if max <= 0 {
		return s
	}
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func (r *textReporter) passFail(q PerQueryResult) string {
	if len(q.FailureReasons) > 0 {
		return "✗"
	}
	return "✓"
}

// mdReporter produces GitHub-flavoured Markdown.
type mdReporter struct {
	w    io.Writer
	opts TextOptions
}

func (r *mdReporter) write(report Report) {
	fmt.Fprintf(r.w, "# Retrieval Evaluation Report\n\n")
	fmt.Fprintf(r.w, "%d queries evaluated across %d categories.\n\n", len(report.Queries), len(report.CategoryBreakdown))

	// Per-category tables.
	cats := make([]string, 0, len(report.CategoryBreakdown))
	for cat := range report.CategoryBreakdown {
		cats = append(cats, cat)
	}
	sort.Strings(cats)
	for _, cat := range cats {
		stats := report.CategoryBreakdown[cat]
		fmt.Fprintf(r.w, "## %s\n\n", cat)
		fmt.Fprintf(r.w, "| Query |")
		for _, k := range r.opts.Cutoffs {
			fmt.Fprintf(r.w, " P@%d |", k)
		}
		fmt.Fprintf(r.w, " MRR | AP | Status |\n")
		fmt.Fprintf(r.w, "|-------|")
		for range r.opts.Cutoffs {
			fmt.Fprintf(r.w, "------|")
		}
		fmt.Fprintf(r.w, "-----|----|--------|\n")

		for _, q := range report.Queries {
			c := q.Category
			if c == "" {
				c = "uncategorized"
			}
			if c != cat {
				continue
			}
			status := "✅"
			if len(q.FailureReasons) > 0 {
				status = "❌"
			}
			fmt.Fprintf(r.w, "| %s |", q.Query)
			for _, k := range r.opts.Cutoffs {
				fmt.Fprintf(r.w, " %.2f |", q.Precision[k])
			}
			fmt.Fprintf(r.w, " %.2f | %.2f | %s |\n", q.MRR, q.AP, status)
		}

		fmt.Fprintf(r.w, "| **Mean** |")
		for _, k := range r.opts.Cutoffs {
			fmt.Fprintf(r.w, " **%.2f** |", stats.MeanPrecision[k])
		}
		fmt.Fprintf(r.w, " **%.2f** | **%.2f** | |\n\n", stats.MeanMRR, stats.MeanMAP)
	}

	// Overall aggregates.
	fmt.Fprintf(r.w, "## Overall Aggregates\n\n")
	for _, k := range r.opts.Cutoffs {
		fmt.Fprintf(r.w, "- **P@%d**: %.3f  **R@%d**: %.3f  **F1@%d**: %.3f  **NDCG@%d**: %.3f  **HR@%d**: %.3f\n",
			k, report.MeanPrecision[k],
			k, report.MeanRecall[k],
			k, report.MeanF1[k],
			k, report.MeanNDCG[k],
			k, report.MeanHitRate[k])
	}
	fmt.Fprintf(r.w, "- **MRR**: %.4f\n", report.MeanMRR)
	fmt.Fprintf(r.w, "- **MAP**: %.4f\n", report.MeanMAP)
	fmt.Fprintf(r.w, "- **R-Precision**: %.4f\n\n", report.MeanRPrecision)
}
