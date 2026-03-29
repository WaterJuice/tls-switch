"""Thin launcher for the tls-switch Go binary. Finds the correct platform
binary and execs it, passing through all arguments."""

import os
import platform
import subprocess
import sys
from pathlib import Path

_BIN = Path(__file__).parent / "bin"
_PLATFORMS = {
    ("Darwin", "arm64"): "tls-switch-darwin-arm64",
    ("Darwin", "x86_64"): "tls-switch-darwin-amd64",
    ("Linux", "aarch64"): "tls-switch-linux-arm64",
    ("Linux", "x86_64"): "tls-switch-linux-amd64",
    ("Windows", "AMD64"): "tls-switch-windows-amd64.exe",
    ("Windows", "ARM64"): "tls-switch-windows-arm64.exe",
}

def main():
    name = _PLATFORMS.get((platform.system(), platform.machine()))
    if not name:
        sys.exit(f"Unsupported platform: {platform.system()} {platform.machine()}")

    binary = str(_BIN / name)

    if os.name != "nt":
        if not os.access(binary, os.X_OK):
            try: os.chmod(binary, os.stat(binary).st_mode | 0o111)
            except OSError: pass
        os.execvp(binary, [binary] + sys.argv[1:])
    else:
        raise SystemExit(subprocess.run([binary] + sys.argv[1:]).returncode)

if __name__ == "__main__":
    main()
