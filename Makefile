.PHONY: test lint bench run

test:
	go test ./...

lint:
	go vet ./...

bench:
	go test -bench=. ./...

run:
	go run ./cmd/penda run
