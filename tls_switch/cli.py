# ----------------------------------------------------------------------------------------
#   cli.py
#   ------
#
#   CLI entry point. Parses arguments, loads config, starts the Go engine,
#   watches for config/cert changes, and handles all user-facing output.
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
import signal
import sys
import threading
import time
from pathlib import Path
from typing import Any
from typing import cast
from .argbuilder import ArgsParser
from .argbuilder import Namespace
from .config import ConfigError
from .config import load_config
from .engine import Engine
from .engine import EngineError
from .version import VERSION_STR

# ----------------------------------------------------------------------------------------
#   Constants
# ----------------------------------------------------------------------------------------

_LICENCE_TEXT = """\
tls-switch — Released under the Unlicense (public domain)

This is free and unencumbered software released into the public domain.

Anyone is free to copy, modify, publish, use, compile, sell, or
distribute this software, either in source code form or as a compiled
binary, for any purpose, commercial or non-commercial, and by any
means.

For more information, please refer to <https://unlicense.org/>
"""

_EXAMPLE_CONFIG = """\
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
"""

_CONFIG_POLL_INTERVAL = 2.0  # seconds between file change checks

# ----------------------------------------------------------------------------------------
#   Argument Parser
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _create_parser() -> ArgsParser:
    """Build the argument parser."""
    parser = ArgsParser(
        prog="tls-switch",
        description="SNI-based TLS reverse proxy.",
        version=f"tls-switch: {VERSION_STR}\npython: {sys.version.split()[0]}",
    )

    parser.add_argument(
        "--license",
        action="store_true",
        dest="license",
        help="Show license information and exit",
    )

    parser.add_argument(
        "--config",
        "-c",
        metavar="FILE",
        help="Path to the JSON config file",
    )

    parser.add_argument(
        "--example-config",
        action="store_true",
        dest="example_config",
        help="Print an example config file and exit",
    )

    return parser


# ----------------------------------------------------------------------------------------
#   Logging
# ----------------------------------------------------------------------------------------

_USE_COLOUR = sys.stderr.isatty()


# ANSI colours chosen to be readable on both light and dark terminal backgrounds
def _dim(s: str) -> str:
    return f"\033[2m{s}\033[0m" if _USE_COLOUR else s


def _cyan(s: str) -> str:
    return f"\033[36m{s}\033[0m" if _USE_COLOUR else s


def _green(s: str) -> str:
    return f"\033[32m{s}\033[0m" if _USE_COLOUR else s


def _yellow(s: str) -> str:
    return f"\033[33m{s}\033[0m" if _USE_COLOUR else s


def _red(s: str) -> str:
    return f"\033[31m{s}\033[0m" if _USE_COLOUR else s


# ----------------------------------------------------------------------------------------
def _log(msg: str) -> None:
    """Print a timestamped log message to stderr."""
    ts = time.strftime("%Y-%m-%d %H:%M:%S %z")
    print(f"{_dim(ts)} {msg}", file=sys.stderr, flush=True)


# ----------------------------------------------------------------------------------------
def _log_hosts(config_payload: dict[str, Any]) -> None:
    """Log the configured hosts."""
    hosts_raw: Any = config_payload.get("hosts", {})
    if not isinstance(hosts_raw, dict):
        return
    hosts = cast("dict[str, Any]", hosts_raw)
    for hostname, host_raw in hosts.items():
        if not isinstance(host_raw, dict):
            continue
        host = cast("dict[str, Any]", host_raw)
        mode: str = str(host.get("mode", "?"))
        backend: str = str(host.get("backend", "?"))
        _log(f"  {_cyan(hostname)} {_dim('(' + mode + ')')} → {_green(backend)}")


# ----------------------------------------------------------------------------------------
def _on_engine_event(event: str, data: Any) -> None:
    """Handle events from the Go engine."""
    if event == "connection" and isinstance(data, dict):
        d = cast("dict[str, str]", data)
        hostname = d.get("hostname", "?")
        source = d.get("source", "?")
        mode = d.get("mode", "")
        backend = d.get("backend", "")
        action = d.get("action", "")
        reason = d.get("reason", "")
        error = d.get("error", "")

        if error:
            _log(f"{_dim(source)} {_red(error)}")
        elif action == "rejected":
            _log(f"{_dim(source)} → {_yellow(hostname)} {_red(reason)}")
        else:
            _log(
                f"{_dim(source)} → {_cyan(hostname)}"
                f" {_dim('(' + mode + ')')}"
                f" → {_green(backend)}"
            )


# ----------------------------------------------------------------------------------------
#   File Watching
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _get_watched_files(config_path: Path) -> dict[Path, float]:
    """Get modification times of config file and all referenced cert/key files."""
    files: dict[Path, float] = {}

    try:
        files[config_path] = config_path.stat().st_mtime
    except OSError:
        return files

    try:
        raw: Any = json.loads(config_path.read_text())
    except (json.JSONDecodeError, OSError):
        return files

    if not isinstance(raw, dict):
        return files

    raw_dict = cast("dict[str, Any]", raw)
    hosts_raw = raw_dict.get("hosts", {})
    if not isinstance(hosts_raw, dict):
        return files

    base_dir = config_path.parent
    hosts = cast("dict[str, Any]", hosts_raw)
    for host_raw in hosts.values():
        if not isinstance(host_raw, dict):
            continue
        host = cast("dict[str, Any]", host_raw)
        for field in ("cert", "key"):
            path_str = host.get(field)
            if not isinstance(path_str, str) or not path_str:
                continue
            p = Path(path_str)
            if not p.is_absolute():
                p = base_dir / p
            try:
                files[p] = p.stat().st_mtime
            except OSError:
                pass

    return files


# ----------------------------------------------------------------------------------------
def _watch_files(
    config_path: Path,
    engine: Engine,
    shutdown: threading.Event,
) -> None:
    """Background thread: poll for config/cert changes and reload."""
    mtimes = _get_watched_files(config_path)

    while not shutdown.is_set():
        shutdown.wait(_CONFIG_POLL_INTERVAL)
        if shutdown.is_set():
            break

        new_mtimes = _get_watched_files(config_path)
        if new_mtimes == mtimes:
            continue

        mtimes = new_mtimes
        _log("Config or certificate change detected, reloading...")

        try:
            config_payload = load_config(config_path)
        except ConfigError as e:
            _log(f"Reload failed (keeping current config): {e}")
            continue

        try:
            result: dict[str, Any] = engine.send("reload", config_payload) or {}  # noqa: F841
            host_count: int = result.get("hosts", 0)
            _log(f"Reloaded {_cyan(str(host_count))} host(s):")
            _log_hosts(config_payload)
        except EngineError as e:
            _log(f"Reload failed: {e}")


# ----------------------------------------------------------------------------------------
#   Main Entry Point
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def main() -> int:
    """Entry point."""
    try:
        return _main_inner()
    except KeyboardInterrupt:
        return 0
    except SystemExit:
        raise
    except BaseException as e:
        import traceback

        t = "-------------------------------------------------------------------\n"
        t += "UNHANDLED EXCEPTION OCCURRED!!\n"
        t += "\n"
        t += traceback.format_exc()
        t += "\n"
        t += f"EXCEPTION: {type(e)} {e}\n"
        t += "-------------------------------------------------------------------\n"
        print(t, file=sys.stderr)
        return 1


# ----------------------------------------------------------------------------------------
def _main_inner() -> int:
    """Inner main function."""
    if "--license" in sys.argv:
        print(_LICENCE_TEXT)
        return 0

    if "--example-config" in sys.argv:
        print(_EXAMPLE_CONFIG, end="")
        return 0

    if "--version" in sys.argv:
        print(f"tls-switch: {VERSION_STR}")
        print(f"python: {sys.version.split()[0]}")
        try:
            with Engine() as engine:
                go_info: dict[str, str] = engine.send("version") or {}
                print(f"go: {go_info.get('go', '?')}")
        except EngineError:
            print("go: (engine unavailable)")
        return 0

    parser = _create_parser()
    args: Namespace = parser.parse()

    if args.example_config:
        print(_EXAMPLE_CONFIG, end="")
        return 0

    config_path_str: str | None = args.config
    if not config_path_str:
        print("Error: --config/-c is required", file=sys.stderr)
        print(
            "Run with --help for usage or --example-config for a template",
            file=sys.stderr,
        )
        return 1

    config_path = Path(config_path_str)

    # Load and validate config
    try:
        config_payload = load_config(config_path)
    except ConfigError as e:
        print(f"Config error: {e}", file=sys.stderr)
        return 1

    try:
        with Engine(event_callback=_on_engine_event) as engine:
            # Get Go version info
            go_info: dict[str, str] = engine.send("version") or {}
            go_ver: str = go_info.get("go", "?")
            _log(
                f"tls-switch {_cyan(VERSION_STR)}"
                f" {_dim('(python ' + sys.version.split()[0] + ', ' + str(go_ver) + ')')}"
            )

            # Send config to engine
            result: dict[str, Any] = engine.send("configure", config_payload) or {}
            host_count: int = result.get("hosts", 0)
            _log(f"Configured {_cyan(str(host_count))} host(s):")
            _log_hosts(config_payload)

            # Start the server
            result = engine.send("start") or {}
            listen_addr: str = result.get("listening", "")
            _log(f"Listening on {_green(listen_addr)}")

            # Start file watcher
            shutdown = threading.Event()
            watcher = threading.Thread(
                target=_watch_files,
                args=(config_path, engine, shutdown),
                daemon=True,
                name="file-watcher",
            )
            watcher.start()

            # Wait for SIGINT or SIGTERM
            def _signal_handler(_signum: int, _frame: object) -> None:
                shutdown.set()

            signal.signal(signal.SIGINT, _signal_handler)
            signal.signal(signal.SIGTERM, _signal_handler)

            _log("Ready (Ctrl+C to stop)")
            shutdown.wait()
            _log("Shutting down (Ctrl+C again to force)...")

            # Second signal kills immediately
            signal.signal(signal.SIGINT, signal.SIG_DFL)
            signal.signal(signal.SIGTERM, signal.SIG_DFL)

            try:
                engine.send("stop")
            except EngineError:
                pass
            engine.stop()
            _log("Stopped")
            return 0

    except EngineError as e:
        print(f"Engine error: {e}", file=sys.stderr)
        return 1
