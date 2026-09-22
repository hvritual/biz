#!/usr/bin/env python3
"""Full Gate proof ownership, bounded legacy debt and API-backed execution audit.

Static command inventory is conservative diagnostics, not a shell equivalence
proof. Real qualification is still owned by the existing domain gates. This
module never removes tests, changes their race scope, or manufactures a success.
"""
from __future__ import annotations

import argparse
from collections import Counter
from datetime import datetime, timezone
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import zipfile

ROOT = Path(__file__).resolve().parents[1]
CONTRACT = 'scripts/ci_proof_contract.json'
TOPOLOGY = 'scripts/ci_topology_contract.json'
RECEIPT = 'ci-proof-execution.json'
PROTECTED = (CONTRACT, 'scripts/ci_proof_governance.py', 'scripts/test_ci_proof_governance.py',
             'scripts/ci_commercial_baseline.json', 'scripts/ci_commercial_runtime.py',
             'scripts/test_ci_commercial_runtime.py', 'scripts/check_ci_commercial.py',
             'scripts/ci_commercial_mysql.sh')
RUNTIMES = {'go-mysql', 'go', 'browser', 'mixed', 'static'}
RULES = {'go.all.test', 'go.all.vet', 'go.all.build', 'generation.check',
         'generation.generate', 'web.fast', 'bootstrap.go-cache-disabled',
         'bootstrap.service-mysql', 'bootstrap.headed-chromium'} | {
         f'overlap.ce{n:02d}.mysql' for n in [2, 4, 5, 6, 7, 8, 9, 10, 12, 13]}


class Violation(ValueError):
    pass


def require(ok, reason):
    if not ok:
        raise Violation(reason)


def strict_json(raw):
    def pairs(items):
        out = {}
        for key, value in items:
            require(key not in out, 'DUPLICATE_JSON_KEY:' + key)
            out[key] = value
        return out
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda x: (_ for _ in ()).throw(Violation('NONFINITE_JSON:' + x)))


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def read(root, path):
    p = root / path
    require(not Path(path).is_absolute() and '..' not in Path(path).parts,
            'UNSAFE_SOURCE_PATH:' + path)
    require(p.is_file() and p.resolve().is_relative_to(root.resolve()), 'SOURCE_MISSING_OR_ESCAPED:' + path)
    return p.read_text(encoding='utf-8')


def job_ids(text):
    """Accept the repository's literal, non-matrix workflow job declarations."""
    require('\njobs:\n' in '\n' + text, 'JOBS_SECTION_MISSING')
    body = text.split('jobs:\n', 1)[1]
    names = re.findall(r'^  ([\w-]+):\s*$', body, re.M)
    require(names and len(names) == len(set(names)), 'JOB_IDS_EMPTY_OR_DUPLICATE')
    require(not re.search(r'^    (strategy|name):', body, re.M), 'DYNAMIC_JOB_NAMES_REQUIRE_CONTRACT_MIGRATION')
    return names


def sources(root, workflow):
    """Follow literal repository script references. Never execute inspected code."""
    pending = ['.github/workflows/' + workflow]
    found = {}
    pattern = r'(?<![\w-])(?:biz/)?((?:scripts|\.github/scripts|docs/[\w/-]+/tools)/[\w./-]+\.(?:sh|py|mjs))'
    while pending:
        path = pending.pop()
        if path in found:
            continue
        text = read(root, path)
        found[path] = text
        for ref in re.findall(pattern, text):
            if (root / ref).is_file() and ref not in found:
                pending.append(ref)
    return found


def inventory(root, workflow):
    found = sources(root, workflow)
    counts, races = Counter(), set()
    for path, text in found.items():
        if path.endswith('.yml'):
            counts['bootstrap.go-cache-disabled'] += len(re.findall(r'cache:\s*false\b', text))
            counts['bootstrap.service-mysql'] += int('services:' in text and 'mysql:' in text)
        # Join shell continuations. String/AST inference is intentionally not used
        # to claim two business tests equivalent or to infer absence of races.
        text = text.replace('\\\n', ' ')
        for line in text.splitlines():
            if not line.strip() or line.lstrip().startswith('#'):
                continue
            try:
                tokens = shlex.split(line.strip(), comments=True)
            except ValueError:
                continue  # inventory only; shell syntax remains a domain check
            if tokens[:1] == ['run:']:
                tokens = tokens[1:]
            if 'go' in tokens:
                tail = tokens[tokens.index('go') + 1:]
                if tail[:1] == ['-C']:
                    tail = tail[2:]
                end = next((i for i, v in enumerate(tail) if v in {'|', '&&', ';'}), len(tail))
                tail = tail[:end]
                if tail and tail[0] in {'test', 'vet', 'build'}:
                    if './...' in tail:
                        counts['go.all.' + tail[0]] += 1
                    selector = next((tail[i + 1] for i, x in enumerate(tail[:-1]) if x == '-run'), '')
                    selector = next((x.split('=', 1)[1] for x in tail if x.startswith('-run=')), selector)
                    match = re.fullmatch(r'\^TestCE(\d{2})(?:MySQL)?\$?', selector)
                    if tail[0] == 'test' and match and '-race' not in tail:
                        code = match.group(1)
                        owner = 'ce02-bootstrap.yml' if code == '02' else 'ce' + code + '-qualification.yml'
                        rule = 'overlap.ce' + code + '.mysql'
                        if workflow != owner and rule in RULES:
                            counts[rule] += 1
                    if tail[0] == 'test' and '-race' in tail:
                        races.add('go ' + ' '.join(tail))
            for command, rule in [('make check', 'generation.check'), ('make generate', 'generation.generate'),
                                  ('npm run check', 'web.fast')]:
                if re.search(r'(?<![\w\x22\x27])' + re.escape(command) + r'(?:\s|$)', line):
                    counts[rule] += 1
            if 'playwright install' in line and 'chromium' in line and '--only-shell' not in line:
                counts['bootstrap.headed-chromium'] += 1
    return {'cost_candidates': {k: v for k, v in sorted(counts.items()) if v},
            'race_commands': sorted(races), 'source_sha256': {p: sha(t.encode()) for p, t in sorted(found.items())}}


def validate(contract, topology, root):
    require(set(contract) == {'schema_version', 'baseline_main', 'metric', 'legacy_debt_expires',
                              'governance_branch_prefix', 'gates', 'prerequisites'}, 'CONTRACT_FIELDS_INVALID')
    require(type(contract['schema_version']) is int and contract['schema_version'] == 1, 'SCHEMA_INVALID')
    require(re.fullmatch(r'[0-9a-f]{40}', contract['baseline_main']) is not None, 'BASELINE_SHA_INVALID')
    require(contract['metric'] == 'github_job_wall_seconds', 'PERFORMANCE_METRIC_INVALID')
    require(contract['governance_branch_prefix'] == 'chore/ci-proof-', 'GOVERNANCE_PREFIX_INVALID')
    require(datetime.now(timezone.utc).date().isoformat() <= contract['legacy_debt_expires'], 'LEGACY_DEBT_EXPIRED')
    full = topology['full_merge_gate']
    workflows = full['expected_workflows']
    require(len(workflows) == full['expected_units'] == 35 and len(set(workflows)) == 35, 'CANONICAL_35_TOPOLOGY_INVALID')
    gates = contract['gates']
    require(isinstance(gates, dict) and set(gates) == set(workflows), 'GATE_CONTRACT_SET_MISMATCH')
    require(contract['prerequisites'] == {'repository.web.fast': {'workflow': 'candidate-qualification.yml',
             'caller': 'fast-web', 'jobs': ['qualify']}}, 'FAST_GATE_AUTHORITY_INVALID')
    owners = {'repository.web.fast': 'PR Qualification'}
    observations = {}
    fields = {'owns', 'delegates', 'runtime', 'jobs', 'target_seconds', 'hard_seconds',
              'race_commands', 'restart', 'legacy_cost_ceiling'}
    for workflow, gate in gates.items():
        require(set(gate) == fields, 'GATE_FIELDS_INVALID:' + workflow)
        require(gate['runtime'] in RUNTIMES, 'RUNTIME_INVALID:' + workflow)
        require(gate['jobs'] == job_ids(read(root, '.github/workflows/' + workflow)), 'JOB_SET_DRIFT:' + workflow)
        require(type(gate['target_seconds']) is int and type(gate['hard_seconds']) is int and
                0 < gate['target_seconds'] < gate['hard_seconds'] <= 300, 'BUDGET_INVALID:' + workflow)
        require(gate['restart'] in {'none', 'process-restart', 'durable-container', 'isolated-durable-container'}, 'RESTART_CONTRACT_INVALID')
        for field in ['owns', 'delegates', 'race_commands']:
            values = gate[field]
            require(isinstance(values, list) and all(isinstance(x, str) and x for x in values) and
                    len(values) == len(set(values)), 'INVALID_LIST:' + workflow + ':' + field)
        require(gate['owns'], 'OWNER_EMPTY:' + workflow)
        for proof in gate['owns']:
            require(re.fullmatch(r'[a-z0-9][a-z0-9._-]+', proof) is not None, 'PROOF_ID_INVALID:' + proof)
            require(proof not in owners, 'DUPLICATE_PROOF_OWNER:' + proof)
            owners[proof] = workflow
        obs = inventory(root, workflow)
        require(gate['race_commands'] == obs['race_commands'], 'RACE_SCOPE_DRIFT:' + workflow)
        ceiling = gate['legacy_cost_ceiling']
        require(isinstance(ceiling, dict) and set(ceiling) <= RULES and
                all(type(v) is int and v >= 0 for v in ceiling.values()), 'LEGACY_CEILING_INVALID')
        for rule, count in obs['cost_candidates'].items():
            require(count <= ceiling.get(rule, 0), 'NEW_BOOTSTRAP_OR_DUPLICATE_COST:' + workflow + ':' + rule)
        corpus = '\n'.join(sources(root, workflow).values())
        has_restart = 'docker restart' in corpus
        require(has_restart == (gate['restart'] in {'durable-container', 'isolated-durable-container'}), 'RESTART_SCOPE_DRIFT:' + workflow)
        if gate['restart'] == 'process-restart':
            require('CE16-restart' in corpus and 'qualify-evolution-mysql.py' in corpus, 'PROCESS_RESTART_PROOF_MISSING')
        if gate['restart'] == 'isolated-durable-container':
            require('restart_container' in corpus and 'fast_container' in corpus, 'RESTART_ISOLATION_MISSING')
        observations[workflow] = obs
    for workflow, gate in gates.items():
        for proof in gate['delegates']:
            require(proof in owners and owners[proof] != workflow, 'DELEGATION_WITHOUT_TERMINAL_OWNER:' + proof)
    return owners, observations


def check_base(root, base_ref, contract):
    if not base_ref:
        return
    require(re.fullmatch(r'[0-9a-f]{40}', base_ref) is not None, 'BASE_REF_NOT_EXACT')
    before = subprocess.run(['git', 'show', base_ref + ':' + CONTRACT], cwd=root,
                            capture_output=True, text=True, timeout=20)
    governance = os.getenv('GITHUB_HEAD_REF', '').startswith(contract['governance_branch_prefix'])
    if before.returncode:
        require(governance and base_ref == contract['baseline_main'], 'UNAPPROVED_CONTRACT_BOOTSTRAP')
        return
    old = strict_json(before.stdout)
    if not governance:
        for path in PROTECTED:
            prior = subprocess.run(['git', 'show', base_ref + ':' + path], cwd=root,
                                   capture_output=True, timeout=20)
            require(prior.returncode == 0 and prior.stdout == (root / path).read_bytes(),
                    'PROOF_CONTROL_CHANGE_REQUIRES_GOVERNANCE:' + path)
    else:
        # The routine migration path may only ratchet debt downward. Raising a
        # ceiling requires an explicit contract version/policy review, not a retry.
        for workflow, gate in contract['gates'].items():
            if workflow in old['gates']:
                prior = old['gates'][workflow]
                require(gate['hard_seconds'] <= prior['hard_seconds'], 'HARD_BUDGET_INCREASE:' + workflow)
                for rule, count in gate['legacy_cost_ceiling'].items():
                    require(count <= prior['legacy_cost_ceiling'].get(rule, 0), 'LEGACY_DEBT_INCREASE:' + workflow)


def hook_check(root):
    hooks = {'pr-qualification.yml': ['ci_proof_governance.py check', 'test_ci_proof_governance.py'],
             'pr-merge-gate.yml': ['ci_proof_governance.py audit-run', 'ci-proof-execution.json'],
             'main-receipt.yml': ['ci_proof_governance.py verify-main']}
    for path, markers in hooks.items():
        text = read(root, '.github/workflows/' + path)
        require(all(x in text for x in markers), 'PROOF_HOOK_MISSING:' + path)
    text = read(root, '.github/workflows/pr-merge-gate.yml')
    require(text.index('ci_proof_governance.py audit-run') < text.index('delivery_execution.py merge-ready'),
            'PROOF_AUDIT_MUST_PRECEDE_MERGE_READY')


def expected_full(contract, topology):
    return {f"full-{i:02d}-{workflow[:-4]} / {job}": workflow
            for i, workflow in enumerate(topology['full_merge_gate']['expected_workflows'], 1)
            for job in contract['gates'][workflow]['jobs']}


def seconds(start, end):
    require(isinstance(start, str) and isinstance(end, str), 'TIMESTAMP_MISSING')
    a, b = (datetime.fromisoformat(v.replace('Z', '+00:00')) for v in (start, end))
    require(a.tzinfo is not None and b.tzinfo is not None and b >= a, 'TIMESTAMP_INVALID')
    return round((b - a).total_seconds(), 3)


def audit_jobs(contract, topology, jobs, run_id, attempt, candidate):
    expected = expected_full(contract, topology)
    actual = [j for j in jobs if j.get('name', '').startswith('full-')]
    require(len(actual) == len(expected) and {j['name'] for j in actual} == set(expected), 'FULL_EXPANDED_JOB_SET_MISMATCH')
    rows = []
    for job in actual:
        name, workflow = job['name'], expected[job['name']]
        require(str(job.get('run_id')) == str(run_id) and job.get('run_attempt') == attempt and
                job.get('head_sha') == candidate, 'JOB_SOURCE_BINDING_MISMATCH:' + name)
        require(job.get('status') == 'completed' and job.get('conclusion') == 'success', 'REQUIRED_JOB_NOT_SUCCESS:' + name)
        wall = seconds(job.get('started_at'), job.get('completed_at'))
        queue = seconds(job.get('created_at'), job.get('started_at'))
        budget = contract['gates'][workflow]
        state = 'HARD_EXCEEDED' if wall > budget['hard_seconds'] else 'TARGET_EXCEEDED' if wall > budget['target_seconds'] else 'PASS'
        rows.append({'job_id': job['id'], 'job': name, 'workflow': workflow,
                     'job_wall_seconds': wall, 'queue_seconds': queue, 'performance': state,
                     'target_seconds': budget['target_seconds'], 'hard_seconds': budget['hard_seconds']})
    rows.sort(key=lambda row: (-row['job_wall_seconds'], row['job']))
    return rows


def check_fast_gate(api, run, contract, candidate):
    spec = contract['prerequisites']['repository.web.fast']
    jobs = api.jobs(run)
    expected = {spec['caller'] + ' / ' + job for job in spec['jobs']}
    found = [j for j in jobs if j.get('name') in expected]
    require(len(found) == len(expected), 'DELEGATED_FAST_GATE_MISSING')
    require(all(j.get('head_sha') == candidate and j.get('run_id') == run['id'] and
                j.get('run_attempt') == run['run_attempt'] and j.get('conclusion') == 'success' and
                j.get('status') == 'completed' for j in found), 'DELEGATED_FAST_GATE_NOT_EXECUTED')


def receipt_zip(data):
    require(len(data) <= 2 * 1024 * 1024, 'RECEIPT_ARCHIVE_TOO_LARGE')
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        require(archive.namelist() == [RECEIPT] and archive.getinfo(RECEIPT).file_size <= 2 * 1024 * 1024,
                'PROOF_RECEIPT_ARCHIVE_INVALID')
        return strict_json(archive.read(RECEIPT))


def verify_binding(receipt, repository, contract_hash, topology_hash, candidate, tree, pr, run, qualification):
    required = {'schema_version': 1, 'state': 'VERIFIED', 'evidence_source': 'github_api',
                'repository': repository, 'contract_sha256': contract_hash, 'topology_sha256': topology_hash,
                'candidate_sha': candidate, 'candidate_tree': tree, 'pr_number': pr,
                'run_id': str(run['id']), 'run_attempt': run['run_attempt'],
                'qualification_run_id': str(qualification['id']),
                'qualification_run_attempt': qualification['run_attempt']}
    for key, value in required.items():
        require(type(receipt.get(key)) is type(value) and receipt[key] == value, 'PROOF_RECEIPT_BINDING:' + key)


def api_audit(api, contract, topology, repository, candidate, pr, run_id, attempt):
    from delivery_execution import bound_refs, assert_single_full, latest_success, run_binds_pr
    delivery = strict_json(read(ROOT, 'scripts/delivery_execution_contract.json'))
    print('CI_PROOF_PROGRESS=bind_candidate', flush=True)
    bound = bound_refs(api, pr, candidate)
    run = api.get('/actions/runs/' + str(run_id))
    require(run.get('head_sha') == candidate and run.get('run_attempt') == attempt and
            run.get('path') == delivery['merge_workflow'] and run_binds_pr(run, pr), 'FULL_RUN_BINDING_MISMATCH')
    runs = api.runs(candidate)
    assert_single_full(runs, delivery, candidate, pr, run_id, attempt)
    qualification = latest_success(runs, delivery['qualification_workflow'], candidate, pr)
    print('CI_PROOF_PROGRESS=verify_delegated_fast_gate', flush=True)
    check_fast_gate(api, qualification, contract, candidate)
    # A reusable workflow can check out github.sha (synthetic merge SHA). It is
    # acceptable only when GitHub's referenced source tree equals the candidate.
    refs = {r['sha'] for r in run.get('referenced_workflows', [])}
    require(refs, 'WORKFLOW_SOURCE_REFERENCES_MISSING')
    for ref in refs:
        require(api.get('/git/commits/' + ref)['tree']['sha'] == bound['candidate_tree'], 'WORKFLOW_SOURCE_TREE_MISMATCH')
    print('CI_PROOF_PROGRESS=verify_all_expanded_jobs', flush=True)
    rows = audit_jobs(contract, topology, api.jobs(run), run_id, attempt, candidate)
    require(bound_refs(api, pr, candidate) == bound, 'FROZEN_BINDING_CHANGED_DURING_AUDIT')
    return {**bound, 'repository': repository, 'evidence_source': 'github_api', 'run_id': str(run_id),
            'run_attempt': attempt, 'qualification_run_id': str(qualification['id']),
            'qualification_run_attempt': qualification['run_attempt'], 'jobs': rows,
            'canonical_units': 35, 'expanded_jobs': len(rows),
            'proof_owners': {p: w for w, g in contract['gates'].items() for p in g['owns']},
            'hard_violations': [r['job'] for r in rows if r['performance'] == 'HARD_EXCEEDED'],
            'target_violations': [r['job'] for r in rows if r['performance'] == 'TARGET_EXCEEDED']}


def verify_main(api, contract, topology, repository, main_sha, contract_hash, topology_hash):
    from delivery_execution import latest_success, assert_single_full
    delivery = strict_json(read(ROOT, 'scripts/delivery_execution_contract.json'))
    require(api.get('/git/ref/heads/main')['object']['sha'] == main_sha, 'MAIN_TIP_CHANGED')
    pulls = [p for p in api.pages('/commits/' + main_sha + '/pulls') if p.get('merged_at') and
             p.get('merge_commit_sha') == main_sha and p.get('base', {}).get('ref') == 'main']
    require(len(pulls) == 1, 'MAIN_PR_BINDING_MISSING')
    pull = pulls[0]
    candidate, pr = pull['head']['sha'], pull['number']
    tree = api.get('/git/commits/' + candidate)['tree']['sha']
    require(tree == api.get('/git/commits/' + main_sha)['tree']['sha'], 'MAIN_CANDIDATE_TREE_MISMATCH')
    runs = api.runs(candidate)
    run = latest_success(runs, delivery['merge_workflow'], candidate, pr)
    qualification = latest_success(runs, delivery['qualification_workflow'], candidate, pr)
    assert_single_full(runs, delivery, candidate, pr, run['id'], run['run_attempt'])
    check_fast_gate(api, qualification, contract, candidate)
    name = f"ci-proof-{run['id']}-{run['run_attempt']}"
    artifacts = [a for a in api.pages(f"/actions/runs/{run['id']}/artifacts", 'artifacts') if a.get('name') == name]
    require(len(artifacts) == 1 and not artifacts[0].get('expired'), 'PROOF_RECEIPT_MISSING_OR_EXPIRED')
    artifact = artifacts[0]
    data = api.raw(api.prefix + f"/actions/artifacts/{artifact['id']}/zip", cap=2 * 1024 * 1024)
    require(artifact.get('digest') == 'sha256:' + sha(data), 'PROOF_ARCHIVE_DIGEST_MISMATCH')
    receipt = receipt_zip(data)
    verify_binding(receipt, repository, contract_hash, topology_hash, candidate, tree, pr, run, qualification)
    rows = audit_jobs(contract, topology, api.jobs(run), run['id'], run['run_attempt'], candidate)
    require(receipt.get('jobs') == rows and not any(r['performance'] == 'HARD_EXCEEDED' for r in rows), 'PROOF_EXECUTION_DRIFT')
    require(api.get('/git/ref/heads/main')['object']['sha'] == main_sha, 'MAIN_TIP_CHANGED')
    return {**receipt, 'state': 'MAIN_VERIFIED', 'main_sha': main_sha, 'artifact_id': artifact['id']}


def emit(report, output):
    path = Path(output)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(report, indent=2, sort_keys=True) + '\n')
    print('CI_PROOF_GOVERNANCE=' + json.dumps({k: report[k] for k in report if k not in {'inventory', 'jobs', 'proof_owners'}}, sort_keys=True), flush=True)
    summary = os.getenv('GITHUB_STEP_SUMMARY')
    if summary:
        with open(summary, 'a', encoding='utf-8') as handle:
            handle.write('\n## Proof Governance\n\nState: **' + report['state'] + '**\n\n')
            handle.write('Job wall time includes bootstrap; queue time is separate. Legacy debt is not an optimization success.\n\n')
            for row in report.get('jobs', [])[:8]:
                handle.write(f"- {row['job']}: {row['job_wall_seconds']}s; queue {row['queue_seconds']}s; {row['performance']}\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['check', 'inventory', 'audit-run', 'verify-main'])
    parser.add_argument('--base-ref', default='')
    parser.add_argument('--output', default=str(Path(os.getenv('RUNNER_TEMP', '.')) / RECEIPT))
    args = parser.parse_args()
    report = {'schema_version': 1, 'state': 'BLOCKED', 'stage': args.command,
              'observed_at': datetime.now(timezone.utc).isoformat()}
    try:
        contract_raw, topology_raw = read(ROOT, CONTRACT), read(ROOT, TOPOLOGY)
        contract, topology = strict_json(contract_raw), strict_json(topology_raw)
        owners, observations = validate(contract, topology, ROOT)
        report.update(contract_sha256=sha(contract_raw.encode()), topology_sha256=sha(topology_raw.encode()),
                      canonical_units=len(contract['gates']), owned_proofs=len(owners),
                      legacy_cost_findings=sum(sum(v['cost_candidates'].values()) for v in observations.values()))
        if args.command in {'check', 'inventory'}:
            hook_check(ROOT)
            check_base(ROOT, args.base_ref, contract)
            report.update(state='CONTRACT_VERIFIED', inventory=observations)
        else:
            from delivery_execution import API
            limits = strict_json(read(ROOT, 'scripts/delivery_execution_contract.json'))['limits']
            repository = os.environ['GITHUB_REPOSITORY']
            api = API(repository, os.getenv('GH_TOKEN') or os.getenv('GITHUB_TOKEN'), limits)
            if args.command == 'audit-run':
                result = api_audit(api, contract, topology, repository, os.environ['CANDIDATE_SHA'],
                                   int(os.environ['PR_NUMBER']), os.environ['GITHUB_RUN_ID'], int(os.environ['GITHUB_RUN_ATTEMPT']))
                report.update(result)
                require(not result['hard_violations'], 'PERFORMANCE_HARD_BUDGET_EXCEEDED')
                report['state'] = 'VERIFIED'
            else:
                report.update(verify_main(api, contract, topology, repository, os.environ['MAIN_SHA'],
                                          report['contract_sha256'], report['topology_sha256']))
        emit(report, args.output)
        return 0
    except Exception as exc:
        report.update(state='BLOCKED', reason=str(exc), error_type=type(exc).__name__)
        emit(report, args.output)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
