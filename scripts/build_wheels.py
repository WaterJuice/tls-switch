# ----------------------------------------------------------------------------------------
#   build_wheels.py
#   ---------------
#
#   Split a fat wheel (containing all platform binaries) into per-platform wheels.
#   Each output wheel contains only the Go binary for its target platform and is
#   tagged with the correct platform tag so pip installs the right one.
#
#   Usage: python build_wheels.py <fat-wheel> <output-dir>
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

import base64
import hashlib
import re
import sys
import zipfile
from pathlib import Path

# ----------------------------------------------------------------------------------------
#   Platform Definitions
# ----------------------------------------------------------------------------------------

# (binary filename, wheel platform tag)
PLATFORMS = [
    ("tls-switch-darwin-arm64", "macosx_11_0_arm64"),
    ("tls-switch-darwin-amd64", "macosx_10_12_x86_64"),
    ("tls-switch-linux-amd64", "manylinux_2_17_x86_64.manylinux2014_x86_64"),
    ("tls-switch-linux-arm64", "manylinux_2_17_aarch64.manylinux2014_aarch64"),
    ("tls-switch-windows-amd64.exe", "win_amd64"),
    ("tls-switch-windows-arm64.exe", "win_arm64"),
]

BIN_PREFIX = "tls_switch/bin/"
ALL_BINARY_NAMES = {name for name, _ in PLATFORMS}

# ----------------------------------------------------------------------------------------
#   Helpers
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _sha256_b64(data: bytes) -> str:
    """Return the sha256 hash in urlsafe base64 for RECORD entries."""
    digest = hashlib.sha256(data).digest()
    return "sha256=" + base64.urlsafe_b64encode(digest).rstrip(b"=").decode()


# ----------------------------------------------------------------------------------------
#   Wheel Splitting
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def split_wheel(fat_wheel: Path, output_dir: Path) -> list[Path]:
    """Split a fat wheel into per-platform wheels.

    Reads the fat wheel once, then for each platform creates a new wheel
    containing only that platform's binary (plus all non-binary files),
    with the WHEEL metadata and filename retagged.
    """
    output_dir.mkdir(parents=True, exist_ok=True)
    results: list[Path] = []

    with zipfile.ZipFile(fat_wheel) as zf:
        # Read all file data and info upfront
        file_data: dict[str, bytes] = {}
        file_info: dict[str, zipfile.ZipInfo] = {}
        for info in zf.infolist():
            file_data[info.filename] = zf.read(info.filename)
            file_info[info.filename] = info

        # Find the dist-info RECORD and WHEEL paths
        record_name = next(f for f in file_data if f.endswith("/RECORD"))
        dist_info = record_name.rsplit("/", 1)[0]
        wheel_meta_name = f"{dist_info}/WHEEL"

        for binary_name, platform_tag in PLATFORMS:
            # Filter files: keep everything except other platforms' binaries
            new_files: dict[str, bytes] = {}
            for fname, data in file_data.items():
                # Skip RECORD — we regenerate it
                if fname == record_name:
                    continue
                # Skip binaries for other platforms
                if fname.startswith(BIN_PREFIX):
                    basename = fname[len(BIN_PREFIX) :]
                    if basename in ALL_BINARY_NAMES and basename != binary_name:
                        continue
                    # Also skip .gitkeep
                    if basename == ".gitkeep":
                        continue
                # Retag WHEEL metadata
                if fname == wheel_meta_name:
                    text = data.decode()
                    text = re.sub(
                        r"Tag: py3-none-any",
                        f"Tag: py3-none-{platform_tag}",
                        text,
                    )
                    data = text.encode()
                new_files[fname] = data

            # Build new wheel with platform tag in filename
            new_wheel_name = fat_wheel.name.replace(
                "-py3-none-any.whl", f"-py3-none-{platform_tag}.whl"
            )
            new_wheel_path = output_dir / new_wheel_name

            with zipfile.ZipFile(
                new_wheel_path, "w", zipfile.ZIP_DEFLATED
            ) as new_zf:
                records: list[str] = []
                for fname, data in new_files.items():
                    # Preserve original ZipInfo (permissions, timestamps)
                    info = file_info[fname]
                    if fname == wheel_meta_name:
                        # Write modified WHEEL metadata
                        new_zf.writestr(info, data)
                    else:
                        new_zf.writestr(info, data)
                    records.append(f"{fname},{_sha256_b64(data)},{len(data)}")

                # Write RECORD as final entry
                records.append(f"{record_name},,")
                new_zf.writestr(record_name, "\n".join(records) + "\n")

            results.append(new_wheel_path)
            print(f"  {new_wheel_name}")

    return results


# ----------------------------------------------------------------------------------------
#   Main
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def main() -> int:
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} <fat-wheel> <output-dir>", file=sys.stderr)
        return 1

    fat_wheel = Path(sys.argv[1])
    output_dir = Path(sys.argv[2])

    if not fat_wheel.exists():
        print(f"Wheel not found: {fat_wheel}", file=sys.stderr)
        return 1

    print(f"Splitting {fat_wheel.name} into platform wheels:")
    results = split_wheel(fat_wheel, output_dir)
    print(f"\nBuilt {len(results)} platform wheels in {output_dir}/")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
