// Command giza-eval indexes a sample of gizapedia content into a local
// SQLite database (mock embeddings for speed) and runs a large set of
// exact-title queries to measure relevance. It's a development tool used
// to drive scoring improvements; not part of the production runtime.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/enekos/marrow/internal/chunker"
	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/index"
	"github.com/enekos/marrow/internal/markdown"
	"github.com/enekos/marrow/internal/search"
	"github.com/enekos/marrow/internal/stemmer"
)

type docCase struct {
	Path  string
	Title string
	Lang  string
}

func main() {
	var (
		contentDir = flag.String("content", "../gizapedia/content", "Path to gizapedia content")
		dbPath     = flag.String("db", "", "Sqlite db (empty = temp file, reused per run)")
		sample     = flag.Int("sample", 1500, "Number of docs to index")
		limit      = flag.Int("limit", 10, "Search results per query")
		queries    = flag.Int("queries", 500, "Number of queries to run (subset of indexed docs)")
		lang       = flag.String("lang", "eu", "Default lang for indexing")
		showFails  = flag.Int("show_fails", 25, "How many ranking failures to print")
		seed       = flag.Int64("seed", 42, "Random seed")
		dumpJSON   = flag.String("dump_json", "", "Optional path to write per-query JSONL")
	)
	flag.Parse()

	rng := rand.New(rand.NewSource(*seed))

	if *dbPath == "" {
		f, err := os.CreateTemp("", "giza-eval-*.db")
		if err != nil {
			die("temp db: %v", err)
		}
		f.Close()
		os.Remove(f.Name())
		*dbPath = f.Name()
		defer os.Remove(*dbPath)
	}

	fmt.Printf("==> walking %s\n", *contentDir)
	all := walkContent(*contentDir)
	fmt.Printf("    found %d markdown files\n", len(all))
	if len(all) == 0 {
		die("no docs")
	}

	rng.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
	if len(all) > *sample {
		all = all[:*sample]
	}

	fmt.Printf("==> opening db %s\n", *dbPath)
	database, err := db.Open(*dbPath)
	if err != nil {
		die("open db: %v", err)
	}
	defer database.Close()

	embedFn := embed.NewMock()
	ix := index.NewIndexer(database)

	fmt.Printf("==> indexing %d docs\n", len(all))
	t0 := time.Now()
	cases := make([]docCase, 0, len(all))
	for i, p := range all {
		title, l, err := indexFile(context.Background(), ix, embedFn, p, *contentDir, *lang)
		if err != nil {
			continue
		}
		if title == "" {
			continue
		}
		cases = append(cases, docCase{Path: relpath(p, *contentDir), Title: title, Lang: l})
		if (i+1)%200 == 0 {
			fmt.Printf("    %d/%d indexed (%.1fs)\n", i+1, len(all), time.Since(t0).Seconds())
		}
	}
	fmt.Printf("    indexed %d in %.1fs\n", len(cases), time.Since(t0).Seconds())

	engine := search.NewEngine(database, embedFn)

	if *queries > len(cases) {
		*queries = len(cases)
	}
	rng.Shuffle(len(cases), func(i, j int) { cases[i], cases[j] = cases[j], cases[i] })
	subset := cases[:*queries]

	fmt.Printf("==> running %d queries\n", len(subset))
	type qres struct {
		Query    string `json:"query"`
		Target   string `json:"target"`
		Rank     int    `json:"rank"`
		Top1     string `json:"top1"`
		TopScore float64 `json:"top_score"`
		Stem     string `json:"stem"`
	}
	results := make([]qres, 0, len(subset))
	var ranks []int
	var top1, top3, top10, miss int
	t1 := time.Now()
	for _, c := range subset {
		res, err := engine.Search(context.Background(), c.Title, c.Lang, *limit, search.Filter{})
		if err != nil {
			continue
		}
		rank := 0
		for i, r := range res {
			rp := relpath(r.Path, *contentDir)
			if rp == c.Path {
				rank = i + 1
				break
			}
		}
		ranks = append(ranks, rank)
		switch {
		case rank == 1:
			top1++
			top3++
			top10++
		case rank > 0 && rank <= 3:
			top3++
			top10++
		case rank > 0 && rank <= 10:
			top10++
		default:
			miss++
		}
		t1Title := ""
		t1Score := 0.0
		if len(res) > 0 {
			t1Title = res[0].Title
			t1Score = res[0].Score
		}
		results = append(results, qres{
			Query:    c.Title,
			Target:   c.Path,
			Rank:     rank,
			Top1:     t1Title,
			TopScore: t1Score,
			Stem:     stemmer.StemText(c.Title, c.Lang),
		})
	}
	took := time.Since(t1).Seconds()

	n := len(ranks)
	mrr := 0.0
	for _, r := range ranks {
		if r > 0 {
			mrr += 1.0 / float64(r)
		}
	}
	if n > 0 {
		mrr /= float64(n)
	}

	fmt.Println()
	fmt.Println("===== RESULTS =====")
	fmt.Printf("queries:       %d  (took %.1fs, %.1f q/s)\n", n, took, float64(n)/took)
	fmt.Printf("P@1 (exact):   %.3f  (%d/%d)\n", float64(top1)/float64(n), top1, n)
	fmt.Printf("Recall@3:      %.3f  (%d/%d)\n", float64(top3)/float64(n), top3, n)
	fmt.Printf("Recall@10:     %.3f  (%d/%d)\n", float64(top10)/float64(n), top10, n)
	fmt.Printf("MRR@10:        %.3f\n", mrr)
	fmt.Printf("Missing:       %d  (%.1f%%)\n", miss, 100*float64(miss)/float64(n))

	// Histogram of ranks
	bucket := map[string]int{}
	for _, r := range ranks {
		switch {
		case r == 0:
			bucket["miss"]++
		case r == 1:
			bucket["1"]++
		case r == 2:
			bucket["2"]++
		case r == 3:
			bucket["3"]++
		case r <= 5:
			bucket["4-5"]++
		case r <= 10:
			bucket["6-10"]++
		}
	}
	fmt.Println()
	fmt.Println("Rank distribution:")
	for _, k := range []string{"1", "2", "3", "4-5", "6-10", "miss"} {
		fmt.Printf("  %-5s %d\n", k, bucket[k])
	}

	if *showFails > 0 {
		sort.Slice(results, func(i, j int) bool {
			if results[i].Rank == 0 && results[j].Rank != 0 {
				return true
			}
			if results[i].Rank != 0 && results[j].Rank == 0 {
				return false
			}
			return results[i].Rank > results[j].Rank
		})
		fmt.Println()
		fmt.Printf("Top %d failures:\n", *showFails)
		printed := 0
		for _, r := range results {
			if r.Rank == 1 {
				break
			}
			fmt.Printf("  rank=%-4s q=%-30s top1=%-30s target=%s\n",
				rankStr(r.Rank), trunc(r.Query, 30), trunc(r.Top1, 30), r.Target)
			printed++
			if printed >= *showFails {
				break
			}
		}
	}

	if *dumpJSON != "" {
		f, err := os.Create(*dumpJSON)
		if err != nil {
			die("dump: %v", err)
		}
		enc := json.NewEncoder(f)
		for _, r := range results {
			_ = enc.Encode(r)
		}
		f.Close()
		fmt.Printf("\nwrote per-query results to %s\n", *dumpJSON)
	}
}

func rankStr(r int) string {
	if r == 0 {
		return "miss"
	}
	return fmt.Sprintf("%d", r)
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

func walkContent(root string) []string {
	var out []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	return out
}

func indexFile(ctx context.Context, ix *index.Indexer, embedFn embed.Func, path, root, defLang string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	md, err := markdown.ParseWithDefault(data, defLang)
	if err != nil {
		return "", "", err
	}
	if md.Title == "" {
		return "", "", nil
	}
	stemmed := stemmer.StemText(md.Text, md.Lang)
	pieces := chunker.Chunk(md.Text, chunker.DefaultMaxChars)
	if len(pieces) == 0 {
		pieces = []string{md.Title}
	}
	chunks := make([]index.Chunk, 0, len(pieces))
	for i, p := range pieces {
		v, err := embedFn(ctx, p)
		if err != nil {
			return "", "", err
		}
		chunks = append(chunks, index.Chunk{Index: i, Text: p, Embedding: v})
	}
	hash := sha256.Sum256(data)
	doc := index.Document{
		Path:        path,
		Hash:        fmt.Sprintf("%x", hash[:]),
		Title:       md.Title,
		Lang:        md.Lang,
		Source:      "giza-eval",
		DocType:     "markdown",
		StemmedText: stemmed,
		Chunks:      chunks,
	}
	if err := ix.Index(ctx, doc); err != nil {
		return "", "", err
	}
	return md.Title, md.Lang, nil
}

func relpath(p, root string) string {
	if root == "" {
		return p
	}
	rp, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return rp
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
