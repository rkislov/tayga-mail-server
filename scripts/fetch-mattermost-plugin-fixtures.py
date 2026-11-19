#!/usr/bin/env python3
"""Download pinned public plugin fixtures for the explicit opt-in SDK test."""
import argparse
import hashlib
import json
import platform
import subprocess
import tempfile
from pathlib import Path


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def main():
    machine = {"x86_64": "amd64", "aarch64": "arm64"}.get(platform.machine(), platform.machine())
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform", default=platform.system().lower() + "-" + machine)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    pins = Path(__file__).resolve().parents[1] / "internal/mmplugins/testdata/marketplace.json"
    args.output.mkdir(parents=True, exist_ok=True, mode=0o700)
    for bundle in json.loads(pins.read_text()):
        asset = bundle["assets"].get(args.platform)
        if asset is None:
            print(f"Skip {bundle['name']}: no pinned native {args.platform} bundle")
            continue
        target = args.output / (bundle["name"] + ".tar.gz")
        if target.exists():
            if target.stat().st_size > 100 << 20 or digest(target) != asset["sha256"]:
                raise RuntimeError(f"Existing fixture has a different checksum: {target}")
        else:
            with tempfile.NamedTemporaryFile(dir=args.output, prefix=".plugin-", delete=False) as temp:
                name = Path(temp.name)
            try:
                subprocess.run(["curl", "--fail", "--location", "--silent", "--show-error",
                                "--max-time", "180", "--max-filesize", str(100 << 20),
                                "--output", str(name), asset["url"]], check=True)
                if digest(name) != asset["sha256"]:
                    raise RuntimeError(f"Checksum mismatch for {bundle['name']}")
                name.replace(target)
            finally:
                name.unlink(missing_ok=True)
        print(f"Verified {bundle['name']} {bundle['release']} for {args.platform}")


if __name__ == "__main__":
    main()
