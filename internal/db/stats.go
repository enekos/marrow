package db

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// FacetValue is a single facet bucket: a value present in the documents table
// and the count of documents that have it.
type FacetValue struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// Facets groups available filter values for the documents table. Each slice is
// sorted by count desc, then value asc, so clients can render them as-is.
type Facets struct {
	Sources  []FacetValue `json:"sources"`
	DocTypes []FacetValue `json:"doc_types"`
	Langs    []FacetValue `json:"langs"`
}

// StatsRepo handles database statistics.
type StatsRepo struct {
	db *DB
}

// NewStatsRepo creates a stats repository backed by db.
func NewStatsRepo(db *DB) *StatsRepo {
	return &StatsRepo{db: db}
}

// Get returns aggregate statistics for the database.
func (r *StatsRepo) Get(ctx context.Context) (*Stats, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&total); err != nil {
		return nil, err
	}

	bySource := make(map[string]int64)
	var sources []string
	rows, err := r.db.QueryContext(ctx, `SELECT source, COUNT(*) FROM documents GROUP BY source ORDER BY source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		var c int64
		if err := rows.Scan(&s, &c); err != nil {
			return nil, err
		}
		bySource[s] = c
		sources = append(sources, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	byDocType := make(map[string]int64)
	rows2, err := r.db.QueryContext(ctx, `SELECT doc_type, COUNT(*) FROM documents GROUP BY doc_type`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var dt string
		var c int64
		if err := rows2.Scan(&dt, &c); err != nil {
			return nil, err
		}
		byDocType[dt] = c
	}
	if err := rows2.Err(); err != nil {
		return nil, err
	}

	var dbSize int64
	if err := r.db.QueryRowContext(ctx, `SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`).Scan(&dbSize); err != nil {
		return nil, err
	}

	var lastSync *time.Time
	var t sql.NullTime
	if err := r.db.QueryRowContext(ctx, `SELECT last_sync_at FROM sync_state ORDER BY last_sync_at DESC LIMIT 1`).Scan(&t); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if t.Valid {
		lastSync = &t.Time
	}

	return &Stats{
		TotalDocs:   total,
		BySource:    bySource,
		ByDocType:   byDocType,
		DBSizeBytes: dbSize,
		LastSyncAt:  lastSync,
		Sources:     sources,
	}, nil
}

// Facets returns the distinct values of `source`, `doc_type` and `lang` in the
// documents table together with their counts. If allowedSources is non-empty,
// the result is restricted to documents whose source is in that list — used to
// scope facets to a multi-tenant site's allowed sources.
func (r *StatsRepo) Facets(ctx context.Context, allowedSources []string) (*Facets, error) {
	where, args := buildSourceFilter(allowedSources)

	sources, err := r.facetColumn(ctx, "source", where, args)
	if err != nil {
		return nil, err
	}
	docTypes, err := r.facetColumn(ctx, "doc_type", where, args)
	if err != nil {
		return nil, err
	}
	langs, err := r.facetColumn(ctx, "lang", where, args)
	if err != nil {
		return nil, err
	}

	return &Facets{Sources: sources, DocTypes: docTypes, Langs: langs}, nil
}

func (r *StatsRepo) facetColumn(ctx context.Context, col, where string, args []any) ([]FacetValue, error) {
	// col is a fixed identifier from a closed set ("source", "doc_type",
	// "lang"); the where clause is built with placeholders only.
	q := "SELECT " + col + ", COUNT(*) FROM documents " + where +
		" GROUP BY " + col + " ORDER BY COUNT(*) DESC, " + col + " ASC"
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FacetValue
	for rows.Next() {
		var v sql.NullString
		var c int64
		if err := rows.Scan(&v, &c); err != nil {
			return nil, err
		}
		out = append(out, FacetValue{Value: v.String, Count: c})
	}
	return out, rows.Err()
}

func buildSourceFilter(allowed []string) (string, []any) {
	if len(allowed) == 0 {
		return "", nil
	}
	placeholders := make([]string, len(allowed))
	args := make([]any, len(allowed))
	for i, s := range allowed {
		placeholders[i] = "?"
		args[i] = s
	}
	return "WHERE source IN (" + strings.Join(placeholders, ",") + ")", args
}
