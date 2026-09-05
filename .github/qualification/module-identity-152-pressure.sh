#!/usr/bin/env bash
set -euo pipefail

: "${YUNKA_BIN:?YUNKA_BIN is required}"
: "${ROOT:?ROOT is required}"
: "${EVIDENCE:?EVIDENCE is required}"
: "${GITHUB_SHA:?GITHUB_SHA is required}"

mkdir -p "$EVIDENCE"
git -C "$ROOT" status --porcelain > "$EVIDENCE/before.status"
test ! -s "$EVIDENCE/before.status"

# A real mixed-identity B13 consumer must be rejected read-only before any
# generated mutation. This specifically proves the T5-discovered state can no
# longer pass `yunka check` and fail only later in `go test`.
if "$YUNKA_BIN" dependency module-identity inspect \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/before-inspect.json"; then
  echo "historical B13 unexpectedly has no legacy Yunka module identity" >&2
  exit 1
fi

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
report = json.loads((Path(os.environ['EVIDENCE']) / 'before-inspect.json').read_text())
assert report['schemaVersion'] == 1, report
assert report['conformant'] is False, report
assert report['findings'], report
legacy = {f['legacy'].split('/')[0] + '/' + f['legacy'].split('/')[1] for f in report['findings'] if f['legacy'].startswith('yunka.io/')}
assert {'yunka.io/framework', 'yunka.io/gateway', 'yunka.io/pkg'}.issubset(legacy), (legacy, report)
PY

git -C "$ROOT" status --porcelain > "$EVIDENCE/after-inspect.status"
diff -u "$EVIDENCE/before.status" "$EVIDENCE/after-inspect.status"

if "$YUNKA_BIN" check \
  --root "$ROOT" \
  --protoc "$(command -v protoc)" \
  --format agent-json > "$EVIDENCE/before-check.json"; then
  echo "mixed Yunka module identities unexpectedly passed yunka check" >&2
  exit 1
fi

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
envelope = json.loads((Path(os.environ['EVIDENCE']) / 'before-check.json').read_text())
assert envelope['schemaVersion'] == 1, envelope
assert envelope['command'] == 'yunka check', envelope
assert envelope['ok'] is False, envelope
assert len(envelope.get('diagnostics', [])) == 1, envelope
item = envelope['diagnostics'][0]
assert item['code'] == 'YUNKA-DX-MODULE-002', item
assert item['stage'] == 'module-identity', item
assert item['cause']['summary'], item
assert item['target']['path'], item
values = {a['value'] for a in item.get('remediation', [])}
assert 'yunka dependency module-identity inspect' in values, item
assert 'yunka dependency module-identity migrate' in values, item
assert item['retry']['value'] == 'yunka check', item
PY

git -C "$ROOT" status --porcelain > "$EVIDENCE/after-failed-check.status"
diff -u "$EVIDENCE/before.status" "$EVIDENCE/after-failed-check.status"

# Explicit project-wide migration is the only mutation step. It rewrites Go
# import specs plus go.mod/go.work module tokens; ordinary descriptor/string
# literals are intentionally outside its authority.
"$YUNKA_BIN" dependency module-identity migrate \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/migration.json"

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
report = json.loads((Path(os.environ['EVIDENCE']) / 'migration.json').read_text())
assert report['schemaVersion'] == 1, report
assert report['conformant'] is True, report
assert report['before'], report
assert report['changedFiles'], report
assert report['after'] == [], report
PY

"$YUNKA_BIN" dependency module-identity inspect \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/after-migrate-inspect.json"

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
report = json.loads((Path(os.environ['EVIDENCE']) / 'after-migrate-inspect.json').read_text())
assert report['conformant'] is True, report
assert report['findings'] == [], report
PY

# B13 also carries an unrelated historical local replace for a compatibility
# module that no longer exists in current Yunka. Keep this consumer dependency
# cleanup explicit and separate from module-identity migration semantics.
go -C "$ROOT" mod edit -dropreplace=github.com/go-kit/kit@v0.10.0

go -C "$ROOT" mod tidy > "$EVIDENCE/go-mod-tidy.log" 2>&1

"$YUNKA_BIN" generate \
  --root "$ROOT" \
  --protoc "$(command -v protoc)" \
  --format agent-json > "$EVIDENCE/generate.json"

"$YUNKA_BIN" dependency module-identity inspect \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/post-generate-inspect.json"

"$YUNKA_BIN" check \
  --root "$ROOT" \
  --protoc "$(command -v protoc)" \
  --full \
  --format agent-json > "$EVIDENCE/check.json"

go -C "$ROOT" test ./... > "$EVIDENCE/go-test.log" 2>&1

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
root = Path(os.environ['EVIDENCE'])
inspect = json.loads((root / 'post-generate-inspect.json').read_text())
check = json.loads((root / 'check.json').read_text())
assert inspect['conformant'] is True and inspect['findings'] == [], inspect
assert check['ok'] is True and check.get('diagnostics', []) == [], check
PY

# Exact same candidate generation must be byte-stable after migration.
git -C "$ROOT" diff --binary > "$EVIDENCE/before-second-generate.diff"
"$YUNKA_BIN" generate \
  --root "$ROOT" \
  --protoc "$(command -v protoc)" \
  --format agent-json > "$EVIDENCE/generate-second.json"
git -C "$ROOT" diff --binary > "$EVIDENCE/after-second-generate.diff"
diff -u "$EVIDENCE/before-second-generate.diff" "$EVIDENCE/after-second-generate.diff"

# Full check remains read-only after migration.
git -C "$ROOT" diff --binary > "$EVIDENCE/before-readonly-check.diff"
"$YUNKA_BIN" check \
  --root "$ROOT" \
  --protoc "$(command -v protoc)" \
  --full \
  --format agent-json > "$EVIDENCE/check-second.json"
git -C "$ROOT" diff --binary > "$EVIDENCE/after-readonly-check.diff"
diff -u "$EVIDENCE/before-readonly-check.diff" "$EVIDENCE/after-readonly-check.diff"

# Preserve the effective migrated consumer baseline identity as evidence without
# publishing it as canonical Biz state.
git -C "$ROOT" config user.email "module-identity-qualification@example.invalid"
git -C "$ROOT" config user.name "Yunka Module Identity Qualification"
git -C "$ROOT" add -A
git -C "$ROOT" commit -m "qualification: canonicalize B13 Yunka module identity" \
  > "$EVIDENCE/consumer-baseline-commit.log" 2>&1
consumer_sha="$(git -C "$ROOT" rev-parse HEAD)"
printf '%s\n' "$consumer_sha" > "$EVIDENCE/consumer-baseline-sha.txt"
git -C "$ROOT" status --porcelain > "$EVIDENCE/migrated.status"
test ! -s "$EVIDENCE/migrated.status"

# Restore exact qualification branch so the workflow itself leaves no consumer
# mutation behind.
git -C "$ROOT" reset --hard "$GITHUB_SHA"
git -C "$ROOT" clean -fdx
git -C "$ROOT" status --porcelain > "$EVIDENCE/final.status"
diff -u "$EVIDENCE/before.status" "$EVIDENCE/final.status"
