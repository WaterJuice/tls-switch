# CLAUDE.md

## Project Overview

tls-switch is a WaterJuice project. The core engine is written in Go, with a Python wrapper that handles CLI argument parsing and dispatches to pre-built Go binaries.

## Architecture

- **Go binary** (`go/main.go`) — the actual engine, cross-compiled for 10 platforms
- **Python package** (`tls_switch/`) — CLI wrapper using argbuilder for argument parsing, detects platform and executes the correct Go binary
- Pre-built Go binaries live in `tls_switch/bin/` (gitignored, included in wheel via hatch artifacts)
- **Platform-specific wheels** — `scripts/build_wheels.py` splits a fat wheel into per-platform wheels, each containing only the relevant binary

### Supported Platforms

| Target | Binary | Wheel Tag |
|--------|--------|-----------|
| macOS arm64 | `tls-switch-darwin-arm64` | `macosx_11_0_arm64` |
| macOS amd64 | `tls-switch-darwin-amd64` | `macosx_10_12_x86_64` |
| Linux amd64 | `tls-switch-linux-amd64` | `manylinux_2_17_x86_64` |
| Linux arm64 | `tls-switch-linux-arm64` | `manylinux_2_17_aarch64` |
| Windows amd64 | `tls-switch-windows-amd64.exe` | `win_amd64` |
| Windows arm64 | `tls-switch-windows-arm64.exe` | `win_arm64` |
| FreeBSD amd64 | `tls-switch-freebsd-amd64` | `freebsd_14_0_x86_64` |
| FreeBSD arm64 | `tls-switch-freebsd-arm64` | `freebsd_14_0_aarch64` |
| OpenBSD amd64 | `tls-switch-openbsd-amd64` | `openbsd_7_0_x86_64` |
| OpenBSD arm64 | `tls-switch-openbsd-arm64` | `openbsd_7_0_aarch64` |

## Build System

- `make dev` — set up Python dev environment (.venv)
- `make go-build` — cross-compile Go binaries for all 10 platforms (static, CGO_ENABLED=0)
- `make build` — full build (lint, go-build, version, docs, platform wheels)
- `make check` — format check (ruff + gofmt) + pyright lint
- `make format` — auto-format Python with ruff, Go with gofmt
- `make clean` — remove build artefacts and compiled binaries

Go binaries are statically linked (`CGO_ENABLED=0 -ldflags='-s -w'`).

## Code Style

- Use ASCII dashes (`-`) for separator lines, not unicode box-drawing characters
- Python separator lines: `# ` followed by 88 dashes (90 chars total)
- Go separator lines: `// ` followed by 87 dashes (90 chars total)
- All source files have a header block with: filename, description, copyright, version history
- Python: ruff formatting + pyright strict type checking
- Go: gofmt formatting
- Zero runtime dependencies — Python stdlib only

## Key Files

- `Makefile` — build orchestration
- `pyproject.toml` — Python package config (hatch + uv-dynamic-versioning)
- `go/main.go` — Go entry point
- `go/go.mod` — Go module definition
- `tls_switch/cli.py` — Python CLI with platform detection and binary execution
- `tls_switch/argbuilder.py` — argument parsing library (shared across WaterJuice projects)
- `tls_switch/version.py` — version handling (imports from generated `_version.py`)
- `scripts/build_wheels.py` — splits fat wheel into per-platform wheels

## Testing

```bash
make go-build            # must build Go binaries first
uv run tls-switch hello  # runs Go binary via Python wrapper
make check               # format check + lint
```
