BINARY_NAME := atlassian-mcp
BIN_DIR := bin
SRC_MAIN := ./cmd/atlassian-mcp

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "1.0.0")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -s -w \
	-X main.Version=$(VERSION)

.PHONY: all build clean test test-race test-coverage run lint tidy

all: build

## build: Compile the binary to bin/atlassian-mcp
build:
	@mkdir -p $(BIN_DIR)
	@echo "==> Building $(BINARY_NAME) (version: $(VERSION))..."
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) $(SRC_MAIN)
	@echo "==> Binary created at $(BIN_DIR)/$(BINARY_NAME)"

## run: Run the server locally
run: build
	./$(BIN_DIR)/$(BINARY_NAME)

## test: Run unit tests
test:
	@echo "==> Running tests..."
	go test -v ./...

## test-race: Run unit tests with data race detector
test-race:
	@echo "==> Running tests with race detector..."
	go test -race ./...

## test-coverage: Run tests and generate coverage profile
test-coverage:
	@echo "==> Running tests with coverage..."
	@mkdir -p coverage
	go test -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "==> HTML coverage report available at coverage/coverage.html"

## lint: Run go vet and staticcheck/golangci-lint if installed
lint:
	@echo "==> Running go vet..."
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "==> Running golangci-lint..."; \
		golangci-lint run; \
	fi

## tidy: Run go mod tidy
tidy:
	@echo "==> Running go mod tidy..."
	go mod tidy

## clean: Remove build artifacts and temporary files
clean:
	@echo "==> Cleaning..."
	rm -rf $(BIN_DIR) coverage
