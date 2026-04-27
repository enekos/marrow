package index

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/testutil"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Serialize SQLite access to a single connection to avoid locking errors.
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	t.Cleanup(func() { database.Close() })
	return database
}

func TestIndex_InsertAndUpdate(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := NewIndexer(database)
	embedFn := embed.NewMock()

	doc := Document{
		Path:        "/notes/go.md",
		Hash:        "abc123",
		Title:       "Go Notes",
		Lang:        "en",
		Source:      "local",
		StemmedText: "go note",
		Embedding:   mustEmbed(embedFn, "go notes"),
	}

	// Insert
	if err := ix.Index(ctx, doc); err != nil {
		t.Fatalf("first index: %v", err)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM documents WHERE path = ?`, doc.Path).Scan(&count); err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 document, got %d", count)
	}

	// Update with new hash
	doc.Hash = "def456"
	doc.Title = "Updated Go Notes"
	doc.StemmedText = "updat go note"
	doc.Embedding = mustEmbed(embedFn, "updated go notes")
	if err := ix.Index(ctx, doc); err != nil {
		t.Fatalf("second index: %v", err)
	}

	var title string
	if err := database.QueryRow(`SELECT title FROM documents WHERE path = ?`, doc.Path).Scan(&title); err != nil {
		t.Fatalf("select title: %v", err)
	}
	if title != "Updated Go Notes" {
		t.Fatalf("expected updated title, got %s", title)
	}

	// Verify FTS updated
	var ftsTitle string
	if err := database.QueryRow(`SELECT title FROM documents_fts WHERE rowid = (SELECT id FROM documents WHERE path = ?)`, doc.Path).Scan(&ftsTitle); err != nil {
		t.Fatalf("select fts title: %v", err)
	}
	// FTS title is stemmed (raw title lives in documents.title).
	if ftsTitle != "updat go note" {
		t.Fatalf("expected stemmed fts title %q, got %q", "updat go note", ftsTitle)
	}
}

func TestIndex_VectorUpdated(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := NewIndexer(database)
	embedFn := embed.NewMock()

	doc := Document{
		Path:        "/notes/vec.md",
		Hash:        "hash1",
		Title:       "Vector Test",
		Lang:        "en",
		Source:      "local",
		StemmedText: "vector test",
		Embedding:   mustEmbed(embedFn, "vector test first"),
	}

	if err := ix.Index(ctx, doc); err != nil {
		t.Fatalf("first index: %v", err)
	}

	var docID int64
	if err := database.QueryRow(`SELECT id FROM documents WHERE path = ?`, doc.Path).Scan(&docID); err != nil {
		t.Fatalf("get doc id: %v", err)
	}

	// Vectors are now keyed on document_chunks.id, so we JOIN back to find
	// the chunk(s) for this document and fetch their embeddings.
	vecByDoc := `
		SELECT v.embedding
		FROM documents_vec v
		JOIN document_chunks c ON c.id = v.rowid
		WHERE c.document_id = ?
	`

	var firstVec []byte
	if err := database.QueryRow(vecByDoc, docID).Scan(&firstVec); err != nil {
		t.Fatalf("get first vec: %v", err)
	}
	firstF32, err := testutil.DeserializeF32(firstVec)
	if err != nil {
		t.Fatalf("deserialize first vec: %v", err)
	}

	// Update with different embedding.
	doc.Hash = "hash2"
	doc.Embedding = mustEmbed(embedFn, "vector test second")
	if err := ix.Index(ctx, doc); err != nil {
		t.Fatalf("second index: %v", err)
	}

	var secondVec []byte
	if err := database.QueryRow(vecByDoc, docID).Scan(&secondVec); err != nil {
		t.Fatalf("get second vec: %v", err)
	}
	secondF32, err := testutil.DeserializeF32(secondVec)
	if err != nil {
		t.Fatalf("deserialize second vec: %v", err)
	}

	if slicesEqual(firstF32, secondF32) {
		t.Fatalf("expected vector to be updated, but it remained the same")
	}

	// Single-embedding docs produce exactly one chunk, so exactly one vec row.
	var vecCount int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM documents_vec v JOIN document_chunks c ON c.id = v.rowid WHERE c.document_id = ?`,
		docID,
	).Scan(&vecCount); err != nil {
		t.Fatalf("count vec rows: %v", err)
	}
	if vecCount != 1 {
		t.Fatalf("expected 1 vector row, got %d", vecCount)
	}
}

func TestIndex_DocTypesAndSources(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := NewIndexer(database)
	embedFn := embed.NewMock()

	docs := []Document{
		{
			Path:        "/notes/a.md",
			Hash:        "h1",
			Title:       "A",
			Lang:        "en",
			Source:      "github",
			DocType:     "readme",
			StemmedText: "doc a",
			Embedding:   mustEmbed(embedFn, "doc a"),
		},
		{
			Path:        "/notes/b.md",
			Hash:        "h2",
			Title:       "B",
			Lang:        "es",
			Source:      "gitlab",
			DocType:     "changelog",
			StemmedText: "doc b",
			Embedding:   mustEmbed(embedFn, "doc b"),
		},
	}

	for _, doc := range docs {
		if err := ix.Index(ctx, doc); err != nil {
			t.Fatalf("index %s: %v", doc.Path, err)
		}
	}

	for _, doc := range docs {
		var source, docType string
		if err := database.QueryRow(`SELECT source, doc_type FROM documents WHERE path = ?`, doc.Path).Scan(&source, &docType); err != nil {
			t.Fatalf("select %s: %v", doc.Path, err)
		}
		if source != doc.Source {
			t.Fatalf("expected source %q for %s, got %q", doc.Source, doc.Path, source)
		}
		if docType != doc.DocType {
			t.Fatalf("expected doc_type %q for %s, got %q", doc.DocType, doc.Path, docType)
		}
	}
}

func TestIndex_Concurrent(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := NewIndexer(database)
	embedFn := embed.NewMock()

	docs := []Document{
		{Path: "/notes/1.md", Hash: "h1", Title: "One", Lang: "en", Source: "local", DocType: "md", StemmedText: "one", Embedding: mustEmbed(embedFn, "one")},
		{Path: "/notes/2.md", Hash: "h2", Title: "Two", Lang: "en", Source: "local", DocType: "md", StemmedText: "two", Embedding: mustEmbed(embedFn, "two")},
		{Path: "/notes/3.md", Hash: "h3", Title: "Three", Lang: "en", Source: "local", DocType: "md", StemmedText: "three", Embedding: mustEmbed(embedFn, "three")},
		{Path: "/notes/4.md", Hash: "h4", Title: "Four", Lang: "en", Source: "local", DocType: "md", StemmedText: "four", Embedding: mustEmbed(embedFn, "four")},
	}

	var wg sync.WaitGroup
	for _, doc := range docs {
		wg.Add(1)
		go func(d Document) {
			defer wg.Done()
			if err := ix.Index(ctx, d); err != nil {
				t.Errorf("index %s: %v", d.Path, err)
			}
		}(doc)
	}
	wg.Wait()

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&count); err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if count != len(docs) {
		t.Fatalf("expected %d documents, got %d", len(docs), count)
	}

	var ftsCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM documents_fts`).Scan(&ftsCount); err != nil {
		t.Fatalf("count fts: %v", err)
	}
	if ftsCount != len(docs) {
		t.Fatalf("expected %d fts rows, got %d", len(docs), ftsCount)
	}

	var vecCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM documents_vec`).Scan(&vecCount); err != nil {
		t.Fatalf("count vec: %v", err)
	}
	if vecCount != len(docs) {
		t.Fatalf("expected %d vec rows, got %d", len(docs), vecCount)
	}

	for _, doc := range docs {
		var title string
		if err := database.QueryRow(`SELECT title FROM documents WHERE path = ?`, doc.Path).Scan(&title); err != nil {
			t.Fatalf("select %s: %v", doc.Path, err)
		}
		if title != doc.Title {
			t.Fatalf("expected title %q for %s, got %q", doc.Title, doc.Path, title)
		}
	}
}

func TestIndex_MultipleChunksPerDocument(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := NewIndexer(database)
	embedFn := embed.NewMock()

	doc := Document{
		Path:        "/notes/chunked.md",
		Hash:        "h1",
		Title:       "Chunked",
		Lang:        "en",
		Source:      "test",
		DocType:     "markdown",
		StemmedText: "chunked doc full text for fts",
		Chunks: []Chunk{
			{Index: 0, Text: "first chunk text", Embedding: mustEmbed(embedFn, "first chunk text")},
			{Index: 1, Text: "second chunk text", Embedding: mustEmbed(embedFn, "second chunk text")},
			{Index: 2, Text: "third chunk text", Embedding: mustEmbed(embedFn, "third chunk text")},
		},
	}

	if err := ix.Index(ctx, doc); err != nil {
		t.Fatalf("index chunked: %v", err)
	}

	var docID int64
	if err := database.QueryRow(`SELECT id FROM documents WHERE path = ?`, doc.Path).Scan(&docID); err != nil {
		t.Fatalf("get doc id: %v", err)
	}

	var chunkCount int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM document_chunks WHERE document_id = ?`, docID,
	).Scan(&chunkCount); err != nil {
		t.Fatalf("count chunks: %v", err)
	}
	if chunkCount != 3 {
		t.Fatalf("expected 3 chunks, got %d", chunkCount)
	}

	var vecCount int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM documents_vec v JOIN document_chunks c ON c.id = v.rowid WHERE c.document_id = ?`,
		docID,
	).Scan(&vecCount); err != nil {
		t.Fatalf("count vec: %v", err)
	}
	if vecCount != 3 {
		t.Fatalf("expected 3 vec rows, got %d", vecCount)
	}

	// Re-indexing must replace chunks atomically — no leftover chunks or vecs.
	doc.Chunks = []Chunk{
		{Index: 0, Text: "replacement", Embedding: mustEmbed(embedFn, "replacement")},
	}
	if err := ix.Index(ctx, doc); err != nil {
		t.Fatalf("re-index: %v", err)
	}

	if err := database.QueryRow(
		`SELECT COUNT(*) FROM document_chunks WHERE document_id = ?`, docID,
	).Scan(&chunkCount); err != nil {
		t.Fatalf("count chunks after: %v", err)
	}
	if chunkCount != 1 {
		t.Fatalf("expected 1 chunk after reindex, got %d", chunkCount)
	}
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM documents_vec v JOIN document_chunks c ON c.id = v.rowid WHERE c.document_id = ?`,
		docID,
	).Scan(&vecCount); err != nil {
		t.Fatalf("count vec after: %v", err)
	}
	if vecCount != 1 {
		t.Fatalf("expected 1 vec row after reindex, got %d", vecCount)
	}
}

func mustEmbed(fn embed.Func, text string) []float32 {
	v, err := fn(context.Background(), text)
	if err != nil {
		panic(err)
	}
	return v
}

func slicesEqual(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
