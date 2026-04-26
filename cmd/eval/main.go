package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/eval"
	"github.com/enekos/marrow/internal/search"
)

func main() {
	var (
		dbPath     = flag.String("db", "marrow.db", "Path to SQLite database")
		qrelsPath  = flag.String("qrels", "", "Path to JSON qrels file")
		cutoffsStr = flag.String("k", "1,3,5,10", "Comma-separated cutoff values")
		limit      = flag.Int("limit", 10, "Search limit per query")
		provider   = flag.String("provider", "mock", "Embedding provider (mock, ollama, openai)")
		model      = flag.String("model", "", "Embedding model (provider-specific default if empty)")
		baseURL    = flag.String("base_url", "", "Provider base URL")
		apiKey     = flag.String("api_key", "", "API key for openai provider")
		format     = flag.String("format", "text", "Output format: text, json, md")
		detail     = flag.Bool("detail", false, "Show per-query ranking details even for passing queries")
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
		results, err := engine.Search(ctx, query, lang, limit, search.Filter{Lang: lang})
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

	qrels = eval.ExpandVariants(qrels)
	report, err := runner.RunAll(ctx, qrels)
	if err != nil {
		fmt.Fprintf(os.Stderr, "evaluation failed: %v\n", err)
		os.Exit(1)
	}

	passCount, failCount := 0, 0
	for _, q := range report.Queries {
		if len(q.FailureReasons) > 0 {
			failCount++
		} else {
			passCount++
		}
	}

	switch *format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(b))
	case "md", "markdown":
		opts := eval.TextOptions{
			ShowDetail:     *detail,
			ShowCategories: true,
			Cutoffs:        cutoffs,
		}
		eval.WriteMarkdown(os.Stdout, report, opts)
	default:
		opts := eval.TextOptions{
			ShowDetail:     *detail,
			ShowCategories: true,
			Cutoffs:        cutoffs,
		}
		eval.WriteText(os.Stdout, report, opts)
	}

	fmt.Printf("Summary: %d passed, %d failed out of %d queries\n", passCount, failCount, len(report.Queries))
	if failCount > 0 {
		os.Exit(2)
	}
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
	data, err := os.ReadFile(path)
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
