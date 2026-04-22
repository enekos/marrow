# Marrow GitHub Action

Build a semantic "related articles" graph for any Markdown-based static site
(Hugo, Astro, Jekyll, Eleventy, …) as part of your CI pipeline. The action
runs `marrow sync` followed by `marrow related` and drops a `related.json`
file into your workspace that the rest of your workflow can consume — commit
it, upload it as an artifact, feed it to your site generator, or publish it to
a CDN.

## Quick start

```yaml
# .github/workflows/graph.yml
name: Build semantic graph

on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  graph:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build related.json
        id: marrow
        uses: enekos/marrow@v0  # or pin to a specific tag
        with:
          content-dir: content
          embedding-provider: openai
          openai-api-key: ${{ secrets.OPENAI_API_KEY }}
          output-path: data/related.json

      - uses: actions/upload-artifact@v4
        with:
          name: related-graph
          path: ${{ steps.marrow.outputs.related-json-path }}
```

## Output format

`related.json` is an object keyed by the document path. Each entry lists the
top-`N` related documents with scores and human-readable reasons:

```json
{
  "posts/hello-world.md": [
    {
      "path": "posts/intro-to-marrow.md",
      "score": 0.83,
      "reasons": ["semantic", "category:meta"]
    }
  ]
}
```

## Inputs

| Name | Default | Notes |
|------|---------|-------|
| `content-dir` | *(required)* | Directory of Markdown files to index. |
| `source-name` | `site` | Logical source name in the DB. |
| `default-lang` | `en` | Fallback language (`en`, `es`, `eu`). |
| `output-path` | `related.json` | Where to write the graph, relative to the workspace. |
| `db-path` | *(temp dir)* | Override to persist the SQLite DB between steps. |
| `embedding-provider` | `mock` | `mock` or `openai`. `mock` is only suitable for smoke tests. |
| `embedding-model` | `text-embedding-3-small` | OpenAI model name. |
| `embedding-base-url` | *(empty)* | Override for OpenAI-compatible gateways. |
| `openai-api-key` | *(empty)* | Required when provider is `openai`. Store as a secret. |
| `limit` | `10` | Related entries per document. |
| `w-sem` / `w-lex` / `w-link` / `w-cat` | `0.55 / 0.20 / 0.15 / 0.10` | Scoring weights. |
| `mmr-lambda` | `0.72` | Diversity knob (1 = pure relevance). |
| `salient-top-k` | `24` | TF-IDF terms kept per document. |
| `taxonomy-fields` | `categories,categories_meta` | Front-matter fields for the shared-taxonomy signal. |
| `taxonomy-reason` | `category` | Reason prefix in the output. |
| `no-semantic` | `false` | Disable the semantic signal entirely. |
| `workers` | `8` | Concurrent related builder workers. |
| `marrow-version` | `latest` | Release tag to install. |

## Outputs

| Name | Description |
|------|-------------|
| `related-json-path` | Absolute path to the generated `related.json`. |
| `db-path` | Absolute path to the SQLite DB produced during sync. |
| `document-count` | Number of documents in the graph. |

## Committing the graph back

```yaml
- uses: enekos/marrow@v0
  with:
    content-dir: content
    embedding-provider: openai
    openai-api-key: ${{ secrets.OPENAI_API_KEY }}
    output-path: data/related.json

- name: Commit graph
  run: |
    if ! git diff --quiet data/related.json; then
      git config user.name "marrow-bot"
      git config user.email "marrow-bot@users.noreply.github.com"
      git add data/related.json
      git commit -m "chore: refresh semantic graph"
      git push
    fi
```

## Platform support

The action installs the prebuilt `marrow` binary from the latest (or pinned)
GitHub Release. Supported runners: `ubuntu-latest`, `ubuntu-24.04`,
`macos-latest`. Windows runners are not supported.
