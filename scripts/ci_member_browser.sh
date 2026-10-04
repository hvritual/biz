#!/usr/bin/env bash
# CI-only prerequisites for the existing headless member lifecycle tests.
set -euo pipefail
[[ "${GITHUB_ACTIONS:-}" == true ]] || { echo 'CI_OWNED_BROWSER_ONLY' >&2; exit 2; }
cd "${GITHUB_WORKSPACE:?}/web"
started="$(date +%s)"
# The Ubuntu runner already supplies Chromium's shared libraries. Do not upgrade
# the OS or download unrelated language fonts merely to run a headless test.
# A real browser launch below fails closed if the image loses a required library.
browser_status=0
font_status=0
npx --no-install playwright install --only-shell chromium &
browser_pid=$!
font_pid=''
if [[ -z "$(fc-list :lang=zh)" ]]; then
  (
    sudo apt-get update -qq
    sudo apt-get install -y --no-install-recommends fonts-wqy-zenhei
  ) &
  font_pid=$!
fi
wait "$browser_pid" || browser_status=$?
if [[ -n "$font_pid" ]]; then
  wait "$font_pid" || font_status=$?
fi
(( browser_status == 0 )) || exit "$browser_status"
(( font_status == 0 )) || exit "$font_status"
[[ -n "$(fc-list :lang=zh)" ]] || { echo 'CHINESE_FONT_UNAVAILABLE' >&2; exit 1; }
node --input-type=module <<'JS'
import { chromium } from '@playwright/test'
const browser = await chromium.launch({ headless: true })
try {
  const page = await browser.newPage()
  await page.setContent('<html lang="zh-CN"><body>成员管理</body></html>')
  if (await page.locator('body').innerText() !== '成员管理') {
    throw new Error('Browser prerequisite smoke failed')
  }
} finally {
  await browser.close()
}
JS
printf 'MEMBER_BROWSER_PREREQUISITES=PASS elapsed_seconds=%s\n' "$(( $(date +%s) - started ))"
