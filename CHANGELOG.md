# tls-switch 1.0.0 Beta 1 — Mar 2026

Initial beta release.

## Features

- **SNI-based TLS routing** — route incoming TLS connections to backends based on hostname
- **TLS termination** — terminate TLS with configured certificates and forward plaintext to backends
- **TLS passthrough** — forward raw TLS streams to backends that handle their own TLS
- **Hot reload** — config and certificate changes apply to new connections without interrupting existing ones
- **Zero buffering** — data forwarded immediately with no processing or modification
- **JSON config** — simple JSON configuration file for host routing
- **10 platform builds** — macOS, Linux, Windows, FreeBSD, OpenBSD (amd64 + arm64)
- **Zero runtime dependencies** — Python 3.12+ stdlib only, statically linked Go binary
