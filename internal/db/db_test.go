package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestOpen_Memory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:) failed: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='documents'`).Scan(&count); err != nil {
		t.Fatalf("checking documents table: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected documents table to exist")
	}
}

func TestOpen_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open(file) failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file not created: %v", err)
	}

	// Re-open existing file
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("re-open failed: %v", err)
	}
	defer db2.Close()
}

func TestSyncState_Roundtrip(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	ts := time.Now().UTC().Truncate(time.Second)

	state := &SyncState{
		Source:     "github.com/foo/bar",
		LastSyncAt: &ts,
		SecretKey:  "secret",
		RepoURL:    "https://github.com/foo/bar",
		LocalPath:  "/tmp/bar",
		Token:      "token123",
	}

	if err := db.UpsertSyncState(ctx, state); err != nil {
		t.Fatalf("UpsertSyncState failed: %v", err)
	}

	loaded, err := db.GetSyncState(ctx, state.Source)
	if err != nil {
		t.Fatalf("GetSyncState failed: %v", err)
	}

	if loaded.Source != state.Source {
		t.Errorf("Source mismatch: got %q, want %q", loaded.Source, state.Source)
	}
	if loaded.LastSyncAt == nil || !loaded.LastSyncAt.Equal(ts) {
		t.Errorf("LastSyncAt mismatch: got %v, want %v", loaded.LastSyncAt, ts)
	}
	if loaded.SecretKey != state.SecretKey {
		t.Errorf("SecretKey mismatch: got %q, want %q", loaded.SecretKey, state.SecretKey)
	}
	if loaded.RepoURL != state.RepoURL {
		t.Errorf("RepoURL mismatch: got %q, want %q", loaded.RepoURL, state.RepoURL)
	}
	if loaded.LocalPath != state.LocalPath {
		t.Errorf("LocalPath mismatch: got %q, want %q", loaded.LocalPath, state.LocalPath)
	}
	if loaded.Token != state.Token {
		t.Errorf("Token mismatch: got %q, want %q", loaded.Token, state.Token)
	}

	// Update and upsert again
	newTS := ts.Add(time.Hour)
	state.LastSyncAt = &newTS
	state.Token = "newtoken"
	if err := db.UpsertSyncState(ctx, state); err != nil {
		t.Fatalf("UpsertSyncState update failed: %v", err)
	}

	loaded2, err := db.GetSyncState(ctx, state.Source)
	if err != nil {
		t.Fatalf("GetSyncState after update failed: %v", err)
	}
	if loaded2.Token != "newtoken" {
		t.Errorf("Token after update mismatch: got %q, want %q", loaded2.Token, "newtoken")
	}
	if loaded2.LastSyncAt == nil || !loaded2.LastSyncAt.Equal(newTS) {
		t.Errorf("LastSyncAt after update mismatch: got %v, want %v", loaded2.LastSyncAt, newTS)
	}

	// Missing source
	_, err = db.GetSyncState(ctx, "does-not-exist")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for missing source, got %v", err)
	}
}

func TestDeleteDocumentsByPaths(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Insert documents
	res, err := db.ExecContext(ctx,
		`INSERT INTO documents (path, hash, title, source, doc_type) VALUES (?, ?, ?, ?, ?)`,
		"a.md", "h1", "A", "src1", "markdown",
	)
	if err != nil {
		t.Fatalf("insert doc a failed: %v", err)
	}
	idA, _ := res.LastInsertId()

	res, err = db.ExecContext(ctx,
		`INSERT INTO documents (path, hash, title, source, doc_type) VALUES (?, ?, ?, ?, ?)`,
		"b.md", "h2", "B", "src1", "markdown",
	)
	if err != nil {
		t.Fatalf("insert doc b failed: %v", err)
	}
	idB, _ := res.LastInsertId()

	// Insert fts and vec rows
	if _, err := db.ExecContext(ctx, `INSERT INTO documents_fts(rowid, title, content) VALUES (?, ?, ?)`, idA, "A", "content A"); err != nil {
		t.Fatalf("insert fts a failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO documents_fts(rowid, title, content) VALUES (?, ?, ?)`, idB, "B", "content B"); err != nil {
		t.Fatalf("insert fts b failed: %v", err)
	}

	blob, _ := SerializeVec(make([]float32, 384))
	if _, err := db.ExecContext(ctx, `INSERT INTO documents_vec(rowid, embedding) VALUES (?, ?)`, idA, blob); err != nil {
		t.Fatalf("insert vec a failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO documents_vec(rowid, embedding) VALUES (?, ?)`, idB, blob); err != nil {
		t.Fatalf("insert vec b failed: %v", err)
	}

	// Empty slice should be no-op
	if err := db.DeleteDocumentsByPaths(ctx, []string{}); err != nil {
		t.Fatalf("DeleteDocumentsByPaths(empty) failed: %v", err)
	}

	// Delete one path
	if err := db.DeleteDocumentsByPaths(ctx, []string{"a.md"}); err != nil {
		t.Fatalf("DeleteDocumentsByPaths(a.md) failed: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE path = ?`, "a.md").Scan(&count); err != nil {
		t.Fatalf("count documents a failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 documents for a.md, got %d", count)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents_fts WHERE rowid = ?`, idA).Scan(&count); err != nil {
		t.Fatalf("count fts a failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 fts rows for idA, got %d", count)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents_vec WHERE rowid = ?`, idA).Scan(&count); err != nil {
		t.Fatalf("count vec a failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 vec rows for idA, got %d", count)
	}

	// Non-existent path should be harmless
	if err := db.DeleteDocumentsByPaths(ctx, []string{"does-not-exist.md"}); err != nil {
		t.Fatalf("DeleteDocumentsByPaths(does-not-exist) failed: %v", err)
	}

	// b.md should still exist
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE path = ?`, "b.md").Scan(&count); err != nil {
		t.Fatalf("count documents b failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 document for b.md, got %d", count)
	}
}

func TestDeleteDocumentsByPaths_MissingDocument(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Deleting a path that does not exist should return nil and not error.
	if err := db.DeleteDocumentsByPaths(ctx, []string{"never-existed.md"}); err != nil {
		t.Fatalf("DeleteDocumentsByPaths(missing) should silently continue, got error: %v", err)
	}
}

func TestGetDocumentPathsBySource(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Empty source
	paths, err := db.GetDocumentPathsBySource(ctx, "src1")
	if err != nil {
		t.Fatalf("GetDocumentPathsBySource(empty) failed: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}

	// Insert rows
	for _, p := range []struct{ path, source string }{
		{"x.md", "src1"},
		{"y.md", "src1"},
		{"z.md", "src2"},
	} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO documents (path, hash, source) VALUES (?, ?, ?)`,
			p.path, "hash", p.source,
		); err != nil {
			t.Fatalf("insert %s failed: %v", p.path, err)
		}
	}

	paths, err = db.GetDocumentPathsBySource(ctx, "src1")
	if err != nil {
		t.Fatalf("GetDocumentPathsBySource(src1) failed: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths for src1, got %d", len(paths))
	}
	m := make(map[string]bool, len(paths))
	for _, p := range paths {
		m[p] = true
	}
	if !m["x.md"] || !m["y.md"] {
		t.Errorf("unexpected paths for src1: %v", paths)
	}
}

func TestGetStats(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Empty DB
	stats, err := db.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats(empty) failed: %v", err)
	}
	if stats.TotalDocs != 0 {
		t.Errorf("TotalDocs expected 0, got %d", stats.TotalDocs)
	}
	if len(stats.BySource) != 0 {
		t.Errorf("BySource expected empty, got %v", stats.BySource)
	}
	if len(stats.ByDocType) != 0 {
		t.Errorf("ByDocType expected empty, got %v", stats.ByDocType)
	}
	if len(stats.Sources) != 0 {
		t.Errorf("Sources expected empty, got %v", stats.Sources)
	}

	// Populate
	docs := []struct {
		path    string
		source  string
		docType string
	}{
		{"a.md", "src1", "markdown"},
		{"b.md", "src1", "markdown"},
		{"c.md", "src2", "html"},
	}
	for _, d := range docs {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO documents (path, hash, source, doc_type) VALUES (?, ?, ?, ?)`,
			d.path, "hash", d.source, d.docType,
		); err != nil {
			t.Fatalf("insert %s failed: %v", d.path, err)
		}
	}

	ts := time.Now().UTC().Truncate(time.Second)
	if err := db.UpsertSyncState(ctx, &SyncState{Source: "src1", LastSyncAt: &ts}); err != nil {
		t.Fatalf("UpsertSyncState failed: %v", err)
	}

	stats, err = db.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats(populated) failed: %v", err)
	}

	if stats.TotalDocs != 3 {
		t.Errorf("TotalDocs expected 3, got %d", stats.TotalDocs)
	}
	if stats.BySource["src1"] != 2 {
		t.Errorf("BySource[src1] expected 2, got %d", stats.BySource["src1"])
	}
	if stats.BySource["src2"] != 1 {
		t.Errorf("BySource[src2] expected 1, got %d", stats.BySource["src2"])
	}
	if stats.ByDocType["markdown"] != 2 {
		t.Errorf("ByDocType[markdown] expected 2, got %d", stats.ByDocType["markdown"])
	}
	if stats.ByDocType["html"] != 1 {
		t.Errorf("ByDocType[html] expected 1, got %d", stats.ByDocType["html"])
	}
	if stats.LastSyncAt == nil || !stats.LastSyncAt.Equal(ts) {
		t.Errorf("LastSyncAt expected %v, got %v", ts, stats.LastSyncAt)
	}
	if len(stats.Sources) != 2 {
		t.Errorf("Sources expected 2 entries, got %v", stats.Sources)
	}
	if stats.DBSizeBytes <= 0 {
		t.Errorf("DBSizeBytes expected >0, got %d", stats.DBSizeBytes)
	}
}

func TestMaintain(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Insert a document with fts and vec
	res, err := db.ExecContext(ctx,
		`INSERT INTO documents (path, hash, title) VALUES (?, ?, ?)`,
		"keep.md", "h1", "Keep",
	)
	if err != nil {
		t.Fatalf("insert keep failed: %v", err)
	}
	idKeep, _ := res.LastInsertId()

	if _, err := db.ExecContext(ctx, `INSERT INTO documents_fts(rowid, title, content) VALUES (?, ?, ?)`, idKeep, "Keep", "keep content"); err != nil {
		t.Fatalf("insert fts keep failed: %v", err)
	}
	blob, _ := SerializeVec(make([]float32, 384))
	if _, err := db.ExecContext(ctx, `INSERT INTO documents_vec(rowid, embedding) VALUES (?, ?)`, idKeep, blob); err != nil {
		t.Fatalf("insert vec keep failed: %v", err)
	}

	// Insert orphaned fts and vec rows (without corresponding documents row)
	orphanID := int64(9999)
	if _, err := db.ExecContext(ctx, `INSERT INTO documents_fts(rowid, title, content) VALUES (?, ?, ?)`, orphanID, "Orphan", "orphan content"); err != nil {
		t.Fatalf("insert fts orphan failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO documents_vec(rowid, embedding) VALUES (?, ?)`, orphanID, blob); err != nil {
		t.Fatalf("insert vec orphan failed: %v", err)
	}

	if err := db.Maintain(ctx); err != nil {
		t.Fatalf("Maintain failed: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents_vec WHERE rowid = ?`, orphanID).Scan(&count); err != nil {
		t.Fatalf("count vec orphan failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected orphaned vec row to be deleted, got %d", count)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents_fts WHERE rowid = ?`, orphanID).Scan(&count); err != nil {
		t.Fatalf("count fts orphan failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected orphaned fts row to be deleted, got %d", count)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents_vec WHERE rowid = ?`, idKeep).Scan(&count); err != nil {
		t.Fatalf("count vec keep failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected valid vec row to remain, got %d", count)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents_fts WHERE rowid = ?`, idKeep).Scan(&count); err != nil {
		t.Fatalf("count fts keep failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected valid fts row to remain, got %d", count)
	}
}

func TestMaintain_PruneFTSAndVacuum(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Insert a document with its FTS and vec rows.
	res, err := db.ExecContext(ctx,
		`INSERT INTO documents (path, hash, title) VALUES (?, ?, ?)`,
		"orphan.md", "h1", "Orphan",
	)
	if err != nil {
		t.Fatalf("insert document failed: %v", err)
	}
	idDoc, _ := res.LastInsertId()

	if _, err := db.ExecContext(ctx, `INSERT INTO documents_fts(rowid, title, content) VALUES (?, ?, ?)`, idDoc, "Orphan", "orphan content"); err != nil {
		t.Fatalf("insert fts failed: %v", err)
	}
	blob, _ := SerializeVec(make([]float32, 384))
	if _, err := db.ExecContext(ctx, `INSERT INTO documents_vec(rowid, embedding) VALUES (?, ?)`, idDoc, blob); err != nil {
		t.Fatalf("insert vec failed: %v", err)
	}

	// Insert another document that will remain to ensure we don't over-prune.
	res2, err := db.ExecContext(ctx,
		`INSERT INTO documents (path, hash, title) VALUES (?, ?, ?)`,
		"keep.md", "h2", "Keep",
	)
	if err != nil {
		t.Fatalf("insert keep document failed: %v", err)
	}
	idKeep, _ := res2.LastInsertId()

	if _, err := db.ExecContext(ctx, `INSERT INTO documents_fts(rowid, title, content) VALUES (?, ?, ?)`, idKeep, "Keep", "keep content"); err != nil {
		t.Fatalf("insert keep fts failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO documents_vec(rowid, embedding) VALUES (?, ?)`, idKeep, blob); err != nil {
		t.Fatalf("insert keep vec failed: %v", err)
	}

	// Delete the first document directly, leaving orphaned FTS and vec rows.
	if _, err := db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, idDoc); err != nil {
		t.Fatalf("delete document failed: %v", err)
	}

	// Call Maintain to prune orphans and vacuum.
	if err := db.Maintain(ctx); err != nil {
		t.Fatalf("Maintain failed: %v", err)
	}

	// Verify orphaned rows are deleted.
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents_fts WHERE rowid = ?`, idDoc).Scan(&count); err != nil {
		t.Fatalf("count orphaned fts failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected orphaned fts row to be deleted, got %d", count)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents_vec WHERE rowid = ?`, idDoc).Scan(&count); err != nil {
		t.Fatalf("count orphaned vec failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected orphaned vec row to be deleted, got %d", count)
	}

	// Verify the remaining document's rows are intact.
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents_fts WHERE rowid = ?`, idKeep).Scan(&count); err != nil {
		t.Fatalf("count keep fts failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected keep fts row to remain, got %d", count)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents_vec WHERE rowid = ?`, idKeep).Scan(&count); err != nil {
		t.Fatalf("count keep vec failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected keep vec row to remain, got %d", count)
	}

	// Vacuum doesn't return an error on success; if Maintain returned nil we're good.
}

func TestBackup(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	destPath := filepath.Join(dir, "backup.db")

	db, err := Open(srcPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `INSERT INTO documents (path, hash) VALUES (?, ?)`, "doc.md", "hash"); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if err := db.Backup(destPath); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify backup is valid and contains data
	db2, err := Open(destPath)
	if err != nil {
		t.Fatalf("open backup failed: %v", err)
	}
	defer db2.Close()

	var count int
	if err := db2.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&count); err != nil {
		t.Fatalf("count backup docs failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 doc in backup, got %d", count)
	}
}

func TestBackup_ErrorPaths(t *testing.T) {
	// Backup on an in-memory database should fail because DBName returns :memory:
	// and os.Open cannot open a file named :memory:.
	mem, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:) failed: %v", err)
	}
	defer mem.Close()

	if err := mem.Backup("/invalid/path/to/backup.db"); err == nil {
		t.Errorf("expected error backing up from :memory:, got nil")
	}
}

func TestDBName(t *testing.T) {
	mem, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:) failed: %v", err)
	}
	defer mem.Close()
	if got := mem.DBName(); got != ":memory:" {
		t.Errorf("DBName for memory db expected ':memory:', got %q", got)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "file.db")
	fileDB, err := Open(path)
	if err != nil {
		t.Fatalf("Open(file) failed: %v", err)
	}
	defer fileDB.Close()
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolvedPath = path
	}
	if got := fileDB.DBName(); got != resolvedPath {
		t.Errorf("DBName for file db expected %q, got %q", resolvedPath, got)
	}
}

func TestSerializeVec(t *testing.T) {
	v := make([]float32, 384)
	v[0] = 1.0
	v[1] = 2.0
	v[2] = 3.0
	blob, err := SerializeVec(v)
	if err != nil {
		t.Fatalf("SerializeVec failed: %v", err)
	}
	if len(blob) == 0 {
		t.Errorf("expected non-empty blob")
	}

	// Verify we can use it in a real table
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	res, err := db.ExecContext(ctx, `INSERT INTO documents (path, hash, title) VALUES (?, ?, ?)`, "vec.md", "h", "Vec")
	if err != nil {
		t.Fatalf("insert doc failed: %v", err)
	}
	id, _ := res.LastInsertId()

	if _, err := db.ExecContext(ctx, `INSERT INTO documents_vec(rowid, embedding) VALUES (?, ?)`, id, blob); err != nil {
		t.Fatalf("insert vec failed: %v", err)
	}

	var retrieved []byte
	if err := db.QueryRowContext(ctx, `SELECT embedding FROM documents_vec WHERE rowid = ?`, id).Scan(&retrieved); err != nil {
		t.Fatalf("select vec failed: %v", err)
	}
	if string(retrieved) != string(blob) {
		t.Errorf("retrieved blob mismatch")
	}
}
