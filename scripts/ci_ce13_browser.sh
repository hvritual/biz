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
      # The hosted mirror list tries Azure HTTP first. Use the existing Ubuntu
      # archive over HTTPS for this font only, keeping its suites and signing keys.
      noto_apt_dir="$(mktemp -d /tmp/headless-noto-apt.XXXXXX)"
      cleanup_apt() {
        local status=$? cleanup_status=0
        trap - EXIT
        sudo rm -rf -- "$noto_apt_dir" || cleanup_status=$?
        if (( cleanup_status != 0 )); then
          printf 'CE13_NOTO_APT_CLEANUP_FAILED exit=%s\n' "$cleanup_status" >&2
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
            sys.exit('CE13_NOTO_APT_SOURCE_UNSUPPORTED')
        fields[name] = matches[0]
    uri = fields['URIs']
    # This runner uses one-line URI fields. Reject every continuation, including
    # one separated from the field by a comment, instead of guessing its meaning.
    for line in block[uri.end():].splitlines():
        if not line.strip() or line.startswith('#'):
            continue
        if line[0].isspace():
            sys.exit('CE13_NOTO_APT_SOURCE_UNSUPPORTED')
        break
    if any(token not in replacements for token in uri[1].split()):
        sys.exit('CE13_NOTO_APT_SOURCE_UNSUPPORTED')
    converted = re.sub(r'\S+', lambda match: replacements[match[0]], uri[1])
    blocks[index] = block[:uri.start(1)] + converted + block[uri.end(1):]
    count += 1
if not count:
    sys.exit('CE13_NOTO_APT_SOURCE_UNSUPPORTED')
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
      echo 'CE13_BROWSER_PREP_STAGE=font-index'
      sudo apt-get update -qq "${apt_options[@]}"
      echo 'CE13_BROWSER_PREP_STAGE=font-download'
      sudo apt-get install -y --no-install-recommends fonts-noto-cjk "${apt_options[@]}"
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
