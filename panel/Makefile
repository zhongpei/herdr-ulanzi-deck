.PHONY: build test clean vet

BINARY := build/herdr-panel

build:
	go build -o $(BINARY) ./cmd/herdr-panel/

test:
	go test ./... -count=1

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
