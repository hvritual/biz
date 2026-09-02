#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
EXPECTED_GOWORK="$ROOT/go.work"
LOCK_FILE="$ROOT/.yunka/source.env"

fail() {
  echo "dependency resolution check: $*" >&2
  exit 1
}

actual_gowork="$(cd "$ROOT" && go env GOWORK)"
[[ "$actual_gowork" == "$EXPECTED_GOWORK" ]] || fail "Biz is not owning workspace resolution: got $actual_gowork, expected $EXPECTED_GOWORK"

[[ -f "$LOCK_FILE" ]] || fail "missing lock file: $LOCK_FILE"
# shellcheck disable=SC1090
source "$LOCK_FILE"

work_json="$(cd "$ROOT" && go work edit -json)"

use_count="$(printf '%s\n' "$work_json" | python3 -c 'import json,sys; print(len(json.load(sys.stdin).get("Use") or []))')"
[[ "$use_count" -eq 1 ]] || fail "go.work must contain exactly one use entry, found $use_count"

biz_use="$(printf '%s\n' "$work_json" | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("Use") or [{}])[0].get("DiskPath", ""))')"
[[ "$biz_use" == "." ]] || fail "go.work must use only the Biz module (.), got $biz_use"

workspace_version() {
  local module="$1"
  (cd "$ROOT" && go list -m -f '{{.Version}}' "$module")
}

check_version() {
  local module="$1"
  local expected="$2"
  local actual
  actual="$(workspace_version "$module")" || fail "cannot resolve $module in Biz workspace"
  [[ "$actual" == "$expected" ]] || fail "$module workspace version is $actual, expected $expected"
}

check_version github.com/hvritual/yunka.io/framework "$YUNKA_VERSION"
check_version github.com/hvritual/yunka.io/gateway "$YUNKA_VERSION"
check_version github.com/hvritual/yunka.io/pkg "$YUNKA_VERSION"

(cd "$ROOT" && go list -m all >/dev/null)

echo "dependency resolution check: Biz owns a single-module development workspace with locked Yunka versions"
