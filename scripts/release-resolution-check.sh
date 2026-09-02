#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"

fail() {
  echo "release resolution check: $*" >&2
  exit 1
}

workspace_raw="$(mktemp)"
release_raw="$(mktemp)"
workspace_graph="$(mktemp)"
release_graph="$(mktemp)"
diff_file="$(mktemp)"
trap 'rm -f "$workspace_raw" "$release_raw" "$workspace_graph" "$release_graph" "$diff_file"' EXIT

normalize_graph() {
  awk '
    NF == 1 { print $1; next }
    { print $1 " " $2 }
  ' | LC_ALL=C sort
}

(
  cd "$ROOT"
  go list -m all
) > "$workspace_raw"
normalize_graph < "$workspace_raw" > "$workspace_graph"

if ! (
  cd "$ROOT"
  GOWORK=off go list -m all
) > "$release_raw"; then
  fail "GOWORK=off cannot resolve the release dependency graph; Biz still depends on unpublished or locally patched Yunka modules"
fi

if grep -q ' => ' "$release_raw"; then
  cat "$release_raw" >&2
  fail "release graph still contains module replacements; go.mod must be a publishable consumer contract"
fi

normalize_graph < "$release_raw" > "$release_graph"

if ! diff -u "$release_graph" "$workspace_graph" > "$diff_file"; then
  cat "$diff_file" >&2
  fail "development workspace and GOWORK=off release select different module path/version identities"
fi

echo "release resolution check: GOWORK=off graph matches the locked development graph by module path/version"
