# Makefile for rift Service

.PHONY: dev build run test clean install lint format help version

# Variables
BINARY_NAME=rift-server
CMD_PATH=./cmd/server

# Version info from git (falls back to 'dev' if not in a git repo)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags with version injection
LDFLAGS = -s -w \
	-X github.com/mohamed-rekiba/rift/internal/cli.Version=$(VERSION) \
	-X github.com/mohamed-rekiba/rift/internal/cli.BuildTime=$(BUILD_TIME) \
	-X github.com/mohamed-rekiba/rift/internal/cli.GitCommit=$(GIT_COMMIT)

# dev: Start the rift server in development mode
dev: clean
	@echo "Starting rift server in development mode..."
	air

## build: Build the rift server binary with version info
build: clean
	@echo "Building rift server $(VERSION)..."
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_PATH)
	@echo "Build complete: $(BINARY_NAME)"

## version: Show version info
version:
	@echo "Version:    $(VERSION)"
	@echo "Build time: $(BUILD_TIME)"
	@echo "Git commit: $(GIT_COMMIT)"

## run: Build and run the rift server with default settings
run: build
	@echo "Starting rift server..."
	./$(BINARY_NAME) -domain localhost -ssh-addr :2222 -http-addr :8080 -log-level info

## run-debug: Build and run with debug logging
run-debug: build
	@echo "Starting rift server (debug mode)..."
	./$(BINARY_NAME) -domain localhost -ssh-addr :2222 -http-addr :8080 -log-level debug

## test: Run tests
test: clean
	@echo "Running tests..."
	go test -v -race ./...

## test-cover: Run tests with coverage
test-cover: clean
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	go clean
	@echo "Clean complete"

## install: Install dependencies
install:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed"

## lint: Run linters
lint: clean
	@echo "Running linters..."
	go vet ./...
	@which staticcheck > /dev/null || go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck ./...
	@which golangci-lint > /dev/null || curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.8.0

	golangci-lint run --fix --color always
	@echo "Linting complete"

## format: Format code
format: clean
	@echo "Formatting code..."
	gofmt -s -w .
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	goimports -w .
	@echo "Formatting complete"

## help: Show this help message
help:
	@echo "rift Service - Makefile Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
