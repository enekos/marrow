package index

import (
	"context"
	"database/sql"
	"fmt"

	"marrow/internal/db"
)

// Document represents a parsed markdown file ready for indexing.
type Document struct {
	Path        string
	Hash        string
	Title       string
	Lang        string
	Source      string
	DocType     string
	StemmedText string
	Embedding   []float32
}

// DBConn is the subset of database operations required by Indexer.
type DBConn interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Indexer handles persistence of documents into the hybrid store.
type Indexer struct {
	db DBConn
}

// NewIndexer creates a new indexer backed by the given database.
func NewIndexer(database DBConn) *Indexer {
	return &Indexer{db: database}
}

// Index persists a document into documents, documents_fts, and documents_vec.
func (ix *Indexer) Index(ctx context.Context, doc Document) error {
	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var rowid int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM documents WHERE path = ?`, doc.Path).Scan(&rowid)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("lookup document: %w", err)
	}

	if err == sql.ErrNoRows {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO documents (path, hash, title, lang, source, doc_type) VALUES (?, ?, ?, ?, ?, ?)`,
			doc.Path, doc.Hash, doc.Title, doc.Lang, doc.Source, doc.DocType)
		if err != nil {
			return fmt.Errorf("insert document: %w", err)
		}
		rowid, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("last insert id: %w", err)
		}
	} else {
		_, err = tx.ExecContext(ctx,
			`UPDATE documents SET hash = ?, title = ?, lang = ?, source = ?, doc_type = ?, last_modified = CURRENT_TIMESTAMP WHERE id = ?`,
			doc.Hash, doc.Title, doc.Lang, doc.Source, doc.DocType, rowid)
		if err != nil {
			return fmt.Errorf("update document: %w", err)
		}
	}

	// Sync FTS row: delete old row first to avoid virtual table quirks
	_, _ = tx.ExecContext(ctx, `DELETE FROM documents_fts WHERE rowid = ?`, rowid)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO documents_fts (rowid, title, content) VALUES (?, ?, ?)`,
		rowid, doc.Title, doc.StemmedText); err != nil {
		return fmt.Errorf("insert fts: %w", err)
	}

	// Sync vector row: delete old row first
	_, _ = tx.ExecContext(ctx, `DELETE FROM documents_vec WHERE rowid = ?`, rowid)
	vecBlob, err := db.SerializeVec(doc.Embedding)
	if err != nil {
		return fmt.Errorf("serialize vec: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO documents_vec (rowid, embedding) VALUES (?, ?)`,
		rowid, vecBlob); err != nil {
		return fmt.Errorf("insert vec: %w", err)
	}

	return tx.Commit()
}
