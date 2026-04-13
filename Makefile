.PHONY: build test clean run-sync run-serve

build:
	go build -tags sqlite_fts5 -o marrow .

test:
	go test -tags sqlite_fts5 ./...

clean:
	rm -f marrow marrow.db

run-sync: build
	./marrow sync -dir ./docs

run-serve: build
	./marrow serve -db marrow.db
