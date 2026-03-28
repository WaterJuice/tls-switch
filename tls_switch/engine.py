# ----------------------------------------------------------------------------------------
#   engine.py
#   ---------
#
#   Manages the Go binary subprocess and provides a Python interface for sending
#   commands and receiving responses via JSON Lines over stdin/stdout.
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
import os
import platform
import subprocess
from pathlib import Path
from typing import Any

# ----------------------------------------------------------------------------------------
#   Constants
# ----------------------------------------------------------------------------------------

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

_STOP_TIMEOUT = 5  # seconds to wait for Go process to exit before killing

# ----------------------------------------------------------------------------------------
#   Exceptions
# ----------------------------------------------------------------------------------------


class EngineError(Exception):
    """Raised when the Go engine returns an error or is unavailable."""


# ----------------------------------------------------------------------------------------
#   Engine
# ----------------------------------------------------------------------------------------


class Engine:
    """Manages the Go binary subprocess for JSON Lines communication.

    Usage:
        with Engine() as engine:
            result = engine.send("hello")
            print(result["message"])
    """

    # ------------------------------------------------------------------------------------
    def __init__(self) -> None:
        self._process: subprocess.Popen[bytes] | None = None

    # ------------------------------------------------------------------------------------
    def start(self) -> None:
        """Start the Go binary subprocess."""
        binary = _get_binary_path()

        # Ensure the binary is executable (may be needed after wheel install on Unix)
        if os.name != "nt" and not os.access(binary, os.X_OK):
            binary.chmod(binary.stat().st_mode | 0o111)

        try:
            self._process = subprocess.Popen(
                [str(binary)],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=None,
            )
        except FileNotFoundError:
            raise EngineError(
                f"Binary not found: {binary}\n"
                f"Run 'make go-build' to compile the Go binaries."
            ) from None

    # ------------------------------------------------------------------------------------
    def stop(self) -> None:
        """Stop the Go binary subprocess gracefully, with kill fallback."""
        if self._process is None:
            return
        if self._process.stdin:
            self._process.stdin.close()
        try:
            self._process.wait(timeout=_STOP_TIMEOUT)
        except subprocess.TimeoutExpired:
            self._process.kill()
            self._process.wait()
        self._process = None

    # ------------------------------------------------------------------------------------
    def send(self, command: str, args: dict[str, Any] | None = None) -> Any:
        """Send a command to the Go binary and return the response data.

        Raises EngineError if the engine returns an error status or is not running.
        """
        if (
            self._process is None
            or self._process.stdin is None
            or self._process.stdout is None
        ):
            raise EngineError("Engine is not running")

        request: dict[str, Any] = {"command": command}
        if args is not None:
            request["args"] = args

        line = json.dumps(request) + "\n"
        try:
            self._process.stdin.write(line.encode())
            self._process.stdin.flush()
        except (BrokenPipeError, OSError) as e:
            raise EngineError(f"Engine process died: {e}") from None

        response_line = self._process.stdout.readline()
        if not response_line:
            raise EngineError("Engine process exited unexpectedly")

        try:
            response: dict[str, Any] = json.loads(response_line)
        except json.JSONDecodeError as e:
            raise EngineError(f"Invalid response from engine: {e}") from None

        if response.get("status") == "error":
            raise EngineError(response.get("error", "unknown error"))

        return response.get("data")

    # ------------------------------------------------------------------------------------
    def __enter__(self) -> "Engine":
        self.start()
        return self

    # ------------------------------------------------------------------------------------
    def __exit__(self, *_: object) -> None:
        self.stop()


# ----------------------------------------------------------------------------------------
#   Binary Resolution
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _get_binary_path() -> Path:
    """Return the path to the Go binary for the current platform."""
    key = (platform.system(), platform.machine())
    binary_name = _PLATFORM_MAP.get(key)
    if binary_name is None:
        raise EngineError(f"Unsupported platform: {key[0]} {key[1]}")

    return _BIN_DIR / binary_name
