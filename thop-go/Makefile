# thop-go Makefile

# Binary name
BINARY_NAME=thop

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build directory
BUILD_DIR=bin

# Main package
MAIN_PKG=./cmd/thop

# Version info
VERSION?=0.1.0
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Linker flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build: $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)

# Build directory
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Build for release (with optimizations)
.PHONY: release
release: $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -trimpath -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)

# Run the application
.PHONY: run
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run in proxy mode
.PHONY: run-proxy
run-proxy: build
	./$(BUILD_DIR)/$(BINARY_NAME) --proxy

# Run tests
.PHONY: test
test:
	$(GOTEST) -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) ./...

# Vet code
.PHONY: vet
vet:
	$(GOVET) ./...

# Lint code (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Download dependencies
.PHONY: deps
deps:
	$(GOMOD) download

# Tidy dependencies
.PHONY: tidy
tidy:
	$(GOMOD) tidy

# Install binary to GOPATH/bin
.PHONY: install
install:
	$(GOCMD) install $(LDFLAGS) $(MAIN_PKG)

# Cross-compilation targets
.PHONY: build-linux
build-linux: $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PKG)

.PHONY: build-darwin
build-darwin: $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PKG)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PKG)

.PHONY: build-all
build-all: build-linux build-darwin

# Development helpers
.PHONY: dev
dev: fmt vet build

# Check everything before commit
.PHONY: check
check: fmt vet test

# Show help
.PHONY: help
help:
	@echo "thop-go Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all          Build the binary (default)"
	@echo "  build        Build the binary"
	@echo "  release      Build optimized release binary"
	@echo "  run          Build and run the application"
	@echo "  run-proxy    Build and run in proxy mode"
	@echo "  test         Run tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  fmt          Format code"
	@echo "  vet          Vet code"
	@echo "  lint         Lint code (requires golangci-lint)"
	@echo "  clean        Clean build artifacts"
	@echo "  deps         Download dependencies"
	@echo "  tidy         Tidy dependencies"
	@echo "  install      Install binary to GOPATH/bin"
	@echo "  build-linux  Build for Linux amd64"
	@echo "  build-darwin Build for macOS (amd64 and arm64)"
	@echo "  build-all    Build for all platforms"
	@echo "  dev          Format, vet, and build"
	@echo "  check        Format, vet, and test"
	@echo "  help         Show this help"
