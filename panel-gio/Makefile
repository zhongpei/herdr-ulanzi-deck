.PHONY: build test clean vet run

BINARY := build/herdr-panel

build:
	go build -o $(BINARY) ./cmd/herdr-panel/

run: build
	./$(BINARY)

test:
	go test ./... -count=1

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
