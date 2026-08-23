#!/bin/sh
set -eu

usage() {
  echo "usage: $0 --source DIR [--lock FILE] [--assembly-root DIR] [--write-env FILE]" >&2
  exit 2
}

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
source_dir=
lock_path="$repo_root/locks/kagent-preview.lock.json"
assembly_root="$repo_root"
write_env=

while [ "$#" -gt 0 ]; do
  case "$1" in
    --source)
      [ "$#" -ge 2 ] || usage
      source_dir=$2
      shift 2
      ;;
    --lock)
      [ "$#" -ge 2 ] || usage
      lock_path=$2
      shift 2
      ;;
    --assembly-root)
      [ "$#" -ge 2 ] || usage
      assembly_root=$2
      shift 2
      ;;
    --write-env)
      [ "$#" -ge 2 ] || usage
      write_env=$2
      shift 2
      ;;
    *) usage ;;
  esac
done

[ -n "$source_dir" ] || usage
[ -e "$source_dir/.git" ] || {
  echo "controller-only guard: source is not a Git checkout: $source_dir" >&2
  exit 1
}

env_file=$(mktemp "${TMPDIR:-/tmp}/kagent-preview-env.XXXXXX")
trap 'rm -f "$env_file"' EXIT HUP INT TERM
python3 "$repo_root/scripts/preview-lock-env.py" "$lock_path" >"$env_file"
# preview-lock-env.py emits shell-quoted values from a closed schema.
# shellcheck disable=SC1090
. "$env_file"

head_commit=$(git -C "$source_dir" rev-parse HEAD)
[ "$head_commit" = "$SOURCE_COMMIT" ] || {
  echo "controller-only guard: HEAD $head_commit is not locked commit $SOURCE_COMMIT" >&2
  exit 1
}

if [ -n "$(git -C "$source_dir" status --porcelain)" ]; then
  echo "controller-only guard: source checkout is dirty" >&2
  git -C "$source_dir" status --short >&2
  exit 1
fi

origin=$(git -C "$source_dir" remote get-url origin 2>/dev/null || true)
case "$origin" in
  https://github.com/pilprod/kagent|https://github.com/pilprod/kagent.git) ;;
  *)
    echo "controller-only guard: origin is not the locked pilprod/kagent fork: $origin" >&2
    exit 1
    ;;
esac

git -C "$source_dir" cat-file -e "$COMPARISON_BASE_COMMIT^{commit}" 2>/dev/null || {
  echo "controller-only guard: comparison base is missing from checkout" >&2
  exit 1
}
git -C "$source_dir" merge-base --is-ancestor "$COMPARISON_BASE_COMMIT" "$SOURCE_COMMIT" || {
  echo "controller-only guard: comparison base is not an ancestor of source" >&2
  exit 1
}

protected_changes=
changed_paths=$(git -C "$source_dir" diff --name-only "$COMPARISON_BASE_COMMIT" "$SOURCE_COMMIT")
for changed_path in $changed_paths; do
  case "$changed_path" in
    go/api/*|helm/kagent-crds/*|go/core/pkg/migrations/*|go/adk/*|go/Dockerfile|python/*|docker/*|ui/*)
      protected_changes="${protected_changes}${changed_path}
"
      ;;
  esac
done
[ -z "$protected_changes" ] || {
  echo "controller-only guard: protected API/CRD/migration/runtime paths changed:" >&2
  printf '%s' "$protected_changes" >&2
  exit 1
}

actual_chart_tree=$(git -C "$source_dir" rev-parse "$SOURCE_COMMIT:helm/kagent")
[ "$actual_chart_tree" = "$APPLICATION_CHART_TREE" ] || {
  echo "controller-only guard: application chart tree drifted" >&2
  echo "  locked: $APPLICATION_CHART_TREE" >&2
  echo "  actual: $actual_chart_tree" >&2
  exit 1
}

actual_crd_tree=$(git -C "$source_dir" rev-parse "$SOURCE_COMMIT:helm/kagent-crds")
[ "$actual_crd_tree" = "$CRD_CHART_TREE" ] || {
  echo "controller-only guard: CRD chart tree drifted" >&2
  echo "  locked: $CRD_CHART_TREE" >&2
  echo "  actual: $actual_crd_tree" >&2
  exit 1
}

case "$VALUES_PATH" in
  /*|../*|*/../*|*/..)
    echo "controller-only guard: values path escapes assembly root" >&2
    exit 1
    ;;
esac
values_file="$assembly_root/$VALUES_PATH"
[ -f "$values_file" ] || {
  echo "controller-only guard: missing locked values file: $values_file" >&2
  exit 1
}
if command -v sha256sum >/dev/null 2>&1; then
  actual_values_sha=$(sha256sum "$values_file" | awk '{print $1}')
else
  actual_values_sha=$(shasum -a 256 "$values_file" | awk '{print $1}')
fi
[ "$actual_values_sha" = "$VALUES_SHA256" ] || {
  echo "controller-only guard: Helm values drifted" >&2
  echo "  locked: $VALUES_SHA256" >&2
  echo "  actual: $actual_values_sha" >&2
  exit 1
}

actual_go_version=$(awk '/^go / { print $2; exit }' "$source_dir/go/go.mod")
[ "$actual_go_version" = "$GO_VERSION" ] || {
  echo "controller-only guard: Go version $actual_go_version is not locked $GO_VERSION" >&2
  exit 1
}
[ -f "$source_dir/$DOCKERFILE" ] || {
  echo "controller-only guard: locked Dockerfile is missing" >&2
  exit 1
}
[ "$UI_SOURCE_PATH" = "ui" ] || {
  echo "controller-only guard: UI companion source path is not locked to ui" >&2
  exit 1
}
[ -f "$source_dir/$UI_DOCKERFILE" ] || {
  echo "controller-only guard: locked UI Dockerfile is missing" >&2
  exit 1
}
[ -f "$source_dir/go/$BUILD_PACKAGE" ] || {
  echo "controller-only guard: locked controller package is missing" >&2
  exit 1
}

substrate_replace=$(awk '
  $1 == "replace" && $2 == "github.com/agent-substrate/substrate" {
    version=$NF; sub(/^v/, "", version); print version; exit
  }
' "$source_dir/go/go.mod")
[ "$substrate_replace" = "$SUBSTRATE_VERSION" ] || {
  echo "controller-only guard: Substrate source version $substrate_replace is not locked $SUBSTRATE_VERSION" >&2
  exit 1
}

if [ -n "$write_env" ]; then
  mkdir -p "$(dirname -- "$write_env")"
  cp "$env_file" "$write_env"
fi

echo "controller-only guard: accepted $SOURCE_COMMIT"
