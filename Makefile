.PHONY: all proto build test clean collector query docker-up docker-down lint

BINARY_DIR := bin
GO := go

all: proto build

# Generate protobuf code
proto:
	protoc --proto_path=proto \
		--go_out=proto/gen --go_opt=paths=source_relative \
		--go-grpc_out=proto/gen --go-grpc_opt=paths=source_relative \
		proto/span.proto proto/collector.proto

# Build binaries
build: collector query

collector:
	$(GO) build -o $(BINARY_DIR)/prism-collector ./cmd/collector

query:
	$(GO) build -o $(BINARY_DIR)/prism-query ./cmd/query

# Run tests
test:
	$(GO) test -v -race ./...

# Lint
lint:
	golangci-lint run ./...

# Docker compose
docker-up:
	cd deploy && docker compose up -d

docker-down:
	cd deploy && docker compose down

docker-build:
	cd deploy && docker compose build

# Clean
clean:
	rm -rf $(BINARY_DIR)
	$(GO) clean ./...

# Dependencies
deps:
	$(GO) mod tidy
