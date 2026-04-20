// Package related builds high-quality "related article" recommendations for a
// markdown corpus by combining multiple signals and diversifying with MMR.
//
// Signals:
//   - semantic: cosine similarity of document-level mean-pooled chunk vectors
//   - lexical:  Jaccard overlap of per-document TF-IDF salient stemmed terms
//   - link:     directed graph of in-text internal links and co-citation
//   - category: shared Hugo front-matter categories
//
// The package is designed to run as a one-shot batch from the Marrow
// SQLite index plus a pointer to the source content directory (for link
// extraction and front-matter categories/slugs that are not stored in the
// index).
package related

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"marrow/internal/db"
	"marrow/internal/stemmer"
)

// Config tunes the related-article pipeline. Zero-valued fields fall back to
// DefaultConfig on Builder.Compute.
type Config struct {
	Limit int // number of related docs per source (default 10)

	// Signal weights. All signals are normalised to [0, 1] before scoring.
	WSem  float64 // semantic cosine (default 0.55)
	WLex  float64 // lexical salience Jaccard (default 0.20)
	WLink float64 // link graph (default 0.15)
	WCat  float64 // shared category (default 0.10)

	// MMR tradeoff between relevance and diversity. 1.0 = pure relevance,
	// 0.0 = pure diversity. Default 0.72.
	MMRLambda float64

	// TopKSalient is the number of TF-IDF-top stemmed terms retained per
	// document for the lexical-overlap signal. Default 24.
	TopKSalient int

	// CandidatePoolMultiplier controls how many candidates per source are
	// considered before MMR trims to Limit. Default 6.
	CandidatePoolMultiplier int

	// TrivialTitleOverlap: if a candidate's title stems are a subset of the
	// source's title stems (or vice versa) and the semantic score is below
	// this threshold, the candidate is demoted. Default 0.35.
	TrivialTitleThreshold float64

	// Workers for per-source computation. Default 8.
	Workers int

	// IgnoreSemantic forces WSem to 0 (for mock-embedding runs). Auto-set
	// by Builder.Load when all doc vectors look like mock vectors.
	IgnoreSemantic bool
}

// DefaultConfig returns the tuned defaults.
func DefaultConfig() Config {
	return Config{
		Limit:                   10,
		WSem:                    0.55,
		WLex:                    0.20,
		WLink:                   0.15,
		WCat:                    0.10,
		MMRLambda:               0.72,
		TopKSalient:             24,
		CandidatePoolMultiplier: 6,
		TrivialTitleThreshold:   0.35,
		Workers:                 8,
	}
}

// RelatedDoc is one output row.
type RelatedDoc struct {
	Path    string   `json:"path"`
	Title   string   `json:"title"`
	Slug    string   `json:"slug,omitempty"`
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons,omitempty"`
}

// docRecord is the in-memory representation used during scoring.
type docRecord struct {
	id         int64
	path       string // path relative to the sync dir (what's stored in DB)
	title      string
	lang       string
	slug       string
	categories []string

	// doc-level embedding (mean-pooled chunk vectors, L2-normalised)
	vec []float32

	// stemmed tokens (deduped) retained for lex-salience and title-overlap
	stems        []string
	titleStems   map[string]struct{}
	salientTerms map[string]struct{} // TF-IDF top-K stems (set)
}

// Builder loads the corpus and computes related-article maps.
type Builder struct {
	cfg     Config
	logger  *slog.Logger
	docs    []*docRecord
	byPath  map[string]*docRecord
	bySlug  map[string]*docRecord
	linksFw map[int64]map[int64]struct{} // src -> set of destination IDs
	linksBw map[int64]map[int64]struct{} // dst -> set of source IDs
}

// NewBuilder returns a Builder ready for Load.
func NewBuilder(cfg Config, logger *slog.Logger) *Builder {
	c := DefaultConfig()
	// Overlay any non-zero fields from cfg.
	if cfg.Limit != 0 {
		c.Limit = cfg.Limit
	}
	if cfg.WSem != 0 {
		c.WSem = cfg.WSem
	}
	if cfg.WLex != 0 {
		c.WLex = cfg.WLex
	}
	if cfg.WLink != 0 {
		c.WLink = cfg.WLink
	}
	if cfg.WCat != 0 {
		c.WCat = cfg.WCat
	}
	if cfg.MMRLambda != 0 {
		c.MMRLambda = cfg.MMRLambda
	}
	if cfg.TopKSalient != 0 {
		c.TopKSalient = cfg.TopKSalient
	}
	if cfg.CandidatePoolMultiplier != 0 {
		c.CandidatePoolMultiplier = cfg.CandidatePoolMultiplier
	}
	if cfg.TrivialTitleThreshold != 0 {
		c.TrivialTitleThreshold = cfg.TrivialTitleThreshold
	}
	if cfg.Workers != 0 {
		c.Workers = cfg.Workers
	}
	c.IgnoreSemantic = cfg.IgnoreSemantic
	if logger == nil {
		logger = slog.Default()
	}
	return &Builder{cfg: c, logger: logger}
}

// Load reads all documents, chunk embeddings and markdown files into memory
// and prepares derived structures (doc vectors, salience, link graph).
//
// contentDir is the root directory that was passed to `marrow sync`; paths
// stored in the index are relative to it. The directory is walked to
// extract front-matter (slug, categories) and inline links.
func (b *Builder) Load(ctx context.Context, database *db.DB, source, contentDir string) error {
	start := time.Now()

	if err := b.loadDocs(ctx, database, source); err != nil {
		return fmt.Errorf("load docs: %w", err)
	}
	if err := b.loadDocVectors(ctx, database); err != nil {
		return fmt.Errorf("load doc vectors: %w", err)
	}
	if err := b.loadFrontmatterAndLinks(contentDir); err != nil {
		return fmt.Errorf("load frontmatter+links: %w", err)
	}
	b.computeSalience()

	// Auto-detect mock embeddings: mock vectors are (typically) deterministic
	// short hashes — heuristically, if the first 8 dimensions of every doc
	// look suspiciously similar we treat them as mock. A cheaper proxy:
	// check that not all vectors are length-zero or identical.
	if b.cfg.IgnoreSemantic || b.looksLikeMockVectors() {
		b.cfg.IgnoreSemantic = true
		b.cfg.WSem = 0
		b.logger.Warn("semantic signal disabled (mock or missing embeddings)")
	}

	b.logger.Info("related: corpus loaded",
		"docs", len(b.docs),
		"links", b.linkCount(),
		"elapsed", time.Since(start).String())
	return nil
}

func (b *Builder) linkCount() int {
	n := 0
	for _, m := range b.linksFw {
		n += len(m)
	}
	return n
}

// looksLikeMockVectors returns true when the loaded vectors appear to come
// from embed.NewMock (which returns short hash-derived vectors). We detect
// this by checking whether a large fraction of vectors share the same first
// component — real embeddings are almost never that degenerate.
func (b *Builder) looksLikeMockVectors() bool {
	if len(b.docs) < 8 {
		return false
	}
	var firstNonZero int
	counts := map[float32]int{}
	for _, d := range b.docs {
		if len(d.vec) == 0 {
			continue
		}
		firstNonZero++
		// Bucket to 3 decimals to collapse near-duplicates.
		key := float32(math.Round(float64(d.vec[0])*1000) / 1000)
		counts[key]++
	}
	if firstNonZero == 0 {
		return true
	}
	// If >50% of non-empty vectors share the same bucketed first component,
	// embeddings are almost certainly synthetic.
	for _, c := range counts {
		if float64(c)/float64(firstNonZero) > 0.5 {
			return true
		}
	}
	return false
}

// ------------------------------------------------------------------------
// Data loading
// ------------------------------------------------------------------------

func (b *Builder) loadDocs(ctx context.Context, database *db.DB, source string) error {
	var rows *rowsIter
	q := `SELECT d.id, d.path, d.title, d.lang, f.content
	      FROM documents d
	      JOIN documents_fts f ON f.rowid = d.id`
	args := []any{}
	if source != "" {
		q += ` WHERE d.source = ?`
		args = append(args, source)
	}
	q += ` ORDER BY d.id`

	sqlRows, err := database.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	rows = &rowsIter{rows: sqlRows}
	defer rows.Close()

	b.docs = b.docs[:0]
	b.byPath = map[string]*docRecord{}
	for rows.Next() {
		var id int64
		var path, title, lang, stemmedText string
		if err := rows.Scan(&id, &path, &title, &lang, &stemmedText); err != nil {
			return err
		}
		if lang == "" {
			lang = "eu"
		}
		stems := uniqueFields(stemmedText)
		titleStems := map[string]struct{}{}
		for _, s := range strings.Fields(stemmer.StemText(title, lang)) {
			titleStems[s] = struct{}{}
		}
		rec := &docRecord{
			id:         id,
			path:       path,
			title:      title,
			lang:       lang,
			stems:      stems,
			titleStems: titleStems,
		}
		b.docs = append(b.docs, rec)
		b.byPath[path] = rec
	}
	return rows.Err()
}

func (b *Builder) loadDocVectors(ctx context.Context, database *db.DB) error {
	// Aggregate chunk vectors per document.
	sqlRows, err := database.QueryContext(ctx,
		`SELECT c.document_id, v.embedding
		 FROM documents_vec v
		 JOIN document_chunks c ON c.id = v.rowid`)
	if err != nil {
		return err
	}
	defer sqlRows.Close()

	type accum struct {
		sum []float32
		n   int
	}
	perDoc := map[int64]*accum{}
	for sqlRows.Next() {
		var docID int64
		var blob []byte
		if err := sqlRows.Scan(&docID, &blob); err != nil {
			return err
		}
		vec, err := deserializeVec(blob)
		if err != nil {
			return err
		}
		a := perDoc[docID]
		if a == nil {
			a = &accum{sum: make([]float32, len(vec))}
			perDoc[docID] = a
		}
		if len(a.sum) != len(vec) {
			// Dimension mismatch — shouldn't happen in a consistent index.
			continue
		}
		for i, v := range vec {
			a.sum[i] += v
		}
		a.n++
	}
	if err := sqlRows.Err(); err != nil {
		return err
	}

	for _, doc := range b.docs {
		a, ok := perDoc[doc.id]
		if !ok || a.n == 0 {
			continue
		}
		vec := make([]float32, len(a.sum))
		inv := float32(1.0 / float64(a.n))
		for i := range a.sum {
			vec[i] = a.sum[i] * inv
		}
		l2Normalise(vec)
		doc.vec = vec
	}
	return nil
}

// loadFrontmatterAndLinks walks contentDir for markdown files, reads their
// front matter (slug, categories), and extracts internal links that point
// to other articles via Hugo-style /slug/ permalinks.
func (b *Builder) loadFrontmatterAndLinks(contentDir string) error {
	if contentDir == "" {
		return nil
	}
	abs, err := filepath.Abs(contentDir)
	if err != nil {
		return err
	}

	// First pass: front matter to build slug -> docRecord.
	b.bySlug = map[string]*docRecord{}
	type pending struct {
		doc  *docRecord
		body []byte
	}
	var pend []pending

	// Paths in the DB may be stored verbatim (e.g. "content/artikuluak/foo.md")
	// or relative to the sync dir ("foo.md"). Build a basename index so we
	// can resolve either way.
	byBase := make(map[string]*docRecord, len(b.docs))
	for _, d := range b.docs {
		byBase[filepath.Base(d.path)] = d
	}

	err = filepath.Walk(abs, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(p) != ".md" {
			return nil
		}
		rel, rerr := filepath.Rel(abs, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		doc, ok := b.byPath[rel]
		if !ok {
			// Try with the user-supplied contentDir prefix.
			if d, ok2 := b.byPath[filepath.ToSlash(filepath.Join(contentDir, rel))]; ok2 {
				doc = d
				ok = true
			}
		}
		if !ok {
			// Fall back to basename lookup.
			if d, ok2 := byBase[filepath.Base(rel)]; ok2 {
				doc = d
				ok = true
			}
		}
		if !ok {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		fm, body := splitFrontMatter(data)
		if len(fm) > 0 {
			var meta map[string]any
			if err := yaml.Unmarshal(fm, &meta); err == nil {
				if s, ok := meta["slug"].(string); ok && s != "" {
					doc.slug = s
					b.bySlug[s] = doc
				}
				doc.categories = extractCategories(meta)
			}
		}
		if doc.slug == "" {
			// Fallback slug = file stem.
			doc.slug = strings.TrimSuffix(filepath.Base(rel), ".md")
			b.bySlug[doc.slug] = doc
		}
		pend = append(pend, pending{doc: doc, body: body})
		return nil
	})
	if err != nil {
		return err
	}

	// Second pass: now that bySlug is populated, resolve links.
	b.linksFw = map[int64]map[int64]struct{}{}
	b.linksBw = map[int64]map[int64]struct{}{}
	for _, p := range pend {
		for _, destSlug := range extractInternalLinkSlugs(p.body) {
			dest, ok := b.bySlug[destSlug]
			if !ok || dest.id == p.doc.id {
				continue
			}
			addLink(b.linksFw, p.doc.id, dest.id)
			addLink(b.linksBw, dest.id, p.doc.id)
		}
	}
	return nil
}

func addLink(m map[int64]map[int64]struct{}, a, b int64) {
	s, ok := m[a]
	if !ok {
		s = map[int64]struct{}{}
		m[a] = s
	}
	s[b] = struct{}{}
}

// ------------------------------------------------------------------------
// Compute
// ------------------------------------------------------------------------

// Compute returns related docs for every source document, keyed by the
// document's on-disk path (as stored in the DB).
func (b *Builder) Compute(ctx context.Context) map[string][]RelatedDoc {
	n := len(b.docs)
	if n == 0 {
		return map[string][]RelatedDoc{}
	}

	type job struct{ idx int }
	type res struct {
		path string
		rels []RelatedDoc
	}

	jobs := make(chan job, n)
	out := make(chan res, n)
	workers := b.cfg.Workers
	if workers <= 0 {
		workers = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				src := b.docs[j.idx]
				rels := b.relatedFor(src)
				out <- res{path: src.path, rels: rels}
			}
		}()
	}
	go func() {
		for i := range b.docs {
			jobs <- job{idx: i}
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(out)
	}()

	result := make(map[string][]RelatedDoc, n)
	for r := range out {
		if len(r.rels) > 0 {
			result[r.path] = r.rels
		}
	}
	return result
}
