BINARY := gatherer

.DEFAULT_GOAL := build

build:
	go build -o $(BINARY) ./cmd/gatherer

test:
	go test ./...

vet:
	go vet ./...

scry: build
	./$(BINARY) scry --deck decklist.example.json

install:
	go install ./cmd/gatherer

clean:
	rm -f $(BINARY)

.PHONY: build test vet scry install clean
