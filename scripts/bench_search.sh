#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Run search engine benchmarks with enough iterations for stable results.
# We focus on a representative workload (1k docs, 3 chunks) and emit METRIC lines.

echo "=== Running search engine benchmarks ==="

# Representative single-query benchmark
RESULT=$(go test -tags sqlite_fts5 \
  -bench='BenchmarkSearch_CorpusSize/medium_1k_3chunk' \
  -benchtime=200ms -count=3 -benchmem \
  ./internal/search/ 2>/dev/null)

# Extract ns/op, B/op, allocs/op
NSOP=$(echo "$RESULT" | grep 'medium_1k_3chunk' | awk '{print $3}' | sed 's/ns\/op//' | head -1)
BOP=$(echo "$RESULT" | grep 'medium_1k_3chunk' | awk '{print $5}' | sed 's/B\/op//' | head -1)
ALLOPS=$(echo "$RESULT" | grep 'medium_1k_3chunk' | awk '{print $7}' | sed 's/allocs\/op//' | head -1)

echo "METRIC search_ns=$NSOP"
echo "METRIC search_bytes=$BOP"
echo "METRIC search_allocs=$ALLOPS"

# Component breakdown benchmark
RESULT_COMP=$(go test -tags sqlite_fts5 \
  -bench='BenchmarkSearch_ComponentBreakdown' \
  -benchtime=200ms -count=3 -benchmem \
  ./internal/search/ 2>/dev/null)

# Extract component metrics. Go benchmark output lists metrics as
# "value unit" pairs separated by tabs. We take the first result line,
# split on tabs, find the field containing the metric name, and extract
# the leading number with awk.
for metric in prepare_ns embed_ns fts_ns vec_ns phrase_ns score_ns meta_ns build_ns enrich_ns; do
  VAL=$(echo "$RESULT_COMP" | grep 'ComponentBreakdown' | head -1 | tr '\t' '\n' | grep "${metric}/op" | awk '{print $1}')
  if [ -n "$VAL" ]; then
    echo "METRIC ${metric}=$VAL"
  fi
done

# Parallel throughput benchmark (1k docs)
RESULT_PAR=$(go test -tags sqlite_fts5 \
  -bench='BenchmarkSearch_Parallel/1k' \
  -benchtime=200ms -count=3 -benchmem \
  ./internal/search/ 2>/dev/null)

NSOP_PAR=$(echo "$RESULT_PAR" | grep 'Parallel/1k' | awk '{print $3}' | sed 's/ns\/op//' | head -1)
BOP_PAR=$(echo "$RESULT_PAR" | grep 'Parallel/1k' | awk '{print $5}' | sed 's/B\/op//' | head -1)
ALLOPS_PAR=$(echo "$RESULT_PAR" | grep 'Parallel/1k' | awk '{print $7}' | sed 's/allocs\/op//' | head -1)

echo "METRIC parallel_ns=$NSOP_PAR"
echo "METRIC parallel_bytes=$BOP_PAR"
echo "METRIC parallel_allocs=$ALLOPS_PAR"

# Scale sweep for regression detection
for size in 100 500 1000 5000; do
  RESULT_SCALE=$(go test -tags sqlite_fts5 \
    -bench="BenchmarkSearch_ScaleDocs/${size}_docs" \
    -benchtime=200ms -count=3 -benchmem \
    ./internal/search/ 2>/dev/null)
  NSOP_SCALE=$(echo "$RESULT_SCALE" | grep "${size}_docs" | awk '{print $3}' | sed 's/ns\/op//' | head -1)
  BOP_SCALE=$(echo "$RESULT_SCALE" | grep "${size}_docs" | awk '{print $5}' | sed 's/B\/op//' | head -1)
  if [ -n "$NSOP_SCALE" ]; then
    echo "METRIC scale_${size}_ns=$NSOP_SCALE"
    echo "METRIC scale_${size}_bytes=$BOP_SCALE"
  fi
done

echo "=== Benchmarks complete ==="
