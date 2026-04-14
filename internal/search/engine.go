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
// If langHint is non-empty, it overrides language detection for stemming.
func (e *Engine) Search(ctx context.Context, query string, langHint string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 10
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []Result{}, nil
	}

	// Use default language for query stemming (frontmatter not available for queries)
	lang := langHint
	if lang == "" {
		lang = detectQueryLang(query)
	}
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
		var path, title, docLang string
		err := e.db.QueryRowContext(ctx,
			`SELECT path, title, lang FROM documents WHERE id = ?`, s.id).Scan(&path, &title, &docLang)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		score := s.score
		if hasTokenOverlap(stemmer.StemText(title, docLang), stemmedTokens) {
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
	lower := strings.ToLower(query)
	tokens := stemmer.Tokenize(query)

	const (
		idxEn = 0
		idxEs = 1
		idxEu = 2
	)
	var scores [3]int

	// --- Character-level signals ------------------------------------------
	for _, r := range lower {
		switch r {
		case 'ñ', '¿', '¡':
			scores[idxEs] += 10
		case 'á', 'é', 'í', 'ó', 'ú', 'ü':
			scores[idxEs] += 5
		}
	}
	// tx is extremely rare in English/Spanish; tz is also strongly Basque.
	if strings.Contains(lower, "tx") {
		scores[idxEu] += 5
	}
	if strings.Contains(lower, "tz") {
		scores[idxEu] += 3
	}

	// --- Word-level signals -----------------------------------------------
	spanishWords := map[string]struct{}{
		"el": {}, "la": {}, "los": {}, "las": {},
		"un": {}, "una": {}, "unos": {}, "unas": {},
		"de": {}, "del": {}, "al": {}, "y": {}, "o": {}, "pero": {},
		"sin": {}, "con": {}, "por": {}, "para": {}, "en": {}, "a": {},
		"ante": {}, "bajo": {}, "desde": {}, "entre": {}, "hacia": {},
		"hasta": {}, "mediante": {}, "según": {}, "sobre": {}, "tras": {},
		"durante": {}, "excepto": {}, "salvo": {}, "como": {},
		"lo": {}, "le": {}, "les": {}, "me": {}, "te": {}, "se": {},
		"nos": {}, "os": {}, "que": {}, "qué": {}, "quien": {}, "quién": {},
		"cual": {}, "cuál": {}, "cuales": {}, "cuáles": {}, "cuyo": {},
		"cuya": {}, "cuyos": {}, "cuyas": {},
		"donde": {}, "dónde": {}, "cuando": {}, "cuándo": {},
		"cuanto": {}, "cuánto": {}, "cuanta": {}, "cuánta": {},
		"es": {}, "son": {}, "está": {}, "están": {}, "estoy": {},
		"estamos": {}, "estáis": {}, "fue": {}, "fueron": {},
		"ha": {}, "han": {}, "había": {}, "hay": {},
		"tengo": {}, "tiene": {}, "tienen": {}, "tenemos": {}, "tenéis": {},
		"hago": {}, "hace": {}, "hacen": {}, "hacemos": {}, "hacéis": {},
		"más": {}, "muy": {}, "mucho": {}, "mucha": {}, "muchos": {}, "muchas": {},
		"poco": {}, "poca": {}, "pocos": {}, "pocas": {},
		"todo": {}, "todos": {}, "toda": {}, "todas": {},
		"este": {}, "esta": {}, "estos": {}, "estas": {},
		"ese": {}, "esa": {}, "esos": {}, "esas": {},
		"aquel": {}, "aquella": {}, "aquellos": {}, "aquellas": {},
		"mi": {}, "mis": {}, "tu": {}, "tus": {}, "su": {}, "sus": {},
		"nuestro": {}, "nuestra": {}, "nuestros": {}, "nuestras": {},
		"vuestro": {}, "vuestra": {}, "vuestros": {}, "vuestras": {},
		"si": {}, "sino": {}, "también": {}, "ya": {}, "aún": {},
		"todavía": {}, "siempre": {}, "nunca": {}, "jamás": {},
		"aquí": {}, "ahí": {}, "allí": {}, "acá": {}, "allá": {},
		"ahora": {}, "antes": {}, "después": {}, "luego": {},
		"pronto": {}, "tarde": {}, "temprano": {},
		"bien": {}, "mal": {}, "mejor": {}, "peor": {},
		"cómo": {}, "porqué": {}, "porque": {}, "pues": {},
		"sí": {}, "no": {},
	}

	basqueWords := map[string]struct{}{
		"eta": {}, "edo": {}, "baina": {}, "ez": {}, "bai": {}, "ere": {},
		"bestela": {}, "gainera": {}, "beraz": {}, "ala": {}, "bada": {},
		"hura": {}, "hau": {}, "berori": {}, "hori": {},
		"nor": {}, "zer": {}, "nork": {}, "nori": {}, "noren": {},
		"non": {}, "nola": {}, "noiz": {}, "zergatik": {}, "zenbat": {},
		"ni": {}, "zu": {}, "gu": {}, "zuek": {}, "haiek": {},
		"nire": {}, "zure": {}, "haren": {}, "gure": {}, "zuen": {}, "haien": {},
		"da": {}, "dago": {}, "daude": {}, "du": {}, "ditu": {}, "dute": {},
		"izan": {}, "egin": {}, "bat": {}, "bi": {}, "guzti": {}, "gutxi": {},
		"oso": {}, "inoiz": {}, "beti": {}, "hemen": {}, "han": {}, "hortxe": {},
		"honela": {}, "euskal": {}, "euskara": {}, "herri": {}, "etxe": {},
		"etxea": {}, "izena": {}, "urte": {}, "egun": {}, "baietz": {}, "ezetz": {},
		"alde": {}, "arte": {}, "aurka": {}, "bezala": {}, "gisa": {},
		"kontra": {}, "ondo": {}, "zai": {}, "aurre": {}, "bitartean": {},
		"tartean": {}, "barruan": {}, "kanpoan": {}, "gainean": {},
		"azpian": {}, "aurrean": {}, "atzean": {}, "ondoren": {},
		"lehen": {}, "gero": {}, "orduan": {}, "orain": {},
	}

	englishWords := map[string]struct{}{
		"the": {}, "a": {}, "an": {}, "is": {}, "are": {}, "was": {}, "were": {},
		"be": {}, "been": {}, "being": {}, "have": {}, "has": {}, "had": {}, "do": {},
		"does": {}, "did": {}, "will": {}, "would": {}, "shall": {}, "should": {},
		"can": {}, "could": {}, "may": {}, "might": {}, "must": {}, "ought": {},
		"i": {}, "you": {}, "he": {}, "she": {}, "it": {}, "we": {}, "they": {},
		"me": {}, "him": {}, "her": {}, "us": {}, "them": {}, "my": {}, "your": {},
		"his": {}, "its": {}, "our": {}, "their": {}, "this": {}, "that": {},
		"these": {}, "those": {}, "of": {}, "in": {}, "to": {}, "for": {}, "with": {},
		"on": {}, "at": {}, "by": {}, "from": {}, "as": {}, "into": {}, "through": {},
		"during": {}, "before": {}, "after": {}, "above": {}, "below": {}, "between": {},
		"and": {}, "but": {}, "or": {}, "yet": {}, "so": {}, "if": {}, "because": {},
		"although": {}, "though": {}, "while": {}, "where": {}, "when": {},
		"which": {}, "who": {}, "whom": {}, "whose": {}, "what": {}, "whatever": {},
		"not": {}, "no": {}, "yes": {}, "there": {}, "then": {}, "than": {},
		"here": {}, "how": {}, "why": {}, "all": {}, "any": {}, "both": {},
		"each": {}, "few": {}, "more": {}, "most": {}, "other": {}, "some": {},
		"such": {}, "only": {}, "own": {}, "same": {}, "too": {}, "very": {},
		"just": {}, "now": {}, "also": {}, "back": {}, "still": {}, "already": {},
		"even": {}, "once": {}, "twice": {}, "again": {}, "always": {}, "never": {},
		"often": {}, "sometimes": {}, "usually": {}, "really": {}, "actually": {},
		"probably": {}, "maybe": {}, "perhaps": {}, "sure": {}, "well": {},
		"good": {}, "bad": {}, "new": {}, "old": {}, "first": {}, "last": {},
		"long": {}, "great": {}, "little": {}, "right": {}, "left": {}, "big": {},
		"small": {}, "large": {}, "next": {}, "early": {}, "young": {},
		"important": {}, "different": {}, "following": {}, "public": {}, "able": {},
	}

	for _, tok := range tokens {
		if _, ok := spanishWords[tok]; ok {
			scores[idxEs] += 2
		}
		if _, ok := basqueWords[tok]; ok {
			scores[idxEu] += 2
		}
		if _, ok := englishWords[tok]; ok {
			scores[idxEn] += 2
		}
	}

	// --- Resolve winner ---------------------------------------------------
	maxScore := scores[idxEn]
	lang := "en"
	if scores[idxEs] > maxScore {
		maxScore = scores[idxEs]
		lang = "es"
	}
	if scores[idxEu] > maxScore {
		lang = "eu"
	}
	return lang
}
