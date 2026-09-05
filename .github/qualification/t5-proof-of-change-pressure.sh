#!/usr/bin/env bash
set -euo pipefail

: "${YUNKA_BIN:?YUNKA_BIN is required}"
: "${YUNKA_ROOT:?YUNKA_ROOT is required}"
: "${ROOT:?ROOT is required}"
: "${EVIDENCE:?EVIDENCE is required}"
: "${GITHUB_SHA:?GITHUB_SHA is required}"

new_rel="internal/deviceops/application/t5_new_debt_pressure.go"
existing_rel="internal/deviceops/application/t5_existing_debt_pressure.go"
new_file="$ROOT/$new_rel"
existing_file="$ROOT/$existing_rel"
new_contract="$RUNNER_TEMP/t5-new-debt-contract.json"
existing_contract="$RUNNER_TEMP/t5-existing-debt-contract.json"
mkdir -p "$EVIDENCE"

git -C "$ROOT" status --porcelain > "$EVIDENCE/before.status"
test ! -s "$EVIDENCE/before.status"
git -C "$ROOT" config user.email "t5-proof@example.invalid"
git -C "$ROOT" config user.name "T5 Proof Qualification"

# Canonicalize the preserved B13 consumer through the exact #152 migration
# contract before testing T5. No generated file is hand-edited, no legacy alias
# modules are manufactured, and no test gate is skipped. The historical
# replace RHS paths already point at the sibling exact Yunka checkout, so after
# migration they bind canonical module identities to this exact T5 candidate.
if "$YUNKA_BIN" dependency module-identity inspect \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/pre-migration-inspect.json"; then
  echo "historical B13 unexpectedly has no legacy Yunka module identity" >&2
  exit 1
fi

git -C "$ROOT" status --porcelain > "$EVIDENCE/after-pre-migration-inspect.status"
diff -u "$EVIDENCE/before.status" "$EVIDENCE/after-pre-migration-inspect.status"

if "$YUNKA_BIN" check \
  --root "$ROOT" \
  --protoc "$(command -v protoc)" \
  --format agent-json > "$EVIDENCE/pre-migration-check.json"; then
  echo "historical mixed module identity unexpectedly passed yunka check" >&2
  exit 1
fi

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
item = json.loads((Path(os.environ['EVIDENCE']) / 'pre-migration-check.json').read_text())
assert item['ok'] is False, item
assert len(item.get('diagnostics', [])) == 1, item
assert item['diagnostics'][0]['code'] == 'YUNKA-DX-MODULE-002', item
PY

git -C "$ROOT" status --porcelain > "$EVIDENCE/after-pre-migration-check.status"
diff -u "$EVIDENCE/before.status" "$EVIDENCE/after-pre-migration-check.status"

"$YUNKA_BIN" dependency module-identity migrate \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/module-identity-migration.json"

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
report = json.loads((Path(os.environ['EVIDENCE']) / 'module-identity-migration.json').read_text())
assert report['schemaVersion'] == 1, report
assert report['conformant'] is True, report
assert report['before'], report
assert report['changedFiles'], report
assert report['after'] == [], report
assert 'contracts/third_party/yunka/dsl/v1/options.proto' in report['changedFiles'], report
PY

# This stale B13 compatibility replace is a separate historical consumer
# dependency cleanup already proven unrelated to #152/T5 semantics.
go -C "$ROOT" mod edit -dropreplace=github.com/go-kit/kit@v0.10.0

go -C "$ROOT" mod tidy > "$EVIDENCE/canonical-tidy.log" 2>&1

"$YUNKA_BIN" generate \
  --root "$ROOT" \
  --protoc "$(command -v protoc)" \
  --format agent-json > "$EVIDENCE/canonical-generate.json"

"$YUNKA_BIN" dependency module-identity inspect \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/canonical-module-identity.json"

"$YUNKA_BIN" check \
  --root "$ROOT" \
  --protoc "$(command -v protoc)" \
  --full \
  --format agent-json > "$EVIDENCE/canonical-check.json"

go -C "$ROOT" test ./... > "$EVIDENCE/canonical-go-test.log" 2>&1

"$YUNKA_BIN" audit \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/canonical-audit.json" \
  2> "$EVIDENCE/canonical-audit.stderr"

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
root = Path(os.environ['EVIDENCE'])
identity = json.loads((root / 'canonical-module-identity.json').read_text())
check = json.loads((root / 'canonical-check.json').read_text())
assert identity['conformant'] is True and identity['findings'] == [], identity
assert check['ok'] is True and check.get('diagnostics', []) == [], check
PY

# Freeze the canonicalized real-consumer state as the immutable T5 pressure
# base. It exists only inside this qualification run and is never pushed as Biz
# canonical state.
git -C "$ROOT" add -A
git -C "$ROOT" commit -m "qualification: canonicalize B13 for T5 proof pressure" \
  > "$EVIDENCE/canonical-commit.log" 2>&1
qualification_base_sha="$(git -C "$ROOT" rev-parse HEAD)"
printf '%s\n' "$qualification_base_sha" > "$EVIDENCE/qualification-base-sha.txt"
git -C "$ROOT" status --porcelain > "$EVIDENCE/canonical.status"
test ! -s "$EVIDENCE/canonical.status"

# Pressure A: a bounded handwritten addition that introduces a new proven
# architecture violation must fail final Change Attestation even though it is
# otherwise inside the declared Change Contract and the whole project compiles.
"$YUNKA_BIN" change begin \
  --root "$ROOT" \
  --operation device.get \
  --intent implementation \
  --base HEAD \
  --path "$new_rel" \
  --output "$new_contract" \
  --format agent-json > "$EVIDENCE/new-debt-contract.json"

cat > "$new_file" <<'GO'
package application

import _ "github.com/hvritual/yunka.io/gateway/authz"
GO

if "$YUNKA_BIN" change verify \
  --root "$ROOT" \
  --contract "$new_contract" \
  --protoc "$(command -v protoc)" \
  --output "$EVIDENCE/new-debt-attestation.json" \
  --format agent-json > "$EVIDENCE/new-debt-response.json"; then
  echo "new proven architecture debt unexpectedly conformed" >&2
  exit 1
fi

EVIDENCE="$EVIDENCE" NEW_REL="$new_rel" python3 - <<'PY'
import json, os
from pathlib import Path
root = Path(os.environ['EVIDENCE'])
new_rel = os.environ['NEW_REL']
a = json.loads((root / 'new-debt-attestation.json').read_text())
assert a['schemaVersion'] == 2, a
assert a['conformant'] is False, a
assert a['architectureDebt']['fixed'] == [], a
new = a['architectureDebt']['new']
assert len(new) == 1, a
assert new[0]['rule'] == 'AUDIT-AUTH-001', a
assert any(e.get('path') == new_rel for e in new[0].get('evidence', [])), new[0]
gates = {g['name']: g for g in a['gates']}
assert gates['git-delta']['status'] == 'pass', gates
assert gates['yunka-check']['status'] == 'pass', gates
assert gates['semantic-delta']['status'] == 'pass', gates
assert gates['architecture-debt']['status'] == 'fail', gates
assert gates['go-test']['status'] == 'pass', gates
PY

# Restore the canonical consumer before establishing an immutable bad baseline
# for existing/fixed debt classification.
git -C "$ROOT" reset --hard "$qualification_base_sha"
git -C "$ROOT" clean -fdx
rm -rf "$ROOT/.git/yunka"

cat > "$existing_file" <<'GO'
package application

import _ "github.com/hvritual/yunka.io/framework/platform"
GO
git -C "$ROOT" add "$existing_rel"
git -C "$ROOT" commit -m "qualification: establish T5 existing-debt baseline" \
  > "$EVIDENCE/existing-baseline-commit.log" 2>&1
base_sha="$(git -C "$ROOT" rev-parse HEAD)"
printf '%s\n' "$base_sha" > "$EVIDENCE/existing-base-sha.txt"

"$YUNKA_BIN" change begin \
  --root "$ROOT" \
  --operation device.get \
  --intent implementation \
  --base HEAD \
  --path "$existing_rel" \
  --output "$existing_contract" \
  --format agent-json > "$EVIDENCE/existing-debt-contract.json"

# Pressure B: unchanged proven debt that predates the Change Contract is
# reported as existing and is not promoted into newly blocking debt.
"$YUNKA_BIN" change verify \
  --root "$ROOT" \
  --contract "$existing_contract" \
  --protoc "$(command -v protoc)" \
  --output "$EVIDENCE/existing-debt-attestation.json" \
  --format agent-json > "$EVIDENCE/existing-debt-response.json"

EVIDENCE="$EVIDENCE" EXISTING_REL="$existing_rel" python3 - <<'PY'
import json, os
from pathlib import Path
root = Path(os.environ['EVIDENCE'])
existing_rel = os.environ['EXISTING_REL']
a = json.loads((root / 'existing-debt-attestation.json').read_text())
assert a['schemaVersion'] == 2, a
assert a['conformant'] is True, a
assert a['architectureDebt']['new'] == [], a
assert a['architectureDebt']['fixed'] == [], a
matches = [f for f in a['architectureDebt']['existing']
           if f['rule'] == 'AUDIT-INFRA-001'
           and any(e.get('path') == existing_rel for e in f.get('evidence', []))]
assert len(matches) == 1, a
gates = {g['name']: g for g in a['gates']}
assert gates['architecture-debt']['status'] == 'pass', gates
assert gates['go-test']['status'] == 'pass', gates
PY

# Pressure C: removing the baseline violation is reported as fixed and remains
# conformant when no replacement debt is introduced.
rm "$existing_file"
"$YUNKA_BIN" change verify \
  --root "$ROOT" \
  --contract "$existing_contract" \
  --protoc "$(command -v protoc)" \
  --output "$EVIDENCE/fixed-debt-attestation.json" \
  --format agent-json > "$EVIDENCE/fixed-debt-response.json"

EVIDENCE="$EVIDENCE" EXISTING_REL="$existing_rel" python3 - <<'PY'
import json, os
from pathlib import Path
root = Path(os.environ['EVIDENCE'])
existing_rel = os.environ['EXISTING_REL']
a = json.loads((root / 'fixed-debt-attestation.json').read_text())
assert a['schemaVersion'] == 2, a
assert a['conformant'] is True, a
assert a['architectureDebt']['new'] == [], a
matches = [f for f in a['architectureDebt']['fixed']
           if f['rule'] == 'AUDIT-INFRA-001'
           and any(e.get('path') == existing_rel for e in f.get('evidence', []))]
assert len(matches) == 1, a
gates = {g['name']: g for g in a['gates']}
assert gates['architecture-debt']['status'] == 'pass', gates
assert gates['go-test']['status'] == 'pass', gates
PY

# Re-run successful verification from an unchanged state. The attestation bytes
# themselves—not merely the logical counts—must be deterministic.
cp "$EVIDENCE/fixed-debt-attestation.json" "$EVIDENCE/fixed-debt-attestation.first.json"
"$YUNKA_BIN" change verify \
  --root "$ROOT" \
  --contract "$existing_contract" \
  --protoc "$(command -v protoc)" \
  --output "$EVIDENCE/fixed-debt-attestation.second.json" \
  --format agent-json > "$EVIDENCE/fixed-debt-response.second.json"
diff -u "$EVIDENCE/fixed-debt-attestation.first.json" "$EVIDENCE/fixed-debt-attestation.second.json"

# Qualification must leave the real consumer branch exactly as it arrived.
git -C "$ROOT" reset --hard "$GITHUB_SHA"
git -C "$ROOT" clean -fdx
rm -rf "$ROOT/.git/yunka"
git -C "$ROOT" status --porcelain > "$EVIDENCE/final.status"
diff -u "$EVIDENCE/before.status" "$EVIDENCE/final.status"
