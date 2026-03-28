# ----------------------------------------------------------------------------------------
#   cli.py
#   ------
#
#   Minimal launcher that finds and execs the Go binary for the current platform.
#   All logic is in the Go binary; Python is only used for PyPI distribution.
#
#   (c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
#
#   Version History
#   ---------------
#   Mar 2026 - Created
# ----------------------------------------------------------------------------------------

import os
import platform
import subprocess
import sys
from pathlib import Path

_BIN_DIR = Path(__file__).parent / "bin"

_PLATFORM_MAP: dict[tuple[str, str], str] = {
    ("Darwin", "arm64"): "tls-switch-darwin-arm64",
    ("Darwin", "x86_64"): "tls-switch-darwin-amd64",
    ("Linux", "aarch64"): "tls-switch-linux-arm64",
    ("Linux", "x86_64"): "tls-switch-linux-amd64",
    ("Windows", "AMD64"): "tls-switch-windows-amd64.exe",
    ("Windows", "ARM64"): "tls-switch-windows-arm64.exe",
    ("FreeBSD", "amd64"): "tls-switch-freebsd-amd64",
    ("FreeBSD", "arm64"): "tls-switch-freebsd-arm64",
    ("OpenBSD", "amd64"): "tls-switch-openbsd-amd64",
    ("OpenBSD", "arm64"): "tls-switch-openbsd-arm64",
}


def main() -> int:
    key = (platform.system(), platform.machine())
    name = _PLATFORM_MAP.get(key)
    if name is None:
        print(f"Unsupported platform: {key[0]} {key[1]}", file=sys.stderr)
        return 1

    binary = _BIN_DIR / name
    if os.name != "nt" and not os.access(binary, os.X_OK):
        try:
            binary.chmod(binary.stat().st_mode | 0o111)
        except OSError:
            pass

    # On Unix, replace the process entirely
    if os.name != "nt":
        try:
            os.execvp(str(binary), [str(binary)] + sys.argv[1:])
        except FileNotFoundError:
            print(
                f"Binary not found: {binary}\n"
                f"Run 'make go-build' to compile the Go binaries.",
                file=sys.stderr,
            )
            return 1

    # On Windows, use subprocess (no execvp)
    try:
        result = subprocess.run([str(binary)] + sys.argv[1:])
    except FileNotFoundError:
        print(
            f"Binary not found: {binary}\n"
            f"Run 'make go-build' to compile the Go binaries.",
            file=sys.stderr,
        )
        return 1
    return result.returncode
