#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
out="${RUNNER_TEMP:?}/ce09"
mkdir -p "$out"
printf 'biz=%s\nyunka=%s\n' "$(git rev-parse HEAD)" "$(git -C ../yunka.io rev-parse HEAD)" > "$out/sources.txt"
baseline="${BASE_SHA:-}"
if [[ ! "$baseline" =~ ^[0-9a-f]{40}$ || "$baseline" == 0000000000000000000000000000000000000000 ]]; then baseline="$(git rev-parse origin/main)"; fi
git show "$baseline:contracts/commercial/generated/catalog.json" > "$out/baseline.json"
export COMMERCIAL_BASELINE="$out/baseline.json"
git diff --quiet "$baseline" HEAD -- server
make yunka-source-check workspace-check 2>&1 | tee "$out/resolution.log"
make check 2>&1 | tee "$out/check-before.log"
for pass in 1 2; do
  make generate 2>&1 | tee "$out/generate-$pass.log"
  git diff --exit-code
  test -z "$(git status --porcelain)"
done
make check 2>&1 | tee "$out/check-after.log"

go test -timeout=5m -count=1 -json ./internal/commercial/domain/subscriptionchange ./internal/commercial/application/subscriptionchanges/... ./internal/bizruntime ./internal/architecture -run '^TestCE09' | tee "$out/focused.jsonl"
go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE09MySQL' | tee "$out/mysql.jsonl"
go test -timeout=5m -race -count=1 -tags=integration -json ./integration -run '^TestCE09MySQL' | tee "$out/race.jsonl"
export CE09_RESTART_RECEIPT="$out/restart-receipt.json"
go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE09PersistenceBeforeRestart$' | tee "$out/restart-before.jsonl"
: "${MYSQL_CONTAINER_ID:?workflow-owned MySQL required}"
before_start="$(docker inspect -f '{{.State.StartedAt}}' "$MYSQL_CONTAINER_ID")"
docker restart "$MYSQL_CONTAINER_ID" | tee "$out/restart.log"
ready=0
for attempt in $(seq 1 60); do
  if docker exec "$MYSQL_CONTAINER_ID" mysqladmin ping -h 127.0.0.1 -proot --silent >> "$out/restart.log" 2>&1; then ready=1; break; fi
  sleep 1
done
test "$ready" = 1
after_start="$(docker inspect -f '{{.State.StartedAt}}' "$MYSQL_CONTAINER_ID")"
test "$before_start" != "$after_start"
printf 'before=%s\nafter=%s\n' "$before_start" "$after_start" >> "$out/restart.log"
go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE09PersistenceAfterRestart$' | tee "$out/restart-after.jsonl"
go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^Test(B122|B123|B124|B125|B126|AG02OwnerInvariantUsesCurrentReadAfterSnapshot)' | tee "$out/b12-mysql.jsonl"
go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE08MySQL' | tee "$out/ce08-mysql.jsonl"
go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE07MySQL' | tee "$out/ce07-mysql.jsonl"
go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE06MySQL' | tee "$out/ce06-mysql.jsonl"
go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE05MySQL' | tee "$out/ce05-mysql.jsonl"
go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE04MySQL' | tee "$out/ce04-mysql.jsonl"
go test -timeout=5m -count=1 -tags=integration -json ./integration -run '^TestCE02' | tee "$out/ce02-mysql.jsonl"
go test -timeout=5m -count=1 -json ./... | tee "$out/all.jsonl"
go vet ./... 2>&1 | tee "$out/vet.log"
go build ./... 2>&1 | tee "$out/build.log"
python3 docs/commercial-entitlements/tools/check_plan.py | tee "$out/plan.log"
python3 -m unittest discover -s docs/commercial-entitlements/tools -p 'test_*.py' -v 2>&1 | tee "$out/plan-tests.log"
python3 -m unittest discover -s scripts -p 'test_ce04_runtime_closure.py' -v 2>&1 | tee "$out/runtime-gate-tests.log"
python3 -m unittest discover -s scripts -p 'test_ce03_*.py' -v 2>&1 | tee "$out/evidence-tests.log"

git diff --exit-code
test -z "$(git status --porcelain)"
test -z "$(git -C ../yunka.io status --porcelain)"
git archive HEAD -o "$out/biz-source.tar"
python3 - <<'PY'
import hashlib,json,os,subprocess,sys
from pathlib import Path
sys.path.insert(0,'scripts')
from ce03_test_events import summarize
out=Path(os.environ['RUNNER_TEMP'])/'ce09'
task=next(t for t in json.loads(Path('docs/commercial-entitlements/tasks.json').read_text())['tasks'] if t['id']=='CE-09')
summary={'task_id':'CE-09','verified_commit':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),'framework_commit':os.environ['YUNKA_SHA'],'qualification':'PASS','task_status':task['status'],'suites':{}}
for name in ('focused','mysql','race','restart-before','restart-after','b12-mysql','ce08-mysql','ce07-mysql','ce06-mysql','ce05-mysql','ce04-mysql','ce02-mysql','all'):
 rows=[json.loads(line) for line in (out/(name+'.jsonl')).read_text().splitlines()]
 summary['suites'][name]=summarize(rows)
if task['status']=='DONE':
 subprocess.run(['git','fetch','origin','main:refs/remotes/origin/main'],check=True)
 command=['python3','docs/commercial-entitlements/tools/check_round.py','--task','CE-09']
 if os.environ.get('GITHUB_REF')=='refs/heads/main':command.append('--require-main')
 result=subprocess.run(command,text=True,capture_output=True,check=True)
 (out/'round.log').write_text('COMMAND: '+' '.join(command)+'\n'+result.stdout+result.stderr)
 summary['round_check']='PASS'
else:summary['round_check']='QUALIFIED_NOT_DONE'
summary['sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in out.iterdir() if p.is_file()}
(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary,indent=2))
PY
