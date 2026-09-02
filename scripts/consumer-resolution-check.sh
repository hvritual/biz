#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
EXPECTED_GOWORK="$ROOT/go.work"

fail() {
  echo "dependency resolution check: $*" >&2
  exit 1
}

actual_gowork="$(cd "$ROOT" && go env GOWORK)"
[[ "$actual_gowork" == "$EXPECTED_GOWORK" ]] || fail "Biz is not owning workspace resolution: got $actual_gowork, expected $EXPECTED_GOWORK"

work_json="$(cd "$ROOT" && go work edit -json)"
use_count="$(printf '%s\n' "$work_json" | grep -c '"DiskPath"')"
[[ "$use_count" -eq 1 ]] || fail "go.work must contain exactly one use entry, found $use_count"
printf '%s\n' "$work_json" | grep -Eq '"DiskPath"[[:space:]]*:[[:space:]]*"\."' || fail "go.work must use only the Biz module (.)"

workspace_graph="$(mktemp)"
isolated_graph="$(mktemp)"
diff_file="$(mktemp)"
trap 'rm -f "$workspace_graph" "$isolated_graph" "$diff_file"' EXIT

(
  cd "$ROOT"
  go list -m all | LC_ALL=C sort
) > "$workspace_graph"

(
  cd "$ROOT"
  GOWORK=off go list -m all | LC_ALL=C sort
) > "$isolated_graph"

if ! diff -u "$isolated_graph" "$workspace_graph" > "$diff_file"; then
  cat "$diff_file" >&2
  fail "Biz workspace graph differs from its GOWORK=off consumer graph"
fi

echo "dependency resolution check: Biz-local workspace equals GOWORK=off graph"
