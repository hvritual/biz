#!/usr/bin/env python3
"""One live observation of a bound PR; output the next action, never keep polling a terminal run."""
from __future__ import annotations
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import sys
from delivery_execution import API, matching_runs, run_ref, root_causes, require, digest
from delivery_execution import verify_receipt, read_receipt_zip, expected_jobs, assert_single_full, verify_main

ROOT = Path(__file__).resolve().parents[1]
QUALIFICATION = '.github/workflows/pr-qualification.yml'
MERGE = '.github/workflows/pr-merge-gate.yml'


def derive(pull, files, runs, jobs, issue, ui_required=True):
    require(re.search(r'(?im)^\s*(?:refs|fixes|closes|resolves)\s+#' + str(issue) + r'\b', pull.get('body') or ''), 'ISSUE_PR_BINDING_MISSING')
    candidate = pull['head']['sha']
    changed = sorted(f['filename'] for f in files if f.get('status') != 'removed')
    ui = [p for p in changed if p.startswith('web/src/features/') and p.endswith('.vue')]
    ui_tests = [p for p in changed if p.startswith(('web/e2e/', 'web/tests/')) and p.endswith(('.spec.ts', '.test.ts'))]
    report = {
        'issue': issue, 'pr': pull['number'], 'candidate_sha': candidate,
        'base_sha': pull['base']['sha'], 'changed_files': changed,
        'ui_source_committed': ui, 'ui_tests_committed': ui_tests,
        'ui_state': 'COMMITTED_UNVERIFIED' if ui else 'NO_INCREMENTAL_UI_COMMIT',
        'baseline_ui_is_not_reset_or_counted_as_new_work': True,
        'state': 'NEEDS_QUALIFICATION', 'next_action': 'inspect_missing_candidate_qualification',
    }
    if pull.get('merged') or pull.get('merged_at'):
        return {**report, 'state': 'MERGED_PENDING_MAIN_VERIFICATION', 'next_action': 'verify_exact_main_receipt'}
    found = matching_runs(runs, QUALIFICATION, candidate, pull['number'])
    if not found:
        return report
    q = found[0]
    report['qualification'] = run_ref(q)
    if q.get('status') != 'completed':
        return {**report, 'state': 'QUALIFICATION_RUNNING', 'next_action': 'observe_nonterminal_run_with_budget'}
    if q.get('conclusion') != 'success':
        return {**report, 'state': 'QUALIFICATION_FAILED', 'next_action': 'collect_terminal_failure_and_repair',
                'root_causes': root_causes(jobs.get(q['id'], []))}
    if not jobs.get(q['id']):
        return {**report, 'state': 'QUALIFICATION_EVIDENCE_MISSING', 'next_action': 'fetch_actual_jobs_not_assume_success'}
    if ui_required and (not ui or not ui_tests):
        return {**report, 'state': 'BACKEND_QUALIFIED_UI_INCOMPLETE', 'next_action': 'implement_missing_ui_and_acceptance'}
    full = matching_runs(runs, MERGE, candidate, pull['number'])
    if not full:
        return {**report, 'state': 'QUALIFIED', 'next_action': 'freeze_exact_candidate_and_mark_ready'}
    full = full[0]
    report['merge_gate'] = run_ref(full)
    if full.get('status') != 'completed':
        return {**report, 'state': 'FULL_GATE_RUNNING', 'next_action': 'observe_nonterminal_run_with_budget'}
    if full.get('conclusion') != 'success':
        return {**report, 'state': 'FULL_GATE_FAILED', 'next_action': 'collect_terminal_failure_and_repair',
                'root_causes': root_causes(jobs.get(full['id'], []))}
    return {**report, 'state': 'FULL_GATE_SUCCEEDED_PENDING_RECEIPT', 'next_action': 'verify_exact_merge_receipt'}



def main_run_state(runs, main_sha):
    selected = sorted((run for run in runs if run.get('head_sha') == main_sha
                       and run.get('event') == 'push'
                       and run.get('path', '').split('@')[0] == '.github/workflows/main-receipt.yml'),
                      key=lambda run: (int(run['id']), int(run.get('run_attempt', 1))), reverse=True)
    if not selected:
        return 'MAIN_QUALIFICATION_MISSING', None
    run = selected[0]
    if run.get('status') != 'completed':
        return 'MAIN_QUALIFICATION_RUNNING', run
    if run.get('conclusion') != 'success':
        return 'MAIN_QUALIFICATION_FAILED', run
    return 'MAIN_QUALIFICATION_SUCCEEDED', run


def observe(api, issue, pr, ui_required=True):
    pull = api.get(f'/pulls/{pr}')
    candidate = pull['head']['sha']
    runs = api.runs(candidate)
    files = api.pages(f'/pulls/{pr}/files')
    jobs = {}
    for workflow in (QUALIFICATION, MERGE):
        selected = matching_runs(runs, workflow, candidate, pr)
        if selected:
            jobs[selected[0]['id']] = api.jobs(selected[0])
    report = derive(pull, files, runs, jobs, issue, ui_required)
    contract_raw = (ROOT / 'scripts/delivery_execution_contract.json').read_bytes()
    contract = json.loads(contract_raw)
    expected = expected_jobs(json.loads((ROOT / 'scripts/ci_topology_contract.json').read_bytes()))
    if report['state'] == 'FULL_GATE_SUCCEEDED_PENDING_RECEIPT':
        full = matching_runs(runs, MERGE, candidate, pr)[0]
        q = matching_runs(runs, QUALIFICATION, candidate, pr)[0]
        assert_single_full(runs, contract, candidate, pr, full['id'], full['run_attempt'])
        artifacts = api.pages(f"/actions/runs/{full['id']}/artifacts", 'artifacts')
        artifacts = [a for a in artifacts if a['name'] == f"delivery-execution-{full['id']}-{full['run_attempt']}"]
        require(len(artifacts) == 1 and not artifacts[0].get('expired'), 'MERGE_RECEIPT_MISSING')
        data = api.raw(api.prefix + f"/actions/artifacts/{artifacts[0]['id']}/zip", cap=contract['limits']['max_receipt_bytes'])
        receipt = read_receipt_zip(data, contract['limits']['max_receipt_bytes'])
        tree = api.get('/git/commits/' + candidate)['tree']['sha']
        verify_receipt(receipt, digest(contract_raw), candidate, tree, pr, full, q, expected)
        require(receipt.get('repository') == api.prefix.removeprefix('/repos/'), 'RECEIPT_REPOSITORY_MISMATCH')
        require(api.get('/git/ref/heads/main')['object']['sha'] == receipt['frozen_main_sha'], 'MAIN_CHANGED_REQUALIFY')
        report.update(state='MERGE_READY', next_action='merge_exact_candidate', expected_head_sha=candidate,
                      merge_receipt_artifact=artifacts[0]['id'])
    elif report['state'] == 'MERGED_PENDING_MAIN_VERIFICATION':
        main_sha = pull['merge_commit_sha']
        main_runs = api.pages('/actions/runs?event=push&head_sha=' + main_sha, 'workflow_runs')
        state, main_run = main_run_state(main_runs, main_sha)
        report.update(state=state, main_sha=main_sha,
                      next_action='collect_terminal_failure_and_repair' if state.endswith('FAILED') else 'verify_latest_main_qualification')
        if main_run:
            report['main_qualification'] = run_ref(main_run)
        if state == 'MAIN_QUALIFICATION_SUCCEEDED':
            require(api.jobs(main_run), 'MAIN_QUALIFICATION_JOBS_MISSING')
            receipt = verify_main(api, contract, digest(contract_raw), api.prefix.removeprefix('/repos/'), main_sha, expected)
            report.update(state='MAIN_VERIFIED', next_action='review_issue_acceptance_before_closure', main_sha=receipt['main_sha'])
    require(api.get(f'/pulls/{pr}')['head']['sha'] == candidate, 'HEAD_CHANGED_DURING_OBSERVATION')
    report['observed_at'] = datetime.now(timezone.utc).isoformat()
    report['source'] = 'github_api'
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repository', required=True)
    parser.add_argument('--issue', required=True, type=int)
    parser.add_argument('--pr', required=True, type=int)
    parser.add_argument('--no-ui-required', action='store_true', help='For explicitly backend/governance-only tasks, not unfinished product UI.')
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    try:
        limits = json.loads((ROOT / 'scripts/delivery_execution_contract.json').read_text())['limits']
        api = API(args.repository, os.getenv('GH_TOKEN') or os.getenv('GITHUB_TOKEN'), limits)
        report = observe(api, args.issue, args.pr, not args.no_ui_required)
    except Exception as error:
        report = {'state': 'BLOCKED', 'reason': str(error), 'next_action': 'resolve_missing_or_changed_evidence',
                  'issue': args.issue, 'pr': args.pr, 'observed_at': datetime.now(timezone.utc).isoformat()}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))
    return 1 if report['state'] == 'BLOCKED' else 0


if __name__ == '__main__':
    raise SystemExit(main())
