#!/usr/bin/env bash
# Runs inside scripts/qualify-evolution-mysql.py's backup/reset/restore envelope.
set -euo pipefail

task_root="$(cd "$(dirname "$0")/.." && pwd)"
run_root="$(mktemp -d /tmp/biz-ce13-browser.XXXXXX)"
fixture="$run_root/fixture.json"
idp_key="$run_root/idp-signing.pem"
idp_pid=""
biz_pid=""
web_pid=""
result_report="${EVOLUTION_AFTER_RESET_RESULT_JSON:?EVOLUTION_AFTER_RESET_RESULT_JSON is required}"

cleanup() {
  test -z "$biz_pid" || kill "$biz_pid" 2>/dev/null || true
  test -z "$idp_pid" || kill "$idp_pid" 2>/dev/null || true
  test -z "$web_pid" || kill "$web_pid" 2>/dev/null || true
  mkdir -p "${EVOLUTION_EVIDENCE_DIR:?}/ce13-browser"
  cp "$run_root/idp.log" "${EVOLUTION_EVIDENCE_DIR}/ce13-browser/" 2>/dev/null || true
  cp "$run_root/biz.log" "${EVOLUTION_EVIDENCE_DIR}/ce13-browser/" 2>/dev/null || true
  cp "$run_root/web.log" "${EVOLUTION_EVIDENCE_DIR}/ce13-browser/" 2>/dev/null || true
  mkdir -p "$(dirname "$result_report")"
  cp "$task_root/web/test-results/ce13-platform-results.json" "$result_report" 2>/dev/null || true
  find "$task_root/web/test-results" -name '*.png' -maxdepth 3 -exec cp {} "${EVOLUTION_EVIDENCE_DIR}/ce13-browser/" \; 2>/dev/null || true
  wait "$biz_pid" 2>/dev/null || true
  wait "$idp_pid" 2>/dev/null || true
  wait "$web_pid" 2>/dev/null || true
  for attempt in $(seq 1 20); do
    if ! lsof -nP -iTCP:14183 -sTCP:LISTEN >/dev/null 2>&1; then break; fi
    sleep 1
  done
  ! lsof -nP -iTCP:14183 -sTCP:LISTEN >/dev/null 2>&1
  rm -rf "$run_root"
}
trap cleanup EXIT

export CE13_PLATFORM_E2E_ENV_FILE="$fixture"
export CE13_BIZ_BASE_URL="http://127.0.0.1:18380"
export CE13_IDP_ISSUER="http://127.0.0.1:18381/idp"
export CE13_WEB_BASE_URL="http://127.0.0.1:14183"

go test -count=1 -tags=integration ./integration -run '^TestCE13PlatformWebSessionSeed$'
test -s "$fixture"
chmod 600 "$fixture"

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$idp_key" >/dev/null 2>&1
chmod 600 "$idp_key"

export YUNKA_BIZ_IDP_PUBLIC_URL="http://127.0.0.1:18381"
export YUNKA_BIZ_IDP_REDIRECT_URL="http://127.0.0.1:14183/auth/callback"
export YUNKA_BIZ_IDP_POST_LOGOUT_REDIRECT_URL="http://127.0.0.1:14183/"
export YUNKA_BIZ_IDP_CLIENT_ID="biz-web"
export YUNKA_BIZ_IDP_SIGNING_KEY_FILE="$idp_key"
export YUNKA_BIZ_IDP_COOKIE_SECURE="false"
export YUNKA_BIZ_IDP_LISTEN="127.0.0.1:18381"
export YUNKA_BIZ_IDP_AUTO_MIGRATE="true"
go build -o "$run_root/biz-idp" ./cmd/biz-idp
"$run_root/biz-idp" >"$run_root/idp.log" 2>&1 &
idp_pid="$!"
for attempt in $(seq 1 60); do
  curl -fsS http://127.0.0.1:18381/healthz >/dev/null && break
  sleep 1
done
curl -fsS http://127.0.0.1:18381/healthz >/dev/null

export YUNKA_BIZ_HTTP_LISTEN="127.0.0.1:18380"
export YUNKA_BIZ_GRPC_LISTEN="127.0.0.1:18382"
export YUNKA_BIZ_AUTO_MIGRATE="true"
export YUNKA_BIZ_OIDC_ISSUER="http://127.0.0.1:18381/idp"
export YUNKA_BIZ_OIDC_CLIENT_ID="biz-web"
export YUNKA_BIZ_OIDC_REDIRECT_URL="http://127.0.0.1:14183/auth/callback"
export YUNKA_BIZ_OIDC_POST_LOGOUT_REDIRECT_URL="http://127.0.0.1:14183/"
export YUNKA_BIZ_OIDC_COOKIE_SECURE="false"
export YUNKA_BIZ_OIDC_SCOPES="openid profile email"
go build -o "$run_root/biz" ./cmd/biz
"$run_root/biz" >"$run_root/biz.log" 2>&1 &
biz_pid="$!"
for attempt in $(seq 1 60); do
  curl -fsS http://127.0.0.1:18380/healthz >/dev/null && break
  sleep 1
done
curl -fsS http://127.0.0.1:18380/healthz >/dev/null

cd "$task_root/web"
VITE_DATA_MODE=api BIZ_DEV_API_TARGET="http://127.0.0.1:18380" node "$task_root/web/node_modules/vite/bin/vite.js" --host 127.0.0.1 --port 14183 >"$run_root/web.log" 2>&1 &
web_pid="$!"
for attempt in $(seq 1 60); do
  curl -fsS http://127.0.0.1:14183/ >/dev/null && break
  sleep 1
done
curl -fsS http://127.0.0.1:14183/ >/dev/null
npx playwright test --config playwright.ce13-platform.config.ts
python3 - <<'PY'
import json
from pathlib import Path
result = json.loads(Path('test-results/ce13-platform-results.json').read_text())
stats = result['stats']
assert stats.get('unexpected', 0) == 0, stats
assert stats.get('expected', 0) >= 3, stats
payload = json.dumps(result)
assert 'TestCE13PlatformCommercialTrustedWebSession' in payload
assert 'TestCE13PlatformCommercialLifecycleThroughTrustedWebSession' in payload
assert 'TestCE13PlatformCommercialVisibleConsoleFlow' in payload
print('CE13_PLATFORM_BROWSER_LIFECYCLE=PASS')
PY
