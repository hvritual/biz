#!/usr/bin/env bash
set -euo pipefail

if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
  echo "CE09 optimized qualification is CI-owned; use canonical local verification for local database checks" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

out="${RUNNER_TEMP:?}/ce09"
mkdir -p "$out"
: > "$out/timings.tsv"

record_timing() {
  local name="$1" started="$2" finished
  finished="$(date +%s)"
  printf '%s\t%s\n' "$name" "$((finished-started))" >> "$out/timings.tsv"
}

run_check() {
  local name="$1"; shift
  local started rc
  started="$(date +%s)"
  printf '%s\n' "$*" > "$out/$name.command"
  if "$@" > "$out/$name.log" 2>&1; then rc=0; else rc=$?; fi
  printf '%s\n' "$rc" > "$out/$name.exit"
  record_timing "$name" "$started"
  printf '%s exit=%s\n' "$name" "$rc"
  return "$rc"
}

run_suite() {
  local name="$1"; shift
  local started rc
  started="$(date +%s)"
  printf '%s\n' "$*" > "$out/$name.command"
  if "$@" > "$out/$name.jsonl" 2> "$out/$name.stderr"; then rc=0; else rc=$?; fi
  printf '%s\n' "$rc" > "$out/$name.exit"
  record_timing "$name" "$started"
  printf '%s exit=%s\n' "$name" "$rc"
  return "$rc"
}

printf 'biz=%s\nyunka=%s\n' "$(git rev-parse HEAD)" "$(git -C ../yunka.io rev-parse HEAD)" > "$out/sources.txt"
: "${YUNKA_SHA:?locked framework is required}"

baseline="${BASE_SHA:-}"
if [[ ! "$baseline" =~ ^[0-9a-f]{40}$ || "$baseline" == 0000000000000000000000000000000000000000 ]]; then
  baseline="$(git rev-parse origin/main)"
fi
git show "$baseline:contracts/commercial/generated/catalog.json" > "$out/baseline.json"
export COMMERCIAL_BASELINE="$out/baseline.json"
git diff --quiet "$baseline" HEAD -- server

fast_container="ce09-fast-${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-1}"
restart_container="ce09-restart-${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-1}"
docker rm -f "$fast_container" "$restart_container" >/dev/null 2>&1 || true

cleanup() {
  docker rm -f "$fast_container" "$restart_container" >/dev/null 2>&1 || true
}
trap cleanup EXIT

mysql_started="$(date +%s)"
docker run \
  --name "$fast_container" \
  --tmpfs /var/lib/mysql:rw,nosuid,size=1g \
  -e MYSQL_ROOT_PASSWORD=root \
  -e MYSQL_DATABASE=biz_ce09_mysql \
  -p 127.0.0.1:3306:3306 \
  -d mysql:8.4 > "$out/fast-container.id"

docker run \
  --name "$restart_container" \
  -e MYSQL_ROOT_PASSWORD=root \
  -e MYSQL_DATABASE=biz_ce09_restart \
  -p 127.0.0.1:3307:3306 \
  -d mysql:8.4 > "$out/restart-container.id"

wait_mysql() {
  local container="$1" log="$2" ready=0
  for attempt in $(seq 1 90); do
    if docker exec "$container" mysqladmin ping -h 127.0.0.1 -proot --silent >> "$log" 2>&1; then
      ready=1
      break
    fi
    sleep 1
  done
  test "$ready" = 1
}

wait_mysql "$fast_container" "$out/fast-container.log" &
fast_ready_pid="$!"
wait_mysql "$restart_container" "$out/restart-container.log" &
restart_ready_pid="$!"

run_check resolution make yunka-source-check workspace-check
for pass in 1 2; do
  run_check "generate-$pass" make generate
  run_check "generate-$pass-clean" bash -c 'git diff --exit-code && test -z "$(git status --porcelain)"'
done

wait "$fast_ready_pid"
docker exec "$fast_container" mysql -uroot -proot -e "
  CREATE DATABASE IF NOT EXISTS biz_ce09_mysql CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
  CREATE DATABASE IF NOT EXISTS biz_ce09_race CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
"
record_timing mysql-fast-ready "$mysql_started"

stage_started="$(date +%s)"
declare -a pids=()
declare -a names=()

(
  run_suite focused go test -timeout=5m -count=1 -json \
    ./internal/commercial/domain/subscriptionchange \
    ./internal/commercial/application/subscriptionchanges/... \
    ./internal/bizruntime ./internal/architecture -run '^TestCE09'
) &
pids+=("$!"); names+=("focused")

(
  run_suite mysql env \
    'YUNKA_TEST_MYSQL_DSN=root:root@tcp(127.0.0.1:3306)/biz_ce09_mysql?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true' \
    go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE09MySQL'
) &
pids+=("$!"); names+=("mysql")

(
  run_suite race env \
    'YUNKA_TEST_MYSQL_DSN=root:root@tcp(127.0.0.1:3306)/biz_ce09_race?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true' \
    go test -timeout=5m -race -count=1 -tags=integration -json ./integration \
    -run '^TestCE09MySQLConcurrentUpgradeAndDowngradeHaveOneWinner$'
) &
pids+=("$!"); names+=("race")

wait "$restart_ready_pid"
(
  set -euo pipefail
  export CE09_RESTART_RECEIPT="$out/restart-receipt.json"
  export YUNKA_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3307)/biz_ce09_restart?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true'

  run_suite restart-before go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE09PersistenceBeforeRestart$'

  before_start="$(docker inspect -f '{{.State.StartedAt}}' "$restart_container")"
  docker restart "$restart_container" > "$out/restart.log" 2>&1
  wait_mysql "$restart_container" "$out/restart.log"
  after_start="$(docker inspect -f '{{.State.StartedAt}}' "$restart_container")"
  test "$before_start" != "$after_start"
  printf 'before=%s\nafter=%s\n' "$before_start" "$after_start" >> "$out/restart.log"
  printf 'docker restart isolated durable MySQL; require new StartedAt and healthy database\n' > "$out/restart.command"
  printf '0\n' > "$out/restart.exit"

  run_suite restart-after go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE09PersistenceAfterRestart$'
) &
pids+=("$!"); names+=("restart-proof")

stage_status=0
for i in "${!pids[@]}"; do
  if ! wait "${pids[$i]}"; then
    echo "CE09_STAGE_FAILURE=${names[$i]}" >&2
    stage_status=1
  fi
done
record_timing ce09-parallel-stage "$stage_started"
test "$stage_status" = 0

run_check plan python3 docs/commercial-entitlements/tools/check_plan.py
run_check plan-tests python3 -m unittest discover -s docs/commercial-entitlements/tools -p 'test_*.py' -v
run_check runtime-gate-tests python3 -m unittest discover -s scripts -p 'test_ce04_runtime_closure.py' -v
run_check evidence-tests python3 -m unittest discover -s scripts -p 'test_ce03_*.py' -v

git diff --exit-code
test -z "$(git status --porcelain)"
test -z "$(git -C ../yunka.io status --porcelain)"
git archive HEAD -o "$out/biz-source.tar"

python3 - <<'PY'
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, 'scripts')
from ce03_test_events import summarize

out = Path(os.environ['RUNNER_TEMP']) / 'ce09'
task = next(
    t for t in json.loads(Path('docs/commercial-entitlements/tasks.json').read_text())['tasks']
    if t['id'] == 'CE-09'
)
summary = {
    'task_id': 'CE-09',
    'verified_commit': subprocess.check_output(['git','rev-parse','HEAD'], text=True).strip(),
    'framework_commit': os.environ['YUNKA_SHA'],
    'qualification': 'PASS',
    'task_status': task['status'],
    'suites': {},
    'checks': [],
    'timings_seconds': {},
    'delegated_full_gate_coverage': [
        'mysql-runtime-qualification.yml',
        'b12-multitenant-access-pressure.yml',
        'ce02-bootstrap.yml',
        'ce04-qualification.yml',
        'ce05-qualification.yml',
        'ce06-qualification.yml',
        'ce07-qualification.yml',
        'ce08-qualification.yml',
        'evolution-qualification.yml',
    ],
}

suites = ['focused','mysql','race','restart-before','restart-after']
for name in suites:
    counts = summarize([json.loads(line) for line in (out/(name+'.jsonl')).read_text().splitlines()])
    assert counts['leaf_tests'] > 0 and counts['failed'] == 0 and counts['skipped'] == 0, (name, counts)
    summary['suites'][name] = counts

for command in sorted(out.glob('*.command')):
    name = command.stem
    rc = int((out/(name+'.exit')).read_text())
    assert rc == 0, name
    count = summary['suites'].get(name, {}).get(
        'leaf_tests',
        {'plan-tests':8,'runtime-gate-tests':25,'evidence-tests':6}.get(name,0),
    )
    if name in ['plan-tests','runtime-gate-tests','evidence-tests']:
        log = (out/(name+'.log')).read_text()
        match = re.search(r'Ran (\d+) tests? in', log)
        assert match and int(match.group(1)) == count and '\nOK' in log, name
    summary['checks'].append({
        'command': command.read_text().strip(),
        'exit_code': rc,
        'tests_passed': count,
        'log': name + ('.jsonl' if name in suites else '.log'),
    })

for line in (out/'timings.tsv').read_text().splitlines():
    name, seconds = line.split('\t', 1)
    summary['timings_seconds'][name] = int(seconds)

if task['status'] == 'DONE':
    subprocess.run(['git','fetch','origin','main:refs/remotes/origin/main'], check=True)
    command = ['python3','docs/commercial-entitlements/tools/check_round.py','--task','CE-09']
    if os.environ.get('GITHUB_REF') == 'refs/heads/main':
        command.append('--require-main')
    result = subprocess.run(command, text=True, capture_output=True, check=True)
    (out/'round.log').write_text('COMMAND: '+' '.join(command)+'\n'+result.stdout+result.stderr)
    summary['round_check'] = 'PASS'
else:
    summary['round_check'] = 'QUALIFIED_NOT_DONE'

summary['sha256'] = {
    p.name: hashlib.sha256(p.read_bytes()).hexdigest()
    for p in out.iterdir()
    if p.is_file()
}
(out/'summary.json').write_text(json.dumps(summary, indent=2)+'\n')
print(json.dumps(summary, indent=2))
PY
