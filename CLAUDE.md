# CLAUDE.md

## Project Overview

tls-switch is a WaterJuice project. It is an SNI-based TLS reverse proxy that routes incoming TLS connections to backend servers based on hostname. It supports two modes per host:

- **terminate** — complete the TLS handshake with configured cert/key, forward plaintext to backend
- **passthrough** — forward the raw TLS stream (including ClientHello) to the backend, which handles TLS itself

Optionally, per host, a [PROXY protocol](https://www.haproxy.org/download/3.0/doc/proxy-protocol.txt) v1 or v2 header can be emitted to the backend so it sees the original client IP.

It is a single statically-linked Go binary; there is no Python at runtime. Python is only used as a packaging mechanism (the binary is distributed as platform-specific wheels via PyPI).

### Design Principles

- **Never interrupt existing connections** — hot reload applies only to new connections
- **Zero buffering** — data is forwarded immediately with no processing, filtering, or modification
- **Robust** — designed to stay up forever, efficient in CPU and memory
- **Zero runtime dependencies** — single Go binary, no shared libraries (CGO disabled)

## Architecture

Everything lives in one Go module. The `main` package at the repo root is a thin entry point; all logic is in `internal/`.

### File Structure

- `main.go` — entry point, calls `internal.Run(Version)`
- `internal/cli.go` — CLI argument parsing, signal handling, logging, config file watching, user-facing output
- `internal/config.go` — JSON config parsing, cert/key validation, atomic config swap for hot reload
- `internal/server.go` — TCP listener, accept loop, connection dispatch, unknown-host rejection
- `internal/sni.go` — TLS ClientHello parsing, SNI extraction, peeked-bytes connection wrapper
- `internal/proxy.go` — bidirectional `io.Copy` forwarding for both passthrough and terminate modes
- `internal/proxyproto.go` — PROXY protocol v1 (text) and v2 (binary) header emission

### Hot Reload

`internal/cli.go` polls the config file (and any referenced cert/key files) for mtime changes every 2 seconds. On change, `LoadConfig` is called; on success, the new `*Config` is swapped atomically into `ConfigStore` (an `atomic.Pointer`). Existing connections continue to use the old route they captured at accept time; new connections see the new config.

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

- `make dev` — build the binary for the current platform and symlink it into `.venv/bin/tls-switch`
- `make go-build` — cross-compile binaries for all 6 platforms in parallel (static, `CGO_ENABLED=0 -ldflags='-s -w'`)
- `make build` — full build: `check`, `go-build`, `docs`, then `bin2whl` to produce platform wheels in `output/`
- `make check` — `gofmt -l` + `go vet ./...`
- `make format` — `gofmt -w`
- `make clean` — remove `html/`, `output/`, `dist/`, `.venv/`
- `make run ARGS="-c local/config.json"` — build for the current platform and run

The wheel pipeline is driven by `wheel.json` (consumed by `bin2whl`) — that file holds the published metadata (description, homepage, classifiers, readme, binary mappings). `pyproject.toml` is for dev tooling only.

## Code Style

- Use ASCII dashes (`-`) for separator lines, not Unicode box-drawing characters
- Go separator lines: `// ` followed by 87 dashes (90 chars total) between sections and before each function
- All Go source files start with a header block: filename, description, copyright, version history
- gofmt formatting + `go vet` must pass
- Don't add error handling for impossible scenarios — trust standard library guarantees
- Default to no comments; only add a comment when the *why* is non-obvious

## Key Files

- `Makefile` — build orchestration
- `wheel.json` — published wheel metadata (consumed by `bin2whl`)
- `pyproject.toml` — dev tooling config (uv, dependency groups)
- `main.go` — entry point
- `go.mod` — Go module definition
- `internal/cli.go`, `internal/config.go`, `internal/server.go`, `internal/sni.go`, `internal/proxy.go`, `internal/proxyproto.go` — implementation
- `internal/*_test.go` — unit tests

## Testing

```bash
go test ./...                 # run unit tests
make check                    # gofmt + go vet
make go-build                 # cross-compile all platforms
make run ARGS="--version"     # build + run for current platform
make run ARGS="--example-config"
make run ARGS="-c local/config.json"
```

For PROXY protocol manual end-to-end checks: point a host's backend at `nc -l 9999 | xxd` (or `socat -x TCP-LISTEN:9999,fork -` for passthrough) and inspect the bytes — v1 starts with ASCII `PROXY `, v2 starts with the 12-byte signature `0d0a 0d0a 000d 0a51 5549 540a`.
