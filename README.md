# Marrow

Marrow is a local-first, hybrid search engine for Markdown repositories. It combines full-text search (FTS5) with vector similarity (sqlite-vec) in a single SQLite database.

## Installation (no Go required)

### macOS & Linux — one-liner install

```bash
curl -sSL https://raw.githubusercontent.com/enekos/marrow/main/install.sh | sh
```

This downloads the latest release for your platform and copies the `marrow` binary into `~/.local/bin` (or `/usr/local/bin` if writable).

### Windows

1. Go to the [Releases](https://github.com/enekos/marrow/releases) page.
2. Download `marrow_vX.Y.Z_windows_amd64.zip`.
3. Extract `marrow.exe` and place it in a folder on your `PATH`.

### Manual download

Visit the [Releases](https://github.com/enekos/marrow/releases) page, pick the archive matching your OS and architecture, extract it, and move the `marrow` binary to a directory on your `PATH`.

## Commands

```bash
# Index a local directory of markdown files
make run-sync
# or: go run -tags sqlite_fts5 . sync -dir ./docs -db marrow.db -source local

# Start the search API (with optional GitHub repo sync)
go run -tags sqlite_fts5 . serve \
  -db marrow.db \
  -addr :8080 \
  -repo-url https://github.com/owner/private-repo \
  -repo-token $GITHUB_TOKEN \
  -webhook-secret $WEBHOOK_SECRET \
  -source github \
  -local-path ./repo
```

## API

### Search
`POST /search`

```json
{
  "q": "go best practices",
  "limit": 10,
  "lang": "en"
}
```

### Webhook
`POST /webhook`

Trigger a re-sync from the configured GitHub repo:

```bash
curl -X POST http://localhost:8080/webhook \
  -H "X-Marrow-Secret: your-secret-key"
```

Returns `202 Accepted` immediately and performs the sync in the background.

## Build

```bash
make build   # requires -tags sqlite_fts5
make test
```

## Architecture

- `internal/watcher` — incremental local directory crawler using mtime + SHA-256
- `internal/gitpull` — clones or pulls GitHub repos with token auth; returns changed file list
- `internal/sync` — orchestrates the full ingestion pipeline and deletes stale docs
- `internal/markdown` — extracts YAML frontmatter and plain text via goldmark
- `internal/stemmer` — Unicode-aware tokenization, stopword removal, Snowball stemming (en/es/eu)
- `internal/embed` — deterministic mock embeddings (stable 384-dim vectors)
- `internal/db` — SQLite schema with `fts5`, `sqlite-vec`, and `sync_state` table
- `internal/index` — ingestion pipeline with source tracking
- `internal/search` — hybrid ranking via Reciprocal Rank Fusion (70% FTS + 30% vector) + stemmed title boost

## Incremental Sync

- **Local directories**: Only files with `mtime` newer than `last_sync_at` are hashed and re-indexed. Deleted files are removed from the DB.
- **GitHub repos**: `git fetch --depth 1` + `git reset --hard FETCH_HEAD`. Only files changed in the latest commit are re-indexed.

## Notes

- Build requires `-tags sqlite_fts5` because FTS5 is gated behind a build tag in `mattn/go-sqlite3`.
- Stemming is implemented in pure Go: English (Porter2), Spanish (Snowball), and Basque (Snowball). No external NLP services required.
- Embeddings are deterministic mocks derived from SHA-256 hashes, making the project zero-config and fully testable without external services.
