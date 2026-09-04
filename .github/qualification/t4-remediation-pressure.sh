#!/usr/bin/env bash
set -euo pipefail

: "${YUNKA_BIN:?YUNKA_BIN is required}"
: "${ROOT:?ROOT is required}"
: "${EVIDENCE:?EVIDENCE is required}"
: "${GITHUB_SHA:?GITHUB_SHA is required}"

bad_rel="internal/deviceops/application/t4_remediation_pressure.go"
bad="$ROOT/$bad_rel"
contract="$RUNNER_TEMP/t4-remediation-contract.json"
mkdir -p "$EVIDENCE"

git -C "$ROOT" status --porcelain > "$EVIDENCE/before.status"
test ! -s "$EVIDENCE/before.status"
git -C "$ROOT" config user.email "t4-remediation@example.invalid"
git -C "$ROOT" config user.name "T4 Remediation Qualification"

cat > "$bad" <<'GO'
package application

import _ "yunka.io/framework/platform"
GO
git -C "$ROOT" add "$bad_rel"
git -C "$ROOT" commit -m "qualification: establish T4.3 audit pressure baseline"
base_sha="$(git -C "$ROOT" rev-parse HEAD)"
printf '%s\n' "$base_sha" > "$EVIDENCE/base-sha.txt"

"$YUNKA_BIN" audit --root "$ROOT" --format agent-json > "$EVIDENCE/baseline-audit.json"
finding_id="$(EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
report = json.loads((Path(os.environ['EVIDENCE']) / 'baseline-audit.json').read_text())
matches = [f['id'] for f in report['findings'] if f['rule'] == 'AUDIT-INFRA-001']
assert len(matches) == 1, matches
print(matches[0])
PY
)"
printf '%s\n' "$finding_id" > "$EVIDENCE/finding-id.txt"

"$YUNKA_BIN" change begin \
  --root "$ROOT" \
  --operation device.get \
  --intent implementation \
  --base HEAD \
  --path "$bad_rel" \
  --output "$contract" \
  --format agent-json > "$EVIDENCE/change-contract.json"

"$YUNKA_BIN" change set begin \
  --root "$ROOT" \
  --base HEAD \
  --contract "$contract" \
  --format agent-json > "$EVIDENCE/change-set-begin.json"

"$YUNKA_BIN" change set remediation bind \
  --root "$ROOT" \
  --finding "$finding_id" \
  --format agent-json > "$EVIDENCE/remediation-binding.json"

git -C "$ROOT" status --porcelain > "$EVIDENCE/after-bind.status"
test ! -s "$EVIDENCE/after-bind.status"

if "$YUNKA_BIN" change set remediation check \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/remaining.json"; then
  echo "unchanged remediation target unexpectedly conformed" >&2
  exit 1
fi
EVIDENCE="$EVIDENCE" FINDING_ID="$finding_id" python3 - <<'PY'
import json, os
from pathlib import Path
r = json.loads((Path(os.environ['EVIDENCE']) / 'remaining.json').read_text())
fid = os.environ['FINDING_ID']
assert r['changeSet']['conformant'] is True, r
assert r['conformant'] is False, r
assert r['audit']['fixed'] == [], r
assert r['audit']['remaining'] == [fid], r
assert r['audit']['newDebt'] == [], r
PY

cat > "$bad" <<'GO'
package application

import _ "yunka.io/gateway/authz"
GO
if "$YUNKA_BIN" change set remediation check \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/new-debt.json"; then
  echo "replacement with new debt unexpectedly conformed" >&2
  exit 1
fi
EVIDENCE="$EVIDENCE" FINDING_ID="$finding_id" python3 - <<'PY'
import json, os
from pathlib import Path
r = json.loads((Path(os.environ['EVIDENCE']) / 'new-debt.json').read_text())
fid = os.environ['FINDING_ID']
assert r['changeSet']['conformant'] is True, r
assert r['conformant'] is False, r
assert r['audit']['fixed'] == [fid], r
assert r['audit']['remaining'] == [], r
assert len(r['audit']['newDebt']) == 1, r
PY

git -C "$ROOT" reset --hard "$base_sha"
rm "$bad"
"$YUNKA_BIN" change set remediation check \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/fixed.json"
EVIDENCE="$EVIDENCE" FINDING_ID="$finding_id" python3 - <<'PY'
import json, os
from pathlib import Path
r = json.loads((Path(os.environ['EVIDENCE']) / 'fixed.json').read_text())
fid = os.environ['FINDING_ID']
assert r['changeSet']['conformant'] is True, r
assert r['audit']['conformant'] is True, r
assert r['conformant'] is True, r
assert r['audit']['fixed'] == [fid], r
assert r['audit']['remaining'] == [], r
assert r['audit']['newDebt'] == [], r
PY

git -C "$ROOT" reset --hard "$GITHUB_SHA"
git -C "$ROOT" clean -fdx
rm -rf "$ROOT/.git/yunka"
git -C "$ROOT" status --porcelain > "$EVIDENCE/final.status"
diff -u "$EVIDENCE/before.status" "$EVIDENCE/final.status"
