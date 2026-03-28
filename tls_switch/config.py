# ----------------------------------------------------------------------------------------
#   config.py
#   ---------
#
#   Parses and validates the JSON config file. Reads and validates TLS certificate
#   and key files. Produces the payload to send to the Go engine via the configure
#   command.
#
#   (c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
#
#   Version History
#   ---------------
#   Mar 2026 - Created
# ----------------------------------------------------------------------------------------

# ----------------------------------------------------------------------------------------
#   Imports
# ----------------------------------------------------------------------------------------

import json
import ssl
import tempfile
from pathlib import Path
from typing import Any
from typing import cast

# ----------------------------------------------------------------------------------------
#   Exceptions
# ----------------------------------------------------------------------------------------


class ConfigError(Exception):
    """Raised when the config file is invalid."""


MODE_TERMINATE = "terminate"
MODE_PASSTHROUGH = "passthrough"
_VALID_MODES = (MODE_TERMINATE, MODE_PASSTHROUGH)


# ----------------------------------------------------------------------------------------
#   Config Loading
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def load_config(config_path: Path) -> dict[str, Any]:
    """Load and validate a tls-switch config file.

    Returns the validated config payload ready to send to the Go engine,
    with cert/key PEM data inlined.

    Raises ConfigError if the config is invalid.
    """
    try:
        text = config_path.read_text()
    except FileNotFoundError:
        raise ConfigError(f"Config file not found: {config_path}") from None
    except OSError as e:
        raise ConfigError(f"Failed to read config file: {e}") from None

    try:
        raw: Any = json.loads(text)
    except json.JSONDecodeError as e:
        raise ConfigError(f"Invalid JSON in config file: {e}") from None

    if not isinstance(raw, dict):
        raise ConfigError("Config must be a JSON object")

    return _validate_config(cast("dict[str, Any]", raw), config_path.parent)


# ----------------------------------------------------------------------------------------
def _validate_config(raw: dict[str, Any], base_dir: Path) -> dict[str, Any]:
    """Validate the config structure and load cert/key files."""
    # Listen address
    listen = raw.get("listen")
    if not isinstance(listen, str) or not listen:
        raise ConfigError("'listen' is required and must be a non-empty string")

    # Hosts
    hosts_raw: Any = raw.get("hosts")
    if not isinstance(hosts_raw, dict) or not hosts_raw:
        raise ConfigError("'hosts' is required and must be a non-empty object")

    hosts: dict[str, Any] = {}
    for hostname, host_raw in cast("dict[str, Any]", hosts_raw).items():
        if not isinstance(host_raw, dict):
            raise ConfigError(f"Host '{hostname}': must be a JSON object")
        hosts[hostname] = _validate_host(
            hostname, cast("dict[str, Any]", host_raw), base_dir
        )

    return {"listen": listen, "hosts": hosts}


# ----------------------------------------------------------------------------------------
def _validate_host(
    hostname: str, raw: dict[str, Any], base_dir: Path
) -> dict[str, Any]:
    """Validate a single host entry and load cert/key if needed."""
    mode = raw.get("mode")
    if mode not in _VALID_MODES:
        raise ConfigError(
            f"Host '{hostname}': 'mode' must be 'terminate' or 'passthrough'"
        )

    backend = raw.get("backend")
    if not isinstance(backend, str) or not backend:
        raise ConfigError(
            f"Host '{hostname}': 'backend' is required and must be a non-empty string"
        )

    # Validate backend has host:port format with a valid port
    parts = backend.rsplit(":", 1)
    if len(parts) != 2 or not parts[0] or not parts[1].isdigit():
        raise ConfigError(f"Host '{hostname}': 'backend' must be in host:port format")

    result: dict[str, Any] = {"mode": mode, "backend": backend}

    if mode == MODE_TERMINATE:
        result.update(_load_cert_key(hostname, raw, base_dir))

    return result


# ----------------------------------------------------------------------------------------
def _read_pem_file(hostname: str, path: Path, label: str) -> str:
    """Read a PEM file, raising ConfigError with a clear message on failure."""
    try:
        return path.read_text()
    except FileNotFoundError:
        raise ConfigError(
            f"Host '{hostname}': {label} file not found: {path}"
        ) from None
    except OSError as e:
        raise ConfigError(
            f"Host '{hostname}': failed to read {label} file: {e}"
        ) from None


# ----------------------------------------------------------------------------------------
def _load_cert_key(
    hostname: str, raw: dict[str, Any], base_dir: Path
) -> dict[str, str]:
    """Load and validate cert/key PEM files for a terminate-mode host."""
    cert_path_str = raw.get("cert")
    key_path_str = raw.get("key")

    if not isinstance(cert_path_str, str) or not cert_path_str:
        raise ConfigError(f"Host '{hostname}': 'cert' is required for terminate mode")
    if not isinstance(key_path_str, str) or not key_path_str:
        raise ConfigError(f"Host '{hostname}': 'key' is required for terminate mode")

    cert_path = Path(cert_path_str)
    key_path = Path(key_path_str)

    # Resolve relative paths against config file directory
    if not cert_path.is_absolute():
        cert_path = base_dir / cert_path
    if not key_path.is_absolute():
        key_path = base_dir / key_path

    cert_pem = _read_pem_file(hostname, cert_path, "cert")
    key_pem = _read_pem_file(hostname, key_path, "key")

    # Validate that cert and key are valid PEM and match each other
    _validate_cert_key_pair(hostname, cert_pem, key_pem)

    return {"cert_pem": cert_pem, "key_pem": key_pem}


# ----------------------------------------------------------------------------------------
def _validate_cert_key_pair(hostname: str, cert_pem: str, key_pem: str) -> None:
    """Validate that a cert and key are valid PEM and the key matches the cert."""
    cert_tmp: str | None = None
    key_tmp: str | None = None
    try:
        # Write to temp files for ssl module validation
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".pem", delete=False
        ) as cert_f:
            cert_f.write(cert_pem)
            cert_tmp = cert_f.name

        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".pem", delete=False
        ) as key_f:
            key_f.write(key_pem)
            key_tmp = key_f.name

        ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        ctx.load_cert_chain(cert_tmp, key_tmp)
    except (ssl.SSLError, ValueError, OSError) as e:
        raise ConfigError(
            f"Host '{hostname}': cert/key validation failed: {e}"
        ) from None
    finally:
        if cert_tmp is not None:
            Path(cert_tmp).unlink(missing_ok=True)
        if key_tmp is not None:
            Path(key_tmp).unlink(missing_ok=True)
