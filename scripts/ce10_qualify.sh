#!/usr/bin/env bash
set -euo pipefail
if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
  echo "Use scripts/qualify-evolution-mysql.py for backed-up local database verification" >&2
  exit 2
fi
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
out="${RUNNER_TEMP:?}/ce10"
mkdir -p "$out"
printf 'biz=%s\nyunka=%s\n' "$(git rev-parse HEAD)" "$(git -C ../yunka.io rev-parse HEAD)" > "$out/sources.txt"
: "${MYSQL_CONTAINER_ID:?workflow-owned MySQL is required}"
: "${YUNKA_SHA:?locked framework is required}"
baseline="${BASE_SHA:-}"
if [[ ! "$baseline" =~ ^[0-9a-f]{40}$ || "$baseline" == 0000000000000000000000000000000000000000 ]]; then baseline="$(git rev-parse origin/main)"; fi
git show "$baseline:contracts/commercial/generated/catalog.json" > "$out/baseline.json"
export COMMERCIAL_BASELINE="$out/baseline.json"
git diff --quiet "$baseline" HEAD -- server
run_check() {
  name="$1"; shift
  printf '%s\n' "$*" > "$out/$name.command"
  if "$@" > "$out/$name.log" 2>&1; then rc=0; else rc=$?; fi
  printf '%s\n' "$rc" > "$out/$name.exit"
  printf '%s exit=%s\n' "$name" "$rc"
  return "$rc"
}
run_suite() {
  name="$1"; shift
  printf '%s\n' "$*" > "$out/$name.command"
  if "$@" > "$out/$name.jsonl" 2> "$out/$name.stderr"; then rc=0; else rc=$?; fi
  printf '%s\n' "$rc" > "$out/$name.exit"
  printf '%s exit=%s\n' "$name" "$rc"
  return "$rc"
}
run_check resolution make yunka-source-check workspace-check
run_check check-before make check
for pass in 1 2; do
  run_check "generate-$pass" make generate
  run_check "generate-$pass-clean" bash -c 'git diff --exit-code && test -z "$(git status --porcelain)"'
done
run_check check-after make check
run_suite focused go test -timeout=5m -count=1 -json ./internal/commercial/... ./internal/bizruntime ./internal/architecture -run '^TestCE10'
run_suite lease go test -timeout=2m -count=1 -tags=integration -json ./internal/commercial/infrastructure/persistence -run '^TestCE10MySQL'
run_suite mysql go test -timeout=8m -count=1 -tags=integration -json ./integration -run '^TestCE10MySQL'
run_suite race go test -timeout=8m -race -count=1 -tags=integration -json ./integration -run '^TestCE10MySQL'
export CE10_RESTART_RECEIPT="$out/restart-fixture.json"
run_suite restart-before go test -timeout=3m -count=1 -tags=integration -json ./integration -run '^TestCE10PersistenceBeforeRestart$'
before_start="$(docker inspect -f '{{.State.StartedAt}}' "$MYSQL_CONTAINER_ID")"
docker restart "$MYSQL_CONTAINER_ID" > "$out/restart.log" 2>&1
ready=0
for attempt in $(seq 1 60); do
  if docker exec "$MYSQL_CONTAINER_ID" mysqladmin ping -h 127.0.0.1 -proot --silent >> "$out/restart.log" 2>&1; then ready=1; break; fi
  sleep 1
done
test "$ready" = 1
after_start="$(docker inspect -f '{{.State.StartedAt}}' "$MYSQL_CONTAINER_ID")"
test "$before_start" != "$after_start"
printf 'before=%s\nafter=%s\n' "$before_start" "$after_start" >> "$out/restart.log"
printf 'docker restart workflow-owned MySQL; require new StartedAt and healthy database\n' > "$out/restart.command"
printf '0\n' > "$out/restart.exit"
run_suite restart-after go test -timeout=3m -count=1 -tags=integration -json ./integration -run '^TestCE10PersistenceAfterRestart$'
# Group isolation preserves deliberate invalid/custom CE02 fixtures without
# contaminating the Registry assumed by subsequent legacy groups.
for group in CE09 CE08 CE07 CE06 CE05 CE04 CE02 B12; do
  # Reuse the existing workflow database; reset only test-owned fixture tables.
  schema="${YUNKA_TEST_MYSQL_DSN#*)/}"
  schema="${schema%%\?*}"
  tables="$(docker exec "$MYSQL_CONTAINER_ID" mysql -uroot -proot -N -B "$schema" -e "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME LIKE 'biz\_%'")"
  reset_sql="SET FOREIGN_KEY_CHECKS=0;"
  while IFS= read -r table; do
    [[ "$table" =~ ^biz_[A-Za-z0-9_]+$ ]] || continue
    reset_sql+="DROP TABLE $table;"
  done <<< "$tables"
  reset_sql+="SET FOREIGN_KEY_CHECKS=1;"
  docker exec "$MYSQL_CONTAINER_ID" mysql -uroot -proot "$schema" -e "$reset_sql" > "$out/$group-db.log" 2>&1
  pattern="^Test${group}MySQL"
  if [ "$group" = CE02 ]; then pattern='^TestCE02'; fi
  if [ "$group" = B12 ]; then pattern='^Test(B122|B123|B124|B125|B126|AG02OwnerInvariantUsesCurrentReadAfterSnapshot)'; fi
  run_suite "${group,,}-mysql" go test -timeout=5m -count=1 -tags=integration -json ./integration -run "$pattern"
done
run_suite all go test -timeout=5m -count=1 -json ./...
run_check vet go vet ./...
run_check build go build ./...
run_check plan python3 docs/commercial-entitlements/tools/check_plan.py
run_check plan-tests python3 -m unittest discover -s docs/commercial-entitlements/tools -p 'test_*.py' -v
run_check runtime-gate-tests python3 -m unittest discover -s scripts -p 'test_ce04_runtime_closure.py' -v
run_check evidence-tests python3 -m unittest discover -s scripts -p 'test_ce03_*.py' -v
git diff --exit-code
test -z "$(git status --porcelain)"
test -z "$(git -C ../yunka.io status --porcelain)"
git archive HEAD -o "$out/biz-source.tar"
python3 - <<'PY'
import hashlib,json,os,subprocess,sys
from pathlib import Path
sys.path.insert(0,'scripts');from ce03_test_events import summarize
out=Path(os.environ['RUNNER_TEMP'])/'ce10'
task=next(t for t in json.loads(Path('docs/commercial-entitlements/tasks.json').read_text())['tasks'] if t['id']=='CE-10')
summary={'task_id':'CE-10','verified_commit':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),'framework_commit':os.environ['YUNKA_SHA'],'qualification':'PASS','task_status':task['status'],'suites':{},'checks':[]}
suites=['focused','lease','mysql','race','restart-before','restart-after','ce09-mysql','ce08-mysql','ce07-mysql','ce06-mysql','ce05-mysql','ce04-mysql','ce02-mysql','b12-mysql','all']
for name in suites:
    counts=summarize([json.loads(line) for line in (out/(name+'.jsonl')).read_text().splitlines()])
    assert counts['leaf_tests']>0 and counts['failed']==0 and counts['skipped']==0,name
    summary['suites'][name]=counts
for command in sorted(out.glob('*.command')):
    name=command.stem;rc=int((out/(name+'.exit')).read_text());assert rc==0,name
    count=summary['suites'].get(name,{}).get('leaf_tests',{'plan-tests':8,'runtime-gate-tests':25,'evidence-tests':6}.get(name,0))
    # Python counts are also checked against actual unittest output, not merely named.
    if name in ['plan-tests','runtime-gate-tests','evidence-tests']:
        import re
        log=(out/(name+'.log')).read_text();m=re.search(r'Ran (\d+) tests? in',log);assert m and int(m.group(1))==count and '\nOK' in log,name
    summary['checks'].append({'command':command.read_text().strip(),'exit_code':rc,'tests_passed':count,'log':name+('.jsonl' if name in suites else '.log')})
if task['status']=='DONE':
    subprocess.run(['git','fetch','origin','main:refs/remotes/origin/main'],check=True)
    command=['python3','docs/commercial-entitlements/tools/check_round.py','--task','CE-10']
    if os.environ.get('GITHUB_REF')=='refs/heads/main':command.append('--require-main')
    result=subprocess.run(command,text=True,capture_output=True,check=True)
    (out/'round.log').write_text('COMMAND: '+' '.join(command)+'\n'+result.stdout+result.stderr)
    summary['round_check']='PASS'
else:summary['round_check']='QUALIFIED_NOT_DONE'
summary['sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in out.iterdir() if p.is_file()}
(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary,indent=2))
PY
