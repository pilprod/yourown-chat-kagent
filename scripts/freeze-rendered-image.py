#!/usr/bin/env python3
"""Materialize one exact image replacement Cloud Deploy must make."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("source", type=Path)
    parser.add_argument("destination", type=Path)
    parser.add_argument("--tagged-image", required=True)
    parser.add_argument("--immutable-image", required=True)
    args = parser.parse_args()
    text = args.source.read_text(encoding="utf-8")
    count = text.count(args.tagged_image)
    if count != 1:
        print(f"expected exactly one tagged image occurrence, got {count}", file=sys.stderr)
        return 1
    frozen = text.replace(args.tagged_image, args.immutable_image)
    if args.tagged_image in frozen or args.immutable_image not in frozen:
        print("image replacement did not freeze the manifest", file=sys.stderr)
        return 1
    args.destination.write_text(frozen, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
