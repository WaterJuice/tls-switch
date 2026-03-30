DIST_DIR=dist

# Default target: print usage message
.PHONY: help
help:
	@echo "Usage:"
	@echo "  make build        - Build platform wheels and documentation"
	@echo "  make go-build     - Cross-compile Go binaries for all platforms"
	@echo "  make docs         - Build HTML documentation"
	@echo "  make clean        - Clean build artefacts"
	@echo "  make check        - Format check and lint Go source"
	@echo "  make format       - Format Go source with gofmt"
	@echo "  make dev          - Install dev dependencies"
	@echo "  make publish      - Publish output/ to PyPI and docs"

# Version string from git tags (falls back to commit hash if no tags)
VERSION_STR=$(shell git describe --tags --always 2>/dev/null | sed 's/-/.post.dev/' | sed 's/-g/-/')
GO_LDFLAGS=-s -w -X main.Version=$(VERSION_STR)

# Cross-compile Go binaries for all platforms
.PHONY: go-build
go-build:
	@command -v go >/dev/null 2>&1 || { echo "Error: Go is not installed. Install from https://go.dev/dl/"; exit 1; }
	@mkdir -p $(DIST_DIR)
	(CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags='$(GO_LDFLAGS)' -o $(DIST_DIR)/tls-switch-darwin-arm64      .) & \
	(CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags='$(GO_LDFLAGS)' -o $(DIST_DIR)/tls-switch-darwin-amd64      .) & \
	(CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags='$(GO_LDFLAGS)' -o $(DIST_DIR)/tls-switch-linux-arm64       .) & \
	(CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags='$(GO_LDFLAGS)' -o $(DIST_DIR)/tls-switch-linux-amd64       .) & \
	(CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags='$(GO_LDFLAGS)' -o $(DIST_DIR)/tls-switch-windows-amd64.exe .) & \
	(CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags='$(GO_LDFLAGS)' -o $(DIST_DIR)/tls-switch-windows-arm64.exe .) & \
	wait

# Build platform wheels + docs
.PHONY: build
build: check go-build docs
	rm -rf output/
	uv --version 2>/dev/null && true || pip3 install uv
	uv sync
	sed 's/"version": *"[^"]*"/"version": "$(VERSION_STR)"/' wheel.json > wheel-build.json
	uv run bin2whl -c wheel-build.json
	rm -f wheel-build.json
	cd html && python3 -m zipfile -c ../output/tls-switch-$(VERSION_STR)-docs.zip .

# Publish (requires output/ from make build)
.PHONY: publish
publish:
	uv run cal-publish-python --set-latest output/

# Generate CLI help files for documentation
.PHONY: docs-help
docs-help: go-build
	@mkdir -p docs/mkdocs/_include
	COLUMNS=80 $(DIST_DIR)/tls-switch-$$(go env GOOS)-$$(go env GOARCH) --help > docs/mkdocs/_include/help_main.txt 2>&1 || true

# Build the documentation
.PHONY: docs
docs: docs-help
	rm -rf html/
	uv --version 2>/dev/null && true || pip3 install uv
	uv sync
	VERSION=$(VERSION_STR) uv run cal-mkdocs -f docs/mkdocs.yml -d docs/mkdocs -o html/
	cp docs/docinfo.* html/
	rm -rf docs/mkdocs/_include html/_include

# Clean build artefacts
.PHONY: clean
clean:
	rm -rf html/ output/ dist/ .venv/

# Check format and lint Go source
.PHONY: check
check:
	gofmt -l *.go internal/*.go | grep . && echo "Go files need formatting (run make format)" && exit 1 || true
	go vet ./...

# Format Go source
.PHONY: format
format:
	gofmt -w *.go internal/*.go

# Dev setup: build for current platform + symlink into venv
.PHONY: dev
dev: go-build
	uv --version 2>/dev/null && true || pip3 install uv
	uv sync
	@ln -sf $$(pwd)/dist/tls-switch-$$(go env GOOS)-$$(go env GOARCH) .venv/bin/tls-switch
	@echo "tls-switch linked into .venv/bin/"

# Run with arguments (e.g. make run ARGS="-c local/config.json")
.PHONY: run
run: go-build
	@dist/tls-switch-$$(go env GOOS)-$$(go env GOARCH) $(ARGS)
