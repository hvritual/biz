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
  # The hosted mirror list tries Azure HTTP first. Use the existing Ubuntu
  # archive over HTTPS for this font only, keeping its suites and signing keys.
  noto_apt_dir="$(mktemp -d /tmp/headless-noto-apt.XXXXXX)"
  cleanup_apt() {
    local status=$? cleanup_status=0
    trap - EXIT
    sudo rm -rf -- "$noto_apt_dir" || cleanup_status=$?
    if (( cleanup_status != 0 )); then
      printf 'HEADLESS_NOTO_APT_CLEANUP_FAILED exit=%s\n' "$cleanup_status" >&2
      (( status != 0 )) || status=$cleanup_status
    fi
    exit "$status"
  }
  trap cleanup_apt EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  mkdir "$noto_apt_dir/sourceparts" "$noto_apt_dir/lists" "$noto_apt_dir/archives"
  chmod 755 "$noto_apt_dir" "$noto_apt_dir/sourceparts" "$noto_apt_dir/lists" "$noto_apt_dir/archives"
  cat /etc/apt/sources.list.d/ubuntu.sources > "$noto_apt_dir/ubuntu.sources"
  python3 - "$noto_apt_dir/ubuntu.sources" <<'SOURCE'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
source = path.read_bytes().decode('utf-8')
replacements = {
    'mirror+file:/etc/apt/apt-mirrors.txt': 'https://archive.ubuntu.com/ubuntu/',
    'http://azure.archive.ubuntu.com/ubuntu/': 'https://archive.ubuntu.com/ubuntu/',
    'http://azure.archive.ubuntu.com/ubuntu': 'https://archive.ubuntu.com/ubuntu/',
    'https://archive.ubuntu.com/ubuntu/': 'https://archive.ubuntu.com/ubuntu/',
    'https://archive.ubuntu.com/ubuntu': 'https://archive.ubuntu.com/ubuntu',
    'https://security.ubuntu.com/ubuntu/': 'https://security.ubuntu.com/ubuntu/',
    'https://security.ubuntu.com/ubuntu': 'https://security.ubuntu.com/ubuntu',
}
blocks = re.split(r'(\n[ \t]*\n)', source)
count = 0
for index, block in enumerate(blocks):
    if not any(line.strip() and not line.lstrip().startswith('#') for line in block.splitlines()):
        continue
    fields = {}
    for name in ('Types', 'URIs', 'Suites', 'Components', 'Signed-By'):
        matches = list(re.finditer(r'(?mi)^' + name + r':[ \t]*([^\r\n]*)', block))
        if len(matches) != 1 or not matches[0][1].strip():
            sys.exit('HEADLESS_NOTO_APT_SOURCE_UNSUPPORTED')
        fields[name] = matches[0]
    uri = fields['URIs']
    # This runner uses one-line URI fields. Reject every continuation, including
    # one separated from the field by a comment, instead of guessing its meaning.
    for line in block[uri.end():].splitlines():
        if not line.strip() or line.startswith('#'):
            continue
        if line[0].isspace():
            sys.exit('HEADLESS_NOTO_APT_SOURCE_UNSUPPORTED')
        break
    if any(token not in replacements for token in uri[1].split()):
        sys.exit('HEADLESS_NOTO_APT_SOURCE_UNSUPPORTED')
    converted = re.sub(r'\S+', lambda match: replacements[match[0]], uri[1])
    blocks[index] = block[:uri.start(1)] + converted + block[uri.end(1):]
    count += 1
if not count:
    sys.exit('HEADLESS_NOTO_APT_SOURCE_UNSUPPORTED')
path.write_bytes(''.join(blocks).encode('utf-8'))
SOURCE
  chmod 644 "$noto_apt_dir/ubuntu.sources"
  sudo -u _apt test -r "$noto_apt_dir/ubuntu.sources"
  # Separate lists avoid APT's default cleanup of unrelated system indexes.
  # Keep the real dpkg status and archive keyrings; never bypass APT validation.
  apt_options=(
    -o "Dir::Etc::SourceList=$noto_apt_dir/ubuntu.sources"
    -o "Dir::Etc::SourceParts=$noto_apt_dir/sourceparts"
    -o "Dir::State::Lists=$noto_apt_dir/lists"
    -o "Dir::Cache::Archives=$noto_apt_dir/archives"
    -o Dir::Cache::pkgcache= -o Dir::Cache::srcpkgcache=
    -o Acquire::Retries=1 -o Acquire::http::Timeout=15 -o Acquire::https::Timeout=15
  )
  echo 'HEADLESS_NOTO_BROWSER_PREP_STAGE=font-index'
  sudo apt-get update -qq "${apt_options[@]}"
  echo 'HEADLESS_NOTO_BROWSER_PREP_STAGE=font-download'
  sudo apt-get install -y --no-install-recommends "${apt_options[@]}" fonts-noto-cjk
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
