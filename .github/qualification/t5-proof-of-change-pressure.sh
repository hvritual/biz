#!/usr/bin/env bash
set -euo pipefail

: "${YUNKA_BIN:?YUNKA_BIN is required}"
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

# Pressure A: a bounded handwritten addition that introduces a new proven
# architecture violation must fail final Change Attestation even though it is
# otherwise inside the declared Change Contract.
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

import _ "yunka.io/gateway/authz"
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

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
root = Path(os.environ['EVIDENCE'])
a = json.loads((root / 'new-debt-attestation.json').read_text())
assert a['schemaVersion'] == 2, a
assert a['conformant'] is False, a
assert a['architectureDebt']['existing'] == [], a
assert a['architectureDebt']['fixed'] == [], a
assert len(a['architectureDebt']['new']) == 1, a
assert a['architectureDebt']['new'][0]['rule'] == 'AUDIT-AUTH-001', a
gates = {g['name']: g for g in a['gates']}
assert gates['git-delta']['status'] == 'pass', gates
assert gates['yunka-check']['status'] == 'pass', gates
assert gates['semantic-delta']['status'] == 'pass', gates
assert gates['architecture-debt']['status'] == 'fail', gates
assert gates['go-test']['status'] == 'pass', gates
PY

# Restore the exact qualification branch before establishing an immutable bad
# baseline for existing/fixed debt classification.
git -C "$ROOT" reset --hard "$GITHUB_SHA"
git -C "$ROOT" clean -fdx
rm -rf "$ROOT/.git/yunka"

cat > "$existing_file" <<'GO'
package application

import _ "yunka.io/framework/platform"
GO
git -C "$ROOT" add "$existing_rel"
git -C "$ROOT" commit -m "qualification: establish T5 existing-debt baseline"
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

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
root = Path(os.environ['EVIDENCE'])
a = json.loads((root / 'existing-debt-attestation.json').read_text())
assert a['schemaVersion'] == 2, a
assert a['conformant'] is True, a
assert len(a['architectureDebt']['existing']) == 1, a
assert a['architectureDebt']['existing'][0]['rule'] == 'AUDIT-INFRA-001', a
assert a['architectureDebt']['new'] == [], a
assert a['architectureDebt']['fixed'] == [], a
gates = {g['name']: g for g in a['gates']}
assert gates['architecture-debt']['status'] == 'pass', gates
assert 'existing=1' in gates['architecture-debt'].get('detail', ''), gates
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

EVIDENCE="$EVIDENCE" python3 - <<'PY'
import json, os
from pathlib import Path
root = Path(os.environ['EVIDENCE'])
a = json.loads((root / 'fixed-debt-attestation.json').read_text())
assert a['schemaVersion'] == 2, a
assert a['conformant'] is True, a
assert a['architectureDebt']['existing'] == [], a
assert a['architectureDebt']['new'] == [], a
assert len(a['architectureDebt']['fixed']) == 1, a
assert a['architectureDebt']['fixed'][0]['rule'] == 'AUDIT-INFRA-001', a
gates = {g['name']: g for g in a['gates']}
assert gates['architecture-debt']['status'] == 'pass', gates
assert 'fixed=1' in gates['architecture-debt'].get('detail', ''), gates
assert gates['go-test']['status'] == 'pass', gates
PY

# Re-render the successful attestation contract from an unchanged state to
# prove deterministic machine output for the architecture-debt payload.
cp "$EVIDENCE/fixed-debt-attestation.json" "$EVIDENCE/fixed-debt-attestation.first.json"
"$YUNKA_BIN" change verify \
  --root "$ROOT" \
  --contract "$existing_contract" \
  --protoc "$(command -v protoc)" \
  --output "$EVIDENCE/fixed-debt-attestation.second.json" \
  --format agent-json > "$EVIDENCE/fixed-debt-response.second.json"
diff -u "$EVIDENCE/fixed-debt-attestation.first.json" "$EVIDENCE/fixed-debt-attestation.second.json"

git -C "$ROOT" reset --hard "$GITHUB_SHA"
git -C "$ROOT" clean -fdx
rm -rf "$ROOT/.git/yunka"
git -C "$ROOT" status --porcelain > "$EVIDENCE/final.status"
diff -u "$EVIDENCE/before.status" "$EVIDENCE/final.status"
