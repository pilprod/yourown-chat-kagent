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

chart_cache="${KAGENT_CHART_CACHE:-}"
cleanup_dir=""
if [[ -z "${chart_cache}" && "${KAGENT_VERIFY_REMOTE:-0}" == "1" ]]; then
  command -v helm >/dev/null || fail "helm is required for remote verification"
  cleanup_dir="$(mktemp -d)"
  chart_cache="${cleanup_dir}"
  trap 'rm -rf "${cleanup_dir}"' EXIT
  helm pull "${repository}/kagent" --version "${version}" --destination "${chart_cache}"
  helm pull "${repository}/kagent-crds" --version "${version}" --destination "${chart_cache}"
fi

if [[ -z "${chart_cache}" ]]; then
  printf 'static release checks passed; set KAGENT_VERIFY_REMOTE=1 for exact OCI render\n'
  exit 0
fi

command -v helm >/dev/null || fail "helm is required for chart rendering"
app_chart="${chart_cache}/kagent-${version}.tgz"
crds_chart="${chart_cache}/kagent-crds-${version}.tgz"
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
grep -Fq 'image: "cr.kagent.dev/kagent-dev/kagent/controller:0.9.12"' "${rendered}"
grep -Fq 'replicas: 1' "${rendered}"
! grep -Eq 'type: (LoadBalancer|NodePort)' "${rendered}" || fail "public Service rendered"
! grep -Fq 'kind: WorkerPool' "${rendered}" || fail "Substrate WorkerPool rendered"
! grep -Fq 'name: kagent-tools' "${rendered}" || fail "built-in tools rendered"
! grep -Fq 'name: querydoc' "${rendered}" || fail "querydoc rendered"

printf 'exact kagent %s testbed charts and render passed\n' "${version}"
