#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LOCK_FILE="${YUNKA_LOCK_FILE:-$ROOT/.yunka/source.env}"

fail() {
  echo "yunka source check: $*" >&2
  exit 1
}

[[ -f "$LOCK_FILE" ]] || fail "missing lock file: $LOCK_FILE"
# shellcheck disable=SC1090
source "$LOCK_FILE"

: "${YUNKA_COMMIT:?missing YUNKA_COMMIT in $LOCK_FILE}"
: "${YUNKA_PSEUDO_VERSION:?missing YUNKA_PSEUDO_VERSION in $LOCK_FILE}"

YUNKA_ROOT="${YUNKA_ROOT:-$(cd "$ROOT/.." && pwd -P)/yunka.io}"
[[ -d "$YUNKA_ROOT/.git" ]] || fail "YUNKA_ROOT is not a git checkout: $YUNKA_ROOT"

actual_commit="$(git -C "$YUNKA_ROOT" rev-parse HEAD)"
[[ "$actual_commit" == "$YUNKA_COMMIT" ]] || fail "expected Yunka $YUNKA_COMMIT, got $actual_commit"

if [[ -n "$(git -C "$YUNKA_ROOT" status --porcelain --untracked-files=normal)" ]]; then
  fail "locked Yunka checkout is dirty"
fi

check_module_path() {
  local rel="$1"
  local expected="$2"
  local actual
  actual="$(cd "$YUNKA_ROOT/$rel" && GOWORK=off go list -m -f '{{.Path}}')"
  [[ "$actual" == "$expected" ]] || fail "$rel declares $actual, expected $expected"
}

check_replace() {
  local module="$1"
  local rel="$2"
  local expected_version="$3"
  local version replace_dir resolved expected

  version="$(cd "$ROOT" && GOWORK=off go list -m -f '{{.Version}}' "$module")"
  [[ "$version" == "$expected_version" ]] || fail "$module version is $version, expected $expected_version"

  replace_dir="$(cd "$ROOT" && GOWORK=off go list -m -f '{{with .Replace}}{{.Dir}}{{end}}' "$module")"
  [[ -n "$replace_dir" ]] || fail "$module is not replaced with locked local source"
  if [[ "$replace_dir" = /* ]]; then
    resolved="$(cd "$replace_dir" && pwd -P)"
  else
    resolved="$(cd "$ROOT/$replace_dir" && pwd -P)"
  fi
  expected="$(cd "$YUNKA_ROOT/$rel" && pwd -P)"
  [[ "$resolved" == "$expected" ]] || fail "$module resolves to $resolved, expected $expected"
}

check_module_path framework yunka.io/framework
check_module_path gateway yunka.io/gateway
check_module_path pkg yunka.io/pkg
check_module_path compat/go-kit-kit-log github.com/go-kit/kit

check_replace yunka.io/framework framework "$YUNKA_PSEUDO_VERSION"
check_replace yunka.io/gateway gateway "$YUNKA_PSEUDO_VERSION"
check_replace yunka.io/pkg pkg "$YUNKA_PSEUDO_VERSION"
check_replace github.com/go-kit/kit compat/go-kit-kit-log v0.10.0

echo "yunka source check: locked at $YUNKA_COMMIT"
