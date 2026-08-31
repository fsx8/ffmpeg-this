# ffwiz — common development commands
#
# Prerequisites: Go toolchain (see go.mod) and ffmpeg/ffprobe in PATH
# for running the app, integration tests and fixture generation.

BINARY  := ffwiz
PKG     := ./cmd/ffwiz
GOFILES := $(shell find . -name '*.go' -not -path './npm/*')

.DEFAULT_GOAL := help
.PHONY: help build run fmt fmt-check vet lint tidy test testmedia test-integration test-all check clean demo release-snapshot

##@ Development

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

build: ## Build the ffwiz binary into the repo root
	go build -o $(BINARY) $(PKG)

run: ## Build and run the TUI (pass ARGS="file.mp4" to open with a path)
	go run $(PKG) $(ARGS)

##@ Quality

fmt: ## Format all Go sources in place
	gofmt -w $(GOFILES)

fmt-check: ## Check formatting without modifying files
	@out=$$(gofmt -l $(GOFILES)); \
	if [ -n "$$out" ]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi

vet: ## Run go vet
	go vet ./...

lint: fmt-check vet ## Run all static checks (fmt-check + vet)

tidy: ## Run go mod tidy
	go mod tidy

##@ Testing

test: ## Run unit tests (no FFmpeg execution)
	go test ./...

testmedia: ## Generate deterministic test fixtures into ./testmedia (~25s)
	./scripts/testmedia.sh

test-integration: testmedia ## Run integration tests (real ffmpeg; builds fixtures first)
	go test -tags=integration ./...

test-all: test test-integration ## Run unit + integration tests

check: lint build test ## Full pre-flight: lint, build and unit tests

##@ Cleanup & Extras

clean: ## Remove build artifacts, session log and generated fixtures
	rm -f $(BINARY) ffmpeg_log.txt
	rm -rf testmedia

demo: build testmedia ## Regenerate docs/demo.gif with vhs (needs vhs installed)
	vhs docs/demo.tape

release-snapshot: ## Build a local goreleaser snapshot (no publishing)
	goreleaser release --snapshot --clean
