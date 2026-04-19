package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/eval"
	"marrow/internal/search"
)

func main() {
	var (
		dbPath   = flag.String("db", "marrow.db", "Path to SQLite database")
		qrelsPath = flag.String("qrels", "", "Path to JSON qrels file")
		cutoffsStr = flag.String("k", "1,3,5,10", "Comma-separated cutoff values")
		limit    = flag.Int("limit", 10, "Search limit per query")
		provider = flag.String("provider", "mock", "Embedding provider (mock, ollama, openai)")
		model    = flag.String("model", "", "Embedding model (provider-specific default if empty)")
		baseURL  = flag.String("base_url", "", "Provider base URL")
		apiKey   = flag.String("api_key", "", "API key for openai provider")
	)
	flag.Parse()

	if *qrelsPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: eval -qrels <qrels.json> [-db <db>] [-k 1,3,5,10]")
		os.Exit(1)
	}

	cutoffs, err := parseCutoffs(*cutoffsStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid cutoffs: %v\n", err)
		os.Exit(1)
	}

	qrels, err := loadQrels(*qrelsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load qrels: %v\n", err)
		os.Exit(1)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	embedFn, err := embed.NewProvider(*provider, *model, *baseURL, *apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "embed provider: %v\n", err)
		os.Exit(1)
	}
	if err := embed.Validate(context.Background(), embedFn); err != nil {
		fmt.Fprintf(os.Stderr, "embed validation: %v\n", err)
		os.Exit(1)
	}

	engine := search.NewEngine(database, embedFn)
	searchFn := func(ctx context.Context, query, lang string, limit int) ([]string, error) {
		results, err := engine.Search(ctx, query, lang, limit, search.Filter{})
		if err != nil {
			return nil, err
		}
		paths := make([]string, len(results))
		for i, r := range results {
			paths[i] = r.Path
		}
		return paths, nil
	}
	runner := eval.NewRunner(searchFn)
	runner.Cutoffs = cutoffs
	runner.Limit = *limit

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	report, err := runner.RunAll(ctx, qrels)
	if err != nil {
		fmt.Fprintf(os.Stderr, "evaluation failed: %v\n", err)
		os.Exit(1)
	}

	printReport(report, cutoffs)
}

func parseCutoffs(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func loadQrels(path string) ([]eval.QRel, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Queries []eval.QRel `json:"queries"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Queries, nil
}

func printReport(r eval.Report, cutoffs []int) {
	fmt.Println()
	fmt.Println("=== Retrieval Evaluation Report ===")
	fmt.Printf("Queries evaluated: %d\n\n", len(r.Queries))

	// Per-query table
	fmt.Println("Per-query results:")
	fmt.Printf("%-24s", "Query")
	for _, k := range cutoffs {
		fmt.Printf(" P@%d", k)
	}
	for _, k := range cutoffs {
		fmt.Printf(" R@%d", k)
	}
	fmt.Printf("  MRR    AP\n")

	for _, q := range r.Queries {
		trunc := q.Query
		if len(trunc) > 22 {
			trunc = trunc[:19] + "..."
		}
		fmt.Printf("%-24s", trunc)
		for _, k := range cutoffs {
			fmt.Printf(" %.3f", q.Precision[k])
		}
		for _, k := range cutoffs {
			fmt.Printf(" %.3f", q.Recall[k])
		}
		fmt.Printf("  %.3f  %.3f\n", q.MRR, q.AP)
	}

	fmt.Println()
	fmt.Println("Aggregate:")
	for _, k := range cutoffs {
		fmt.Printf("  Mean P@%d: %.4f  R@%d: %.4f  F1@%d: %.4f  NDCG@%d: %.4f\n",
			k, r.MeanPrecision[k], k, r.MeanRecall[k], k, r.MeanF1[k], k, r.MeanNDCG[k])
	}
	fmt.Printf("  MRR:  %.4f\n", r.MRR)
	fmt.Printf("  MAP:  %.4f\n", r.MAP)
	fmt.Println()
}
