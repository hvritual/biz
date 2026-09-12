#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
credentials_file=${YUNKA_BIZ_LOCAL_CREDENTIALS_FILE:-"$root/.local/biz-local-credentials.json"}
state_dir="$root/.local"
bin_dir="$state_dir/bin"
vite_entry="$root/web/node_modules/vite/bin/vite.js"
export GOTOOLCHAIN=${GOTOOLCHAIN:-go1.25.13}
if [[ -d /tmp/biz-branch-audit/toolchain/bin ]]; then export PATH=/tmp/biz-branch-audit/toolchain/bin:$PATH; fi

die() { echo "local-apps: $*" >&2; exit 1; }
owned() {
  local pid=$1 expected=$2 port=$3 command
  kill -0 "$pid" 2>/dev/null || return 1
  command=$(ps -p "$pid" -o command= 2>/dev/null || true)
  [[ "$command" == *"$expected"* ]] || return 1
  [[ -z "$port" ]] || lsof -nP -a -p "$pid" -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
}
stop_one() {
  local name expected port pid_file pid
  name=$1
  expected=$2
  port=$3
  pid_file="$state_dir/$name.pid"
  [[ -f "$pid_file" ]] || return 0
  pid=$(<"$pid_file")
  if kill -0 "$pid" 2>/dev/null; then owned "$pid" "$expected" "$port" || die "refusing to kill stale $name PID $pid"; kill "$pid"; fi
  rm -f "$pid_file"
}
cleanup_start() { set +e; stop_one web "$vite_entry" 14183; stop_one biz "$bin_dir/biz" 18380; stop_one idp "$bin_dir/biz-idp" 18381; }
start_process() {
  local name=$1 log_file=$2 pid_file
  shift 2
  pid_file="$state_dir/$name.pid"
  python3 - "$pid_file" "$log_file" "$@" <<'PY'
import os
import signal
import subprocess
import sys

pid_file, log_file, *command = sys.argv[1:]
with open(log_file, "ab", buffering=0) as log:
    child = subprocess.Popen(
        command,
        stdin=subprocess.DEVNULL,
        stdout=log,
        stderr=subprocess.STDOUT,
        start_new_session=True,
        close_fds=True,
    )
try:
    with open(pid_file, "x", encoding="ascii") as output:
        output.write(str(child.pid) + "\n")
except BaseException:
    os.killpg(child.pid, signal.SIGTERM)
    raise
PY
}
wait_http() {
  local url=$1 name=$2 attempt
  for attempt in {1..60}; do if curl --fail --silent --show-error "$url" >/dev/null 2>&1; then return 0; fi; sleep 1; done
  echo "local-apps: $name did not become ready; inspect $state_dir/$name.log" >&2
  return 1
}

case "${1:-start}" in
  stop)
    # Stop remains usable after a damaged credential/config file: it reads only
    # PID evidence and terminates only exact owned listeners.
    stop_one web "$vite_entry" 14183
    stop_one biz "$bin_dir/biz" 18380
    stop_one idp "$bin_dir/biz-idp" 18381
    exit 0
    ;;
  start) ;;
  *) die "usage: scripts/local-apps.sh [start|stop]" ;;
esac

if ! command -v jq >/dev/null 2>&1; then die "jq is required to read the mode-0600 local credential file"; fi
command -v lsof >/dev/null 2>&1 || die "lsof is required to verify owned listener PIDs"
[[ -f "$credentials_file" ]] || die "run go run ./cmd/biz-local-setup first"
[[ -f "$root/.env.local" ]] || die "missing ignored .env.local"
[[ -f "$vite_entry" ]] || die "missing Vite entry; install web dependencies first"
mkdir -p "$state_dir" "$bin_dir"
chmod 700 "$state_dir" "$bin_dir"
set -a
source "$root/.env.local"
set +a
worker_token=$(jq -r '.worker_token' "$credentials_file")
[[ -n "$worker_token" && "$worker_token" != null ]] || die "credential file is incomplete"
[[ ! -e "$state_dir/biz.pid" && ! -e "$state_dir/idp.pid" && ! -e "$state_dir/web.pid" ]] || die "a pid file exists; run scripts/local-apps.sh stop first"

trap cleanup_start ERR INT TERM
(cd "$root"; go build -o "$bin_dir/biz-local-setup" ./cmd/biz-local-setup; "$bin_dir/biz-local-setup" --validate-config; go build -o "$bin_dir/biz" ./cmd/biz; go build -o "$bin_dir/biz-idp" ./cmd/biz-idp)

key_file="$state_dir/biz-idp-signing-key.pem"
if [[ ! -f "$key_file" ]]; then umask 077; openssl genrsa -out "$key_file" 2048 >/dev/null 2>&1; fi
chmod 600 "$key_file"
YUNKA_BIZ_IDP_PUBLIC_URL=http://127.0.0.1:18381 YUNKA_BIZ_IDP_LISTEN=127.0.0.1:18381 YUNKA_BIZ_IDP_REDIRECT_URL=http://127.0.0.1:14183/auth/callback YUNKA_BIZ_IDP_POST_LOGOUT_REDIRECT_URL=http://127.0.0.1:14183/ YUNKA_BIZ_IDP_SIGNING_KEY_FILE="$key_file" YUNKA_BIZ_IDP_AUTO_MIGRATE=true YUNKA_BIZ_IDP_COOKIE_SECURE=false start_process idp "$state_dir/idp.log" "$bin_dir/biz-idp"
wait_http http://127.0.0.1:18381/idp/.well-known/openid-configuration idp
YUNKA_BIZ_HTTP_LISTEN=127.0.0.1:18380 YUNKA_BIZ_GRPC_LISTEN=127.0.0.1:18382 YUNKA_BIZ_AUTO_MIGRATE=true YUNKA_BIZ_OIDC_ISSUER=http://127.0.0.1:18381/idp YUNKA_BIZ_OIDC_CLIENT_ID=biz-web YUNKA_BIZ_OIDC_REDIRECT_URL=http://127.0.0.1:14183/auth/callback YUNKA_BIZ_OIDC_POST_LOGOUT_REDIRECT_URL=http://127.0.0.1:14183/ YUNKA_BIZ_OIDC_COOKIE_SECURE=false YUNKA_BIZ_OIDC_PLATFORM_EXTERNAL_SUBJECT=biz-user:local-platform-admin YUNKA_BIZ_OIDC_PLATFORM_SUBJECT=local-platform-admin YUNKA_BIZ_OIDC_PLATFORM_EMAIL=platform-admin@local.biz.invalid YUNKA_BIZ_PROVISIONING_WORKER_TOKEN="$worker_token" start_process biz "$state_dir/biz.log" "$bin_dir/biz"
wait_http http://127.0.0.1:18380/healthz biz
(cd "$root/web"; BIZ_DEV_API_TARGET=http://127.0.0.1:18380 VITE_DATA_MODE=api start_process web "$state_dir/web.log" node "$vite_entry" --host 127.0.0.1 --port 14183)
wait_http http://127.0.0.1:14183/ web
trap - ERR INT TERM
echo "local apps ready: web http://127.0.0.1:14183, Biz http://127.0.0.1:18380, IdP http://127.0.0.1:18381"
