# CLAUDE.md

## Project Overview

tls-switch is a WaterJuice project. The core engine is written in Go, with a Python wrapper that handles CLI argument parsing and dispatches to pre-built Go binaries.

## Architecture

- **Go binary** (`go/main.go`) — the actual engine, cross-compiled for darwin/arm64, linux/arm64, linux/amd64
- **Python package** (`tls_switch/`) — CLI wrapper using argbuilder for argument parsing, detects platform and executes the correct Go binary
- Pre-built Go binaries live in `tls_switch/bin/` (gitignored, included in wheel via hatch artifacts)

## Build System

- `make dev` — set up Python dev environment (.venv)
- `make go-build` — cross-compile Go binaries (static, CGO_ENABLED=0)
- `make build` — full build (lint, go-build, version, docs, wheel)
- `make check` — format check + pyright lint
- `make format` — auto-format Python with ruff
- `make clean` — remove build artefacts and compiled binaries

Go binaries are statically linked (`CGO_ENABLED=0 -ldflags='-s -w'`).

## Code Style

- Use ASCII dashes (`-`) for separator lines, not unicode box-drawing characters
- Python separator lines: `# ` followed by 88 dashes (90 chars total)
- Go separator lines: `// ` followed by 87 dashes (90 chars total)
- All source files have a header block with: filename, description, copyright, version history
- Python follows ruff formatting and pyright strict type checking
- Zero runtime dependencies — Python stdlib only

## Key Files

- `Makefile` — build orchestration
- `pyproject.toml` — Python package config (hatch + uv-dynamic-versioning)
- `go/main.go` — Go entry point
- `go/go.mod` — Go module definition
- `tls_switch/cli.py` — Python CLI with platform detection and binary execution
- `tls_switch/argbuilder.py` — argument parsing library (shared across WaterJuice projects)
- `tls_switch/version.py` — version handling (imports from generated `_version.py`)

## Testing

```bash
make go-build          # must build Go binaries first
uv run tls-switch hello  # runs Go binary via Python wrapper
```
