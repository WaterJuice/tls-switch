# CLAUDE.md

## Project Overview

tls-switch is a WaterJuice project. It is an SNI-based TLS reverse proxy that routes incoming TLS connections to backend servers based on hostname. It supports two modes per host:

- **terminate** — complete the TLS handshake with configured cert/key, forward plaintext to backend
- **passthrough** — forward the raw TLS stream (including ClientHello) to the backend, which handles TLS itself

The core networking engine is written in Go. Python handles CLI, config parsing, certificate validation, file watching, and error reporting.

### Design Principles

- **Never interrupt existing connections** — hot reload applies only to new connections
- **Zero buffering** — data is forwarded immediately with no processing, filtering, or modification
- **Robust** — designed to stay up forever, efficient in CPU and memory
- **All user output from Python** — Go engine never prints to console, communicates only via JSON Lines

## Architecture

- **Go engine** (`go/`) — TCP listener, SNI extraction from ClientHello, TLS termination via `crypto/tls`, bidirectional `io.Copy` forwarding. Runs as a persistent subprocess.
- **Python package** (`tls_switch/`) — CLI via argbuilder, config file parsing/validation, cert/key validation, file watching for hot reload, all user-facing output.
- Pre-built Go binaries live in `tls_switch/bin/` (gitignored, included in wheel via hatch artifacts)
- **Platform-specific wheels** — `scripts/build_wheels.py` splits a fat wheel into per-platform wheels

### Go File Structure

- `go/main.go` — JSON Lines protocol loop, command dispatch
- `go/server.go` — TCP listener, connection accept loop
- `go/sni.go` — ClientHello parsing, SNI extraction
- `go/config.go` — config types, atomic swap for hot reload
- `go/proxy.go` — bidirectional copy, passthrough and terminate modes

### Protocol Commands

- `configure` — send validated config (host routes, PEM cert/key data, listen address)
- `start` — begin accepting connections
- `stop` — graceful shutdown (drain existing connections)
- `status` — return listener state, connection count
- `reload` — atomic config swap for new connections

### Python-Go Communication Protocol

JSON Lines over stdin/stdout. Python sends one JSON object per line, Go responds with one JSON object per line.

Request: `{"command": "configure", "args": {...}}`
Response (success): `{"status": "ok", "data": {...}}`
Response (error): `{"status": "error", "error": "message"}`

Go command handlers are registered in the `commands` map. The `Engine` class in Python manages the subprocess lifecycle and provides `engine.send(command, args)` which returns the `data` field or raises `EngineError`.

### Supported Platforms

| Target | Binary | Wheel Tag |
|--------|--------|-----------|
| macOS arm64 | `tls-switch-darwin-arm64` | `macosx_11_0_arm64` |
| macOS amd64 | `tls-switch-darwin-amd64` | `macosx_10_12_x86_64` |
| Linux amd64 | `tls-switch-linux-amd64` | `manylinux_2_17_x86_64` |
| Linux arm64 | `tls-switch-linux-arm64` | `manylinux_2_17_aarch64` |
| Windows amd64 | `tls-switch-windows-amd64.exe` | `win_amd64` |
| Windows arm64 | `tls-switch-windows-arm64.exe` | `win_arm64` |

## Build System

- `make dev` — set up Python dev environment (.venv)
- `make go-build` — cross-compile Go binaries for all 6 platforms (static, CGO_ENABLED=0)
- `make build` — full build (lint, go-build, version, docs, platform wheels)
- `make check` — format check (ruff + gofmt) + pyright + go vet
- `make format` — auto-format Python with ruff, Go with gofmt
- `make clean` — remove build artefacts and compiled binaries

Go binaries are statically linked (`CGO_ENABLED=0 -ldflags='-s -w'`).

## Code Style

- Use ASCII dashes (`-`) for separator lines, not unicode box-drawing characters
- Python separator lines: `# ` followed by 88 dashes (90 chars total)
- Go separator lines: `// ` followed by 87 dashes (90 chars total)
- All source files have a header block with: filename, description, copyright, version history
- Python: ruff formatting + pyright strict type checking
- Go: gofmt formatting + go vet
- Zero runtime dependencies — Python stdlib only

## Key Files

- `Makefile` — build orchestration
- `pyproject.toml` — Python package config (hatch + uv-dynamic-versioning)
- `go/main.go` — Go entry point, protocol loop
- `go/go.mod` — Go module definition
- `tls_switch/cli.py` — Python CLI, argument parsing, logging, file watching, user-facing output
- `tls_switch/config.py` — JSON config parsing, cert/key validation
- `tls_switch/engine.py` — manages Go subprocess, JSON Lines protocol, platform detection
- `tls_switch/argbuilder.py` — argument parsing library (shared across WaterJuice projects)
- `tls_switch/version.py` — version handling (imports from generated `_version.py`)
- `scripts/build_wheels.py` — splits fat wheel into per-platform wheels

## Testing

```bash
make go-build                          # must build Go binaries first
uv run tls-switch --version            # check versions (python + go)
uv run tls-switch --example-config     # print example config
uv run tls-switch -c local/config.json # run with a config file
make check                             # format check + lint
```
