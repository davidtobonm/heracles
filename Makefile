VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILT ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/davidtobonm/heracles/internal/buildinfo.version=$(VERSION) -X github.com/davidtobonm/heracles/internal/buildinfo.commit=$(COMMIT) -X github.com/davidtobonm/heracles/internal/buildinfo.built=$(BUILT)

.PHONY: build check clean install test

build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/heracles ./cmd/heracles

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test -race ./...
	go test -tags=integration ./integration/...
	mkdir -p bin
	go build -o bin/heracles-check ./cmd/heracles

clean:
	rm -rf bin dist

install:
	go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/heracles

test:
	go test ./...
