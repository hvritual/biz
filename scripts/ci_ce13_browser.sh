#!/usr/bin/env bash
# Bounded CE13 browser dependency preparation. CI only.
set -euo pipefail
[[ "${GITHUB_ACTIONS:-}" == true ]] || { echo 'CI_OWNED_BROWSER_ONLY' >&2; exit 2; }
operation="${1:?}"; lane="${2:?}"
[[ "$lane" =~ ^(plan|session)$ && "${GITHUB_RUN_ID:-}" =~ ^[0-9]+$ ]] || exit 2
out="${RUNNER_TEMP:?}/ce13-$lane"
mkdir -p "$out"

case "$operation" in
start)
  [[ ! -f "$out/browser-prep.pid" ]] || { echo 'CE13_BROWSER_PREP_ALREADY_STARTED' >&2; exit 2; }
  date +%s > "$out/browser-prep.started"
  nohup setsid bash "$0" prepare "$lane" >"$out/browser-prep.log" 2>&1 < /dev/null &
  echo $! > "$out/browser-prep.pid"
  echo "CE13_BROWSER_PREP_STARTED=$lane"
  ;;
prepare)
  trap 'rc=$?; echo "$rc" > "$out/browser-prep.exit"' EXIT
  cd "${GITHUB_WORKSPACE:?}/biz/web"
  npm ci
  npx playwright install --with-deps --only-shell chromium
  if ! fc-match "Noto Sans CJK SC" | grep -qi 'Noto Sans CJK'; then
    sudo apt-get install -y --no-install-recommends fonts-noto-cjk
  fi
  fc-match "Noto Sans CJK SC"
  date +%s > "$out/browser-prep.ready"
  ;;
wait)
  for attempt in $(seq 1 150); do
    if [[ -f "$out/browser-prep.exit" ]]; then
      [[ "$(cat "$out/browser-prep.exit")" == 0 && -f "$out/browser-prep.ready" ]] || {
        cat "$out/browser-prep.log" >&2
        exit 1
      }
      echo "CE13_BROWSER_PREP_SECONDS=$(( $(cat "$out/browser-prep.ready") - $(cat "$out/browser-prep.started") ))"
      exit 0
    fi
    elapsed="$(( $(date +%s) - $(cat "$out/browser-prep.started") ))"
    (( elapsed < 150 )) || {
      cat "$out/browser-prep.log" >&2
      echo 'CE13_BROWSER_PREP_TIMEOUT' >&2
      exit 1
    }
    if (( attempt % 10 == 1 )); then
      echo "CE13_BROWSER_PREP_WAITING=$lane elapsed=${elapsed}s"
    fi
    sleep 1
  done
  exit 1
  ;;
stop)
  if [[ -f "$out/browser-prep.pid" && ! -f "$out/browser-prep.exit" ]]; then
    kill -- -"$(cat "$out/browser-prep.pid")" 2>/dev/null || true
    sleep 1
  fi
  ;;
*) exit 2 ;;
esac
