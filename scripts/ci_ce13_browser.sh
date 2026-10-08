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
  source_web="${GITHUB_WORKSPACE:?}/biz/web"
  isolated_web="$out/web"
  rm -rf "$isolated_web"
  mkdir -p "$isolated_web"
  tar -C "$source_web" \
    --exclude='./node_modules' \
    --exclude='./test-results' \
    --exclude='./playwright-report' \
    --exclude='./dist' \
    -cf - . | tar -C "$isolated_web" -xf -
  cd "$isolated_web"
  # Font installation and the lockfile-selected browser download are independent.
  # Start both cold-run prerequisites together; wait and fail closed on either.
  echo 'CE13_BROWSER_PREP_STAGE=font'
  (
    set -euo pipefail
    if ! fc-match "Noto Sans CJK SC" | grep -qi 'Noto Sans CJK'; then
      sudo apt-get update -qq -o Acquire::Retries=1 -o Acquire::http::Timeout=15 -o Acquire::https::Timeout=15
      sudo apt-get install -y --no-install-recommends fonts-noto-cjk
    fi
    fc-match "Noto Sans CJK SC" | grep -qi 'Noto Sans CJK' || {
      echo 'CE13_CHINESE_FONT_UNAVAILABLE' >&2
      exit 1
    }
  ) &
  font_pid=$!
  echo 'CE13_BROWSER_PREP_STAGE=npm'
  npm_status=0
  npm ci || npm_status=$?
  browser_status=0
  if (( npm_status == 0 )); then
    echo 'CE13_BROWSER_PREP_STAGE=browser'
    # Keep the original pinned headless-shell install and launch prerequisite.
    npx --no-install playwright install --only-shell chromium || browser_status=$?
  fi
  font_status=0
  wait "$font_pid" || font_status=$?
  if (( npm_status != 0 )); then exit "$npm_status"; fi
  if (( browser_status != 0 )); then exit "$browser_status"; fi
  if (( font_status != 0 )); then exit "$font_status"; fi
  echo 'CE13_BROWSER_PREP_STAGE=launch'
  node --input-type=module <<'JS'
import { chromium } from '@playwright/test'
const browser = await chromium.launch({ headless: true, timeout: 20000 })
try {
  const page = await browser.newPage()
  await page.setContent('<html lang="zh-CN"><body>套餐与会话验证</body></html>')
  if (await page.locator('body').innerText() !== '套餐与会话验证') {
    throw new Error('CE13 browser prerequisite smoke failed')
  }
} finally {
  await browser.close()
}
JS
  echo 'CE13_BROWSER_LAUNCH=PASS'
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
