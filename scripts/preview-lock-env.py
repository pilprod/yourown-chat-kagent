#!/usr/bin/env python3
"""Validate a preview lock and export its immutable build inputs."""

from __future__ import annotations

import argparse
import json
import re
import shlex
import sys
from pathlib import Path
from typing import Any


GIT_OBJECT = re.compile(r"^[0-9a-f]{40}$")
SHA256 = re.compile(r"^[0-9a-f]{64}$")
GO_VERSION = re.compile(r"^[1-9][0-9]*\.[0-9]+\.[0-9]+$")


def fail(message: str) -> None:
    raise ValueError(message)


def exact_keys(value: Any, path: str, keys: set[str]) -> dict[str, Any]:
    if not isinstance(value, dict):
        fail(f"{path} must be an object")
    actual = set(value)
    if actual != keys:
        missing = sorted(keys - actual)
        extra = sorted(actual - keys)
        fail(f"{path} keys differ (missing={missing}, extra={extra})")
    return value


def expect(value: Any, expected: Any, path: str) -> None:
    if value != expected:
        fail(f"{path} must be {expected!r}, got {value!r}")


def match(value: Any, pattern: re.Pattern[str], path: str) -> str:
    if not isinstance(value, str) or not pattern.fullmatch(value):
        fail(f"{path} has an invalid format")
    return value


def validate(lock: Any) -> dict[str, str]:
    root = exact_keys(
        lock,
        "$",
        {
            "schemaVersion",
            "classification",
            "qualificationStatus",
            "source",
            "build",
            "tooling",
            "deployment",
            "evidence",
            "release",
        },
    )
    expect(
        root["schemaVersion"],
        "k8s-agents-platform/kagent-preview-lock/v1alpha1",
        "schemaVersion",
    )
    expect(root["classification"], "preview-controller-only", "classification")
    if root["qualificationStatus"] not in {
        "assembly-unqualified",
        "testbed-qualified",
    }:
        fail("qualificationStatus is not allowed")

    source = exact_keys(
        root["source"],
        "source",
        {
            "forkRepository",
            "upstreamRepository",
            "commit",
            "comparisonBaseCommit",
            "applicationChartTree",
            "crdChartTree",
        },
    )
    expect(
        source["forkRepository"],
        "https://github.com/pilprod/kagent.git",
        "source.forkRepository",
    )
    expect(
        source["upstreamRepository"],
        "https://github.com/kagent-dev/kagent.git",
        "source.upstreamRepository",
    )
    source_commit = match(source["commit"], GIT_OBJECT, "source.commit")
    comparison_base = match(
        source["comparisonBaseCommit"], GIT_OBJECT, "source.comparisonBaseCommit"
    )
    chart_tree = match(
        source["applicationChartTree"], GIT_OBJECT, "source.applicationChartTree"
    )
    crd_tree = match(source["crdChartTree"], GIT_OBJECT, "source.crdChartTree")

    build = exact_keys(
        root["build"],
        "build",
        {
            "goVersion",
            "platform",
            "package",
            "dockerfile",
            "imageRepository",
            "imageTag",
            "ui",
        },
    )
    go_version = match(build["goVersion"], GO_VERSION, "build.goVersion")
    expect(build["platform"], "linux/amd64", "build.platform")
    expect(
        build["package"], "core/cmd/controller-v2/main.go", "build.package"
    )
    expect(build["dockerfile"], "go/Dockerfile", "build.dockerfile")
    expect(
        build["imageRepository"],
        "europe-west3-docker.pkg.dev/yourown-chat/docker/kagent-controller",
        "build.imageRepository",
    )
    expect(build["imageTag"], f"git-{source_commit}", "build.imageTag")
    ui_build = exact_keys(
        build["ui"],
        "build.ui",
        {"sourcePath", "dockerfile", "imageRepository", "imageTag"},
    )
    expect(ui_build["sourcePath"], "ui", "build.ui.sourcePath")
    expect(ui_build["dockerfile"], "ui/Dockerfile", "build.ui.dockerfile")
    expect(
        ui_build["imageRepository"],
        "europe-west3-docker.pkg.dev/yourown-chat/docker/kagent-ui",
        "build.ui.imageRepository",
    )
    expect(ui_build["imageTag"], f"git-{source_commit}", "build.ui.imageTag")

    tooling = exact_keys(
        root["tooling"],
        "tooling",
        {
            "goBuilderImage",
            "goToolchain",
            "helmImage",
            "trivyImage",
            "buildkitImage",
        },
    )
    expect(
        tooling["goBuilderImage"],
        "docker.io/library/golang:1.26.5-bookworm@sha256:53eeac89074db483fdf0ab3be1df32bf6e47562263d2d0d6baa7f26acb4957dd",
        "tooling.goBuilderImage",
    )
    expect(tooling["goToolchain"], go_version, "tooling.goToolchain")
    expect(
        tooling["helmImage"],
        "docker.io/alpine/helm:3.19.0@sha256:aef9b56f64e866207d9591d0abd8f6d767b36aadd12edf68f8a719716d9d29c9",
        "tooling.helmImage",
    )
    expect(
        tooling["trivyImage"],
        "docker.io/aquasec/trivy:0.67.2@sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08",
        "tooling.trivyImage",
    )
    expect(
        tooling["buildkitImage"],
        "docker.io/moby/buildkit:v0.23.0@sha256:a38cf64aa6415899097fac5bfcf6c07c95d5a68c67a21e3d254ba398a3c9187f",
        "tooling.buildkitImage",
    )

    deployment = exact_keys(
        root["deployment"],
        "deployment",
        {
            "skaffoldFile",
            "skaffoldProfile",
            "chartPath",
            "valuesPath",
            "valuesSHA256",
            "namespace",
            "cloudDeployPipeline",
            "cloudDeployTarget",
            "controllerServiceType",
            "uiReplicas",
            "uiServiceType",
            "uiOrigin",
            "bootstrapCRDs",
            "excludedTemplates",
            "verificationImage",
            "database",
            "substrate",
        },
    )
    expect(deployment["skaffoldFile"], "deploy/skaffold.preview.yaml", "deployment.skaffoldFile")
    expect(deployment["skaffoldProfile"], "kagent-testbed", "deployment.skaffoldProfile")
    expect(deployment["chartPath"], "../source/kagent/helm/kagent", "deployment.chartPath")
    expect(deployment["valuesPath"], "deploy/helm/values.preview.yaml", "deployment.valuesPath")
    values_sha = match(deployment["valuesSHA256"], SHA256, "deployment.valuesSHA256")
    expect(deployment["namespace"], "kagent-system", "deployment.namespace")
    expect(deployment["cloudDeployPipeline"], "kagent-preview", "deployment.cloudDeployPipeline")
    expect(deployment["cloudDeployTarget"], "kagent-testbed", "deployment.cloudDeployTarget")
    expect(deployment["controllerServiceType"], "ClusterIP", "deployment.controllerServiceType")
    expect(deployment["uiReplicas"], 1, "deployment.uiReplicas")
    expect(deployment["uiServiceType"], "ClusterIP", "deployment.uiServiceType")
    expect(
        deployment["uiOrigin"],
        "http://kagent-preview-ui.kagent-system.svc.cluster.local:8080",
        "deployment.uiOrigin",
    )
    bootstrap_crds = exact_keys(
        deployment["bootstrapCRDs"],
        "deployment.bootstrapCRDs",
        {"mode", "artifactPath", "automaticApply", "bundleSHA256"},
    )
    expect(
        bootstrap_crds["mode"],
        "one-time-platform-admin",
        "deployment.bootstrapCRDs.mode",
    )
    expect(
        bootstrap_crds["artifactPath"],
        "evidence/bootstrap-crds.yaml",
        "deployment.bootstrapCRDs.artifactPath",
    )
    expect(
        bootstrap_crds["automaticApply"],
        False,
        "deployment.bootstrapCRDs.automaticApply",
    )
    crd_bundle_sha = match(
        bootstrap_crds["bundleSHA256"],
        SHA256,
        "deployment.bootstrapCRDs.bundleSHA256",
    )
    expect(
        deployment["excludedTemplates"],
        ["templates/rbac", "templates/substrate-ate-api-rbac.yaml"],
        "deployment.excludedTemplates",
    )
    expect(
        deployment["verificationImage"],
        "docker.io/curlimages/curl:8.10.1@sha256:d9b4541e214bcd85196d6e92e2753ac6d0ea699f0af5741f8c6cccbfcf00ef4b",
        "deployment.verificationImage",
    )
    database = exact_keys(
        deployment["database"], "deployment.database", {"mode", "image"}
    )
    expect(database["mode"], "bundled-testbed", "deployment.database.mode")
    expect(
        database["image"],
        "docker.io/library/postgres:18.3-alpine@sha256:54451ecb8ab38c24c3ec123f2fd501303a3a1856a5c66e98cecf2460d5e1e9d7",
        "deployment.database.image",
    )
    substrate = exact_keys(
        deployment["substrate"],
        "deployment.substrate",
        {"mode", "version", "namespace", "workerPool"},
    )
    expect(substrate["mode"], "external", "deployment.substrate.mode")
    expect(substrate["version"], "0.0.20", "deployment.substrate.version")
    expect(substrate["namespace"], "ate-system", "deployment.substrate.namespace")
    expect(substrate["workerPool"], "kagent-default", "deployment.substrate.workerPool")

    evidence = exact_keys(root["evidence"], "evidence", {"bucket", "prefix"})
    expect(
        evidence["bucket"],
        "yourown-chat-kagent-preview-europe-west3",
        "evidence.bucket",
    )
    expect(
        evidence["prefix"],
        "evidence/yourown-chat-kagent/preview",
        "evidence.prefix",
    )

    release = exact_keys(
        root["release"],
        "release",
        {
            "owner",
            "triggerRepository",
            "triggerTagPattern",
            "releaseNameTemplate",
            "forkTagsRelease",
            "productionEligible",
        },
    )
    expect(release["owner"], "pilprod/yourown-chat-kagent", "release.owner")
    expect(
        release["triggerRepository"],
        "pilprod/yourown-chat-kagent",
        "release.triggerRepository",
    )
    expect(
        release["triggerTagPattern"],
        "^preview-[0-9]{8}-[1-9][0-9]*$",
        "release.triggerTagPattern",
    )
    expect(
        release["releaseNameTemplate"],
        "kagent-{tag}-{sourceShortSHA}",
        "release.releaseNameTemplate",
    )
    expect(release["forkTagsRelease"], False, "release.forkTagsRelease")
    expect(release["productionEligible"], False, "release.productionEligible")

    return {
        "SOURCE_REPOSITORY": source["forkRepository"],
        "SOURCE_COMMIT": source_commit,
        "COMPARISON_BASE_COMMIT": comparison_base,
        "APPLICATION_CHART_TREE": chart_tree,
        "CRD_CHART_TREE": crd_tree,
        "CRD_BUNDLE_SHA256": crd_bundle_sha,
        "GO_VERSION": go_version,
        "GO_BUILDER_IMAGE": tooling["goBuilderImage"],
        "GO_TOOLCHAIN": tooling["goToolchain"],
        "HELM_IMAGE": tooling["helmImage"],
        "TRIVY_IMAGE": tooling["trivyImage"],
        "BUILDKIT_IMAGE": tooling["buildkitImage"],
        "BUILD_PLATFORM": build["platform"],
        "BUILD_PACKAGE": build["package"],
        "DOCKERFILE": build["dockerfile"],
        "IMAGE_REPOSITORY": build["imageRepository"],
        "IMAGE_TAG": build["imageTag"],
        "UI_SOURCE_PATH": ui_build["sourcePath"],
        "UI_DOCKERFILE": ui_build["dockerfile"],
        "UI_IMAGE_REPOSITORY": ui_build["imageRepository"],
        "UI_IMAGE_TAG": ui_build["imageTag"],
        "UI_ORIGIN": deployment["uiOrigin"],
        "SKAFFOLD_FILE": deployment["skaffoldFile"],
        "SKAFFOLD_PROFILE": deployment["skaffoldProfile"],
        "VALUES_PATH": deployment["valuesPath"],
        "VALUES_SHA256": values_sha,
        "VERIFY_IMAGE": deployment["verificationImage"],
        "CLOUD_DEPLOY_PIPELINE": deployment["cloudDeployPipeline"],
        "CLOUD_DEPLOY_TARGET": deployment["cloudDeployTarget"],
        "SUBSTRATE_VERSION": substrate["version"],
        "EVIDENCE_BUCKET": evidence["bucket"],
        "EVIDENCE_PREFIX": evidence["prefix"],
        "DATABASE_IMAGE": database["image"],
        "TRIGGER_TAG_PATTERN": release["triggerTagPattern"],
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("lock", type=Path)
    parser.add_argument("--format", choices=("shell", "json"), default="shell")
    args = parser.parse_args()
    try:
        with args.lock.open(encoding="utf-8") as handle:
            exported = validate(json.load(handle))
    except (OSError, json.JSONDecodeError, ValueError) as error:
        print(f"preview lock rejected: {error}", file=sys.stderr)
        return 1

    if args.format == "json":
        json.dump(exported, sys.stdout, sort_keys=True, indent=2)
        sys.stdout.write("\n")
    else:
        for key in sorted(exported):
            print(f"{key}={shlex.quote(exported[key])}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
