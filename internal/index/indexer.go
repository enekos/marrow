package index

import (
	"context"
	"database/sql"
	"fmt"

	"marrow/internal/db"
)

// Chunk is one embedded passage of a document. Chunks are what vector
// similarity is evaluated against; one long document typically produces
// multiple chunks to preserve local semantic signal.
type Chunk struct {
	Index     int
	Text      string
	Embedding []float32
}

// Document represents a parsed markdown file ready for indexing.
//
// StemmedText is the document-level text written into the FTS content
// column (keyword search still operates at document granularity).
//
// Chunks carries one entry per embedded passage. If Chunks is empty and
// Embedding is set, the indexer synthesizes a single chunk covering the
// whole document — a convenience that keeps short docs and unit tests
// from needing to build a Chunks slice by hand.
type Document struct {
	Path        string
	Hash        string
	Title       string
	Lang        string
	Source      string
	DocType     string
	StemmedText string
	Chunks      []Chunk
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

// Index persists a document into documents, documents_fts, document_chunks,
// and documents_vec. Re-indexing an existing document replaces its chunks
// and their vectors atomically.
func (ix *Indexer) Index(ctx context.Context, doc Document) error {
	chunks := doc.Chunks
	if len(chunks) == 0 {
		if doc.Embedding == nil {
			return fmt.Errorf("index: document %q has no Chunks and no Embedding", doc.Path)
		}
		chunks = []Chunk{{Index: 0, Text: doc.StemmedText, Embedding: doc.Embedding}}
	}

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

	// Sync FTS row: delete old row first to avoid virtual table quirks.
	_, _ = tx.ExecContext(ctx, `DELETE FROM documents_fts WHERE rowid = ?`, rowid)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO documents_fts (rowid, title, content) VALUES (?, ?, ?)`,
		rowid, doc.Title, doc.StemmedText); err != nil {
		return fmt.Errorf("insert fts: %w", err)
	}

	// Replace chunks + vectors. Delete existing chunk vectors first (vec
	// is a virtual table, so FK cascade cannot reach it) then the chunk
	// rows themselves. Re-inserting lets AUTOINCREMENT hand out fresh IDs
	// so we never collide with stale vec rowids.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM documents_vec WHERE rowid IN (SELECT id FROM document_chunks WHERE document_id = ?)`,
		rowid,
	); err != nil {
		return fmt.Errorf("delete old vec: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM document_chunks WHERE document_id = ?`, rowid,
	); err != nil {
		return fmt.Errorf("delete old chunks: %w", err)
	}

	for _, c := range chunks {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO document_chunks (document_id, chunk_index, text) VALUES (?, ?, ?)`,
			rowid, c.Index, c.Text,
		)
		if err != nil {
			return fmt.Errorf("insert chunk: %w", err)
		}
		chunkID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("chunk last insert id: %w", err)
		}
		vecBlob, err := db.SerializeVec(c.Embedding)
		if err != nil {
			return fmt.Errorf("serialize vec: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO documents_vec (rowid, embedding) VALUES (?, ?)`,
			chunkID, vecBlob,
		); err != nil {
			return fmt.Errorf("insert vec: %w", err)
		}
	}

	return tx.Commit()
}
