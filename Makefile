.PHONY: build build-vps test clean run-sync run-serve landing eval eval-verbose eval-bench eval-cli eval-run eval-report eval-md

# Eval defaults — override on the command line, e.g.
#   make eval-run QRELS=path/to/qrels.json DB=marrow.db PROVIDER=ollama
QRELS    ?= internal/testdata/fixtures/qrels/sample.json
DB       ?= marrow.db
PROVIDER ?= mock
MODEL    ?=
BASE_URL ?=
API_KEY  ?=
CUTOFFS  ?= 1,3,5,10
LIMIT    ?= 10
FORMAT   ?= text

build:
	go build -tags sqlite_fts5 -o marrow .

build-vps:
	go build -tags "sqlite_fts5 vps" -o marrow .

test:
	go test -tags sqlite_fts5 ./...

# Run the in-tree retrieval-quality eval against the deterministic corpus.
# Fast: no external services, uses a mock embedder.
eval:
	go test -tags sqlite_fts5 -run TestRetrievalEvaluation ./internal/search/...

# Same eval with per-query output and verbose logging — useful when a regression
# fires and you need to see which queries are degrading.
eval-verbose:
	go test -tags sqlite_fts5 -v -run TestRetrievalEvaluation ./internal/search/...

# Benchmark end-to-end search+eval throughput on the deterministic corpus.
eval-bench:
	go test -tags sqlite_fts5 -run=^$$ -bench=BenchmarkRetrievalEvaluation -benchmem ./internal/search/...

# Build the standalone eval CLI (`bin/eval`) for running against a real index.
eval-cli:
	go build -tags sqlite_fts5 -o bin/eval ./cmd/eval

# Run the standalone eval CLI against $(QRELS) using the index at $(DB).
# Override PROVIDER/MODEL/BASE_URL/API_KEY to evaluate against real embedders.
# Use FORMAT=md for Markdown output, DETAIL=1 for per-query ranking diffs.
eval-run: eval-cli
	./bin/eval \
		-qrels $(QRELS) \
		-db $(DB) \
		-k $(CUTOFFS) \
		-limit $(LIMIT) \
		-provider $(PROVIDER) \
		-format $(FORMAT) \
		$(if $(DETAIL),-detail) \
		$(if $(MODEL),-model $(MODEL)) \
		$(if $(BASE_URL),-base_url $(BASE_URL)) \
		$(if $(API_KEY),-api_key $(API_KEY))

# Convenience: run the CLI against the in-tree fixture and print a text report.
eval-report: eval-cli
	./bin/eval \
		-qrels internal/testdata/fixtures/qrels/sample.json \
		-db $(DB) \
		-k $(CUTOFFS) \
		-limit $(LIMIT) \
		-provider $(PROVIDER) \
		-format text

# Convenience: run the CLI and emit Markdown (good for CI/GitHub comments).
eval-md: eval-cli
	./bin/eval \
		-qrels $(QRELS) \
		-db $(DB) \
		-k $(CUTOFFS) \
		-limit $(LIMIT) \
		-provider $(PROVIDER) \
		-format md

clean:
	rm -f marrow marrow.db
	rm -rf bin

landing:
	cd landing && npm install && npm run build
	cp landing/dist/index.html index.html

run-sync: build
	./marrow sync -dir ./docs

run-serve: build
	./marrow serve -db marrow.db
