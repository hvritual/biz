#!/usr/bin/env bash
# Run from the checked-out biz root against a workflow-owned MySQL service.
set +e
set -uo pipefail
out="$RUNNER_TEMP/ce10-repair"
mkdir -p "$out"
git rev-parse HEAD > "$out/source.txt"
git -C ../yunka.io rev-parse HEAD >> "$out/source.txt"
git archive HEAD -o "$out/biz-source.tar"
run_suite() {
  name="$1"; shift
  printf '%s\n' "$*" > "$out/$name.command"
  "$@" > "$out/$name.jsonl" 2> "$out/$name.stderr"
  echo $? > "$out/$name.exit"
}
make yunka-source-check workspace-check > "$out/workspace.log" 2>&1; echo $? > "$out/workspace.exit"
make check > "$out/check.log" 2>&1; echo $? > "$out/check.exit"
make generate > "$out/generate-1.log" 2>&1; echo $? > "$out/generate-1.exit"
git diff --exit-code > "$out/generate-1-diff.log" 2>&1; echo $? > "$out/generate-1-diff.exit"
make generate > "$out/generate-2.log" 2>&1; echo $? > "$out/generate-2.exit"
git diff --exit-code > "$out/generate-2-diff.log" 2>&1; echo $? > "$out/generate-2-diff.exit"
run_suite focused go test -timeout=2m -count=1 -json ./internal/commercial/... ./internal/bizruntime -run '^TestCE10'
run_suite persistence go test -timeout=2m -count=1 -tags=integration -json ./internal/commercial/infrastructure/persistence -run '^TestCE10MySQL'
run_suite mysql go test -timeout=8m -count=1 -tags=integration -json ./integration -run '^TestCE10MySQL'
run_suite race go test -timeout=8m -race -count=1 -tags=integration -json ./integration -run '^TestCE10MySQL'
# Give legacy groups separate schemas: deliberate CE02 custom catalog rows
# must not leak into another group's production-registry fixture.
for group in CE09 CE08 CE07 CE06 CE05 CE04 CE02 B12; do
  schema="ce10_reg_${group,,}"
  docker exec "$MYSQL_CONTAINER_ID" mysql -uroot -proot -e "CREATE DATABASE $schema" > "$out/$group-db.log" 2>&1
  created=$?; echo "$created" > "$out/$group-db.exit"
  if [ "$created" -ne 0 ]; then continue; fi
  export YUNKA_TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/$schema?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true"
  pattern="^Test${group}MySQL"
  if [ "$group" = CE02 ]; then pattern='^TestCE02'; fi
  if [ "$group" = B12 ]; then pattern='^Test(B122|B123|B124|B125|B126|AG02OwnerInvariantUsesCurrentReadAfterSnapshot)'; fi
  run_suite "$group" go test -timeout=5m -count=1 -tags=integration -json ./integration -run "$pattern"
done
run_suite all go test -timeout=5m -count=1 -json ./...
go vet ./... > "$out/vet.log" 2>&1; echo $? > "$out/vet.exit"
go build ./... > "$out/build.log" 2>&1; echo $? > "$out/build.exit"
python3 - <<'PY'
import hashlib,json,os,sys
from pathlib import Path
sys.path.insert(0,'scripts');from ce03_test_events import summarize
out=Path(os.environ['RUNNER_TEMP'])/'ce10-repair'
result={'candidate':(out/'source.txt').read_text().splitlines()[0],'framework':os.environ['YUNKA_SHA'],'run_id':os.environ['GITHUB_RUN_ID'],'suites':{},'commands':{}}
ok=True
for name in ['focused','persistence','mysql','race','CE09','CE08','CE07','CE06','CE05','CE04','CE02','B12','all']:
    path=out/(name+'.jsonl');events=[json.loads(l) for l in path.read_text().splitlines()] if path.exists() else []
    rc=int((out/(name+'.exit')).read_text()) if (out/(name+'.exit')).exists() else -1
    try: counts=summarize(events)
    except ValueError:
        counts={'failed_test_events':[e.get('Test') for e in events if e.get('Action')=='fail' and e.get('Test')], 'package_failures':[e.get('Package') for e in events if e.get('Action')=='fail' and not e.get('Test')], 'passed_test_events':sum(e.get('Action')=='pass' and bool(e.get('Test')) for e in events)}
        ok=False
    result['suites'][name]={'exit_code':rc,**counts}
    ok=ok and rc==0 and counts.get('leaf_tests',0)>0 and counts.get('failed',1)==0 and counts.get('skipped',1)==0
for p in out.glob('*.exit'):
    rc=int(p.read_text());result['commands'][p.stem]=rc;ok=ok and rc==0
result['result']='PASS' if ok else 'FAIL'
result['sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in out.iterdir() if p.is_file()}
(out/'summary.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(result,indent=2));sys.exit(0 if ok else 1)
PY
