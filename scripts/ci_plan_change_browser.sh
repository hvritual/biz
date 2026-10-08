#!/usr/bin/env bash
# CI-only headless shell/Noto prerequisites for the plan-change suite.
set -euo pipefail
[[ "${GITHUB_ACTIONS:-}" == true ]] || { echo 'CI_OWNED_BROWSER_ONLY' >&2; exit 2; }
if (( $# != 1 )) || [[ "${1:-}" != /* || ! -d "${1:-}" || ! -d "${GITHUB_WORKSPACE:-}" ]]; then
  echo 'HEADLESS_NOTO_WEB_DIRECTORY_REQUIRED' >&2
  exit 2
fi
web_dir="$(cd -- "$1" && pwd -P)"
workspace_dir="$(cd -- "$GITHUB_WORKSPACE" && pwd -P)"
case "$web_dir" in
  "$workspace_dir/web"|"$workspace_dir/biz/web") ;;
  *) echo 'HEADLESS_NOTO_WEB_DIRECTORY_INVALID' >&2; exit 2 ;;
esac
if [[ ! -f "$web_dir/package.json" || ! -f "$web_dir/package-lock.json" || ! -x "$web_dir/node_modules/.bin/playwright" ]]; then
  echo 'HEADLESS_NOTO_DEPENDENCIES_UNAVAILABLE' >&2
  exit 2
fi
cd -- "$web_dir"
started="$(date +%s)"
browser_pid=''
font_pid=''
cleanup() {
  local pid
  for pid in "$browser_pid" "$font_pid"; do
    if [[ -n "$pid" ]]; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# The plan-change suite uses default headless Chromium without native zoom. Use the lockfile's
# headless shell, retaining a real launch check for the runner's shared libraries.
# APT transport timeouts alone do not bound a slow download that keeps making
# progress. Bound each entire installation branch, including font readiness.
echo 'HEADLESS_NOTO_BROWSER_PREP_STAGE=browser'
timeout --signal=TERM --kill-after=5s 90s \
  npx --no-install playwright install --only-shell chromium &
browser_pid=$!
timeout --signal=TERM --kill-after=5s 90s bash -euo pipefail <<'FONT' &
echo 'HEADLESS_NOTO_BROWSER_PREP_STAGE=font-check'
if ! fc-match "Noto Sans CJK SC" | grep -qi 'Noto Sans CJK'; then
  echo 'HEADLESS_NOTO_BROWSER_PREP_STAGE=font-index'
  sudo apt-get update -qq -o Acquire::Retries=1 -o Acquire::http::Timeout=15 -o Acquire::https::Timeout=15
  echo 'HEADLESS_NOTO_BROWSER_PREP_STAGE=font-download'
  sudo apt-get install -y --no-install-recommends \
    -o Acquire::Retries=1 -o Acquire::http::Timeout=15 -o Acquire::https::Timeout=15 fonts-noto-cjk
fi
fc-match "Noto Sans CJK SC" | grep -qi 'Noto Sans CJK' || {
  echo 'HEADLESS_NOTO_FONT_UNAVAILABLE' >&2
  exit 1
}
FONT
font_pid=$!
browser_status=0
font_status=0
wait "$browser_pid" || browser_status=$?
browser_pid=''
wait "$font_pid" || font_status=$?
font_pid=''
printf 'HEADLESS_NOTO_BROWSER_PREP_RESULT=browser exit=%s\n' "$browser_status"
printf 'HEADLESS_NOTO_BROWSER_PREP_RESULT=font exit=%s\n' "$font_status"
(( browser_status == 0 )) || exit "$browser_status"
(( font_status == 0 )) || exit "$font_status"

echo 'HEADLESS_NOTO_BROWSER_PREP_STAGE=launch'
launch_status=0
timeout --signal=TERM --kill-after=5s 30s node --input-type=module <<'JS' || launch_status=$?
import { chromium } from '@playwright/test'
const browser = await chromium.launch({ headless: true, timeout: 20000 })
try {
  const page = await browser.newPage()
  await page.setContent('<html lang="zh-CN"><body style="font-family: Noto Sans CJK SC">套餐变更</body></html>')
  if (await page.locator('body').innerText() !== '套餐变更') {
    throw new Error('Headless Noto browser prerequisite smoke failed')
  }
} finally {
  await browser.close()
}
JS
printf 'HEADLESS_NOTO_BROWSER_PREP_RESULT=launch exit=%s\n' "$launch_status"
(( launch_status == 0 )) || exit "$launch_status"
printf 'HEADLESS_NOTO_BROWSER_PREREQUISITES=PASS elapsed_seconds=%s\n' "$(( $(date +%s) - started ))"
