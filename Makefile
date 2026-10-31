SHELL := /bin/bash
GO ?= go
BIN_DIR ?= bin
VERSION := $(shell sed -n 's/^var version = "\([^"]*\)"/\1/p' model.go)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all help build build-arm64 build-amd64 release test vet fmt fmt-check check install clean

all: build

help:
	@printf '%s\n' 'make build       Build antmove for this Mac' \
		'make build-arm64 Build a macOS Apple Silicon binary' \
		'make build-amd64 Build a macOS Intel binary' \
		'make release     Create both release archives in dist/' \
		'make test        Run Go tests' \
		'make check       Check formatting, vet, and run tests' \
		'make install     Build and install antmove/am for this user'

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build: $(BIN_DIR)
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/antmove .

build-arm64: $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/antmove-darwin-arm64 .

build-amd64: $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/antmove-darwin-amd64 .

release:
	./build.sh

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w *.go

fmt-check:
	@files="$$(gofmt -l *.go)"; if [[ -n "$$files" ]]; then printf 'Go files need formatting:\n%s\n' "$$files"; echo 'Run make fmt'; exit 1; fi

check: fmt-check vet test

install: build
	./$(BIN_DIR)/antmove install

clean:
	rm -f $(BIN_DIR)/antmove $(BIN_DIR)/antmove-darwin-arm64 $(BIN_DIR)/antmove-darwin-amd64

