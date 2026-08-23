#!/usr/bin/env python3
"""Require a kagent-only, deterministic CRD bootstrap bundle."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("bundle", type=Path)
    parser.add_argument("--expected-count", type=int, default=10)
    args = parser.parse_args()
    text = args.bundle.read_text(encoding="utf-8")
    documents = [part for part in re.split(r"^---\s*$", text, flags=re.MULTILINE) if part.strip()]
    names: list[str] = []
    for document in documents:
        kind = re.search(r"^kind:\s*(\S+)\s*$", document, re.MULTILINE)
        if kind is None or kind.group(1) != "CustomResourceDefinition":
            print("CRD bundle contains a non-CRD document", file=sys.stderr)
            return 1
        name = re.search(r"^metadata:\s*\n(?:^[ ]+.*\n)*?^[ ]{2}name:\s*(\S+)\s*$", document, re.MULTILINE)
        if name is None or not name.group(1).endswith(".kagent.dev"):
            print("CRD bundle contains a CRD outside kagent.dev", file=sys.stderr)
            return 1
        names.append(name.group(1))
    if len(names) != args.expected_count or len(set(names)) != len(names):
        print(f"expected {args.expected_count} unique kagent CRDs, got {len(names)}", file=sys.stderr)
        return 1
    print(f"CRD bundle: accepted {len(names)} kagent.dev definitions")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
