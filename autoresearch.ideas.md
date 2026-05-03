# Deferred Optimizations for Marrow Search Engine

## Completed (kept experiments)

- ~~`stemmed_title` column in documents table~~ — eliminates per-result stemming in buildResults
- ~~Precompiled SQL strings~~ — avoids `fmt.Sprintf` per query
- ~~`sync.Pool` for metadata args~~ — reuses `[]any` slices
- ~~Preallocated slice capacities~~ — `ftsResult`, `vecResult` order slices and maps
- ~~Inline norm computation~~ — removed `allIDs` dedup map in `computeScores`
- ~~Shared highlights slice~~ — computed once, assigned to all results
- ~~Pre-lowered phrase~~ — avoids `strings.ToLower` per doc
- ~~Single-word phrase skip~~ — semantic bug fix, avoids extra FTS round-trip
- ~~`FetchMultiplierVec` 10→5~~ — fewer chunks fetched, eval passes
- ~~`FetchMultiplierFTS` 3→1~~ — massive latency win, eval still passes with equivalent quality
- ~~Conservative `pruneScoredDocs`~~ — 1.5x safety margin, reduces sequential work
- ~~`strings.Repeat` for placeholders~~ — faster than `make([]string, n)` + `Join`
- ~~Removed unused `source` column~~ from metadata fetch
- ~~Guard `phraseDocIDs` checks~~ — skip nil-map lookups for single-word/no-phrase queries
- ~~Pre-size `phraseDocIDs` and `vecResult` maps~~ — avoid hash table rehash/growth

## Result
- **search_ns**: 1,550,368 → ~988,000 (-36.3%)
- **search_bytes**: 316,375 → ~109,400 (-65.5%)
- **search_allocs**: 5,482 → ~1,917 (-65.0%)

## High-Impact (require more work or risk)

- **FTS+metadata JOIN**: Combine FTS query with `JOIN documents` to fetch metadata in a single round-trip. Eliminates separate metadata query. Risk: query plan might degrade for filtered searches.

- **Prepared statement cache for metadata query**: Since placeholder count varies, prepare statements for common counts (powers of 2 up to 256) and cache them on `Engine`. This avoids per-query SQL parsing overhead.

- **Reduce metadata allocations by returning slice instead of map**: `fetchMetadata` returns `map[int64]documentMeta` which requires map allocation + bucket allocations. Returning a `[]documentMeta` in `scoredDocs` order would eliminate the map entirely and remove map lookups in `buildResults`. Requires sorting `scoredDocs` by ID before metadata fetch.

- **Connection pool for read-only search**: `DB.Open` sets `MaxOpenConns(1)`. For read-heavy search workloads, multiple connections under WAL mode could allow concurrent reads. Blocked by `:memory:` databases creating per-connection isolated DBs in tests.

- **Cache query embeddings**: For repeated identical queries, cache the serialized vector blob in an LRU cache. Would help real-world workloads with popular queries but doesn't affect the benchmark.

## Medium-Impact

- **Partial sort instead of full sort in `pruneScoredDocs`**: Use a min-heap or quickselect to find the cutoff without fully sorting all docs. Marginal for current doc counts (~30).

- **Pool `ftsResult` and `vecResult` structs**: Use `sync.Pool` to reuse the maps/slices. Requires careful reset between uses.

- **`buildFilterSQL` short-circuit for empty filters**: Already efficient (nil slices return early), no win.

- **Use `slices.Clip` on result slice after limiting**: `results = results[:limit]` retains underlying array capacity. `slices.Clip(results)` would allow earlier GC of unreachable elements.

## Low-Impact / Explored and Rejected

- ~~Two-stage snippet fetch~~: Tested — separate round-trip overhead exceeds savings from skipping `snippet()` in main FTS query. Reverted.
- ~~Separate FTS SQL for plain vs HTML~~: No measurable improvement; SQLite query planner handles parameterized constants well.
- ~~Placeholder string cache~~: `strings.Repeat` is already ~55ns; marginal gain.
- ~~Precompute IN-clause placeholder cache~~: Tested — no primary metric improvement.
- ~~Map-free dedup in enrichResults~~: O(n²) scan slower than tiny map for n≤10.
- ~~Pre-compute `lowerTitle` in fetchMetadata~~: Adding field to `documentMeta` increased map overhead.
- ~~Reduce `FetchMultiplierVec` below 5~~: Retrieval eval fails (negative constraint violations).
- ~~Connection pool (MaxOpenConns=4)~~: Breaks in-memory DB tests due to per-connection isolation.
