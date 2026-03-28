# ----------------------------------------------------------------------------------------
#   cli.py
#   ------
#
#   CLI argument parsing and command handlers. Uses the Engine class to
#   communicate with the Go binary via JSON Lines over stdin/stdout.
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

import signal
import sys
import threading
from pathlib import Path
from typing import Any
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

# ----------------------------------------------------------------------------------------
#   Argument Parser
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _create_parser() -> ArgsParser:
    """Build the argument parser with subcommands."""
    parser = ArgsParser(
        prog="tls-switch",
        description="SNI-based TLS reverse proxy.",
        version=f"tls-switch {VERSION_STR}",
    )

    # Top-level options -------------------------------------------------------
    parser.add_argument(
        "--license",
        action="store_true",
        dest="license",
        help="Show license information and exit",
    )

    # run ----------------------------------------------------------------------
    run_cmd = parser.add_command(
        "run",
        help="Start the TLS switch server",
    )
    run_cmd.add_argument(
        "config",
        metavar="CONFIG",
        help="Path to the JSON config file",
    )

    # hello --------------------------------------------------------------------
    parser.add_command(
        "hello",
        help="Print a hello world message (via Go engine)",
    )

    return parser


# ----------------------------------------------------------------------------------------
#   Subcommand Handlers
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _cmd_run(engine: Engine, args: Namespace) -> int:
    """Load config and start the TLS switch server."""
    config_path = Path(args.config)

    # Load and validate config (Python side)
    try:
        config_payload = load_config(config_path)
    except ConfigError as e:
        print(f"Config error: {e}", file=sys.stderr)
        return 1

    # Send config to Go engine
    result: dict[str, Any] = engine.send("configure", config_payload) or {}
    host_count: int = result.get("hosts", 0)
    print(f"Configured {host_count} host(s)")

    # Start the server
    result = engine.send("start") or {}
    listen_addr: str = result.get("listening", "")
    print(f"Listening on {listen_addr}")

    # Wait until SIGINT or SIGTERM
    shutdown = threading.Event()

    def _signal_handler(_signum: int, _frame: object) -> None:
        shutdown.set()

    signal.signal(signal.SIGINT, _signal_handler)
    signal.signal(signal.SIGTERM, _signal_handler)

    print("Press Ctrl+C to stop")
    shutdown.wait()
    print("\nShutting down...")

    try:
        engine.send("stop")
    except EngineError:
        pass  # engine may already be dead

    print("Stopped")
    return 0


# ----------------------------------------------------------------------------------------
def _cmd_hello(engine: Engine) -> int:
    """Send hello command to the Go engine and print the result."""
    result: dict[str, Any] = engine.send("hello") or {}
    print(result.get("message", ""))
    return 0


# ----------------------------------------------------------------------------------------
#   Main Entry Point
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def main() -> int:
    """Entry point: parse arguments and dispatch to subcommand."""
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
    """Inner main function that does the actual work."""
    # Handle --licence/--license before parsing (no command needed).
    if "--license" in sys.argv:
        print(_LICENCE_TEXT)
        return 0

    parser = _create_parser()
    args: Namespace = parser.parse()

    command = args.command if hasattr(args, "command") else None

    if command is None:
        parser.parse(["--help"])
        return 0

    try:
        with Engine() as engine:
            if command == "run":
                return _cmd_run(engine, args)
            if command == "hello":
                return _cmd_hello(engine)
    except EngineError as e:
        print(f"Engine error: {e}", file=sys.stderr)
        return 1

    return 0
