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

mod_json="$(cd "$ROOT" && GOWORK=off go mod edit -json)"
work_json="$(cd "$ROOT" && go work edit -json)"

required_version() {
  local module="$1"
  printf '%s\n' "$mod_json" | MODULE="$module" python3 -c '
import json, os, sys
m = os.environ["MODULE"]
data = json.load(sys.stdin)
for req in data.get("Require") or []:
    if req.get("Path") == m:
        print(req.get("Version", ""))
        raise SystemExit(0)
raise SystemExit(1)
'
}

mod_has_replace() {
  local module="$1"
  printf '%s\n' "$mod_json" | MODULE="$module" python3 -c '
import json, os, sys
m = os.environ["MODULE"]
data = json.load(sys.stdin)
for rep in data.get("Replace") or []:
    if (rep.get("Old") or {}).get("Path") == m:
        raise SystemExit(0)
raise SystemExit(1)
'
}

work_replace_path() {
  local module="$1"
  printf '%s\n' "$work_json" | MODULE="$module" python3 -c '
import json, os, sys
m = os.environ["MODULE"]
data = json.load(sys.stdin)
for rep in data.get("Replace") or []:
    if (rep.get("Old") or {}).get("Path") == m:
        print((rep.get("New") or {}).get("Path", ""))
        raise SystemExit(0)
raise SystemExit(1)
'
}

check_release_contract() {
  local module="$1"
  local expected_version="$2"
  local actual_version
  actual_version="$(required_version "$module")" || fail "$module is not required by go.mod"
  [[ "$actual_version" == "$expected_version" ]] || fail "$module version is $actual_version, expected $expected_version"
  if mod_has_replace "$module"; then
    fail "$module must not be replaced in go.mod; local source belongs in go.work"
  fi
}

check_workspace_replace() {
  local module="$1"
  local rel="$2"
  local replace_path resolved expected
  replace_path="$(work_replace_path "$module")" || fail "$module is not replaced by go.work"
  [[ -n "$replace_path" ]] || fail "$module go.work replacement has no path"
  if [[ "$replace_path" = /* ]]; then
    resolved="$(cd "$replace_path" && pwd -P)"
  else
    resolved="$(cd "$ROOT/$replace_path" && pwd -P)"
  fi
  expected="$(cd "$YUNKA_ROOT/$rel" && pwd -P)"
  [[ "$resolved" == "$expected" ]] || fail "$module workspace source is $resolved, expected $expected"
}

check_module_path framework yunka.io/framework
check_module_path gateway yunka.io/gateway
check_module_path pkg yunka.io/pkg
check_module_path compat/go-kit-kit-log github.com/go-kit/kit

check_release_contract yunka.io/framework "$YUNKA_PSEUDO_VERSION"
check_release_contract yunka.io/gateway "$YUNKA_PSEUDO_VERSION"
check_release_contract yunka.io/pkg "$YUNKA_PSEUDO_VERSION"
check_release_contract github.com/go-kit/kit v0.10.0

check_workspace_replace yunka.io/framework framework
check_workspace_replace yunka.io/gateway gateway
check_workspace_replace yunka.io/pkg pkg
check_workspace_replace github.com/go-kit/kit compat/go-kit-kit-log

echo "yunka source check: local workspace source locked at $YUNKA_COMMIT; go.mod remains release-only"
