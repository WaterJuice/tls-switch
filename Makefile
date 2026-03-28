GO_BIN_DIR=tls_switch/bin

# Default target: print usage message
.PHONY: help
help:
	@echo "Usage:"
	@echo "  make build        - Build project, platform wheels, and documentation"
	@echo "  make go-build     - Cross-compile Go binaries for all platforms"
	@echo "  make docs         - Build HTML documentation"
	@echo "  make clean        - Clean built package and documentation"
	@echo "  make check        - Format check and lint Go source"
	@echo "  make format       - Format Go source with gofmt"
	@echo "  make dev          - Just create dev (.venv) setup"
	@echo "  make publish      - Publish output/ to PyPI and docs"

# Version string from git tags (falls back to commit hash if no tags)
VERSION_STR=$(shell git describe --tags --always 2>/dev/null | sed 's/-/.post.dev/' | sed 's/-g/-/')

# Cross-compile Go binaries (static, fully self-contained)
# Runs all 10 builds in parallel using background jobs.
.PHONY: go-build
go-build:
	@command -v go >/dev/null 2>&1 || { echo "Error: Go is not installed. Install from https://go.dev/dl/"; exit 1; }
	@mkdir -p $(GO_BIN_DIR)
	(cd go && CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags='-s -w' -o ../$(GO_BIN_DIR)/tls-switch-darwin-arm64      .) & \
	(cd go && CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags='-s -w' -o ../$(GO_BIN_DIR)/tls-switch-darwin-amd64      .) & \
	(cd go && CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags='-s -w' -o ../$(GO_BIN_DIR)/tls-switch-linux-arm64       .) & \
	(cd go && CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags='-s -w' -o ../$(GO_BIN_DIR)/tls-switch-linux-amd64       .) & \
	(cd go && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags='-s -w' -o ../$(GO_BIN_DIR)/tls-switch-windows-amd64.exe .) & \
	(cd go && CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags='-s -w' -o ../$(GO_BIN_DIR)/tls-switch-windows-arm64.exe .) & \
	wait

# Build the project (platform-specific wheels + docs)
.PHONY: build
build: check go-build docs
	rm -rf output/
	mkdir -p output/
	uv build --wheel --out-dir output/tmp
	uv run python scripts/build_wheels.py output/tmp/*.whl output/
	rm -rf output/tmp
	cd html && uv run python -m zipfile -c ../output/tls-switch-$(VERSION_STR)-docs.zip .

# Publish (requires output/ from make build)
.PHONY: publish
publish:
	uv run cal-publish-python --set-latest output/

# Generate CLI help files for documentation
.PHONY: docs-help
docs-help: go-build
	@mkdir -p docs/mkdocs/_include
	COLUMNS=80 $(GO_BIN_DIR)/tls-switch-$$(go env GOOS)-$$(go env GOARCH) --help > docs/mkdocs/_include/help_main.txt 2>&1 || true

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
	rm -rf html/ output/ .venv/
	rm -f $(GO_BIN_DIR)/tls-switch-*

# Check format and lint Go source
.PHONY: check
check:
	gofmt -l go/ | grep . && echo "Go files need formatting (run make format)" && exit 1 || true
	cd go && go vet ./...

# Format Go source
.PHONY: format
format:
	gofmt -w go/

# Create dev setup
.PHONY: dev
dev:
	uv --version 2>/dev/null && true || pip3 install uv
	uv sync
