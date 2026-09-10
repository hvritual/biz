"""Read a fixed, latest biz main; retain actual Actions, sources and gates."""
from pathlib import Path
import hashlib,json,os,re,subprocess,time,urllib.request

sha=os.environ['TARGET_SHA']
assert re.fullmatch(r'[0-9a-f]{40}',sha)
repo='hvritual/biz'
out=Path(os.environ['RUNNER_TEMP'])/'ce09-readback';out.mkdir(exist_ok=True)
root=Path('biz')
def revision(ref):
 return subprocess.check_output(['git','-C',str(root),'rev-parse',ref],text=True).strip()
def api(path):
 request=urllib.request.Request('https://api.github.com/repos/'+repo+'/'+path,headers={'Authorization':'Bearer '+os.environ['GH_TOKEN'],'Accept':'application/vnd.github+json','X-GitHub-Api-Version':'2022-11-28'})
 for attempt in range(4):
  try:
   with urllib.request.urlopen(request,timeout=30) as r:return json.load(r)
  except Exception:
   if attempt==3:raise
   time.sleep(2*(attempt+1))
assert revision('HEAD')==sha
assert api('git/ref/heads/main')['object']['sha']==sha
terminal=False
for attempt in range(60):
 runs=api('actions/runs?head_sha='+sha+'&event=push&per_page=100')['workflow_runs']
 checks=api('commits/'+sha+'/check-runs?per_page=100')['check_runs']
 expected={'CE-09 qualification','b12-7-runtime-qualification','CE round receipts'}
 names={r['name'] for r in runs}
 assert all(r['head_sha']==sha and r['head_branch']=='main' for r in runs)
 failures=[r for r in runs if r['status']=='completed' and r['conclusion']!='success']
 failures += [r for r in checks if r['status']=='completed' and r['conclusion']!='success']
 if failures:
  (out/'failed-checks.json').write_text(json.dumps(failures,indent=2)+'\n')
  raise AssertionError('terminal main failure; retain evidence and repair, do not mark DONE')
 if expected<=names and checks and all(r['status']=='completed' and r['conclusion']=='success' for r in runs+checks):terminal=True;break
 print('main checks still running; observed',len(runs),len(checks),flush=True)
 time.sleep(10)
assert terminal,'main qualifications not terminal within bounded readback'
(out/'runs.json').write_text(json.dumps(runs,indent=2)+'\n')
(out/'checks.json').write_text(json.dumps(checks,indent=2)+'\n')
selected={}
for name in sorted(expected):
 candidates=[r for r in runs if r['name']==name]
 chosen=max(candidates,key=lambda r:r['id'])
 artifacts=api('actions/runs/'+str(chosen['id'])+'/artifacts')['artifacts']
 selected[name]={'run_id':chosen['id'],'head_sha':chosen['head_sha'],'conclusion':chosen['conclusion'],'artifacts':artifacts}
(out/'qualification-artifacts.json').write_text(json.dumps(selected,indent=2)+'\n')
subprocess.run(['git','-C',str(root),'fetch','origin','main:refs/remotes/origin/main'],check=True)
assert revision('origin/main')==sha
plan=json.loads((root/'docs/commercial-entitlements/tasks.json').read_text())
commands=[]
for task in plan['tasks']:
 if task['status']!='DONE':continue
 cmd=['python3','docs/commercial-entitlements/tools/check_round.py','--task',task['id'],'--require-main']
 r=subprocess.run(cmd,cwd=root,text=True,capture_output=True)
 commands.append({'command':' '.join(cmd),'exit_code':r.returncode,'output':r.stdout+r.stderr})
 assert r.returncode==0,(task['id'],r.stdout,r.stderr)
(out/'rounds.json').write_text(json.dumps(commands,indent=2)+'\n')
(out/'rounds.log').write_text('\n'.join('COMMAND: '+r['command']+'\n'+r['output'] for r in commands))
ce09=next(t for t in plan['tasks'] if t['id']=='CE-09')
if os.environ.get('REQUIRE_CE09_DONE')=='1':assert ce09['status']=='DONE'
subprocess.run(['git','-C',str(root),'archive','HEAD','-o',str(out/'biz-source.tar')],check=True)
assert not subprocess.check_output(['git','-C',str(root),'status','--porcelain'],text=True)
assert api('git/ref/heads/main')['object']['sha']==sha
summary={'main_commit':sha,'task_status':ce09['status'],'workflow_count':len(runs),'check_count':len(checks),'result':'PASS','completed_rounds':[r['command'] for r in commands]}
summary['sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in out.iterdir() if p.is_file()}
(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
print('CE09_MAIN_READBACK=PASS',json.dumps(summary,sort_keys=True))
