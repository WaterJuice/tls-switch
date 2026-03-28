# Usage

## Running the Server

```bash
tls-switch run config.json
```

The server starts listening on the configured address (default `:443`) and routes incoming TLS connections based on the SNI hostname.

The server runs in the foreground and logs connection events to stderr. Press Ctrl+C to stop.

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
      "backend": "10.0.0.5:443"
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

### Unknown Hostnames

If a client requests a hostname not in the configuration, tls-switch sends a TLS `unrecognized_name` alert and closes the connection. No certificate is presented for unknown hostnames — this avoids accidentally signing responses with the wrong hostname's certificate (a common problem with servers like nginx that fall back to the first configured certificate).

## Hot Reload

tls-switch watches the config file and all referenced certificate and key files for changes. When a change is detected:

1. The new config is parsed and validated
2. Certificate and key files are checked (correct format, cert matches key, etc.)
3. If valid, the new config is applied — **new connections** use the updated config
4. **Existing connections** continue with their original config until they close naturally
5. If the new config is invalid, the change is rejected and a warning is printed — the server continues with the previous valid config

This means you can:

- Update certificates (e.g. after a Let's Encrypt renewal) without any downtime
- Add or remove host routes while the server is running
- Change backend addresses for individual hosts

## Common Options

- `--version` — show version and exit
- `--license` — show license information and exit
- `--help` — show help
