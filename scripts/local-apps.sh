#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
credentials_file=${YUNKA_BIZ_LOCAL_CREDENTIALS_FILE:-"$root/.local/biz-local-credentials.json"}
state_dir="$root/.local"
bin_dir="$state_dir/bin"
export GOTOOLCHAIN=${GOTOOLCHAIN:-go1.25.13}
if [[ -d /tmp/biz-branch-audit/toolchain/bin ]]; then export PATH=/tmp/biz-branch-audit/toolchain/bin:$PATH; fi

die() { echo "local-apps: $*" >&2; exit 1; }

if ! command -v jq >/dev/null 2>&1; then die "jq is required to read the mode-0600 local credential file"; fi
[[ -f "$credentials_file" ]] || die "run go run ./cmd/biz-local-setup first"
[[ -f "$root/.env.local" ]] || die "missing ignored .env.local"
mkdir -p "$state_dir" "$bin_dir"
chmod 700 "$state_dir" "$bin_dir"
set -a
source "$root/.env.local"
set +a

case "${YUNKA_BIZ_MYSQL_DSN:-}" in
  *'tcp(127.0.0.1:13316)'*'/biz_evolution'*) ;;
  *) die "the designated biz_evolution DSN at 127.0.0.1:13316 is required" ;;
esac
worker_token=$(jq -r '.worker_token' "$credentials_file")
[[ -n "$worker_token" && "$worker_token" != null ]] || die "credential file is incomplete"

key_file="$state_dir/biz-idp-signing-key.pem"
if [[ ! -f "$key_file" ]]; then umask 077; openssl genrsa -out "$key_file" 2048 >/dev/null 2>&1; fi
chmod 600 "$key_file"

owned() { local pid=$1 expected=$2 command; kill -0 "$pid" 2>/dev/null || return 1; command=$(ps -p "$pid" -o command= 2>/dev/null || true); [[ "$command" == *"$expected"* ]]; }
stop_one() {
  local name=$1 expected=$2 pid_file="$state_dir/$name.pid" pid
  [[ -f "$pid_file" ]] || return 0
  pid=$(<"$pid_file")
  if kill -0 "$pid" 2>/dev/null; then owned "$pid" "$expected" || die "refusing to kill stale $name PID $pid"; kill "$pid"; fi
  rm -f "$pid_file"
}
cleanup_start() { set +e; stop_one web "node_modules/vite/bin/vite.js"; stop_one biz "$bin_dir/biz"; stop_one idp "$bin_dir/biz-idp"; }
wait_http() {
  local url=$1 name=$2 attempt
  for attempt in {1..60}; do if curl --fail --silent --show-error "$url" >/dev/null 2>&1; then return 0; fi; sleep 1; done
  echo "local-apps: $name did not become ready; inspect $state_dir/$name.log" >&2
  return 1
}

case "${1:-start}" in
  start)
    [[ ! -e "$state_dir/biz.pid" && ! -e "$state_dir/idp.pid" && ! -e "$state_dir/web.pid" ]] || die "a pid file exists; run scripts/local-apps.sh stop first"
    trap cleanup_start ERR INT TERM
    (cd "$root"; go build -o "$bin_dir/biz" ./cmd/biz; go build -o "$bin_dir/biz-idp" ./cmd/biz-idp)
    YUNKA_BIZ_IDP_PUBLIC_URL=http://127.0.0.1:18381 YUNKA_BIZ_IDP_LISTEN=127.0.0.1:18381 YUNKA_BIZ_IDP_REDIRECT_URL=http://127.0.0.1:14183/auth/callback YUNKA_BIZ_IDP_POST_LOGOUT_REDIRECT_URL=http://127.0.0.1:14183/ YUNKA_BIZ_IDP_SIGNING_KEY_FILE="$key_file" YUNKA_BIZ_IDP_AUTO_MIGRATE=true YUNKA_BIZ_IDP_COOKIE_SECURE=false "$bin_dir/biz-idp" >"$state_dir/idp.log" 2>&1 & echo $! >"$state_dir/idp.pid"
    wait_http http://127.0.0.1:18381/idp/.well-known/openid-configuration idp
    YUNKA_BIZ_HTTP_LISTEN=127.0.0.1:18380 YUNKA_BIZ_GRPC_LISTEN=127.0.0.1:18382 YUNKA_BIZ_AUTO_MIGRATE=true YUNKA_BIZ_OIDC_ISSUER=http://127.0.0.1:18381/idp YUNKA_BIZ_OIDC_CLIENT_ID=biz-web YUNKA_BIZ_OIDC_REDIRECT_URL=http://127.0.0.1:14183/auth/callback YUNKA_BIZ_OIDC_POST_LOGOUT_REDIRECT_URL=http://127.0.0.1:14183/ YUNKA_BIZ_OIDC_COOKIE_SECURE=false YUNKA_BIZ_OIDC_PLATFORM_EXTERNAL_SUBJECT=biz-user:local-platform-admin YUNKA_BIZ_OIDC_PLATFORM_SUBJECT=local-platform-admin YUNKA_BIZ_OIDC_PLATFORM_EMAIL=platform-admin@local.biz.invalid YUNKA_BIZ_PROVISIONING_WORKER_TOKEN="$worker_token" "$bin_dir/biz" >"$state_dir/biz.log" 2>&1 & echo $! >"$state_dir/biz.pid"
    wait_http http://127.0.0.1:18380/healthz biz
    (cd "$root/web"; BIZ_DEV_API_TARGET=http://127.0.0.1:18380 VITE_DATA_MODE=api node ./node_modules/vite/bin/vite.js --host 127.0.0.1 --port 14183 >"$state_dir/web.log" 2>&1 & echo $! >"$state_dir/web.pid")
    wait_http http://127.0.0.1:14183/ web
    trap - ERR INT TERM
    echo "local apps ready: web http://127.0.0.1:14183, Biz http://127.0.0.1:18380, IdP http://127.0.0.1:18381"
    ;;
  stop)
    stop_one web "node_modules/vite/bin/vite.js"
    stop_one biz "$bin_dir/biz"
    stop_one idp "$bin_dir/biz-idp"
    ;;
  *) die "usage: scripts/local-apps.sh [start|stop]" ;;
esac
