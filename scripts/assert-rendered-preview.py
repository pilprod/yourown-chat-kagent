#!/usr/bin/env python3
"""Validate the security boundary of the rendered preview manifest."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


def scalar(document: str, pattern: str) -> str | None:
    match = re.search(pattern, document, re.MULTILINE)
    return match.group(1).strip().strip('"\'') if match else None


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--controller-image", required=True)
    parser.add_argument("--ui-image", required=True)
    parser.add_argument("--database-image", required=True)
    args = parser.parse_args()

    try:
        text = args.manifest.read_text(encoding="utf-8")
    except OSError as error:
        print(f"render gate could not read manifest: {error}", file=sys.stderr)
        return 1
    documents = []
    for part in re.split(r"^---\s*$", text, flags=re.MULTILINE):
        meaningful = "\n".join(
            line for line in part.splitlines() if not line.lstrip().startswith("#")
        ).strip()
        if meaningful:
            documents.append(part)
    if not documents:
        print("render gate rejected an empty manifest", file=sys.stderr)
        return 1

    errors: list[str] = []
    expected_resources = {
        ("ServiceAccount", "kagent-preview-controller"),
        ("ServiceAccount", "kagent-preview-postgresql"),
        ("ServiceAccount", "kagent-preview-ui"),
        ("Secret", "kagent-preview-postgresql"),
        ("ConfigMap", "kagent-builtin-prompts"),
        ("ConfigMap", "kagent-preview-controller"),
        ("ConfigMap", "kagent-preview-ui-config"),
        ("PersistentVolumeClaim", "kagent-preview-postgresql"),
        ("Service", "kagent-preview-controller"),
        ("Service", "kagent-preview-postgresql"),
        ("Service", "kagent-preview-ui"),
        ("Deployment", "kagent-preview-controller"),
        ("Deployment", "kagent-preview-postgresql"),
        ("Deployment", "kagent-preview-ui"),
        ("ModelConfig", "default-model-config"),
    }
    actual_resources: set[tuple[str, str]] = set()
    controller_seen = False
    controller_service_seen = False
    ui_seen = False
    ui_service_seen = False
    database_seen = False

    for document in documents:
        kind = scalar(document, r"^kind:\s*(\S+)\s*$") or ""
        name = scalar(document, r"^metadata:\s*\n(?:^[ ]+.*\n)*?^[ ]{2}name:\s*(\S+)\s*$") or ""
        namespace = scalar(document, r"^metadata:\s*\n(?:^[ ]+.*\n)*?^[ ]{2}namespace:\s*(\S+)\s*$") or ""
        resource = (kind, name)
        if resource in actual_resources:
            errors.append(f"duplicate rendered resource {kind} {name}")
        actual_resources.add(resource)
        if namespace != "kagent-system":
            errors.append(f"{kind} {name} is outside kagent-system")

        for unique_label in (
            "app.kubernetes.io/managed-by:",
            "app.kubernetes.io/part-of:",
            "platform.yourown.chat/release-channel:",
        ):
            if document.count(unique_label) > 1:
                errors.append(f"{kind} {name} repeats YAML label key {unique_label[:-1]}")

        if kind in {
            "Role",
            "RoleBinding",
            "ClusterRole",
            "ClusterRoleBinding",
            "CustomResourceDefinition",
            "Ingress",
            "Gateway",
            "HTTPRoute",
            "GRPCRoute",
            "TCPRoute",
            "TLSRoute",
        }:
            errors.append(f"release-owned {kind} {name} is forbidden")
        if kind == "Pod":
            errors.append(f"standalone Pod {name} is forbidden")
        if kind == "Service":
            service_type = scalar(document, r"^[ ]{2}type:\s*(\S+)\s*$") or "ClusterIP"
            if service_type in {"LoadBalancer", "NodePort"}:
                errors.append(f"public Service {name} uses {service_type}")
            if name == "kagent-preview-controller":
                controller_service_seen = service_type == "ClusterIP"
            if name == "kagent-preview-ui":
                ui_service_seen = (
                    service_type == "ClusterIP"
                    and re.search(r"^[ ]{4}- port:\s*8080\s*$", document, re.MULTILINE)
                    is not None
                    and re.search(r"^[ ]{6}targetPort:\s*8080\s*$", document, re.MULTILINE)
                    is not None
                )
        if kind == "Deployment":
            replicas = scalar(document, r"^[ ]{2}replicas:\s*([0-9]+)\s*$")
            if name == "kagent-preview-controller":
                controller_seen = replicas == "1" and args.controller_image in document
            if name == "kagent-preview-ui":
                ui_seen = replicas == "1" and args.ui_image in document
                if replicas != "1":
                    errors.append("UI Deployment must have exactly one replica")
            if name == "kagent-preview-postgresql":
                database_seen = args.database_image in document

    if not controller_seen:
        errors.append("controller Deployment is missing, not single-replica, or not image-pinned")
    if not controller_service_seen:
        errors.append("controller ClusterIP Service is missing")
    if not ui_seen:
        errors.append("UI Deployment is missing, not single-replica, or not image-pinned")
    if not ui_service_seen:
        errors.append("UI ClusterIP Service on port 8080 is missing")
    if not database_seen:
        errors.append("bundled testbed PostgreSQL is missing or not digest-pinned")
    missing = sorted(expected_resources - actual_resources)
    unexpected = sorted(actual_resources - expected_resources)
    for kind, name in missing:
        errors.append(f"expected rendered resource {kind} {name} is missing")
    for kind, name in unexpected:
        errors.append(f"unexpected rendered resource {kind} {name}")
    if errors:
        for error in errors:
            print(f"render gate: {error}", file=sys.stderr)
        return 1
    print("render gate: controller-delta preview manifest accepted")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
