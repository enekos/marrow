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
	// go-sqlite3's DSN-pragma support is version-sensitive; set FKs
	// explicitly so cascade behavior on document_chunks is reliable.
	if _, err := sqlDB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	db := &DB{DB: sqlDB}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) migrate() error {
	// Pre-chunking schema detection: the old schema stored one vector per
	// document keyed on documents.id. The new schema stores one vector per
	// chunk keyed on document_chunks.id. The two rowid semantics are
	// incompatible, so if we find a vec table without the companion chunks
	// table we drop the vec table and recreate it below. Users lose vector
	// data and must re-sync to rebuild; FTS and document rows survive.
	var chunksExists int
	if err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='document_chunks'`,
	).Scan(&chunksExists); err != nil {
		return fmt.Errorf("detect chunks table: %w", err)
	}
	if chunksExists == 0 {
		var oldVecExists int
		if err := db.QueryRow(
			`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='documents_vec'`,
		).Scan(&oldVecExists); err != nil {
			return fmt.Errorf("detect legacy vec table: %w", err)
		}
		if oldVecExists > 0 {
			if _, err := db.Exec(`DROP TABLE documents_vec`); err != nil {
				return fmt.Errorf("drop legacy documents_vec: %w", err)
			}
		}
	}

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
		// documents_vec.rowid refers to document_chunks.id, not documents.id.
		`CREATE VIRTUAL TABLE IF NOT EXISTS documents_vec USING vec0(
			embedding FLOAT[384]
		);`,
		`CREATE TABLE IF NOT EXISTS document_chunks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			document_id INTEGER NOT NULL,
			chunk_index INTEGER NOT NULL,
			text TEXT NOT NULL,
			FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
			UNIQUE (document_id, chunk_index)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_document ON document_chunks(document_id);`,
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
	// Best-effort column additions for DBs created before these columns
	// existed. ALTER TABLE ADD COLUMN is idempotent-by-error so we ignore.
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

// Stats holds database and index statistics.
type Stats struct {
	TotalDocs   int64
	BySource    map[string]int64
	ByDocType   map[string]int64
	DBSizeBytes int64
	LastSyncAt  *time.Time
	Sources     []string
}

// Maintain runs VACUUM and prunes orphaned vec/fts/chunk rows.
//
// Order matters: orphaned chunks are pruned first so that the subsequent
// vec pruner sees an up-to-date chunks table. Without that, a force-deleted
// document would leave its chunks behind (if FK cascade failed to fire on
// this connection) and the vec prune would believe they were still live.
func (db *DB) Maintain(ctx context.Context) error {
	if _, err := db.ExecContext(ctx,
		`DELETE FROM document_chunks WHERE document_id NOT IN (SELECT id FROM documents)`,
	); err != nil {
		return fmt.Errorf("prune chunks: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`DELETE FROM documents_vec WHERE rowid NOT IN (SELECT id FROM document_chunks)`,
	); err != nil {
		return fmt.Errorf("prune vec: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`DELETE FROM documents_fts WHERE rowid NOT IN (SELECT id FROM documents)`,
	); err != nil {
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
