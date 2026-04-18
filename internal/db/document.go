package db

import (
	"context"
	"database/sql"
)

// DocumentRepo handles persistence operations for documents.
type DocumentRepo struct {
	db *DB
}

// NewDocumentRepo creates a document repository backed by db.
func NewDocumentRepo(db *DB) *DocumentRepo {
	return &DocumentRepo{db: db}
}

// DeleteDocumentsByPaths removes documents and their FTS/vector rows for the given paths.
func (r *DocumentRepo) DeleteDocumentsByPaths(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
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
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM documents_vec WHERE rowid IN (SELECT id FROM document_chunks WHERE document_id = ?)`,
			rowid,
		); err != nil {
			return err
		}
		// FK ON DELETE CASCADE will remove document_chunks when the parent
		// document row is deleted. Still do FTS and documents cleanup here
		// because FTS is a virtual table and the cascade is document-keyed.
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
func (r *DocumentRepo) GetDocumentPathsBySource(ctx context.Context, source string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT path FROM documents WHERE source = ?`, source)
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
