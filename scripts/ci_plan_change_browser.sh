#!/usr/bin/env bash
# CI-only prerequisites for the existing default-headless plan-change suite.
set -euo pipefail
[[ "${GITHUB_ACTIONS:-}" == true ]] || { echo 'CI_OWNED_BROWSER_ONLY' >&2; exit 2; }
cd "${GITHUB_WORKSPACE:?}/web"
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

# The suite has no Chromium channel/native-zoom consumer. Use the lockfile's
# headless shell, retaining a real launch check for the runner's shared libraries.
# APT transport timeouts alone do not bound a slow download that keeps making
# progress. Bound each entire installation branch, including font readiness.
echo 'PLAN_CHANGE_BROWSER_PREP_STAGE=browser'
timeout --signal=TERM --kill-after=5s 90s \
  npx --no-install playwright install --only-shell chromium &
browser_pid=$!
timeout --signal=TERM --kill-after=5s 90s bash -euo pipefail <<'FONT' &
echo 'PLAN_CHANGE_BROWSER_PREP_STAGE=font-check'
if ! fc-match "Noto Sans CJK SC" | grep -qi 'Noto Sans CJK'; then
  echo 'PLAN_CHANGE_BROWSER_PREP_STAGE=font-index'
  sudo apt-get update -qq -o Acquire::Retries=1 -o Acquire::http::Timeout=15 -o Acquire::https::Timeout=15
  echo 'PLAN_CHANGE_BROWSER_PREP_STAGE=font-download'
  sudo apt-get install -y --no-install-recommends \
    -o Acquire::Retries=1 -o Acquire::http::Timeout=15 -o Acquire::https::Timeout=15 fonts-noto-cjk
fi
fc-match "Noto Sans CJK SC" | grep -qi 'Noto Sans CJK' || {
  echo 'PLAN_CHANGE_NOTO_CJK_UNAVAILABLE' >&2
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
printf 'PLAN_CHANGE_BROWSER_PREP_RESULT=browser exit=%s\n' "$browser_status"
printf 'PLAN_CHANGE_BROWSER_PREP_RESULT=font exit=%s\n' "$font_status"
(( browser_status == 0 )) || exit "$browser_status"
(( font_status == 0 )) || exit "$font_status"

echo 'PLAN_CHANGE_BROWSER_PREP_STAGE=launch'
launch_status=0
timeout --signal=TERM --kill-after=5s 30s node --input-type=module <<'JS' || launch_status=$?
import { chromium } from '@playwright/test'
const browser = await chromium.launch({ headless: true, timeout: 20000 })
try {
  const page = await browser.newPage()
  await page.setContent('<html lang="zh-CN"><body style="font-family: Noto Sans CJK SC">套餐变更</body></html>')
  if (await page.locator('body').innerText() !== '套餐变更') {
    throw new Error('Plan-change browser prerequisite smoke failed')
  }
} finally {
  await browser.close()
}
JS
printf 'PLAN_CHANGE_BROWSER_PREP_RESULT=launch exit=%s\n' "$launch_status"
(( launch_status == 0 )) || exit "$launch_status"
printf 'PLAN_CHANGE_BROWSER_PREREQUISITES=PASS elapsed_seconds=%s\n' "$(( $(date +%s) - started ))"
