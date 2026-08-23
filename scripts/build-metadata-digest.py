#!/usr/bin/env python3
"""Extract and validate the immutable digest emitted by Docker Buildx."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


DIGEST = re.compile(r"^sha256:[0-9a-f]{64}$")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("metadata", type=Path)
    args = parser.parse_args()
    try:
        with args.metadata.open(encoding="utf-8") as handle:
            metadata = json.load(handle)
    except (OSError, json.JSONDecodeError) as error:
        print(f"build metadata rejected: {error}", file=sys.stderr)
        return 1
    digest = metadata.get("containerimage.digest")
    if not isinstance(digest, str) or not DIGEST.fullmatch(digest):
        print("build metadata has no immutable containerimage.digest", file=sys.stderr)
        return 1
    print(digest)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
