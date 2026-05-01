package search

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/enekos/marrow/internal/config"
	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/stemmer"
)

// Config holds tunable parameters for the search engine.
// All fields have sensible defaults via DefaultConfig.
type Config struct {
	RRFK                 float64
	ScoreBlendAlpha      float64
	FTSWeight            float64
	VecWeight            float64
	BM25TitleWeight      float64
	BM25ContentWeight    float64
	MaxRecencyDays       float64
	RecencyBoostMax      float64
	SnippetMaxTokens     int
	FallbackSnippetChars int
	FetchMultiplierFTS   int
	FetchMultiplierVec   int
	PhraseBoost          float64
	TitleBoostCoeff      float64
	MaxChunksPerDoc      int
	ChunkBoost2          float64
	ChunkBoost3Plus      float64
}

// DefaultConfig returns the built-in search defaults. These values are tuned
// for a hybrid BM25+vector ranking on small-to-medium documentation corpora.
func DefaultConfig() Config {
	return Config{
		RRFK:                 60,
		ScoreBlendAlpha:      0.35,
		FTSWeight:            0.7,
		VecWeight:            0.3,
		BM25TitleWeight:      5.0,
		BM25ContentWeight:    1.0,
		MaxRecencyDays:       180.0,
		RecencyBoostMax:      0.05,
		SnippetMaxTokens:     32,
		FallbackSnippetChars: 300,
		FetchMultiplierFTS:   3,
		FetchMultiplierVec:   10,
		PhraseBoost:          1.10,
		TitleBoostCoeff:      0.5,
		MaxChunksPerDoc:      3,
		ChunkBoost2:          1.02,
		ChunkBoost3Plus:      1.04,
	}
}

// Result represents a single search result.
//
// Snippet is a short excerpt extracted around the strongest match and is
// intended to be fed directly into an LLM context window. Highlights lists
// the stemmed query tokens that contributed to the match (useful for
// client-side rendering and for signalling salience to an LLM).
// TokenEstimate is a coarse estimate of the snippet's token count
// (len/4 heuristic) to help callers budget context windows.
//
// Context contains the full text of the best matching chunk(s) and is
// significantly longer than Snippet. It is designed for LLM consumption
// where more surrounding text improves relevance judgement.
// MatchReasons explains why the document was selected (e.g. "semantic",
// "fts_content", "exact_phrase", "title_match", "recency_boost").
// ChunkMatches is the number of semantically similar chunks found for
// this document.
type Result struct {
	ID            int64    `json:"id"`
	Path          string   `json:"path"`
	Title         string   `json:"title"`
	DocType       string   `json:"doc_type"`
	Score         float64  `json:"score"`
	Snippet       string   `json:"snippet,omitempty"`
	Context       string   `json:"context,omitempty"`
	Highlights    []string `json:"highlights,omitempty"`
	TokenEstimate int      `json:"token_estimate,omitempty"`
	ContextTokens int      `json:"context_tokens,omitempty"`
	MatchReasons  []string `json:"match_reasons,omitempty"`
	ChunkMatches  int      `json:"chunk_matches,omitempty"`
}

// Filter constrains search results.
type Filter struct {
	Sources []string
	DocType string
	Lang    string
	// HighlightFormat controls how matched terms are marked in snippets.
	// "" (default) returns plain text with no markup; "html" wraps each
	// matched term in <mark> tags and HTML-escapes the surrounding text.
	HighlightFormat string
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
	cfg         Config
	DetectLang  bool
	DefaultLang string
	ftsSQL      string   // precompiled FTS query template
	vecSQL      string   // precompiled vector query
	metaSQLTmpl string   // metadata query template up to the IN clause
	argsPool    sync.Pool // pool for []any metadata arg slices
}

// NewEngine creates a search engine with default configuration.
// embedFn must not be nil; callers should obtain one from embed.NewProvider
// (or embed.NewMock for tests).
func NewEngine(database DBConn, embedFn embed.Func) *Engine {
	return NewEngineWithConfig(database, embedFn, nil)
}

// NewEngineWithConfig creates a search engine with the given configuration.
// A nil cfg falls back to DefaultConfig().
func NewEngineWithConfig(database DBConn, embedFn embed.Func, cfg *Config) *Engine {
	if embedFn == nil {
		panic("search.NewEngine: embedFn must not be nil")
	}
	c := DefaultConfig()
	if cfg != nil {
		c = *cfg
	}
	ftsSQL := fmt.Sprintf(
		`SELECT rowid, bm25(documents_fts, %g, %g), snippet(documents_fts, 1, ?, ?, '…', %d) `+
			`FROM documents_fts WHERE documents_fts MATCH ? ORDER BY bm25(documents_fts, %g, %g) LIMIT ?`,
		c.BM25TitleWeight, c.BM25ContentWeight, c.SnippetMaxTokens,
		c.BM25TitleWeight, c.BM25ContentWeight,
	)
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
	metaSQLTmpl := `SELECT id, path, title, lang, doc_type, source, last_modified FROM documents WHERE id IN (`
	e := &Engine{
		db:          database,
		embedFn:     embedFn,
		cfg:         c,
		DetectLang:  true,
		DefaultLang: config.DefaultLang,
		ftsSQL:      ftsSQL,
		vecSQL:      vecSQL,
		metaSQLTmpl: metaSQLTmpl,
	}
	e.argsPool = sync.Pool{New: func() any { return make([]any, 0, 256) }}
	return e
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

	phrase, _, stemmedQuery, stemmedTokens, err := e.prepareQuery(query, langHint)
	if err != nil {
		return nil, err
	}
	ftsExpr := buildFTSMatchExpr(stemmedTokens)
	if ftsExpr == "" {
		ftsExpr = stemmedQuery
	}

	qblob, err := e.embedQuery(ctx, phrase, query)
	if err != nil {
		return nil, err
	}

	filterSQL, filterArgs := buildFilterSQL(filter)

	// Run FTS and vector queries concurrently since they are independent.
	var ftsRes *ftsResult
	var vecRes *vecResult
	var ftsErr, vecErr error
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		ftsRes, ftsErr = e.queryFTS(ctx, ftsExpr, limit, filter.HighlightFormat)
	}()

	go func() {
		defer wg.Done()
		vecRes, vecErr = e.queryVectors(ctx, qblob, limit)
	}()

	wg.Wait()
	if ftsErr != nil {
		return nil, ftsErr
	}
	if vecErr != nil {
		return nil, vecErr
	}

	phraseDocIDs, err := e.detectPhraseMatches(ctx, phrase, limit)
	if err != nil {
		return nil, err
	}

	scoredDocs := e.computeScores(ftsRes, vecRes)

	// Batch-fetch metadata to eliminate N+1 queries.
	metadata, err := e.fetchMetadata(ctx, scoredDocs, filterSQL, filterArgs)
	if err != nil {
		return nil, err
	}

	results := e.buildResults(scoredDocs, metadata, ftsRes, vecRes, phraseDocIDs, phrase, stemmedTokens)

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

	e.enrichResults(results, stemmedTokens)

	return results, nil
}

// ---------------------------------------------------------------------------
// Query preparation
// ---------------------------------------------------------------------------

func (e *Engine) prepareQuery(query, langHint string) (phrase, lang, stemmedQuery string, stemmedTokens []string, err error) {
	phrase = stripOuterQuotes(query)

	lang = langHint
	if lang == "" {
		if e.DetectLang {
			lang = stemmer.DetectLanguage(query)
		} else {
			lang = e.DefaultLang
		}
	}
	if lang == "" {
		lang = config.DefaultLang
	}
	stemmedQuery = stemmer.StemText(query, lang)
	stemmedTokens = strings.Fields(stemmedQuery)
	return phrase, lang, stemmedQuery, stemmedTokens, nil
}

func (e *Engine) embedQuery(ctx context.Context, phrase, query string) ([]byte, error) {
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
	return qblob, nil
}

// ---------------------------------------------------------------------------
// FTS query
// ---------------------------------------------------------------------------

type ftsResult struct {
	infos map[int64]ftsInfo
	order []int64
}

type ftsInfo struct {
	rank    int
	bm25    float64
	snippet string
}

// FTS5's snippet() takes literal pre/post markers and inserts them verbatim
// into the matched text. To produce safe HTML, we ask SQLite for sentinel
// bytes that cannot appear in real content, then HTML-escape the snippet and
// replace the sentinels with <mark> tags.
const (
	highlightOpenSentinel  = "\x01"
	highlightCloseSentinel = "\x02"
)

func (e *Engine) queryFTS(ctx context.Context, stemmedQuery string, limit int, highlightFormat string) (*ftsResult, error) {
	openMark, closeMark := "", ""
	if highlightFormat == "html" {
		openMark = highlightOpenSentinel
		closeMark = highlightCloseSentinel
	}

	rows, err := e.db.QueryContext(ctx, e.ftsSQL, openMark, closeMark, stemmedQuery, limit*e.cfg.FetchMultiplierFTS)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer rows.Close()

	res := &ftsResult{infos: make(map[int64]ftsInfo, limit*e.cfg.FetchMultiplierFTS), order: make([]int64, 0, limit*e.cfg.FetchMultiplierFTS)}
	rank := 1
	for rows.Next() {
		var id int64
		var bm25 float64
		var snip string
		if err := rows.Scan(&id, &bm25, &snip); err != nil {
			return nil, fmt.Errorf("scan fts row: %w", err)
		}
		if highlightFormat == "html" {
			snip = formatSnippetHTML(snip)
		}
		res.infos[id] = ftsInfo{rank: rank, bm25: bm25, snippet: snip}
		res.order = append(res.order, id)
		rank++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fts rows: %w", err)
	}
	return res, nil
}

func formatSnippetHTML(snip string) string {
	escaped := html.EscapeString(snip)
	escaped = strings.ReplaceAll(escaped, highlightOpenSentinel, "<mark>")
	escaped = strings.ReplaceAll(escaped, highlightCloseSentinel, "</mark>")
	return escaped
}

// ---------------------------------------------------------------------------
// Vector query
// ---------------------------------------------------------------------------

type vecResult struct {
	infos         map[int64]vecInfo
	order         []int64
	bestChunkText map[int64]string
	docChunks     map[int64][]string
}

type vecInfo struct {
	rank     int
	distance float64
}

func (e *Engine) queryVectors(ctx context.Context, qblob []byte, limit int) (*vecResult, error) {
	rows, err := e.db.QueryContext(ctx, e.vecSQL, qblob, limit*e.cfg.FetchMultiplierVec)
	if err != nil {
		return nil, fmt.Errorf("vec query: %w", err)
	}
	defer rows.Close()

	res := &vecResult{
		infos:         make(map[int64]vecInfo, limit*e.cfg.FetchMultiplierVec),
		bestChunkText: make(map[int64]string),
		docChunks:     make(map[int64][]string),
		order:         make([]int64, 0, limit*e.cfg.FetchMultiplierVec),
	}
	rank := 1
	for rows.Next() {
		var docID int64
		var dist float64
		var text string
		if err := rows.Scan(&docID, &dist, &text); err != nil {
			return nil, fmt.Errorf("scan vec row: %w", err)
		}
		if _, seen := res.infos[docID]; !seen {
			res.infos[docID] = vecInfo{rank: rank, distance: dist}
			res.order = append(res.order, docID)
			res.bestChunkText[docID] = text
			rank++
		}
		if len(res.docChunks[docID]) < e.cfg.MaxChunksPerDoc {
			res.docChunks[docID] = append(res.docChunks[docID], text)
		}
		if rank > limit*e.cfg.FetchMultiplierVec {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vec rows: %w", err)
	}
	return res, nil
}

// ---------------------------------------------------------------------------
// Phrase detection
// ---------------------------------------------------------------------------

func (e *Engine) detectPhraseMatches(ctx context.Context, phrase string, limit int) (map[int64]struct{}, error) {
	if phrase == "" {
		return map[int64]struct{}{}, nil
	}
	ftsPhrase := `"` + strings.ReplaceAll(phrase, `"`, ``) + `"`
	rows, err := e.db.QueryContext(ctx,
		`SELECT rowid FROM documents_fts WHERE documents_fts MATCH ? LIMIT ?`,
		ftsPhrase, limit*e.cfg.FetchMultiplierFTS)
	if err != nil {
		return nil, fmt.Errorf("phrase query: %w", err)
	}
	defer rows.Close()

	ids := make(map[int64]struct{})
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan phrase row: %w", err)
		}
		ids[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phrase rows: %w", err)
	}
	return ids, nil
}

// ---------------------------------------------------------------------------
// Score computation
// ---------------------------------------------------------------------------

type scoredDoc struct {
	id    int64
	score float64
}

func (e *Engine) computeScores(ftsRes *ftsResult, vecRes *vecResult) []scoredDoc {
	allIDs := make(map[int64]struct{}, len(ftsRes.infos)+len(vecRes.infos))
	for id := range ftsRes.infos {
		allIDs[id] = struct{}{}
	}
	for id := range vecRes.infos {
		allIDs[id] = struct{}{}
	}

	scoredDocs := make([]scoredDoc, 0, len(allIDs))
	for id := range allIDs {
		var s float64
		if info, ok := ftsRes.infos[id]; ok {
			rrf := 1.0 / (e.cfg.RRFK + float64(info.rank))
			norm := 1.0 / (1.0 - info.bm25)
			s += e.cfg.FTSWeight * ((1.0-e.cfg.ScoreBlendAlpha)*rrf + e.cfg.ScoreBlendAlpha*norm)
		}
		if info, ok := vecRes.infos[id]; ok {
			rrf := 1.0 / (e.cfg.RRFK + float64(info.rank))
			norm := 1.0 / (1.0 + info.distance)
			s += e.cfg.VecWeight * ((1.0-e.cfg.ScoreBlendAlpha)*rrf + e.cfg.ScoreBlendAlpha*norm)
		}
		scoredDocs = append(scoredDocs, scoredDoc{id: id, score: s})
	}
	return scoredDocs
}

// ---------------------------------------------------------------------------
// Batch metadata fetch (eliminates N+1)
// ---------------------------------------------------------------------------

type documentMeta struct {
	path      string
	title     string
	lang      string
	docType   string
	source    string
	lastMod   sql.NullTime
}

func (e *Engine) fetchMetadata(ctx context.Context, docs []scoredDoc, filterSQL string, filterArgs []any) (map[int64]documentMeta, error) {
	if len(docs) == 0 {
		return map[int64]documentMeta{}, nil
	}

	placeholders := make([]string, len(docs))
	args := e.argsPool.Get().([]any)
	args = args[:0]
	for i, d := range docs {
		placeholders[i] = "?"
		args = append(args, d.id)
	}

	q := e.metaSQLTmpl + strings.Join(placeholders, ",") + `)`
	if filterSQL != "" {
		q += " AND " + filterSQL
		args = append(args, filterArgs...)
	}

	rows, err := e.db.QueryContext(ctx, q, args...)
	if err != nil {
		e.argsPool.Put(args)
		return nil, fmt.Errorf("fetch metadata: %w", err)
	}
	defer func() {
		rows.Close()
		e.argsPool.Put(args)
	}()

	result := make(map[int64]documentMeta, len(docs))
	for rows.Next() {
		var m documentMeta
		var id int64
		if err := rows.Scan(&id, &m.path, &m.title, &m.lang, &m.docType, &m.source, &m.lastMod); err != nil {
			return nil, fmt.Errorf("scan metadata: %w", err)
		}
		result[id] = m
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("metadata rows: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Result assembly
// ---------------------------------------------------------------------------

func (e *Engine) buildResults(
	scoredDocs []scoredDoc,
	metadata map[int64]documentMeta,
	ftsRes *ftsResult,
	vecRes *vecResult,
	phraseDocIDs map[int64]struct{},
	phrase string,
	stemmedTokens []string,
) []Result {
	now := time.Now()
	results := make([]Result, 0, len(scoredDocs))

	// Pre-lower phrase once for case-insensitive title matching.
	phraseLower := ""
	if phrase != "" {
		phraseLower = strings.ToLower(phrase)
	}

	for _, s := range scoredDocs {
		meta, ok := metadata[s.id]
		if !ok {
			continue
		}
		score := s.score

		stemmedTitle := stemmer.StemText(meta.title, meta.lang)
		tb := titleBoost(stemmedTitle, stemmedTokens, e.cfg.TitleBoostCoeff)
		score *= tb

		if phraseLower != "" && strings.Contains(strings.ToLower(meta.title), phraseLower) {
			score *= e.cfg.PhraseBoost
		}

		if _, ok := phraseDocIDs[s.id]; ok {
			score *= e.cfg.PhraseBoost
		}

		rb := 1.0
		if meta.lastMod.Valid {
			rb = recencyBoost(meta.lastMod.Time, now, e.cfg.MaxRecencyDays, e.cfg.RecencyBoostMax)
			score *= rb
		}

		snippet := ftsRes.infos[s.id].snippet
		if snippet == "" {
			if ct, ok := vecRes.bestChunkText[s.id]; ok {
				snippet = ct
			}
		}

		contextChunks := vecRes.docChunks[s.id]
		contextText := ""
		if len(contextChunks) > 0 {
			contextText = strings.Join(contextChunks, "\n\n---\n\n")
		} else if snippet != "" {
			contextText = snippet
		}

		score *= chunkBoost(len(contextChunks), e.cfg.ChunkBoost2, e.cfg.ChunkBoost3Plus)

		reasons := make([]string, 0, 4)
		if _, ok := ftsRes.infos[s.id]; ok {
			reasons = append(reasons, "fts_content")
		}
		if _, ok := vecRes.infos[s.id]; ok {
			reasons = append(reasons, "semantic")
		}
		if _, ok := phraseDocIDs[s.id]; ok {
			reasons = append(reasons, "exact_phrase")
		}
		if tb > 1.0 {
			reasons = append(reasons, "title_match")
		}
		if rb > 1.0 {
			reasons = append(reasons, "recency_boost")
		}

		results = append(results, Result{
			ID:           s.id,
			Path:         meta.path,
			Title:        meta.title,
			DocType:      meta.docType,
			Score:        score,
			Snippet:      snippet,
			Context:      contextText,
			MatchReasons: reasons,
			ChunkMatches: len(contextChunks),
		})
	}

	return results
}

// ---------------------------------------------------------------------------
// Enrichment
// ---------------------------------------------------------------------------

func (e *Engine) enrichResults(results []Result, stemmedTokens []string) {
	if len(results) == 0 {
		return
	}

	// Compute highlights once — they are identical for every result.
	var highlights []string
	if len(stemmedTokens) > 0 {
		seen := make(map[string]struct{}, len(stemmedTokens))
		highlights = make([]string, 0, len(stemmedTokens))
		for _, t := range stemmedTokens {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			highlights = append(highlights, t)
		}
	}

	for i := range results {
		if results[i].Snippet != "" && len(results[i].Snippet) > e.cfg.FallbackSnippetChars {
			results[i].Snippet = strings.TrimSpace(results[i].Snippet[:e.cfg.FallbackSnippetChars]) + "…"
		}
		results[i].Highlights = highlights
		results[i].TokenEstimate = estimateTokens(results[i].Snippet)
		results[i].ContextTokens = estimateTokens(results[i].Context)
	}
}

func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 3) / 4
}

func buildFilterSQL(f Filter) (string, []any) {
	var parts []string
	var args []any
	if len(f.Sources) > 0 {
		placeholders := make([]string, len(f.Sources))
		for i := range f.Sources {
			placeholders[i] = "?"
			args = append(args, f.Sources[i])
		}
		parts = append(parts, fmt.Sprintf("source IN (%s)", strings.Join(placeholders, ",")))
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

// buildFTSMatchExpr produces a recall-friendly FTS5 MATCH expression from a
// list of stemmed query tokens. Each token is sanitized (FTS5 syntax chars
// stripped), wrapped as a quoted prefix term ("tok"*), and OR-joined. Returns
// "" if no usable tokens remain — the caller should fall back to the raw
// stemmed query.
func buildFTSMatchExpr(stemmedTokens []string) string {
	if len(stemmedTokens) == 0 {
		return ""
	}
	parts := make([]string, 0, len(stemmedTokens))
	seen := make(map[string]struct{}, len(stemmedTokens))
	for _, t := range stemmedTokens {
		clean := sanitizeFTSToken(t)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		parts = append(parts, `"`+clean+`"*`)
	}
	if len(parts) == 0 {
		return ""
	}
	// Space-separated terms are ANDed by FTS5 — this preserves precision
	// while quoted-prefix terms ("tok"*) widen recall to handle morphology
	// the stemmer didn't normalize.
	return strings.Join(parts, " ")
}

// sanitizeFTSToken strips characters that have special meaning in FTS5 MATCH
// expressions so a stemmed token can be safely embedded as a quoted prefix
// term. We keep letters, digits, and underscore; anything else is dropped.
func sanitizeFTSToken(t string) string {
	var b strings.Builder
	b.Grow(len(t))
	for _, r := range t {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
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

func titleBoost(stemmedTitle string, stemmedTokens []string, coeff float64) float64 {
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
	return 1.0 + coeff*coverage
}

func recencyBoost(lastModified, now time.Time, maxDays, maxBoost float64) float64 {
	days := now.Sub(lastModified).Hours() / 24.0
	if days <= 0 {
		return 1.0 + maxBoost
	}
	if days >= maxDays {
		return 1.0
	}
	factor := 1.0 - (days / maxDays)
	return 1.0 + maxBoost*factor
}

func chunkBoost(n int, boost2, boost3plus float64) float64 {
	switch n {
	case 0, 1:
		return 1.0
	case 2:
		return boost2
	default:
		return boost3plus
	}
}
