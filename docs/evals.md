# Retrieval Evaluation Guide

Marrow includes a built-in retrieval-evaluation framework for measuring search quality against ground-truth relevance judgments. Evals help catch regressions when you change stemming, ranking weights, embedding providers, or the search pipeline.

## Quick Start

### In-tree eval (fast, no external services)

```bash
make eval          # run the deterministic corpus eval
make eval-verbose  # same, with per-query output
make eval-bench    # benchmark search+eval throughput
```

### Standalone CLI (evaluate your real index)

```bash
make eval-cli                      # build the CLI
make eval-run QRELS=my-qrels.json  # run against marrow.db
make eval-md QRELS=my-qrels.json   # emit Markdown for CI
```

## QRel Format

A **QRel** (query-relevance) file is JSON that defines what you expect the search engine to return for a set of queries.

```json
{
  "queries": [
    {
      "query": "go modules",
      "lang": "en",
      "category": "language-specific",
      "description": "Tests retrieval of Go module documentation",
      "relevant": ["/go/modules.md", "/go/best-practices.md"],
      "negative": ["/python/asyncio.md"],
      "graded_relevance": {
        "/go/modules.md": 3,
        "/go/best-practices.md": 1
      },
      "variants": ["golang modules", "go dependency management"],
      "min_metrics": {
        "MRR": 0.5,
        "P@5": 0.4
      }
    }
  ]
}
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `query` | **yes** | The search query string. |
| `lang` | no | Language hint (`en`, `es`, `eu`). When set, results are filtered to that language. |
| `relevant` | **yes** | List of document paths that should be returned for this query. |
| `category` | no | Grouping label for reporting (e.g. `language-specific`, `cross-language`, `edge-case`). |
| `description` | no | Human-readable explanation of what this query tests. |
| `negative` | no | Document paths that must **not** appear in the top-N results. |
| `graded_relevance` | no | Map of doc path → relevance grade (0–3). Used for graded NDCG. |
| `variants` | no | Semantically equivalent phrasings. Each variant is evaluated as a separate query. |
| `min_metrics` | no | Per-query pass/fail thresholds (e.g. `{"P@5": 0.6, "MRR": 0.5}`). |

## Metrics

The evaluator computes standard IR metrics at every cutoff you specify (default: 1, 3, 5, 10):

- **P@K** — Precision at K
- **R@K** — Recall at K
- **F1@K** — Harmonic mean of precision and recall
- **NDCG@K** — Normalized Discounted Cumulative Gain (uses graded relevance when provided)
- **HR@K** — Hit Rate (1.0 if ≥1 relevant doc is in top K)
- **MRR** — Mean Reciprocal Rank of the first relevant document
- **AP** — Average Precision
- **R-Prec** — Precision at rank = number of relevant documents

## Reading the Report

### Text output (default)

```
=== Retrieval Evaluation Report ===
48 queries evaluated across 7 categories

--- Category: language-specific (11 queries) ---
Query                         P@1   P@3   P@5   P@10  MRR   AP    ✓/✗
────────────────────────────  ────  ────  ────  ────  ────  ────  ───
go                            1.00  1.00  1.00  1.00  1.00  1.00  ✓
go modules                    1.00  0.33  0.20  0.10  1.00  1.00  ✓
...
Category mean                 1.00  0.70  0.58  0.45  1.00  0.90

✓ All queries passed their thresholds and constraints.

Aggregate:
  Mean P@1: 0.938  R@1: 0.425  F1@1: 0.527  NDCG@1: 0.938  HR@1: 0.938
  ...
  MRR:  0.9531
  MAP:  0.8820

Per-category summary:
  language-specific       MAP: 0.900  MRR: 1.000  pass: 11/11  ✓
  ...
```

Symbols:
- **✓** — query passed all thresholds and negative constraints
- **✗** — query failed at least one threshold or negative constraint

When failures occur, the report lists the specific queries, their failure reasons, and the actual ranking.

### Markdown output

Use `-format md` for GitHub-flavoured Markdown tables, useful for CI comments or documentation.

## Adding New Evaluation Queries

1. Edit `internal/search/eval_test.go`.
2. Add documents to `evalCorpus` if needed.
3. Add a `eval.QRel` to `evalQrels` with a `category` and `description`.
4. Run `make eval-verbose` to see how the engine currently ranks for your query.
5. Adjust `relevant` and `negative` judgments to match reality.
6. Optionally set `min_metrics` to enforce a quality floor.
7. Run `UPDATE_TRUTH=1 make eval` to regenerate the approved golden file.

## Golden File Regression Testing

The in-tree eval stores a full text snapshot in:

```
internal/search/testdata/TestRetrievalEvaluation.retrieval_evaluation.approved.txt
```

If you change ranking logic and the snapshot changes, the test fails with a diff. To accept the new output:

```bash
UPDATE_TRUTH=1 make eval
```

Always review the diff before updating the truth file — a change in scores may indicate a real regression.

## Tips

- **Start with `eval-verbose`** when debugging a failing query. It prints the actual ranking.
- **Use `negative` judgments** to ensure broad queries don't surface irrelevant docs.
- **Use `variants`** to test robustness against rephrasing (requires a real embedder for semantic variants).
- **Use `graded_relevance`** when some docs are "somewhat relevant" rather than fully relevant.
- **Keep categories small and focused** so the per-category summary quickly pinpoints problem areas.
