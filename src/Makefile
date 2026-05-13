VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BINARY  := lazytalos
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build clean test vet all

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/lazytalos

clean:
	rm -f $(BINARY)

test:
	go test -race ./...

vet:
	go vet ./...

all: vet test build
