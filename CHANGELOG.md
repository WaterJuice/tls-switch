# tls-switch 1.0.0 Beta 4 — 29 Mar 2026

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
