MODULE_NAME="tls_switch"

# Go binary targets
GO_SRC=go/main.go
GO_BIN_DIR=tls_switch/bin
GO_TARGETS=$(GO_BIN_DIR)/tls-switch-darwin-arm64 $(GO_BIN_DIR)/tls-switch-linux-arm64 $(GO_BIN_DIR)/tls-switch-linux-amd64

# Default target: print usage message
.PHONY: help
help:
	@echo "Usage:"
	@echo "  make build        - Build project and documentation"
	@echo "  make go-build     - Cross-compile Go binaries"
	@echo "  make docs         - Build HTML documentation"
	@echo "  make clean        - Clean built package and documentation"
	@echo "  make check        - Format check and lint source"
	@echo "  make format       - Format source using Ruff"
	@echo "  make black        - Format source using Black (for long strings)"
	@echo "  make lint         - Lint source using pyright"
	@echo "  make dev          - Just create dev (.venv) setup"
	@echo "  make publish      - Publish output/ to PyPI and docs"

# Version string from git tags (falls back to commit hash if no tags)
VERSION_STR=$(shell git describe --tags --always 2>/dev/null | sed 's/-/.post.dev/' | sed 's/-g/-/')

# Generate _version.py with the current version
.PHONY: version
version:
	@echo '__version__ = "$(VERSION_STR)"' > tls_switch/_version.py

# Cross-compile Go binaries (static, fully self-contained)
.PHONY: go-build
go-build:
	cd go && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags='-s -w' -o ../$(GO_BIN_DIR)/tls-switch-darwin-arm64 .
	cd go && CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build -ldflags='-s -w' -o ../$(GO_BIN_DIR)/tls-switch-linux-arm64  .
	cd go && CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -ldflags='-s -w' -o ../$(GO_BIN_DIR)/tls-switch-linux-amd64  .

# Build the project
.PHONY: build
build: check-dependencies format-check lint go-build version docs
	rm -rf output/
	uv build --out-dir output
	rm -f output/*.tar.gz
	cd html && uv run python -m zipfile -c ../output/tls-switch-$(VERSION_STR)-docs.zip .

# Publish (requires output/ from make build)
.PHONY: publish
publish: check-dependencies
	uv run cal-publish-python --set-latest output/

# Generate CLI help files for documentation
.PHONY: docs-help
docs-help: check-dependencies version
	@mkdir -p docs/mkdocs/_include
	COLUMNS=80 uv run tls-switch --help > docs/mkdocs/_include/help_main.txt

# Build the documentation
.PHONY: docs
docs: docs-help
	rm -rf html/
	VERSION=$(VERSION_STR) uv run cal-mkdocs -f docs/mkdocs.yml -d docs/mkdocs -o html/
	cp docs/docinfo.* html/
	rm -rf docs/mkdocs/_include html/_include

# Clean build artefacts
.PHONY: clean
clean: check-dependencies
	rm -rf html/ output/
	rm -f $(GO_TARGETS)
	uv clean

# Check the format of code
.PHONY: check
check: format-check lint

# Check the format of code
.PHONY: format-check
format-check: check-dependencies
	uv run ruff format --check .
	uv run ruff check .

# Fix format of the code
.PHONY: format
format: check-dependencies
	uv run ruff format .
	uv run ruff check . --fix

# Fix format of the code
.PHONY: black
black: check-dependencies
	uv run black --preview --enable-unstable-feature string_processing .
	uv run ruff format .
	uv run ruff check . --fix

# Lint the code
.PHONY: lint
lint: check-dependencies
	uv run pyright

# Just create dev (.venv) setup
.PHONY: dev
dev: check-dependencies

# Check if uv is installed, install it if not
.PHONY: check-dependencies
check-dependencies: version
	uv --version 2>/dev/null && true || pip3 install uv
	uv sync
