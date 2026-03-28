# tls-switch

A TLS reverse proxy that routes incoming TLS connections to backend servers based on the requested hostname using SNI (Server Name Indication). Supports both TLS termination and TLS passthrough on a per-host basis.

tls-switch sits in front of your services on port 443 and inspects the TLS ClientHello to determine which hostname the client is requesting. SNI is a TLS extension that allows the client to indicate which hostname it is trying to connect to before the TLS handshake completes — this is how tls-switch knows where to route the connection without needing a separate IP address per service. It then routes the connection to the appropriate backend, either terminating TLS and forwarding plaintext, or passing the encrypted stream through unmodified.

## Features

- **SNI-based routing** — route TLS connections to different backends based on hostname
- **TLS termination** — terminate TLS with your certificates and forward plaintext to backends
- **TLS passthrough** — forward the raw TLS stream to a backend that handles its own TLS
- **Hot reload** — config and certificate changes take effect on new connections without interrupting existing ones
- **Zero buffering** — data is forwarded immediately with no processing, filtering, or modification
- **Efficient** — Go networking engine with zero-copy forwarding, Python CLI for configuration and management
- **Zero runtime dependencies** — Python 3.12+ stdlib only, Go binary is statically linked

## Use Cases

- Run multiple HTTPS services on a single IP address, each with its own certificate
- Put a TLS-terminating proxy in front of plain HTTP services
- Route some domains through to their own TLS servers while terminating others locally
- Consolidate port 443 across multiple services without a full reverse proxy

## Requirements

- Python 3.12+
- Root/administrator privileges (if binding to port 443)

## Installation

```bash
pip install tls-switch
```

Or run directly with uv:

```bash
uvx tls-switch
```

## Quick Start

Create a config file (`config.json`):

```json
{
  "listen": ":443",
  "hosts": {
    "app.example.com": {
      "mode": "terminate",
      "cert": "/etc/tls-switch/app.example.com.crt",
      "key": "/etc/tls-switch/app.example.com.key",
      "backend": "127.0.0.1:8080"
    },
    "legacy.example.com": {
      "mode": "passthrough",
      "backend": "10.0.0.5:443"
    }
  }
}
```

Run the server:

```bash
tls-switch -c config.json
```

In this example:
- Connections to `app.example.com` have TLS terminated by tls-switch, and plaintext HTTP is forwarded to `127.0.0.1:8080`
- Connections to `legacy.example.com` are forwarded as raw TLS to `10.0.0.5:443`, which handles its own certificates

## How It Works

1. A client connects to port 443 and begins a TLS handshake
2. tls-switch reads the TLS ClientHello message and extracts the SNI (Server Name Indication) hostname
3. The hostname is looked up in the configuration
4. Depending on the mode:
   - **terminate**: tls-switch completes the TLS handshake using the configured certificate and key, then opens a plaintext TCP connection to the backend and copies data bidirectionally with no buffering or processing
   - **passthrough**: tls-switch opens a TCP connection to the backend, replays the original ClientHello, and then copies data bidirectionally — the backend server handles the TLS handshake itself
5. If the hostname is not found in the configuration, tls-switch completes a TLS handshake using any available configured certificate and returns an HTTP 421 Misdirected Request error page — browsers display a clear error rather than a cryptic "can't connect" message

## Configuration

### Config File

The config file is JSON with the following structure:

```json
{
  "listen": ":443",
  "hosts": {
    "hostname": {
      "mode": "terminate|passthrough",
      "cert": "/path/to/cert.pem",
      "key": "/path/to/key.pem",
      "backend": "host:port"
    }
  }
}
```

| Field | Description |
|-------|-------------|
| `listen` | Address to listen on (e.g. `:443`, `0.0.0.0:8443`) |
| `hosts` | Map of hostname to route configuration |
| `mode` | `terminate` (TLS termination) or `passthrough` (forward raw TLS) |
| `cert` | Path to PEM certificate file (terminate mode only) |
| `key` | Path to PEM private key file (terminate mode only) |
| `backend` | Backend address as `host:port` |

### Hot Reload

tls-switch watches the config file and certificate files for changes. When a change is detected:

- The new config is validated
- If valid, new connections use the updated config
- Existing connections continue with their original config until they close naturally
- If invalid, the change is rejected and the current config remains active

## Development

```bash
# Set up development environment
make dev

# Cross-compile Go binaries
make go-build

# Run linting and type checking
make check

# Auto-format code
make format

# Build wheel and docs
make build
```

## Architecture

tls-switch is a Python+Go hybrid:

- **Go** handles all networking — TCP listener, TLS handshakes, SNI extraction, and bidirectional data forwarding. It runs as a persistent subprocess communicating with Python via JSON Lines over stdin/stdout.
- **Python** handles everything user-facing — CLI, config file parsing and validation, certificate validation, file watching, and error reporting.

## Licence

Released under the [Unlicense](https://unlicense.org/) — public domain.
