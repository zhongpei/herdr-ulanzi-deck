# herdr-deck build system
BINARY := herdr-deck
BUILD_DIR := build

.PHONY: all build test fmt vet clean run

all: fmt vet test build

build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -o $(BUILD_DIR)/$(BINARY) ./cmd/herdr-deck/

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
