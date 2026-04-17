.PHONY: build test clean run-sync run-serve landing

build:
	go build -tags sqlite_fts5 -o marrow .

test:
	go test -tags sqlite_fts5 ./...

clean:
	rm -f marrow marrow.db

landing:
	cd landing && npm install && npm run build
	cp landing/dist/index.html index.html

run-sync: build
	./marrow sync -dir ./docs

run-serve: build
	./marrow serve -db marrow.db
