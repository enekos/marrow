package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite connection.
type DB struct {
	*sql.DB
}

// Open creates or opens the SQLite database at path and runs migrations.
func Open(path string) (*DB, error) {
	// Auto-register sqlite-vec for all future connections in this process.
	vec.Auto()

	sqlDB, err := sql.Open("sqlite3", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	db := &DB{DB: sqlDB}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT UNIQUE NOT NULL,
			hash TEXT NOT NULL,
			title TEXT,
			lang TEXT,
			source TEXT DEFAULT 'local',
			last_modified DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
			title,
			content,
			tokenize='unicode61'
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS documents_vec USING vec0(
			embedding FLOAT[384]
		);`,
		`CREATE TABLE IF NOT EXISTS sync_state (
			source TEXT PRIMARY KEY,
			last_sync_at DATETIME,
			secret_key TEXT,
			repo_url TEXT,
			local_path TEXT,
			token TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_source ON documents(source);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	// Best-effort migration: add source column if table was created before it existed.
	_, _ = db.Exec(`ALTER TABLE documents ADD COLUMN source TEXT DEFAULT 'local'`)
	return nil
}

// SerializeVec uses the sqlite-vec binding helper to serialize a float32 slice.
func SerializeVec(v []float32) ([]byte, error) {
	return vec.SerializeFloat32(v)
}

// SyncState holds sync configuration and state for a source.
type SyncState struct {
	Source     string
	LastSyncAt *time.Time
	SecretKey  string
	RepoURL    string
	LocalPath  string
	Token      string
}

// GetSyncState loads sync state for a source.
func (db *DB) GetSyncState(ctx context.Context, source string) (*SyncState, error) {
	var s SyncState
	var t sql.NullTime
	err := db.QueryRowContext(ctx,
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

// UpsertSyncState inserts or updates sync state.
func (db *DB) UpsertSyncState(ctx context.Context, s *SyncState) error {
	_, err := db.ExecContext(ctx,
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

// DeleteDocumentsByPaths removes documents and their FTS/vector rows for the given paths.
func (db *DB) DeleteDocumentsByPaths(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	// SQLite has a limit on host parameters, but for reasonable batch sizes this is fine.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, p := range paths {
		var rowid int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM documents WHERE path = ?`, p).Scan(&rowid)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM documents_vec WHERE rowid = ?`, rowid); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM documents_fts WHERE rowid = ?`, rowid); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, rowid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetDocumentPathsBySource returns all paths tracked for a source.
func (db *DB) GetDocumentPathsBySource(ctx context.Context, source string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT path FROM documents WHERE source = ?`, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
