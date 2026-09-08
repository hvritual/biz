#!/usr/bin/env bash
# Linux qualification of real consumer import boundaries. Uses overlays: tracked
# sources are never edited, and downloads are disabled after normal build setup.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
command -v timeout >/dev/null || { echo 'AG02 INCOMPLETE: GNU timeout is required' >&2; exit 1; }
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
export GOWORK=off GOTOOLCHAIN=local GOFLAGS= GOPROXY=off GOSUMDB=off CGO_ENABLED=0
for probe in control hidden-import aliased-hidden-import; do
  if [[ "$probe" = control ]]; then
    cat > "$work/main.go" <<'GO'
package main
import owner "github.com/hvritual/biz/internal/access/application/tenantlifecycle"
func main() { _ = owner.Build }
GO
  else
    alias='_'
    [[ "$probe" != aliased-hidden-import ]] || alias='renamed'
    printf 'package main\nimport %s "github.com/hvritual/biz/internal/access/application/tenantlifecycle/internal/usecase"\nfunc main() { ' "$alias" > "$work/main.go"
    [[ "$alias" = _ ]] || printf '_ = renamed.New; ' >> "$work/main.go"
    printf '}\n' >> "$work/main.go"
  fi
  python3 - "$root" "$work" <<'PY'
import json,sys,pathlib
root,work=map(pathlib.Path,sys.argv[1:])
(work/'overlay.json').write_text(json.dumps({'Replace':{str(root/'cmd/biz/main.go'):str(work/'main.go')}}))
PY
  code=0
  timeout -k 5s 90s go build -overlay="$work/overlay.json" -o "$work/probe" ./cmd/biz > "$work/$probe.log" 2>&1 || code=$?
  python3 - "$probe" "$code" "$work/$probe.log" <<'PY'
import sys,pathlib,re
name,code,path=sys.argv[1:];code=int(code);out=pathlib.Path(path).read_text()
if name=='control':
    assert code==0, f'AG02 INCOMPLETE: legal control failed: {out}'
else:
    assert code==1, f'AG02 unexpected process exit {code}: {out}'
    lines=[x.strip() for x in out.splitlines() if x.strip()]
    diagnostics=[x for x in lines if re.search(r'\.go:\d+:\d+:',x)]
    assert len(diagnostics)==1, f'AG02 unexpected diagnostics: {out}'
    expected='use of internal package github.com/hvritual/biz/internal/access/application/tenantlifecycle/internal/usecase not allowed'
    assert diagnostics[0].endswith(expected), f'AG02 wrong rejection: {out}'
    assert all(x in diagnostics or x.startswith(('package github.com/hvritual/biz/','# github.com/hvritual/biz/','imports github.com/hvritual/biz/')) for x in lines), f'AG02 unexpected infrastructure output: {out}'
print(f'AG02 {name}: PASS')
PY
  if [[ -n "${BIZ_AG02_EVIDENCE:-}" ]]; then
    mkdir -p "$BIZ_AG02_EVIDENCE"
    cp "$work/$probe.log" "$BIZ_AG02_EVIDENCE/$probe.log"
  fi
done
