package db

import (
	"context"
	"database/sql"
	"time"
)

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
