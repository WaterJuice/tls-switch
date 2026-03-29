# ----------------------------------------------------------------------------------------
#   engine.py
#   ---------
#
#   Manages the Go binary subprocess and provides a Python interface for sending
#   commands and receiving responses via JSON Lines over stdin/stdout. A reader
#   thread handles both command responses and unsolicited events.
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
import queue
import subprocess
import threading
from collections.abc import Callable
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
}

_STOP_TIMEOUT = 5

# ----------------------------------------------------------------------------------------
#   Exceptions
# ----------------------------------------------------------------------------------------


class EngineError(Exception):
    """Raised when the Go engine returns an error or is unavailable."""


# ----------------------------------------------------------------------------------------
#   Engine
# ----------------------------------------------------------------------------------------

EventCallback = Callable[[str, Any], None]


class Engine:
    """Manages the Go binary subprocess for JSON Lines communication.

    Events from the Go engine are dispatched to the event_callback if provided.
    Command responses are queued internally for the send() method.
    """

    # ------------------------------------------------------------------------------------
    def __init__(self, event_callback: EventCallback | None = None) -> None:
        self._process: subprocess.Popen[bytes] | None = None
        self._event_callback = event_callback
        self._response_queue: queue.Queue[dict[str, Any]] = queue.Queue()
        self._reader_thread: threading.Thread | None = None

    # ------------------------------------------------------------------------------------
    def start(self) -> None:
        """Start the Go binary subprocess and reader thread."""
        binary = _get_binary_path()

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

        self._reader_thread = threading.Thread(
            target=self._read_loop, daemon=True, name="engine-reader"
        )
        self._reader_thread.start()

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
        """Send a command and return the response data.

        Raises EngineError on error status, broken pipe, or unexpected exit.
        """
        if self._process is None or self._process.stdin is None:
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

        try:
            response = self._response_queue.get(timeout=30)
        except queue.Empty:
            raise EngineError("Engine did not respond in time") from None

        if response.get("status") == "error":
            raise EngineError(response.get("error", "unknown error"))

        return response.get("data")

    # ------------------------------------------------------------------------------------
    def _read_loop(self) -> None:
        """Background thread: read JSON lines, dispatch events, queue responses."""
        assert self._process is not None and self._process.stdout is not None
        for raw_line in self._process.stdout:
            if not raw_line:
                break
            try:
                msg: dict[str, Any] = json.loads(raw_line)
            except json.JSONDecodeError:
                continue

            if "event" in msg:
                if self._event_callback:
                    self._event_callback(msg["event"], msg.get("data"))
            else:
                self._response_queue.put(msg)

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
