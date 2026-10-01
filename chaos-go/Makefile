.PHONY: build run test tidy

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy
