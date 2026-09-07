.PHONY: dev build run test fmt

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

dev:
	go run ./cmd/server

test:
	go test ./... -count=1

fmt:
	go fmt ./...