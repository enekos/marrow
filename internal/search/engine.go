package search

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/stemmer"
)

const rrfK = 60

// Result represents a single search result.
type Result struct {
	ID    int64   `json:"id"`
	Path  string  `json:"path"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

// Engine executes hybrid search queries.
type Engine struct {
	db      *db.DB
	embedFn embed.Func
}

// NewEngine creates a search engine.
func NewEngine(database *db.DB, embedFn embed.Func) *Engine {
	if embedFn == nil {
		embedFn = embed.NewMock()
	}
	return &Engine{
		db:      database,
		embedFn: embedFn,
	}
}

// Search runs a hybrid query and returns ranked results.
func (e *Engine) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 10
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []Result{}, nil
	}

	// Use default language for query stemming (frontmatter not available for queries)
	lang := detectQueryLang(query)
	stemmedQuery := stemmer.StemText(query, lang)
	stemmedTokens := strings.Fields(stemmedQuery)

	// Generate query embedding
	qvec, err := e.embedFn(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	qblob, err := db.SerializeVec(qvec)
	if err != nil {
		return nil, fmt.Errorf("serialize query vec: %w", err)
	}

	// 1. Query FTS5 and build rank map
	ftsRanks := make(map[int64]int)
	ftsRows, err := e.db.QueryContext(ctx,
		`SELECT rowid FROM documents_fts WHERE documents_fts MATCH ? ORDER BY bm25(documents_fts) LIMIT ?`,
		stemmedQuery, limit*3)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer ftsRows.Close()
	rank := 1
	for ftsRows.Next() {
		var id int64
		if err := ftsRows.Scan(&id); err != nil {
			return nil, err
		}
		ftsRanks[id] = rank
		rank++
	}
	if err := ftsRows.Err(); err != nil {
		return nil, err
	}

	// 2. Query Vector and build rank map
	vecRanks := make(map[int64]int)
	vecRows, err := e.db.QueryContext(ctx,
		`SELECT rowid FROM documents_vec WHERE embedding MATCH ? ORDER BY distance LIMIT ?`,
		qblob, limit*3)
	if err != nil {
		return nil, fmt.Errorf("vec query: %w", err)
	}
	defer vecRows.Close()
	rank = 1
	for vecRows.Next() {
		var id int64
		if err := vecRows.Scan(&id); err != nil {
			return nil, err
		}
		vecRanks[id] = rank
		rank++
	}
	if err := vecRows.Err(); err != nil {
		return nil, err
	}

	// 3. RRF merge
	allIDs := make(map[int64]struct{})
	for id := range ftsRanks {
		allIDs[id] = struct{}{}
	}
	for id := range vecRanks {
		allIDs[id] = struct{}{}
	}

	type scored struct {
		id    int64
		score float64
	}
	scoredDocs := make([]scored, 0, len(allIDs))
	for id := range allIDs {
		var s float64
		if r, ok := ftsRanks[id]; ok {
			s += 0.7 * (1.0 / (rrfK + float64(r)))
		}
		if r, ok := vecRanks[id]; ok {
			s += 0.3 * (1.0 / (rrfK + float64(r)))
		}
		scoredDocs = append(scoredDocs, scored{id: id, score: s})
	}

	// 4. Fetch metadata and apply title boost using stemmed token overlap
	results := make([]Result, 0, len(scoredDocs))
	for _, s := range scoredDocs {
		var path, title string
		err := e.db.QueryRowContext(ctx,
			`SELECT path, title FROM documents WHERE id = ?`, s.id).Scan(&path, &title)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		score := s.score
		if hasTokenOverlap(stemmer.StemText(title, lang), stemmedTokens) {
			score *= 1.2
		}
		results = append(results, Result{
			ID:    s.id,
			Path:  path,
			Title: title,
			Score: score,
		})
	}

	// 5. Sort descending with slices.SortFunc
	slices.SortFunc(results, func(a, b Result) int {
		if b.Score > a.Score {
			return 1
		}
		if b.Score < a.Score {
			return -1
		}
		return 0
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func hasTokenOverlap(stemmedText string, stemmedTokens []string) bool {
	if len(stemmedTokens) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(stemmedTokens))
	for _, t := range stemmedTokens {
		set[t] = struct{}{}
	}
	for _, t := range strings.Fields(stemmedText) {
		if _, ok := set[t]; ok {
			return true
		}
	}
	return false
}

func detectQueryLang(query string) string {
	// Very naive heuristic: check for common Spanish-specific characters/words.
	lower := strings.ToLower(query)
	spanishMarkers := []string{"ñ", "á", "é", "í", "ó", "ú", "ü", "¿", "¡"}
	for _, m := range spanishMarkers {
		if strings.Contains(lower, m) {
			return "es"
		}
	}
	return "en"
}
