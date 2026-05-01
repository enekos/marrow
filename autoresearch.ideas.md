# Deferred Optimizations for Marrow Search Engine

## High-Impact (require more work or risk)

- **FTS query optimization with SQLite schema changes**: Add `stemmed_title` to `documents` table (DONE). Consider adding a `documents_fts` covering index or experimenting with `rank` vs `bm25()` ordering.

- **Prepared statement cache for metadata query**: Since placeholder count varies, prepare statements for common counts (powers of 2 up to 256) and cache them on `Engine`. This avoids per-query SQL parsing overhead.

- **Reduce metadata allocations by returning slice instead of map**: `fetchMetadata` returns `map[int64]documentMeta` which requires map allocation + bucket allocations. Returning a `[]documentMeta` in `scoredDocs` order would eliminate the map entirely and remove map lookups in `buildResults`.

- **Vector query deduplication at DB level**: sqlite-vec returns one row per chunk. We deduplicate by `document_id` in Go. If sqlite-vec ever supports `GROUP BY` or window functions efficiently, we could push deduplication to the DB.

- **Connection pool for read-only search**: `DB.Open` sets `MaxOpenConns(1)`. For read-heavy search workloads, multiple connections could allow concurrent reads under WAL mode. Need to verify go-sqlite3 behavior.

- **Cache query embeddings**: For repeated identical queries, cache the serialized vector blob in an LRU cache. Would help real-world workloads with popular queries.

## Medium-Impact

- **Precompute `placeholders` strings in a lookup table**: `strings.Repeat("?,", n)` is fast (~55ns) but a `[257]string` cache would be even faster and allocation-free.

- **Pool `ftsResult` and `vecResult` structs**: Use `sync.Pool` to reuse the maps/slices. Requires careful reset between uses.

- **Optimize `buildFilterSQL` for empty filters**: Short-circuit early when `Filter` is zero-value to avoid slice allocations.

- **Use `slices.Clip` on result slice after limiting**: `results = results[:limit]` retains the underlying array capacity. `slices.Clip(results)` would allow earlier GC of unreachable elements.

## Low-Impact / Explored and Rejected

- ~~Two-stage snippet fetch~~: Tested — separate round-trip overhead exceeds savings from skipping `snippet()` in main FTS query. Reverted.
- ~~Separate FTS SQL for plain vs HTML~~: No measurable improvement; SQLite query planner handles parameterized constants well.
- ~~Placeholder string cache~~: `strings.Repeat` is already ~55ns; marginal gain.
- ~~Precompute IN-clause placeholder cache~~: Tested — no primary metric improvement.
