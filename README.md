# Marrow

Marrow is a local-first, hybrid search engine for Markdown repositories. It combines full-text search (FTS5) with vector similarity (sqlite-vec) in a single SQLite database.

It can also index live GitHub issues and pull requests via a GitHub App, making them searchable alongside your documentation.

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
# or: go run -tags sqlite_fts5 . sync -dir ./docs -db marrow.db -source local -default-lang en

# Start the search API (with optional GitHub repo sync)
go run -tags sqlite_fts5 . serve \
  -db marrow.db \
  -addr :8080 \
  -detect-lang=true \
  -default-lang en \
  -repo-url https://github.com/owner/private-repo \
  -repo-token $GITHUB_TOKEN \
  -webhook-secret $WEBHOOK_SECRET \
  -source github \
  -local-path ./repo

# Start with GitHub App integration (issues + PRs)
go run -tags sqlite_fts5 . serve \
  -db marrow.db \
  -addr :8080 \
  -repo-url https://github.com/owner/private-repo \
  -github-app-id 3384614 \
  -github-app-private-key /path/to/app-private-key.pem \
  -github-webhook-secret $GITHUB_WEBHOOK_SECRET
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

When the JSON request does not include `lang`, the server uses automatic language detection by default. You can disable detection and force a default language with the `-detect-lang` and `-default-lang` CLI flags:

- `-detect-lang` — Enable or disable automatic query-language detection (default: `true`).
- `-default-lang` — Fallback language code used when detection is disabled or when a document has no `lang` frontmatter (default: `en`, supported: `en`, `es`, `eu`).

### Webhook
`POST /webhook`

Trigger a re-sync from the configured GitHub repo:

```bash
curl -X POST http://localhost:8080/webhook \
  -H "X-Marrow-Secret: your-secret-key"
```

Returns `202 Accepted` immediately and performs the sync in the background.

### GitHub App Webhooks
When `-github-webhook-secret` is set, `/webhook` also accepts real GitHub App webhooks. It automatically handles:

- `issues` (opened, edited, reopened, closed)
- `pull_request` (opened, edited, reopened, synchronize, closed)
- `issue_comment` / `pull_request_review_comment` (created, edited)

Configure your GitHub App's webhook URL to point to `https://your-host/webhook`.

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
- `internal/githubapi` — GitHub App authenticated client for issues and PRs

## Incremental Sync

- **Local directories**: Only files with `mtime` newer than `last_sync_at` are hashed and re-indexed. Deleted files are removed from the DB.
- **GitHub repos**: `git fetch --depth 1` + `git reset --hard FETCH_HEAD`. Only files changed in the latest commit are re-indexed.
- **GitHub issues & PRs**: Fetched via the GitHub API on startup and kept in sync via webhooks. Closed items are automatically removed from the search index.

## GitHub App Setup

1. Create a GitHub App and note the **App ID**.
2. Generate and download a **private key** (PEM file).
3. Install the app on the target repository or organization.
4. Run Marrow with `-github-app-id`, `-github-app-private-key`, and optionally `-github-webhook-secret`.
5. Set the app's webhook URL to your Marrow instance's `/webhook` endpoint.

GitHub items are indexed with synthetic paths like `gh:owner/repo/issues/123` and `gh:owner/repo/pull/456`. The search API returns a `doc_type` field so you can distinguish between markdown files, issues, and pull requests.

## Notes

- Build requires `-tags sqlite_fts5` because FTS5 is gated behind a build tag in `mattn/go-sqlite3`.
- Stemming is implemented in pure Go: English (Porter2), Spanish (Snowball), and Basque (Snowball). No external NLP services required.
- Embeddings are deterministic mocks derived from SHA-256 hashes, making the project zero-config and fully testable without external services.
