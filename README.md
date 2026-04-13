# Marrow

Marrow is a local-first, hybrid search engine for Markdown repositories. It combines full-text search (FTS5) with vector similarity (sqlite-vec) in a single SQLite database.

## Commands

```bash
# Index a directory of markdown files
make run-sync
# or: go run -tags sqlite_fts5 . sync -dir ./docs -db marrow.db

# Start the search API
make run-serve
# or: go run -tags sqlite_fts5 . serve -db marrow.db -addr :8080
```

## API

`POST /search`

```json
{
  "q": "go best practices",
  "limit": 10
}
```

## Build

```bash
make build   # requires -tags sqlite_fts5
make test
```

## Architecture

- `internal/watcher` — crawls directories and tracks file changes via SHA-256
- `internal/markdown` — extracts YAML frontmatter and plain text via goldmark
- `internal/stemmer` — snowball stemming for English/Spanish; Basque falls back to lowercasing
- `internal/embed` — deterministic mock embeddings (stable 384-dim vectors)
- `internal/db` — SQLite schema with `fts5` and `sqlite-vec`
- `internal/index` — ingestion pipeline
- `internal/search` — hybrid ranking: 70% FTS + 30% vector, with 1.2x title boost

## Notes

- The build requires `-tags sqlite_fts5` because FTS5 is enabled via a Go build tag in `mattn/go-sqlite3`.
- `blevesearch/snowball` is used for stemming; it supports English and Spanish. Basque (`eu`) falls back to simple lowercasing because that library does not include a Basque stemmer.
- Embeddings are deterministic mocks derived from SHA-256 hashes, making the project zero-config and fully testable without external services.
