.PHONY: build test test-unit test-integration run docker clean help

# Variables
BINARY_NAME := hlnode-websocket
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)"

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build the binary
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/server

## test: Run all tests (unit tests only, integration tests require WS_COMPARE)
test: test-unit

## test-unit: Run unit tests with coverage
test-unit:
	go test -v -race -coverprofile=coverage.txt ./...

## test-integration: Run integration tests against a nanoreth node
## Usage: make test-integration WS_COMPARE=ws://your-node:8545
test-integration:
	@if [ -z "$(WS_COMPARE)" ]; then \
		echo "Error: WS_COMPARE is required. Set it to a nanoreth node WebSocket URL."; \
		echo "Usage: make test-integration WS_COMPARE=ws://your-node:8545"; \
		exit 1; \
	fi
	WS_COMPARE=$(WS_COMPARE) go test -v ./internal/integration/...

## run: Run the server locally
run: build
	./$(BINARY_NAME)

## docker: Build Docker image
docker:
	docker build -t $(BINARY_NAME):$(VERSION) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) .

## docker-run: Run Docker container
docker-run: docker
	docker run --rm -p 8080:8080 -e RPC_URL=$(RPC_URL) $(BINARY_NAME):$(VERSION)

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME) coverage.txt

## deps: Download dependencies
deps:
	go mod download
	go mod tidy
