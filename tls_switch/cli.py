# ----------------------------------------------------------------------------------------
#   cli.py
#   ------
#
#   CLI argument parsing and command handlers. Detects the current platform
#   and executes the appropriate pre-built Go binary.
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

import os
import platform
import subprocess
import sys
import traceback
from pathlib import Path
from .argbuilder import ArgsParser
from .argbuilder import Namespace
from .version import VERSION_STR

# ----------------------------------------------------------------------------------------
#   Constants
# ----------------------------------------------------------------------------------------

_BIN_DIR = Path(__file__).parent / "bin"

# Map (system, machine) to binary name
_PLATFORM_MAP: dict[tuple[str, str], str] = {
    ("Darwin", "arm64"): "tls-switch-darwin-arm64",
    ("Darwin", "x86_64"): "tls-switch-darwin-amd64",
    ("Linux", "aarch64"): "tls-switch-linux-arm64",
    ("Linux", "x86_64"): "tls-switch-linux-amd64",
    ("Windows", "AMD64"): "tls-switch-windows-amd64.exe",
    ("Windows", "ARM64"): "tls-switch-windows-arm64.exe",
}

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
#   Binary Resolution
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _get_binary_path() -> Path:
    """Return the path to the Go binary for the current platform."""
    key = (platform.system(), platform.machine())
    binary_name = _PLATFORM_MAP.get(key)
    if binary_name is None:
        print(
            f"Unsupported platform: {key[0]} {key[1]}",
            file=sys.stderr,
        )
        sys.exit(1)

    binary = _BIN_DIR / binary_name
    if not binary.exists():
        print(
            f"Binary not found: {binary}\n"
            f"Run 'make go-build' to compile the Go binaries.",
            file=sys.stderr,
        )
        sys.exit(1)

    return binary


# ----------------------------------------------------------------------------------------
def _run_binary(args: list[str]) -> int:
    """Execute the Go binary with the given arguments."""
    binary = _get_binary_path()

    # Ensure the binary is executable (may be needed after wheel install on Unix)
    if os.name != "nt" and not os.access(binary, os.X_OK):
        binary.chmod(binary.stat().st_mode | 0o755)

    result = subprocess.run(
        [str(binary)] + args,
        stdin=sys.stdin,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )
    return result.returncode


# ----------------------------------------------------------------------------------------
#   Argument Parser
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _create_parser() -> ArgsParser:
    """Build the argument parser with subcommands."""
    parser = ArgsParser(
        prog="tls-switch",
        description="TLS switch utility.",
        version=f"tls-switch {VERSION_STR}",
    )

    # Top-level options -------------------------------------------------------
    parser.add_argument(
        "--license",
        action="store_true",
        dest="license",
        help="Show license information and exit",
    )

    # hello --------------------------------------------------------------------
    parser.add_command(
        "hello",
        help="Print a hello world message (via Go binary)",
    )

    return parser


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

    if command == "hello":
        return _run_binary([])

    # Default: show help
    parser.parse(["--help"])
    return 0
