# tls-switch

A TLS reverse proxy that routes incoming TLS connections to backend servers based on the requested hostname using SNI (Server Name Indication).

tls-switch sits in front of your services on port 443 and inspects the TLS ClientHello to determine which hostname the client is requesting. SNI is a TLS extension that allows the client to indicate which hostname it is trying to connect to before the TLS handshake completes — this is how tls-switch knows where to route the connection without needing a separate IP address per service. It then routes the connection to the appropriate backend, either terminating TLS and forwarding plaintext, or passing the encrypted stream through unmodified.

## Features

- **SNI-based routing** — route TLS connections to different backends based on hostname
- **TLS termination** — terminate TLS with your certificates and forward plaintext to backends
- **TLS passthrough** — forward the raw TLS stream to a backend that handles its own TLS
- **Hot reload** — config and certificate changes take effect without interrupting existing connections
- **Zero buffering** — data is forwarded immediately with no processing or modification
- **PROXY protocol** — optional v1 or v2 header emission per host so backends see the original client IP
- **Efficient** — zero-copy forwarding via `io.Copy`, no goroutine bloat, no allocations on the hot path

## Requirements

- Python 3.12+ (only required to install from PyPI; the binary itself has no runtime dependencies)
- Root/administrator privileges (if binding to port 443)

## Quick Start

### Install

```bash
pip install tls-switch
```

Or run directly with uv:

```bash
uvx tls-switch
```

### Configure

Create a config file (`config.json`):

```json
{
  "listen": ":443",
  "hosts": {
    "app.example.com": {
      "mode": "terminate",
      "cert": "/etc/tls-switch/app.crt",
      "key": "/etc/tls-switch/app.key",
      "backend": "127.0.0.1:8080"
    },
    "legacy.example.com": {
      "mode": "passthrough",
      "backend": "10.0.0.5:443",
      "proxy_protocol": "v2"
    }
  }
}
```

### Run

```bash
tls-switch -c config.json
```

See the [Usage](usage.md) page for full details on configuration and operation.

## How It Works

1. A client connects to port 443 and begins a TLS handshake
2. tls-switch reads the ClientHello and extracts the SNI hostname
3. The hostname is looked up in the configuration
4. Depending on the mode:
   - **terminate**: tls-switch completes the TLS handshake, then forwards plaintext to the backend
   - **passthrough**: tls-switch forwards the raw TLS stream to the backend, which handles TLS itself
5. Data is copied bidirectionally with no buffering, filtering, or modification

## Architecture

tls-switch is a single statically-linked Go binary with no runtime dependencies. It includes the TCP listener, TLS handshake, SNI extraction, bidirectional forwarding, optional PROXY protocol header emission, config and certificate file watching, and the CLI — all in one process. It is distributed as platform-specific Python wheels for ease of installation via `pip` or `uvx`, but no Python is required at runtime.
