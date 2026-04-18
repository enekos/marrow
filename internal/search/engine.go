package search

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/stemmer"
)

const (
	rrfK            = 60
	scoreBlendAlpha = 0.35
	maxRecencyDays  = 180.0
	recencyBoostMax = 0.05
)

// Result represents a single search result.
//
// Snippet is a short excerpt extracted around the strongest match and is
// intended to be fed directly into an LLM context window. Highlights lists
// the stemmed query tokens that contributed to the match (useful for
// client-side rendering and for signalling salience to an LLM).
// TokenEstimate is a coarse estimate of the snippet's token count
// (len/4 heuristic) to help callers budget context windows.
type Result struct {
	ID            int64    `json:"id"`
	Path          string   `json:"path"`
	Title         string   `json:"title"`
	DocType       string   `json:"doc_type"`
	Score         float64  `json:"score"`
	Snippet       string   `json:"snippet,omitempty"`
	Highlights    []string `json:"highlights,omitempty"`
	TokenEstimate int      `json:"token_estimate,omitempty"`
}

// snippetMaxTokens controls FTS5 snippet() length. 32 FTS tokens is ~150–200
// characters in English, which fits comfortably in small context budgets.
const snippetMaxTokens = 32

// fallbackSnippetChars bounds content-prefix fallbacks for vector-only hits.
const fallbackSnippetChars = 300

// FTS5 bm25 column weights: title is weighted 5x relative to content. BM25
// returns non-positive scores where more-negative = better match, so a higher
// weight makes title matches dominate the ranking.
const (
	bm25TitleWeight   = 5.0
	bm25ContentWeight = 1.0
)

// Filter constrains search results.
type Filter struct {
	Source   string
	DocType  string
	Lang     string
}

// DBConn is the subset of database operations required by Engine.
type DBConn interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Engine executes hybrid search queries.
type Engine struct {
	db          DBConn
	embedFn     embed.Func
	DetectLang  bool
	DefaultLang string
}

// NewEngine creates a search engine. embedFn must not be nil; callers should
// obtain one from embed.NewProvider (or embed.NewMock for tests).
func NewEngine(database DBConn, embedFn embed.Func) *Engine {
	if embedFn == nil {
		panic("search.NewEngine: embedFn must not be nil")
	}
	return &Engine{
		db:          database,
		embedFn:     embedFn,
		DetectLang:  true,
		DefaultLang: "en",
	}
}

// Search runs a hybrid query and returns ranked results.
// If langHint is non-empty, it overrides language detection for stemming.
func (e *Engine) Search(ctx context.Context, query string, langHint string, limit int, filter Filter) ([]Result, error) {
	if limit <= 0 {
		limit = 10
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []Result{}, nil
	}

	// Preserve phrase for exact-match boosting, but use unquoted text for embedding.
	phrase := stripOuterQuotes(query)

	// Use default language for query stemming (frontmatter not available for queries)
	lang := langHint
	if lang == "" {
		if e.DetectLang {
			lang = stemmer.DetectLanguage(query)
		} else {
			lang = e.DefaultLang
		}
	}
	if lang == "" {
		lang = "en"
	}
	stemmedQuery := stemmer.StemText(query, lang)
	stemmedTokens := strings.Fields(stemmedQuery)

	// Generate query embedding
	embedText := query
	if phrase != query {
		embedText = phrase
	}
	qvec, err := e.embedFn(ctx, embedText)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	qblob, err := db.SerializeVec(qvec)
	if err != nil {
		return nil, fmt.Errorf("serialize query vec: %w", err)
	}

	// Build metadata filter subquery for joining with FTS/vec results.
	filterSQL, filterArgs := buildFilterSQL(filter)

	// 1. Query FTS5 and build rank map with BM25 scores and snippets.
	// snippet() extracts a short excerpt around the match; we scope it to the
	// content column (index 1) since title matches are already surfaced
	// verbatim via the Title field.
	type ftsInfo struct {
		rank    int
		bm25    float64
		snippet string
	}
	ftsInfos := make(map[int64]ftsInfo)
	var ftsOrder []int64
	ftsSQL := fmt.Sprintf(
		`SELECT rowid, bm25(documents_fts, %g, %g), snippet(documents_fts, 1, '', '', '…', %d) `+
			`FROM documents_fts WHERE documents_fts MATCH ? ORDER BY bm25(documents_fts, %g, %g) LIMIT ?`,
		bm25TitleWeight, bm25ContentWeight, snippetMaxTokens,
		bm25TitleWeight, bm25ContentWeight,
	)
	ftsRows, err := e.db.QueryContext(ctx, ftsSQL, stemmedQuery, limit*3)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer ftsRows.Close()
	rank := 1
	for ftsRows.Next() {
		var id int64
		var bm25 float64
		var snip string
		if err := ftsRows.Scan(&id, &bm25, &snip); err != nil {
			return nil, err
		}
		ftsInfos[id] = ftsInfo{rank: rank, bm25: bm25, snippet: snip}
		ftsOrder = append(ftsOrder, id)
		rank++
	}
	if err := ftsRows.Err(); err != nil {
		return nil, err
	}

	// 2. Query vectors over chunks and collapse to best chunk per document.
	//
	// Each vector now belongs to a chunk, so k nearest chunks can map to
	// fewer distinct documents. We over-fetch (limit*10) from vec and then
	// keep only the first — i.e. lowest-distance — chunk seen per document.
	// This gives us limit*3 unique docs in rank order while still letting
	// vec0 push the LIMIT into its KNN traversal.
	type vecInfo struct {
		rank     int
		distance float64
	}
	vecInfos := make(map[int64]vecInfo)
	var vecOrder []int64
	bestChunkText := make(map[int64]string)
	vecSQL := `
		SELECT c.document_id, v.distance, c.text
		FROM (
			SELECT rowid, distance
			FROM documents_vec
			WHERE embedding MATCH ?
			ORDER BY distance
			LIMIT ?
		) v
		JOIN document_chunks c ON c.id = v.rowid
		ORDER BY v.distance
	`
	vecRows, err := e.db.QueryContext(ctx, vecSQL, qblob, limit*10)
	if err != nil {
		return nil, fmt.Errorf("vec query: %w", err)
	}
	defer vecRows.Close()
	rank = 1
	for vecRows.Next() {
		var docID int64
		var dist float64
		var text string
		if err := vecRows.Scan(&docID, &dist, &text); err != nil {
			return nil, err
		}
		if _, seen := vecInfos[docID]; seen {
			continue
		}
		vecInfos[docID] = vecInfo{rank: rank, distance: dist}
		vecOrder = append(vecOrder, docID)
		bestChunkText[docID] = text
		rank++
		if rank > limit*3 {
			break
		}
	}
	if err := vecRows.Err(); err != nil {
		return nil, err
	}

	// 3. Normalise BM25 and vector similarities independently
	ftsNormMap := make(map[int64]float64, len(ftsOrder))
	for _, id := range ftsOrder {
		bm25 := ftsInfos[id].bm25
		if bm25 < 0 {
			bm25 = 0
		}
		ftsNormMap[id] = 1.0 / (1.0 + bm25)
	}
	vecNormMap := make(map[int64]float64, len(vecOrder))
	for _, id := range vecOrder {
		vecNormMap[id] = 1.0 / (1.0 + vecInfos[id].distance)
	}

	// 4. RRF merge with score blending
	allIDs := make(map[int64]struct{})
	for id := range ftsInfos {
		allIDs[id] = struct{}{}
	}
	for id := range vecInfos {
		allIDs[id] = struct{}{}
	}

	// 5. Exact phrase match detection
	phraseDocIDs := make(map[int64]struct{})
	if phrase != "" {
		ftsPhrase := `"` + phrase + `"`
		phRows, err := e.db.QueryContext(ctx,
			`SELECT rowid FROM documents_fts WHERE documents_fts MATCH ? LIMIT ?`,
			ftsPhrase, limit*3)
		if err == nil {
			for phRows.Next() {
				var id int64
				if err := phRows.Scan(&id); err == nil {
					phraseDocIDs[id] = struct{}{}
				}
			}
			phRows.Close()
		}
	}

	type scored struct {
		id    int64
		score float64
	}
	scoredDocs := make([]scored, 0, len(allIDs))
	for id := range allIDs {
		var s float64
		if info, ok := ftsInfos[id]; ok {
			rrf := 1.0 / (rrfK + float64(info.rank))
			norm := ftsNormMap[id]
			s += 0.7 * ((1.0-scoreBlendAlpha)*rrf + scoreBlendAlpha*norm)
		}
		if info, ok := vecInfos[id]; ok {
			rrf := 1.0 / (rrfK + float64(info.rank))
			norm := vecNormMap[id]
			s += 0.3 * ((1.0-scoreBlendAlpha)*rrf + scoreBlendAlpha*norm)
		}
		scoredDocs = append(scoredDocs, scored{id: id, score: s})
	}

	// 6. Fetch metadata and apply heuristics + filters
	now := time.Now()
	results := make([]Result, 0, len(scoredDocs))
	for _, s := range scoredDocs {
		var path, title, docLang, docType, docSource string
		var lastMod sql.NullTime
		q := `SELECT path, title, lang, doc_type, source, last_modified FROM documents WHERE id = ?`
		args := []interface{}{s.id}
		if filterSQL != "" {
			q = `SELECT path, title, lang, doc_type, source, last_modified FROM documents WHERE id = ? AND ` + filterSQL
			args = append(args, filterArgs...)
		}
		err := e.db.QueryRowContext(ctx, q, args...).Scan(&path, &title, &docLang, &docType, &docSource, &lastMod)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		score := s.score

		stemmedTitle := stemmer.StemText(title, docLang)
		score *= titleBoost(stemmedTitle, stemmedTokens)

		if phrase != "" && strings.Contains(strings.ToLower(title), strings.ToLower(phrase)) {
			score *= 1.10
		}

		if _, ok := phraseDocIDs[s.id]; ok {
			score *= 1.10
		}

		if lastMod.Valid {
			score *= recencyBoost(lastMod.Time, now)
		}

		snippet := ftsInfos[s.id].snippet
		if snippet == "" {
			// Vector-only hit: use the best matching chunk text directly.
			if ct, ok := bestChunkText[s.id]; ok {
				snippet = ct
			}
		}
		results = append(results, Result{
			ID:      s.id,
			Path:    path,
			Title:   title,
			DocType: docType,
			Score:   score,
			Snippet: snippet,
		})
	}

	// 7. Sort descending with slices.SortFunc
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

	// 8. Attach highlights and token estimates. Snippets are already filled
	// from step 6 (FTS snippet or best-matching chunk text).
	e.enrichResults(results, stemmedTokens)

	return results, nil
}

// enrichResults attaches Highlights and TokenEstimate to each result.
// Snippet enrichment happens inline during result assembly.
func (e *Engine) enrichResults(results []Result, stemmedTokens []string) {
	for i := range results {
		if results[i].Snippet != "" && len(results[i].Snippet) > fallbackSnippetChars {
			// Chunk text can be up to chunker.DefaultMaxChars (~1500); cap
			// it here so clients don't accidentally blow their context
			// window on a single result.
			results[i].Snippet = strings.TrimSpace(results[i].Snippet[:fallbackSnippetChars]) + "…"
		}
		if len(stemmedTokens) > 0 {
			// Return unique tokens in stable order for deterministic output.
			seen := make(map[string]struct{}, len(stemmedTokens))
			hl := make([]string, 0, len(stemmedTokens))
			for _, t := range stemmedTokens {
				if _, ok := seen[t]; ok {
					continue
				}
				seen[t] = struct{}{}
				hl = append(hl, t)
			}
			results[i].Highlights = hl
		}
		results[i].TokenEstimate = estimateTokens(results[i].Snippet)
	}
}

// estimateTokens is a crude char/4 heuristic. Good enough for context-budget
// planning; callers needing exact counts should tokenize with their model.
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 3) / 4
}

func buildFilterSQL(f Filter) (string, []interface{}) {
	var parts []string
	var args []interface{}
	if f.Source != "" {
		parts = append(parts, "source = ?")
		args = append(args, f.Source)
	}
	if f.DocType != "" {
		parts = append(parts, "doc_type = ?")
		args = append(args, f.DocType)
	}
	if f.Lang != "" {
		parts = append(parts, "lang = ?")
		args = append(args, f.Lang)
	}
	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, " AND "), args
}

func stripOuterQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}

func titleBoost(stemmedTitle string, stemmedTokens []string) float64 {
	if len(stemmedTokens) == 0 {
		return 1.0
	}
	set := make(map[string]struct{}, len(stemmedTokens))
	for _, t := range stemmedTokens {
		set[t] = struct{}{}
	}
	matched := 0
	for _, t := range strings.Fields(stemmedTitle) {
		if _, ok := set[t]; ok {
			matched++
			delete(set, t) // count each query term at most once
		}
	}
	coverage := float64(matched) / float64(len(stemmedTokens))
	return 1.0 + 0.25*coverage
}

func recencyBoost(lastModified, now time.Time) float64 {
	days := now.Sub(lastModified).Hours() / 24.0
	if days <= 0 {
		return 1.0 + recencyBoostMax
	}
	if days >= maxRecencyDays {
		return 1.0
	}
	factor := 1.0 - (days / maxRecencyDays)
	return 1.0 + recencyBoostMax*factor
}


