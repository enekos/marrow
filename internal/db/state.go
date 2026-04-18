package db

import (
	"context"
	"database/sql"
)

// SyncStateRepo handles sync state persistence.
type SyncStateRepo struct {
	db *DB
}

// NewSyncStateRepo creates a sync-state repository backed by db.
func NewSyncStateRepo(db *DB) *SyncStateRepo {
	return &SyncStateRepo{db: db}
}

// Get loads sync state for a source.
func (r *SyncStateRepo) Get(ctx context.Context, source string) (*SyncState, error) {
	var s SyncState
	var t sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT source, last_sync_at, secret_key, repo_url, local_path, token FROM sync_state WHERE source = ?`,
		source,
	).Scan(&s.Source, &t, &s.SecretKey, &s.RepoURL, &s.LocalPath, &s.Token)
	if err != nil {
		return nil, err
	}
	if t.Valid {
		s.LastSyncAt = &t.Time
	}
	return &s, nil
}

// Upsert inserts or updates sync state.
func (r *SyncStateRepo) Upsert(ctx context.Context, s *SyncState) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sync_state (source, last_sync_at, secret_key, repo_url, local_path, token)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(source) DO UPDATE SET
		   last_sync_at=excluded.last_sync_at,
		   secret_key=excluded.secret_key,
		   repo_url=excluded.repo_url,
		   local_path=excluded.local_path,
		   token=excluded.token`,
		s.Source, s.LastSyncAt, s.SecretKey, s.RepoURL, s.LocalPath, s.Token)
	return err
}
