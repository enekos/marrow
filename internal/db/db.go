package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
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
			doc_type TEXT DEFAULT 'markdown',
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
	// Best-effort migrations: add columns if table was created before they existed.
	_, _ = db.Exec(`ALTER TABLE documents ADD COLUMN source TEXT DEFAULT 'local'`)
	_, _ = db.Exec(`ALTER TABLE documents ADD COLUMN doc_type TEXT DEFAULT 'markdown'`)
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

// Stats holds database and index statistics.
type Stats struct {
	TotalDocs    int64
	BySource     map[string]int64
	ByDocType    map[string]int64
	DBSizeBytes  int64
	LastSyncAt   *time.Time
	Sources      []string
}

// GetStats returns aggregate statistics for the database.
func (db *DB) GetStats(ctx context.Context) (*Stats, error) {
	var total int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&total); err != nil {
		return nil, err
	}

	bySource := make(map[string]int64)
	var sources []string
	rows, err := db.QueryContext(ctx, `SELECT source, COUNT(*) FROM documents GROUP BY source`)
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
	rows2, err := db.QueryContext(ctx, `SELECT doc_type, COUNT(*) FROM documents GROUP BY doc_type`)
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
	_ = db.QueryRowContext(ctx, `SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`).Scan(&dbSize)

	var lastSync *time.Time
	var t sql.NullTime
	_ = db.QueryRowContext(ctx, `SELECT last_sync_at FROM sync_state ORDER BY last_sync_at DESC LIMIT 1`).Scan(&t)
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

// Maintain runs VACUUM and prunes orphaned vec/fts rows.
func (db *DB) Maintain(ctx context.Context) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM documents_vec WHERE rowid NOT IN (SELECT id FROM documents)`); err != nil {
		return fmt.Errorf("prune vec: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM documents_fts WHERE rowid NOT IN (SELECT id FROM documents)`); err != nil {
		return fmt.Errorf("prune fts: %w", err)
	}
	if _, err := db.ExecContext(ctx, `VACUUM`); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	return nil
}

// Backup copies the database file to destPath.
func (db *DB) Backup(destPath string) error {
	src, err := os.Open(db.DBName())
	if err != nil {
		return err
	}
	defer src.Close()
	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dest.Close()
	_, err = io.Copy(dest, src)
	return err
}

// DBName returns the path to the underlying SQLite database.
func (db *DB) DBName() string {
	var file string
	row := db.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`)
	if err := row.Scan(&file); err == nil && file != "" {
		return file
	}
	return ":memory:"
}
