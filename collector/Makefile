# herdr-collector build system
BINARY := herdr-collector
BUILD_DIR := build

.PHONY: all build test fmt vet clean run

all: fmt vet test build

build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY) ./cmd/herdr-collector/

test:
	go test -v -race -count=1 ./internal/...

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR)

run: build
	$(BUILD_DIR)/$(BINARY) --debug
