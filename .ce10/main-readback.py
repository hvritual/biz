from pathlib import Path
import hashlib,json,os,re,subprocess,time,urllib.request
sha=os.environ['TARGET_SHA']; repo='hvritual/biz'; root=Path('biz'); out=Path(os.environ['RUNNER_TEMP'])/'ce10-main-readback'; out.mkdir(parents=True,exist_ok=True)
assert re.fullmatch(r'[0-9a-f]{40}',sha)
def api(path):
 req=urllib.request.Request('https://api.github.com/repos/'+repo+'/'+path,headers={'Authorization':'Bearer '+os.environ['GH_TOKEN'],'Accept':'application/vnd.github+json','X-GitHub-Api-Version':'2022-11-28'})
 for attempt in range(5):
  try:
   with urllib.request.urlopen(req,timeout=30) as r:return json.load(r)
  except Exception:
   if attempt==4:raise
   time.sleep(2*(attempt+1))
def rev(ref):return subprocess.check_output(['git','-C',str(root),'rev-parse',ref],text=True).strip()
assert rev('HEAD')==sha
assert api('git/ref/heads/main')['object']['sha']==sha
terminal=False
for attempt in range(90):
 runs=api('actions/runs?head_sha='+sha+'&event=push&per_page=100')['workflow_runs']
 checks=api('commits/'+sha+'/check-runs?per_page=100')['check_runs']
 expected={'CE-10 qualification','b12-7-runtime-qualification','CE round receipts'}
 names={r['name'] for r in runs}
 assert expected<=names,(expected-names,names)
 failures=[{'kind':'run','name':r['name'],'id':r['id'],'conclusion':r['conclusion']} for r in runs if r['status']=='completed' and r['conclusion']!='success']
 failures += [{'kind':'check','name':r['name'],'id':r['id'],'conclusion':r['conclusion']} for r in checks if r['status']=='completed' and r['conclusion']!='success']
 if failures:
  (out/'failures.json').write_text(json.dumps(failures,indent=2)+'\n'); raise AssertionError(failures)
 if runs and checks and all(r['status']=='completed' and r['conclusion']=='success' for r in runs+checks):terminal=True;break
 print('waiting main',len(runs),len(checks),attempt,flush=True);time.sleep(10)
assert terminal,'main workflow/check terminalization timeout'
(out/'runs.json').write_text(json.dumps(runs,indent=2)+'\n');(out/'checks.json').write_text(json.dumps(checks,indent=2)+'\n')
selected={}
for name in sorted(expected):
 r=max((x for x in runs if x['name']==name),key=lambda x:x['id']); arts=api('actions/runs/'+str(r['id'])+'/artifacts')['artifacts']; selected[name]={'run_id':r['id'],'head_sha':r['head_sha'],'conclusion':r['conclusion'],'artifacts':arts}
(out/'selected.json').write_text(json.dumps(selected,indent=2)+'\n')
subprocess.run(['git','-C',str(root),'fetch','origin','main:refs/remotes/origin/main'],check=True);assert rev('origin/main')==sha
plan=json.loads((root/'docs/commercial-entitlements/tasks.json').read_text());commands=[]
for task in plan['tasks']:
 if task['id']=='CE-10':
  assert task['status']=='VERIFYING';continue
 if task['status']!='DONE':continue
 cmd=['python3','docs/commercial-entitlements/tools/check_round.py','--task',task['id'],'--require-main'];r=subprocess.run(cmd,cwd=root,text=True,capture_output=True);commands.append({'command':' '.join(cmd),'exit_code':r.returncode,'output':r.stdout+r.stderr});assert r.returncode==0,(task['id'],r.stdout,r.stderr)
(out/'rounds.json').write_text(json.dumps(commands,indent=2)+'\n');(out/'rounds.log').write_text('\n'.join('COMMAND: '+x['command']+'\n'+x['output'] for x in commands))
subprocess.run(['git','-C',str(root),'archive','HEAD','-o',str(out/'biz-source.tar')],check=True);assert not subprocess.check_output(['git','-C',str(root),'status','--porcelain'],text=True);assert api('git/ref/heads/main')['object']['sha']==sha
summary={'main_commit':sha,'task_status':'VERIFYING','workflow_count':len(runs),'check_count':len(checks),'result':'PASS','completed_dependency_rounds':[x['command'] for x in commands]};summary['sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in out.iterdir() if p.is_file()};(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');print('CE10_BUSINESS_MAIN_READBACK=PASS',json.dumps(summary,sort_keys=True))
