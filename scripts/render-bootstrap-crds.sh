#!/bin/sh
set -eu

usage() {
  echo "usage: $0 --source DIR --output FILE [--lock FILE]" >&2
  exit 2
}

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
source_dir=
output=
lock_path="$repo_root/locks/kagent-preview.lock.json"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --source)
      [ "$#" -ge 2 ] || usage
      source_dir=$2
      shift 2
      ;;
    --output)
      [ "$#" -ge 2 ] || usage
      output=$2
      shift 2
      ;;
    --lock)
      [ "$#" -ge 2 ] || usage
      lock_path=$2
      shift 2
      ;;
    *) usage ;;
  esac
done
[ -n "$source_dir" ] && [ -n "$output" ] || usage
[ ! -e "$output" ] || {
  echo "refusing to overwrite existing CRD bundle: $output" >&2
  exit 1
}
command -v helm >/dev/null 2>&1 || {
  echo "helm is required to render the CRD bootstrap bundle" >&2
  exit 1
}
helm_version=$(helm version --short)
case "$helm_version" in
  v3.19.0+*) ;;
  *)
    echo "locked Helm v3.19.0 is required, got $helm_version" >&2
    exit 1
    ;;
esac

"$repo_root/scripts/assert-controller-only.sh" \
  --source "$source_dir" --lock "$lock_path" --assembly-root "$repo_root"

env_file=$(mktemp "${TMPDIR:-/tmp}/kagent-preview-env.XXXXXX")
chart_dir=$(mktemp -d "${TMPDIR:-/tmp}/kagent-preview-crds.XXXXXX")
rendered=$(mktemp "${TMPDIR:-/tmp}/kagent-preview-crds-rendered.XXXXXX")
cleanup() {
  rm -f "$env_file" "$rendered"
  rm -rf "$chart_dir"
}
trap cleanup EXIT HUP INT TERM
python3 "$repo_root/scripts/preview-lock-env.py" "$lock_path" >"$env_file"
# shellcheck disable=SC1090
. "$env_file"

cp -R "$source_dir/helm/kagent-crds/." "$chart_dir/"
cp "$repo_root/deploy/bootstrap-crds/Chart.preview.yaml" "$chart_dir/Chart.yaml"
helm template kagent-preview-crds "$chart_dir" \
  -f "$repo_root/deploy/bootstrap-crds/values.preview.yaml" >"$rendered"
python3 "$repo_root/scripts/assert-crd-bundle.py" "$rendered" --expected-count 10
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$rendered" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$rendered" | awk '{print $1}')
fi
[ "$actual" = "$CRD_BUNDLE_SHA256" ] || {
  echo "rendered CRD digest $actual does not match lock $CRD_BUNDLE_SHA256" >&2
  exit 1
}
mkdir -p "$(dirname -- "$output")"
mv "$rendered" "$output"
rendered=
printf '%s  %s\n' "$actual" "$output"
