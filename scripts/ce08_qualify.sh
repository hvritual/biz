#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

out="${RUNNER_TEMP:?}/ce08"
manifest="scripts/ci_ce08_shards.json"
mkdir -p "$out"
: > "$out/timings.tsv"

record_timing() {
  local name="$1"
  local started="$2"
  local finished
  finished="$(date +%s)"
  printf '%s\t%s\n' "$name" "$((finished-started))" >> "$out/timings.tsv"
}

printf 'biz=%s\nyunka=%s\n' "$(git rev-parse HEAD)" "$(git -C ../yunka.io rev-parse HEAD)" > "$out/sources.txt"

baseline="${BASE_SHA:-}"
if [[ ! "$baseline" =~ ^[0-9a-f]{40}$ || "$baseline" == 0000000000000000000000000000000000000000 ]]; then
  baseline="$(git rev-parse origin/main)"
fi
git show "$baseline:contracts/commercial/generated/catalog.json" > "$out/baseline.json"
export COMMERCIAL_BASELINE="$out/baseline.json"

started="$(date +%s)"
make yunka-source-check workspace-check 2>&1 | tee "$out/resolution.log"
record_timing source-resolution "$started"

started="$(date +%s)"
go test -count=1 -json \
  ./internal/commercial/domain/subscription \
  ./internal/commercial/application/subscriptionmanagement/... \
  ./internal/access/application/tenantlifecycle/... \
  ./internal/access/policy \
  ./internal/bizruntime \
  -run '^TestCE08' | tee "$out/focused.jsonl"
record_timing focused "$started"

started="$(date +%s)"
go test -tags=integration ./integration -list '^TestCE08MySQL' \
  | grep '^TestCE08MySQL' \
  | sort > "$out/mysql-tests.actual"

python3 - <<'PY'
import json
import shlex
from pathlib import Path

manifest = json.loads(Path('scripts/ci_ce08_shards.json').read_text())
assert manifest.get('schema_version') == 1
shards = manifest['shards']
expected = []
seen = set()
for shard in shards:
    assert shard['id'] and shard['tests']
    for test in shard['tests']:
        if test in seen:
            raise SystemExit(f'CE08_SHARD_DUPLICATE={test}')
        seen.add(test)
        expected.append(test)
actual = [line.strip() for line in Path(Path(__import__('os').environ['RUNNER_TEMP'])/'ce08/mysql-tests.actual').read_text().splitlines() if line.strip()]
if sorted(expected) != sorted(actual):
    missing = sorted(set(actual) - set(expected))
    stale = sorted(set(expected) - set(actual))
    raise SystemExit(f'CE08_SHARD_SET_MISMATCH missing={missing} stale={stale}')
race = manifest['race_tests']
if not race or any(test not in seen for test in race):
    raise SystemExit('CE08_RACE_SET_INVALID')
with (Path(__import__('os').environ['RUNNER_TEMP'])/'ce08/shards.env').open('w') as out:
    for shard in shards:
        pattern = '^(' + '|'.join(shard['tests']) + ')$'
        out.write(f"CE08_{shard['id'].upper()}_REGEX={shlex.quote(pattern)}\n")
    race_pattern = '^(' + '|'.join(race) + ')$'
    out.write(f"CE08_RACE_REGEX={shlex.quote(race_pattern)}\n")
print(f"CE08_SHARD_CONTRACT=PASS tests={len(actual)} shards={len(shards)} race={len(race)}")
PY
# shellcheck disable=SC1090
source "$out/shards.env"
record_timing shard-contract "$started"

: "${MYSQL_CONTAINER_ID:?workflow-owned MySQL required}"
docker exec "$MYSQL_CONTAINER_ID" mysql -uroot -proot -e "
  CREATE DATABASE IF NOT EXISTS biz_ce08_s1 CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
  CREATE DATABASE IF NOT EXISTS biz_ce08_s2 CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
  CREATE DATABASE IF NOT EXISTS biz_ce08_s3 CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
  CREATE DATABASE IF NOT EXISTS biz_ce08_race CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
  CREATE DATABASE IF NOT EXISTS biz_ce08_restart CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
"

started="$(date +%s)"
declare -a pids=()
for shard in s1 s2 s3; do
  upper="${shard^^}"
  regex_var="CE08_${upper}_REGEX"
  regex="${!regex_var}"
  (
    set -euo pipefail
    export YUNKA_TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/biz_ce08_${shard}?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true"
    go test -count=1 -tags=integration -json ./integration -run "$regex" \
      | tee "$out/mysql-${shard}.jsonl"
  ) &
  pids+=("$!")
done

shard_status=0
for pid in "${pids[@]}"; do
  if ! wait "$pid"; then
    shard_status=1
  fi
done
test "$shard_status" = 0
cat "$out/mysql-s1.jsonl" "$out/mysql-s2.jsonl" "$out/mysql-s3.jsonl" > "$out/mysql.jsonl"
record_timing mysql-shards "$started"

started="$(date +%s)"
YUNKA_TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/biz_ce08_race?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true" \
  go test -race -count=1 -tags=integration -json ./integration -run "$CE08_RACE_REGEX" \
  | tee "$out/race.jsonl"
record_timing race-focused "$started"

export CE08_RESTART_RECEIPT="$out/restart-receipt.json"
export YUNKA_TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/biz_ce08_restart?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true"

started="$(date +%s)"
go test -count=1 -tags=integration -json ./integration -run '^TestCE08PersistenceBeforeRestart$' \
  | tee "$out/restart-before.jsonl"

before_start="$(docker inspect -f '{{.State.StartedAt}}' "$MYSQL_CONTAINER_ID")"
docker restart "$MYSQL_CONTAINER_ID" | tee "$out/restart.log"
ready=0
for attempt in $(seq 1 60); do
  if docker exec "$MYSQL_CONTAINER_ID" mysqladmin ping -h 127.0.0.1 -proot --silent >> "$out/restart.log" 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
test "$ready" = 1
after_start="$(docker inspect -f '{{.State.StartedAt}}' "$MYSQL_CONTAINER_ID")"
test "$before_start" != "$after_start"
printf 'before=%s\nafter=%s\n' "$before_start" "$after_start" >> "$out/restart.log"

go test -count=1 -tags=integration -json ./integration -run '^TestCE08PersistenceAfterRestart$' \
  | tee "$out/restart-after.jsonl"
record_timing restart-proof "$started"

started="$(date +%s)"
python3 docs/commercial-entitlements/tools/check_plan.py | tee "$out/plan.log"
python3 -m unittest discover -s docs/commercial-entitlements/tools -p 'test_*.py' -v 2>&1 | tee "$out/plan-tests.log"
python3 -m unittest discover -s scripts -p 'test_ce04_runtime_closure.py' -v 2>&1 | tee "$out/runtime-gate-tests.log"
python3 -m unittest discover -s scripts -p 'test_ce03_*.py' -v 2>&1 | tee "$out/evidence-tests.log"
record_timing policy-evidence "$started"

python3 - <<'PY'
import hashlib
import json
import os
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, 'scripts')
from ce03_test_events import summarize

out = Path(os.environ['RUNNER_TEMP']) / 'ce08'
manifest = json.loads(Path('scripts/ci_ce08_shards.json').read_text())
task = next(
    t for t in json.loads(Path('docs/commercial-entitlements/tasks.json').read_text())['tasks']
    if t['id'] == 'CE-08'
)
summary = {
    'task_id': 'CE-08',
    'verified_commit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
    'framework_commit': os.environ['YUNKA_SHA'],
    'qualification': 'PASS',
    'task_status': task['status'],
    'suites': {},
    'delegated_full_gate_coverage': manifest['delegated_full_gate_coverage'],
    'timings_seconds': {},
}
for name in ('focused', 'mysql', 'race', 'restart-before', 'restart-after'):
    rows = [json.loads(line) for line in (out / (name + '.jsonl')).read_text().splitlines()]
    summary['suites'][name] = summarize(rows)

for line in (out / 'timings.tsv').read_text().splitlines():
    name, seconds = line.split('\t', 1)
    summary['timings_seconds'][name] = int(seconds)

if task['status'] == 'DONE':
    subprocess.run(['git', 'fetch', 'origin', 'main:refs/remotes/origin/main'], check=True)
    command = ['python3', 'docs/commercial-entitlements/tools/check_round.py', '--task', 'CE-08']
    if os.environ.get('GITHUB_REF') == 'refs/heads/main':
        command.append('--require-main')
    result = subprocess.run(command, text=True, capture_output=True, check=True)
    (out / 'round.log').write_text('COMMAND: ' + ' '.join(command) + '\n' + result.stdout + result.stderr)
    summary['round_check'] = 'PASS'
else:
    summary['round_check'] = 'QUALIFIED_NOT_DONE'

summary['sha256'] = {
    p.name: hashlib.sha256(p.read_bytes()).hexdigest()
    for p in out.iterdir()
    if p.is_file()
}
(out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
print(json.dumps(summary, indent=2))
PY

git diff --exit-code
test -z "$(git status --porcelain)"
test -z "$(git -C ../yunka.io status --porcelain)"
git archive HEAD -o "$out/biz-source.tar"
