# Usage

## Running the Server

```bash
tls-switch -c config.json
```

Or with the long form:

```bash
tls-switch --config config.json
```

The server starts listening on the configured address (default `:443`) and routes incoming TLS connections based on the SNI hostname.

The server runs in the foreground and logs connection events to stderr with timestamps and ANSI colours (when connected to a terminal). Press Ctrl+C to stop gracefully (drains active connections), or Ctrl+C twice to force-stop immediately.

## Example Config

To generate an example config file:

```bash
tls-switch --example-config > config.json
```

## Configuration

### Config File Format

The config file is JSON:

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
      "backend": "10.0.0.5:443",
      "proxy_protocol": "v2"
    }
  }
}
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `listen` | Yes | Address to listen on (e.g. `:443`, `0.0.0.0:8443`, `192.168.1.1:443`) |
| `hosts` | Yes | Map of hostname to route configuration |

Each host entry:

| Field | Required | Description |
|-------|----------|-------------|
| `mode` | Yes | `terminate` or `passthrough` |
| `backend` | Yes | Backend address as `host:port` |
| `cert` | terminate only | Path to PEM certificate file (may include full chain) |
| `key` | terminate only | Path to PEM private key file |
| `proxy_protocol` | No | `v1` or `v2` to emit a PROXY protocol header to the backend (see [PROXY Protocol](#proxy-protocol) below). Omit to disable (default). |

Certificate and key paths may be absolute or relative to the config file's directory.

### Routing Modes

#### `terminate` — TLS Termination

tls-switch performs the TLS handshake using the configured certificate and private key. After the handshake completes, it opens a plain TCP connection to the backend and copies decrypted data bidirectionally.

Use this when your backend speaks plain HTTP (or any other plaintext protocol) and you want tls-switch to handle the TLS layer.

```json
{
  "mode": "terminate",
  "cert": "/etc/tls-switch/cert.pem",
  "key": "/etc/tls-switch/key.pem",
  "backend": "127.0.0.1:8080"
}
```

The certificate file may contain the full chain (leaf + intermediates). The key file must contain the private key matching the leaf certificate.

#### `passthrough` — TLS Passthrough

tls-switch reads just enough of the TLS ClientHello to extract the SNI hostname, then forwards the entire TLS stream (including the ClientHello) to the backend. The backend server handles the TLS handshake itself.

Use this when the backend has its own TLS certificate and you just need port 443 routing.

```json
{
  "mode": "passthrough",
  "backend": "10.0.0.5:443"
}
```

No `cert` or `key` fields are needed for passthrough mode.

### PROXY Protocol

Set the optional `proxy_protocol` field on a host to emit a [PROXY protocol](https://www.haproxy.org/download/3.0/doc/proxy-protocol.txt) header to the backend so it sees the original client IP and port instead of the address of tls-switch. The header is written to the backend connection before any other bytes flow, and works the same way in both `terminate` and `passthrough` modes.

```json
{
  "mode": "passthrough",
  "backend": "10.0.0.5:443",
  "proxy_protocol": "v2"
}
```

Two formats are supported:

- **`v1`** — human-readable text format. A single ASCII line like `PROXY TCP4 1.2.3.4 5.6.7.8 12345 443\r\n`. Easy to debug with `tcpdump` or `nc`. Maximum ~100 bytes per connection.
- **`v2`** — compact binary format. A 28-byte (IPv4) or 52-byte (IPv6) header. Faster to parse, supports more transport types, and is the default for most modern proxies.

Use whichever format your backend expects. If you have a choice, prefer `v2`.

#### Backend support

The backend **must** be configured to expect a PROXY protocol header on the listener tls-switch connects to — if it isn't, the bytes will be misinterpreted as the start of a TLS handshake or HTTP request, and the connection will fail. Common configurations:

- **nginx** — `listen 443 ssl proxy_protocol;` plus `set_real_ip_from <tls-switch-IP>;`
- **Apache** — `RemoteIPProxyProtocol On` with `RemoteIPProxyProtocolExceptions` if needed
- **HAProxy** — `accept-proxy` on the bind line
- **Caddy** — the `proxy_protocol` directive on the listener
- **Traefik** — `proxyProtocol.trustedIPs` set to the tls-switch address

#### Security: trust boundary

PROXY protocol headers carry the source IP that tls-switch saw on the incoming connection. **The backend must trust PROXY headers _only_ from the tls-switch listener address.** If the backend accepts PROXY headers from arbitrary sources, any client can spoof their source IP simply by prepending their own header to the connection. Most backends provide a "trusted proxies" or "set real IP from" setting for exactly this reason — make sure it points only to the tls-switch host.

If the field is omitted (or set to anything other than `v1` / `v2`, which is rejected at config load), no header is sent and the backend sees only the tls-switch source address.

### Unknown Hostnames

If a client requests a hostname not in the configuration, tls-switch completes a TLS handshake using any available configured certificate and returns an HTTP 421 Misdirected Request error page. This gives browsers a clear error message rather than a cryptic "can't connect".

If no terminate-mode hosts are configured (all hosts are passthrough), a TLS `unrecognized_name` alert is sent instead.

## Hot Reload

tls-switch watches the config file and all referenced certificate and key files for changes (polling every 2 seconds). When a change is detected:

1. The new config is parsed and validated
2. Certificate and key files are checked (correct format, cert matches key, etc.)
3. If valid, the new config is applied — **new connections** use the updated config
4. **Existing connections** continue with their original config until they close naturally
5. If the new config is invalid, the change is rejected and a warning is logged — the server continues with the previous valid config

This means you can:

- Update certificates (e.g. after a Let's Encrypt renewal) without any downtime
- Add or remove host routes while the server is running
- Change backend addresses for individual hosts

## Connection Logging

tls-switch logs each connection to stderr with timestamps, source IP, hostname, routing mode, and backend address. When connected to a terminal, output is coloured for readability. Colours are automatically disabled when stderr is piped to a file.

Example output:

```
2026-03-28 16:05:52 +1100 tls-switch 1.0.0b1 (go 1.26.0)
2026-03-28 16:05:52 +1100 Configured 2 host(s):
2026-03-28 16:05:52 +1100   app.example.com (terminate) → 127.0.0.1:8080
2026-03-28 16:05:52 +1100   legacy.example.com (passthrough) → 10.0.0.5:443
2026-03-28 16:05:52 +1100 Listening on :443
2026-03-28 16:05:52 +1100 Ready (Ctrl+C to stop)
2026-03-28 16:05:54 +1100 10.0.0.1:54321 → app.example.com (terminate) → 127.0.0.1:8080
2026-03-28 16:05:55 +1100 10.0.0.2:54322 → unknown.example.com — unknown hostname
```

## Common Options

- `--config FILE`, `-c FILE` — path to the JSON config file (required)
- `--example-config` — print an example config file and exit
- `--version` — show the tls-switch version
- `--license` — show license information and exit
- `--help` — show help
