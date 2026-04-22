// Command related computes high-quality "related article" lists for every
// indexed document and emits a JSON map keyed by document path.
//
// Scoring blends semantic (doc-vector cosine), lexical salience
// (per-document TF-IDF top-K overlap), internal link graph and shared
// categories, then diversifies via MMR. See the `internal/related`
// package for details.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime/pprof"
	"strings"

	"marrow/internal/db"
	"marrow/internal/related"
)

func main() {
	fs := flag.NewFlagSet("related", flag.ExitOnError)
	dbPath := fs.String("db", "marrow.db", "Path to SQLite database")
	source := fs.String("source", "", "Source filter (empty = all)")
	contentDir := fs.String("content-dir", "", "Content directory (for link graph and front-matter categories). Defaults to the source's synced dir if present.")
	limit := fs.Int("limit", 10, "Number of related documents per document")
	workers := fs.Int("workers", 8, "Concurrent per-source workers")
	out := fs.String("out", "", "Output JSON file (empty = stdout)")

	wSem := fs.Float64("w-sem", 0.55, "Weight: semantic cosine")
	wLex := fs.Float64("w-lex", 0.20, "Weight: lexical salience Jaccard")
	wLink := fs.Float64("w-link", 0.15, "Weight: internal link graph")
	wCat := fs.Float64("w-cat", 0.10, "Weight: shared category")
	mmrLambda := fs.Float64("mmr-lambda", 0.72, "MMR relevance/diversity tradeoff (1=pure relevance, 0=pure diversity)")
	topK := fs.Int("salient-top-k", 24, "TF-IDF salient terms retained per doc")
	noSem := fs.Bool("no-semantic", false, "Disable semantic signal (force lex+link+cat only)")
	taxFields := fs.String("taxonomy-fields", "categories,categories_meta", "Comma-separated front-matter fields to read for the shared-taxonomy signal. Values may be lists of strings or of maps ({slug|name|value}). Defaults match Hugo's `categories` shape; pass e.g. `tag_pairs` for dictionary-style name|value pairs.")
	taxLabel := fs.String("taxonomy-reason", "category", "Prefix used for taxonomy reasons in the output (e.g. `category` → `category:politika`, `tag` → `tag:es|padre`).")
	catIDF := fs.Bool("cat-idf", false, "IDF-weight the shared-taxonomy signal. Off by default for backwards compatibility; recommended for short-gloss corpora like the dictionary where generic tags dominate.")
	cpuProfile := fs.String("cpuprofile", "", "Write CPU profile to this file")

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "parse flags: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	database, err := db.Open(*dbPath)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	var fields []string
	for _, f := range strings.Split(*taxFields, ",") {
		if f = strings.TrimSpace(f); f != "" {
			fields = append(fields, f)
		}
	}

	cfg := related.Config{
		Limit:               *limit,
		WSem:                *wSem,
		WLex:                *wLex,
		WLink:               *wLink,
		WCat:                *wCat,
		MMRLambda:           *mmrLambda,
		TopKSalient:         *topK,
		Workers:             *workers,
		IgnoreSemantic:      *noSem,
		TaxonomyFields:      fields,
		TaxonomyReasonLabel: *taxLabel,
		UseCatIDF:           *catIDF,
	}

	builder := related.NewBuilder(cfg, logger)

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			logger.Error("create cpu profile", "err", err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			logger.Error("start cpu profile", "err", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	ctx := context.Background()
	if err := builder.Load(ctx, database, *source, *contentDir); err != nil {
		logger.Error("load corpus", "err", err)
		os.Exit(1)
	}

	results := builder.Compute(ctx)
	if len(results) == 0 {
		logger.Warn("no related articles produced")
	}

	outData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		logger.Error("marshal json", "err", err)
		os.Exit(1)
	}

	if *out != "" {
		if err := os.WriteFile(*out, outData, 0o644); err != nil {
			logger.Error("write file", "err", err)
			os.Exit(1)
		}
		logger.Info("wrote related.json", "path", *out, "docs", len(results))
	} else {
		fmt.Println(string(outData))
	}
}
