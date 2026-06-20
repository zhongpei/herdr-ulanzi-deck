# herdr-deck Go build system
#
# Usage:
#   make build          # Build for current platform
#   make test           # Run all tests
#   make fmt            # Format all Go code
#   make vet            # Run go vet
#   make lint           # Run golangci-lint (if installed)
#   make clean          # Remove build artifacts
#   MODE=release make build  # Strip debug info, smaller binary

BINARY := herdrdeck
BUILD_DIR := build
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LD_FLAGS := -ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build test fmt vet lint clean

all: fmt vet test build

build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LD_FLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/herdrdeck/

test:
	go test -v -race -count=1 ./pkg/...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	@which golangci-lint 2>/dev/null > /dev/null || (echo "golangci-lint not installed, skipping"; exit 0)
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)

# Cross-compilation helpers
build-linux:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LD_FLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 ./cmd/herdrdeck/
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LD_FLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/herdrdeck/

build-darwin:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LD_FLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd/herdrdeck/
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LD_FLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd/herdrdeck/

build-all: build-linux build-darwin

# Deploy to UlanziDeck (standalone mode - just run the binary, no plugin dir needed)
run: build
	@echo "=== Starting herdr-deck ==="
	@echo "Connect to UlanziDeck at 127.0.0.1:3906"
	$(BUILD_DIR)/$(BINARY)

# Run tests with coverage
coverage:
	go test -coverprofile=$(BUILD_DIR)/coverage.out ./pkg/...
	go tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report: $(BUILD_DIR)/coverage.html"
