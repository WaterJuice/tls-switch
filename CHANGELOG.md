# tls-switch 1.1.1 - 30 May 2026

Moved to new GitHub location: https://github.com/WaterJuice/tls-switch

# tls-switch 1.1.0 - 27 Apr 2026

## Features

- **PROXY protocol** — new optional per-host `proxy_protocol` field emits a [PROXY protocol](https://www.haproxy.org/download/3.0/doc/proxy-protocol.txt) header to the backend so it sees the original client IP. Set to `"v1"` for the human-readable text format or `"v2"` for the compact binary format. Works in both terminate and passthrough modes; default off. The backend must be configured to expect it.

# tls-switch 1.0.0 - 9 Apr 2026

Initial release.

## Features

- **SNI-based TLS routing** — route incoming TLS connections to backends based on hostname
- **TLS termination** — terminate TLS with configured certificates and forward plaintext to backends
- **TLS passthrough** — forward raw TLS streams to backends that handle their own TLS
- **Hot reload** — config and certificate changes apply to new connections without interrupting existing ones
- **Unknown host handling** — returns HTTP 421 Misdirected Request with a clear error page for unconfigured hostnames
- **Connection logging** — timestamped log output with source IP, hostname, mode, and backend, with ANSI colours in terminals
- **Config file watching** — automatically detects and reloads config and certificate changes
- **Zero buffering** — data forwarded immediately with no processing or modification
- **JSON config** — simple JSON configuration file for host routing
- **`--example-config`** — generate a template config file
- **Graceful shutdown** — first Ctrl+C drains active connections, second force-stops
- **6 platform builds** — macOS, Linux, Windows (amd64 + arm64)
- **Zero runtime dependencies** — Python 3.12+ stdlib only, statically linked Go binary
