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
legacy_root="$RUNNER_TEMP/yunka-legacy"
mkdir -p "$EVIDENCE"

git -C "$ROOT" status --porcelain > "$EVIDENCE/before.status"
test ! -s "$EVIDENCE/before.status"
git -C "$ROOT" config user.email "t5-proof@example.invalid"
git -C "$ROOT" config user.name "T5 Proof Qualification"

# The preserved B13 pressure tree uses the historical yunka.io module identity,
# while the current exact candidate modules declare github.com/hvritual/yunka.io.
# Do not rewrite Biz or generated source to hide that fact. Instead create
# qualification-only legacy aliases by copying the exact candidate module source
# and changing only module/import identity strings inside the copies. The
# original candidate checkout remains untouched and is also available under its
# canonical GitHub module identity. If this dual-identity compatibility setup
# cannot compile the candidate-generated consumer, that is evidence of a real
# module-identity/compiler compatibility gap rather than a T5 harness problem.
rm -rf "$legacy_root"
mkdir -p "$legacy_root"
for module in framework gateway pkg; do
  cp -a "$YUNKA_ROOT/$module" "$legacy_root/$module"
done
LEGACY_ROOT="$legacy_root" python3 - <<'PY' > "$EVIDENCE/legacy-alias.log" 2>&1
import os
from pathlib import Path
root = Path(os.environ['LEGACY_ROOT'])
replacements = {
    'github.com/hvritual/yunka.io/framework': 'yunka.io/framework',
    'github.com/hvritual/yunka.io/gateway': 'yunka.io/gateway',
    'github.com/hvritual/yunka.io/pkg': 'yunka.io/pkg',
}
changed = []
for path in sorted(root.rglob('*')):
    if not path.is_file() or '.git' in path.parts:
        continue
    try:
        data = path.read_text()
    except UnicodeDecodeError:
        continue
    updated = data
    for old, new in replacements.items():
        updated = updated.replace(old, new)
    if updated != data:
        path.write_text(updated)
        changed.append(str(path.relative_to(root)))
print('\n'.join(changed))
PY

# Bind historical imports to mechanically derived legacy aliases and any
# candidate-generated GitHub imports to the untouched exact candidate modules.
# The stale historical go-kit compatibility replace no longer exists on the
# current candidate and is removed only in the ephemeral qualification baseline.
{
  go -C "$ROOT" mod edit -dropreplace=github.com/go-kit/kit@v0.10.0
  go -C "$ROOT" mod edit -replace=yunka.io/framework="$legacy_root/framework"
  go -C "$ROOT" mod edit -replace=yunka.io/gateway="$legacy_root/gateway"
  go -C "$ROOT" mod edit -replace=yunka.io/pkg="$legacy_root/pkg"
  go -C "$ROOT" mod edit -require=github.com/hvritual/yunka.io/framework@v0.1.0
  go -C "$ROOT" mod edit -require=github.com/hvritual/yunka.io/gateway@v0.1.0
  go -C "$ROOT" mod edit -require=github.com/hvritual/yunka.io/pkg@v0.1.0
  go -C "$ROOT" mod edit -replace=github.com/hvritual/yunka.io/framework="$YUNKA_ROOT/framework"
  go -C "$ROOT" mod edit -replace=github.com/hvritual/yunka.io/gateway="$YUNKA_ROOT/gateway"
  go -C "$ROOT" mod edit -replace=github.com/hvritual/yunka.io/pkg="$YUNKA_ROOT/pkg"
} > "$EVIDENCE/canonical-mod-edit.log" 2>&1

"$YUNKA_BIN" generate \
  --root "$ROOT" \
  --protoc "$(command -v protoc)" \
  > "$EVIDENCE/canonical-generate.log" 2>&1

grep -RhoE '"(yunka\.io|github\.com/hvritual/yunka\.io)/(framework|gateway|pkg)[^"]*"' \
  "$ROOT" --include='*.go' | sort -u > "$EVIDENCE/yunka-import-identities.txt" || true

go -C "$ROOT" mod tidy \
  > "$EVIDENCE/canonical-tidy.log" 2>&1
"$YUNKA_BIN" check \
  --root "$ROOT" \
  --protoc "$(command -v protoc)" \
  --format agent-json > "$EVIDENCE/canonical-check.json" \
  2> "$EVIDENCE/canonical-check.stderr"
go -C "$ROOT" test ./... \
  > "$EVIDENCE/canonical-go-test.log" 2>&1
"$YUNKA_BIN" audit \
  --root "$ROOT" \
  --format agent-json > "$EVIDENCE/canonical-audit.json" \
  2> "$EVIDENCE/canonical-audit.stderr"

git -C "$ROOT" add -A
git -C "$ROOT" commit -m "qualification: canonicalize B13 for T5 proof pressure" \
  > "$EVIDENCE/canonical-commit.log" 2>&1
qualification_base_sha="$(git -C "$ROOT" rev-parse HEAD)"
printf '%s\n' "$qualification_base_sha" > "$EVIDENCE/qualification-base-sha.txt"
git -C "$ROOT" status --porcelain > "$EVIDENCE/canonical.status"
test ! -s "$EVIDENCE/canonical.status"

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

# Restore the candidate-canonical consumer before establishing an immutable bad
# baseline for existing/fixed debt classification.
git -C "$ROOT" reset --hard "$qualification_base_sha"
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

# Re-render the successful attestation from an unchanged state to prove
# deterministic machine output for the architecture-debt payload.
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
