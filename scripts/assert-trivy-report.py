#!/usr/bin/env python3
"""Fail closed when a Trivy JSON report contains HIGH/CRITICAL findings."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    parser.add_argument("--summary", type=Path)
    parser.add_argument("--allow-findings", action="store_true")
    args = parser.parse_args()

    try:
        with args.report.open(encoding="utf-8") as handle:
            report = json.load(handle)
    except (OSError, json.JSONDecodeError) as error:
        print(f"Trivy gate rejected unreadable report: {error}", file=sys.stderr)
        return 1

    results = report.get("Results")
    if not isinstance(results, list) or not results:
        print("Trivy gate rejected report without scan results", file=sys.stderr)
        return 1

    findings: list[tuple[str, str, str, str]] = []
    for result in results:
        if not isinstance(result, dict):
            print("Trivy gate rejected malformed result", file=sys.stderr)
            return 1
        vulnerabilities = result.get("Vulnerabilities") or []
        if not isinstance(vulnerabilities, list):
            print("Trivy gate rejected malformed vulnerabilities", file=sys.stderr)
            return 1
        for vulnerability in vulnerabilities:
            severity = str(vulnerability.get("Severity", "")).upper()
            if severity in {"HIGH", "CRITICAL"}:
                findings.append(
                    (
                        severity,
                        str(vulnerability.get("VulnerabilityID", "UNKNOWN")),
                        str(vulnerability.get("PkgName", "UNKNOWN")),
                        str(vulnerability.get("InstalledVersion", "UNKNOWN")),
                    )
                )

    findings.sort()
    lines = [
        f"artifact={report.get('ArtifactName', 'UNKNOWN')}",
        f"high_or_critical={len(findings)}",
    ]
    lines.extend("\t".join(finding) for finding in findings)
    summary = "\n".join(lines) + "\n"
    if args.summary:
        args.summary.write_text(summary, encoding="utf-8")
    sys.stdout.write(summary)

    if findings and not args.allow_findings:
        print("Trivy gate blocked the preview release", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
