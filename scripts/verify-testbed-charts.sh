#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
lock_file="${repo_root}/locks/kagent-testbed.lock.json"
values_file="${repo_root}/deploy/testbed/kagent-values.yaml"
crds_values_file="${repo_root}/deploy/testbed/kagent-crds-values.yaml"

fail() {
  printf 'kagent testbed verification: %s\n' "$*" >&2
  exit 1
}

command -v jq >/dev/null || fail "jq is required"

version="$(jq -er '.charts.version' "${lock_file}")"
repository="$(jq -er '.charts.repository' "${lock_file}")"
expected_app_sha="$(jq -er '.charts.application.archiveSHA256' "${lock_file}")"
expected_crds_sha="$(jq -er '.charts.crds.archiveSHA256' "${lock_file}")"
expected_app_oci="$(jq -er '.charts.application.ociDigest' "${lock_file}")"
expected_crds_oci="$(jq -er '.charts.crds.ociDigest' "${lock_file}")"

chart_cache="${KAGENT_CHART_CACHE:-}"
cleanup_dir=""
if [[ -z "${chart_cache}" && "${KAGENT_VERIFY_REMOTE:-0}" == "1" ]]; then
  command -v helm >/dev/null || fail "helm is required for remote verification"
  cleanup_dir="$(mktemp -d)"
  chart_cache="${cleanup_dir}"
  trap 'rm -rf "${cleanup_dir}"' EXIT
  # Pull by immutable manifest digest. The version remains descriptive release
  # metadata and is never trusted as the artifact selector.
  helm pull "${repository}/kagent@${expected_app_oci}" --destination "${chart_cache}"
  helm pull "${repository}/kagent-crds@${expected_crds_oci}" --destination "${chart_cache}"
fi

if [[ -z "${chart_cache}" ]]; then
  printf 'static release checks passed; set KAGENT_VERIFY_REMOTE=1 for exact OCI render\n'
  exit 0
fi

command -v helm >/dev/null || fail "helm is required for chart rendering"
app_chart="${chart_cache}/kagent-${version}.tgz"
crds_chart="${chart_cache}/kagent-crds-${version}.tgz"
digest_app_chart="${chart_cache}/kagent@sha256-${expected_app_oci#sha256:}.tgz"
digest_crds_chart="${chart_cache}/kagent-crds@sha256-${expected_crds_oci#sha256:}.tgz"
[[ -f "${app_chart}" ]] || app_chart="${digest_app_chart}"
[[ -f "${crds_chart}" ]] || crds_chart="${digest_crds_chart}"
[[ -f "${app_chart}" ]] || fail "missing ${app_chart}"
[[ -f "${crds_chart}" ]] || fail "missing ${crds_chart}"

actual_app_sha="$(shasum -a 256 "${app_chart}" | awk '{print $1}')"
actual_crds_sha="$(shasum -a 256 "${crds_chart}" | awk '{print $1}')"
[[ "${actual_app_sha}" == "${expected_app_sha}" ]] || fail "application chart checksum mismatch"
[[ "${actual_crds_sha}" == "${expected_crds_sha}" ]] || fail "CRD chart checksum mismatch"

rendered="$(mktemp)"
trap 'rm -f "${rendered}"; [[ -z "${cleanup_dir}" ]] || rm -rf "${cleanup_dir}"' EXIT
helm template kagent "${app_chart}" \
  --namespace kagent-system \
  --values "${values_file}" > "${rendered}"
helm template kagent-crds "${crds_chart}" \
  --namespace kagent-system \
  --values "${crds_values_file}" >/dev/null

grep -Fq 'name: kagent-controller' "${rendered}"
grep -Fq 'name: kagent-ui' "${rendered}"
grep -Fq 'WATCH_NAMESPACES: "kagent-system,kagent-testbed"' "${rendered}"
grep -Fq 'image: "cr.kagent.dev/kagent-dev/kagent/controller:0.9.12@sha256:d1ea7b70bb8d97de9f0774d44b598971c944b3ab4e88294b0bb78e59d1a63c15"' "${rendered}"
grep -Fq 'image: "cr.kagent.dev/kagent-dev/kagent/ui:0.9.12@sha256:1d5ada8d7f65a6b9ad28232463f9fd670c4c20875baa1c8008aaa1f1f988382e"' "${rendered}"
grep -Fq 'name: POSTGRES_DATABASE_URL_FILE' "${rendered}"
grep -Fq 'secretProviderClass: kagent-database-gcp' "${rendered}"
grep -Fq 'replicas: 1' "${rendered}"
! grep -Eq 'type: (LoadBalancer|NodePort)' "${rendered}" || fail "public Service rendered"
! grep -Fq 'name: kagent-postgresql' "${rendered}" || fail "bundled PostgreSQL rendered"
! grep -Fq 'kind: WorkerPool' "${rendered}" || fail "Substrate WorkerPool rendered"
! grep -Fq 'name: kagent-tools' "${rendered}" || fail "built-in tools rendered"
! grep -Fq 'name: querydoc' "${rendered}" || fail "querydoc rendered"

printf 'exact kagent %s testbed charts and render passed\n' "${version}"
