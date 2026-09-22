#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
out="${RUNNER_TEMP:?}/ce05"
mkdir -p "$out"
[[ "${GITHUB_ACTIONS:-}" == true ]] || { echo 'CI_OWNED_QUALIFICATION_ONLY' >&2; exit 2; }
run_suite() {
 local name="$1"; shift
 python3 scripts/ci_commercial_runtime.py run "$out/$name.jsonl" -- "$@"
}
printf 'biz=%s\nyunka=%s\n' "$(git rev-parse HEAD)" "$(git -C ../yunka.io rev-parse HEAD)" > "$out/sources.txt"
baseline="${BASE_SHA:-}"
if [[ ! "$baseline" =~ ^[0-9a-f]{40}$ || "$baseline" == 0000000000000000000000000000000000000000 ]]; then baseline="$(git rev-parse origin/main)"; fi
git show "$baseline:contracts/commercial/generated/catalog.json" > "$out/baseline.json"
export COMMERCIAL_BASELINE="$out/baseline.json"
make yunka-source-check workspace-check 2>&1 | tee "$out/resolution.log"
make check 2>&1 | tee "$out/check-before.log"
for pass in 1 2; do
 make generate 2>&1 | tee "$out/generate-$pass.log"
 git diff --exit-code
 test -z "$(git status --porcelain)"
done
make check 2>&1 | tee "$out/check-after.log"
run_suite focused go test -count=1 -json ./internal/commercial/... ./internal/architecture -run '^TestCE05'
run_suite race go test -race -count=1 -json ./internal/commercial/enforcement
bash scripts/ci_commercial_mysql.sh wait ce05
run_suite mysql go test -count=1 -tags=integration -json ./integration -run '^TestCE05MySQL'
python3 docs/commercial-entitlements/tools/check_plan.py | tee "$out/plan.log"
python3 -m unittest discover -s docs/commercial-entitlements/tools -p 'test_*.py' -v 2>&1 | tee "$out/plan-tests.log"
python3 -m unittest discover -s scripts -p 'test_ce03_*.py' -v 2>&1 | tee "$out/evidence-tests.log"
python3 - <<'PY'
import hashlib,json,os,subprocess,sys
from pathlib import Path
sys.path.insert(0,'scripts')
from ce03_test_events import summarize
out=Path(os.environ['RUNNER_TEMP'])/'ce05'
task=next(t for t in json.loads(Path('docs/commercial-entitlements/tasks.json').read_text())['tasks'] if t['id']=='CE-05')
summary={'task_id':'CE-05','verified_commit':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),'framework_commit':os.environ['YUNKA_SHA'],'qualification':'PASS','task_status':task['status'],'suites':{}}
from ci_commercial_runtime import verify_output
summary['suites']=verify_output('ce05',out)
summary['tested_tree']=subprocess.check_output(['git','rev-parse','HEAD^{tree}'],text=True).strip()
summary['run_id']=os.environ['GITHUB_RUN_ID']
summary['run_attempt']=os.environ['GITHUB_RUN_ATTEMPT']
contract=json.loads(Path('scripts/ci_proof_contract.json').read_text())
summary['delegates']=contract['gates']['ce05-qualification.yml']['delegates']
summary['delegation_status']='REQUIRES_SAME_CANDIDATE_FULL_GATE'
summary['phase_seconds']={p.stem:float(p.read_text()) for p in out.glob('*.seconds')}
summary['mysql_ready_seconds']=int((out/'mysql.ready').read_text())-int((out/'mysql.started').read_text())
if task['status']=='DONE':
 subprocess.run(['git','fetch','origin','main:refs/remotes/origin/main'],check=True)
 command=['python3','docs/commercial-entitlements/tools/check_round.py','--task','CE-05']
 if os.environ['GITHUB_REF']=='refs/heads/main':command.append('--require-main')
 result=subprocess.run(command,text=True,capture_output=True,check=True)
 (out/'round.log').write_text('COMMAND: '+' '.join(command)+'\n'+result.stdout+result.stderr)
 summary['round_check']='PASS'
else:summary['round_check']='QUALIFIED_NOT_DONE'
summary['sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in out.iterdir() if p.is_file()}
(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary,indent=2))
PY
git diff --exit-code
test -z "$(git status --porcelain)"
test -z "$(git -C ../yunka.io status --porcelain)"
git archive HEAD -o "$out/biz-source.tar"
