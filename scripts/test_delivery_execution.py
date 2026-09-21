#!/usr/bin/env python3
"""No-Stall adversarial cases. Tests are offline and never fabricate qualification evidence."""
import copy
import io
import json
import unittest
import urllib.request
import zipfile
from pathlib import Path
from unittest.mock import patch

import delivery_execution as d

CONTRACT = json.loads((Path(__file__).parent / 'delivery_execution_contract.json').read_text())
CANDIDATE, TREE = 'a' * 40, 'b' * 40
PR = 213


def run(number=10, workflow='qualification_workflow', **overrides):
    return {'id': number, 'run_attempt': 1, 'path': CONTRACT[workflow], 'head_sha': CANDIDATE,
            'event': 'pull_request', 'pull_requests': [{'number': PR}], 'status': 'completed',
            'conclusion': 'success', **overrides}


class Facts:
    def __init__(self, qualification=None, jobs=None):
        self.items = [run(20, 'merge_workflow', status='in_progress', conclusion=None)]
        if qualification is not None:
            self.items.append(qualification)
        self.job_items = jobs or []

    def runs(self, candidate):
        return self.items

    def jobs(self, current):
        return self.job_items


class Clock:
    def __init__(self):
        self.value = 0
        self.sleeps = 0

    def now(self):
        return self.value

    def sleep(self, seconds):
        self.value += seconds
        self.sleeps += 1


class ControlTests(unittest.TestCase):
    def blocked(self, code, function, *args):
        with self.assertRaises(d.Blocked) as result:
            function(*args)
        self.assertEqual(result.exception.code, code)

    def test_latest_failure_cannot_be_hidden_by_old_success(self):
        self.blocked('LATEST_RUN_NOT_SUCCESS', d.latest_success,
                     [run(), run(11, conclusion='failure')], CONTRACT['qualification_workflow'], CANDIDATE, PR)

    def test_latest_pending_cannot_be_hidden_by_old_success(self):
        self.blocked('LATEST_RUN_NOT_SUCCESS', d.latest_success,
                     [run(), run(11, status='in_progress', conclusion=None)], CONTRACT['qualification_workflow'], CANDIDATE, PR)

    def test_exact_path_head_pr_and_event_are_required(self):
        for mutation in ({'path': 'fake.yml'}, {'head_sha': 'c' * 40}, {'pull_requests': [{'number': 2}]},
                         {'pull_requests': []}, {'event': 'workflow_dispatch'}):
            with self.subTest(mutation=mutation):
                self.blocked('MATCHING_RUN_MISSING', d.latest_success,
                             [run(**mutation)], CONTRACT['qualification_workflow'], CANDIDATE, PR)

    def test_latest_attempt_is_selected(self):
        latest = d.latest_success([run(run_attempt=1), run(run_attempt=2)], CONTRACT['qualification_workflow'], CANDIDATE, PR)
        self.assertEqual(latest['run_attempt'], 2)

    def test_full_rerun_is_rejected(self):
        self.blocked('FULL_RUN_BUDGET_EXCEEDED', d.assert_single_full,
                     [run(19, 'merge_workflow'), run(20, 'merge_workflow')], CONTRACT, CANDIDATE, PR, 20, 1)

    def test_full_attempt_is_rejected(self):
        self.blocked('FULL_ATTEMPT_BUDGET_EXCEEDED', d.assert_single_full,
                     [run(20, 'merge_workflow', run_attempt=2)], CONTRACT, CANDIDATE, PR, 20, 2)

    def test_empty_full_cannot_pass(self):
        self.blocked('FULL_RESULT_SET_MISMATCH', d.full_results, {}, ['full-01-one'])

    def test_missing_or_extra_full_cannot_pass(self):
        for actual in ({'full-01-one': {}}, {'full-01-one': {}, 'full-02-two': {}, 'full-03-extra': {}}):
            self.blocked('FULL_RESULT_SET_MISMATCH', d.full_results, actual, ['full-01-one', 'full-02-two'])

    def test_all_42_required_results_pass(self):
        expected = [f'full-{n:02d}-test' for n in range(1, 43)]
        needs = {k: {'result': 'success'} for k in expected + ['route', 'freeze-candidate', 'wait-qualification']}
        self.assertEqual(len(d.full_results(needs, expected)), 42)

    def test_skipped_cancelled_failure_or_unknown_full_is_not_success(self):
        for value in ['skipped', 'cancelled', 'failure', None]:
            self.blocked('FULL_GATE_FAILED', d.full_results, {'full-01-one': {'result': value}}, ['full-01-one'])

    def test_freeze_and_wait_cannot_be_omitted(self):
        self.blocked('CONTROL_PREREQUISITE_FAILED', d.full_results,
                     {'full-01-one': {'result': 'success'}}, ['full-01-one'])

    def test_duplicate_topology_is_rejected(self):
        self.blocked('FULL_TOPOLOGY_INVALID', d.expected_jobs,
                     {'full_merge_gate': {'expected_units': 2, 'expected_workflows': ['x.yml', 'x.yml']}})

    def test_contract_budget_cannot_grow(self):
        contract = copy.deepcopy(CONTRACT)
        contract['limits']['full_runs_per_candidate'] = 2
        self.blocked('FULL_BUDGET_DRIFT', d.validate_contract, contract)

    def test_nonpositive_limits_are_rejected(self):
        contract = copy.deepcopy(CONTRACT)
        contract['limits']['poll_seconds'] = 0
        self.blocked('CONTRACT_LIMIT_INVALID', d.validate_contract, contract)

    def wait(self, api, clock, contract=CONTRACT):
        with patch.object(d, 'bound_refs', return_value={'candidate_sha': CANDIDATE}):
            return d.wait_qualification(api, contract, PR, CANDIDATE, 20, 1, clock.now, clock.sleep)

    def test_failed_fast_step_stops_before_workflow_finishes(self):
        clock = Clock()
        job = {'id': 3, 'name': 'fast-web / qualify', 'status': 'in_progress', 'conclusion': None,
               'steps': [{'number': 2, 'name': 'Fast Web qualification', 'conclusion': 'failure'}]}
        self.blocked('QUALIFICATION_JOB_FAILED', self.wait,
                     Facts(run(status='in_progress', conclusion=None), [job]), clock)
        self.assertEqual(clock.sleeps, 0)

    def test_success_does_not_poll_again(self):
        clock = Clock()
        result = self.wait(Facts(run()), clock)
        self.assertEqual(result['state'], 'DOMAIN_QUALIFIED')
        self.assertEqual(clock.sleeps, 0)

    def test_missing_run_has_discovery_deadline(self):
        clock = Clock()
        self.blocked('QUALIFICATION_RUN_MISSING', self.wait, Facts(), clock)
        self.assertEqual(clock.value, CONTRACT['limits']['discovery_seconds'])

    def test_no_progress_expires_lease(self):
        clock = Clock()
        self.blocked('QUALIFICATION_PROGRESS_LEASE_EXPIRED', self.wait,
                     Facts(run(status='in_progress', conclusion=None)), clock)
        self.assertEqual(clock.value, CONTRACT['limits']['no_progress_seconds'])

    def test_global_wait_is_bounded(self):
        clock = Clock()
        contract = copy.deepcopy(CONTRACT)
        contract['limits']['qualification_seconds'] = 30
        self.blocked('QUALIFICATION_DEADLINE_EXCEEDED', self.wait,
                     Facts(run(status='in_progress', conclusion=None)), clock, contract)
        self.assertEqual(clock.value, 30)

    def test_terminal_cancel_is_not_retried(self):
        clock = Clock()
        self.blocked('QUALIFICATION_NOT_SUCCESS', self.wait, Facts(run(conclusion='cancelled')), clock)
        self.assertEqual(clock.sleeps, 0)

    def test_head_change_is_not_waited_out(self):
        clock = Clock()
        with patch.object(d, 'bound_refs', side_effect=d.Blocked('CANDIDATE_HEAD_CHANGED')):
            self.blocked('CANDIDATE_HEAD_CHANGED', d.wait_qualification,
                         Facts(run()), CONTRACT, PR, CANDIDATE, 20, 1, clock.now, clock.sleep)
        self.assertEqual(clock.sleeps, 0)

    def test_frozen_binding_cannot_change_during_wait(self):
        clock = Clock()
        with patch.object(d, 'bound_refs', side_effect=[{'main': 'old'}, {'main': 'new'}]):
            self.blocked('FROZEN_BINDING_CHANGED', d.wait_qualification,
                         Facts(run(status='in_progress', conclusion=None)), CONTRACT, PR, CANDIDATE, 20, 1, clock.now, clock.sleep)

    def test_failure_signatures_are_stable_and_do_not_invent_code_locations(self):
        jobs = [{'id': 3, 'name': 'fast', 'conclusion': 'failure',
                 'steps': [{'name': 'Type  check', 'number': 2, 'conclusion': 'failure'}]}]
        roots = d.root_causes(jobs + jobs)
        self.assertEqual(len(roots), 1)
        self.assertIsNone(roots[0]['file'])
        self.assertEqual(roots[0]['normalized_error'], 'Type check')

    def receipt(self):
        return {'schema_version': 1, 'state': 'MERGE_READY', 'contract_sha256': 'contract',
                'candidate_sha': CANDIDATE, 'candidate_tree': TREE, 'pr_number': PR,
                'merge_run_id': '20', 'merge_run_attempt': 1, 'active_full_jobs': 0,
                'root_cause_signatures': [], 'qualification_run': d.run_ref(run()),
                'full_results': {'full-01-one': 'success'}}

    def check_receipt(self, receipt):
        return d.verify_receipt(receipt, 'contract', CANDIDATE, TREE, PR,
                                run(20, 'merge_workflow'), run(), ['full-01-one'])

    def test_exact_receipt_passes(self):
        self.check_receipt(self.receipt())

    def test_receipt_binding_tampering_fails_closed(self):
        for field in ['schema_version', 'state', 'contract_sha256', 'candidate_sha', 'candidate_tree',
                      'pr_number', 'merge_run_id', 'merge_run_attempt', 'active_full_jobs', 'root_cause_signatures']:
            with self.subTest(field=field):
                receipt = self.receipt()
                receipt[field] = 'wrong'
                self.blocked('RECEIPT_BINDING_MISMATCH', self.check_receipt, receipt)

    def test_receipt_cannot_reuse_old_qualification_attempt(self):
        receipt = self.receipt()
        receipt['qualification_run']['run_attempt'] = 2
        self.blocked('QUALIFICATION_PROOF_SUPERSEDED', self.check_receipt, receipt)

    def test_receipt_requires_entire_result_set(self):
        receipt = self.receipt()
        receipt['full_results'] = {}
        self.blocked('RECEIPT_FULL_SET_INVALID', self.check_receipt, receipt)

    def test_archive_cannot_escape_or_add_another_receipt(self):
        for names in [['../delivery-execution.json'], ['x.json', d.RECEIPT]]:
            buffer = io.BytesIO()
            with zipfile.ZipFile(buffer, 'w') as archive:
                for name in names:
                    archive.writestr(name, '{}')
            self.blocked('RECEIPT_ARCHIVE_INVALID', d.read_receipt_zip, buffer.getvalue(), 10000)

    def test_archive_expansion_is_bounded(self):
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
            archive.writestr(d.RECEIPT, 'x' * 200)
        self.blocked('RECEIPT_TOO_LARGE', d.read_receipt_zip, buffer.getvalue(), 100)

    def test_cross_host_redirect_strips_token(self):
        request = urllib.request.Request('https://api.github.com/repos/a/b', headers={'Authorization': 'Bearer secret'})
        redirected = d.SafeRedirect().redirect_request(request, None, 302, 'Found', {}, 'https://example.org/archive')
        self.assertIsNone(redirected.get_header('Authorization'))

    def test_plain_http_redirect_is_rejected(self):
        request = urllib.request.Request('https://api.github.com/repos/a/b')
        self.blocked('UNSAFE_REDIRECT', d.SafeRedirect().redirect_request,
                     request, None, 302, 'Found', {}, 'http://example.org/archive')


class MainFacts(Facts):
    def __init__(self, receipt):
        super().__init__(run())
        self.items[0] = run(20, 'merge_workflow')
        self.tip = 'd' * 40
        self.tree = TREE
        self.receipt = receipt
        self.pull = {'number': PR, 'merged_at': '2026-09-21T12:00:00Z',
                     'merge_commit_sha': self.tip, 'base': {'ref': 'main'}, 'head': {'sha': CANDIDATE}}
        self.artifact = {'id': 99, 'name': 'delivery-execution-20-1', 'expired': False, 'size_in_bytes': 100}
        self.prefix = '/repos/hvritual/biz'

    def get(self, path):
        if path == '/git/ref/heads/main':
            return {'object': {'sha': self.tip}}
        if path == '/git/commits/' + CANDIDATE:
            return {'tree': {'sha': TREE}}
        if path == '/git/commits/' + 'd' * 40:
            return {'tree': {'sha': self.tree}}
        raise AssertionError(path)

    def pages(self, path, key=None):
        if path.startswith('/commits/'):
            return [self.pull]
        if path == '/actions/runs/20/artifacts':
            return [self.artifact]
        raise AssertionError(path)

    def raw(self, path, cap):
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, 'w') as archive:
            archive.writestr(d.RECEIPT, json.dumps(self.receipt))
        return buffer.getvalue()


class MainProofTests(unittest.TestCase):
    def setUp(self):
        self.receipt = ControlTests().receipt()
        self.receipt['repository'] = 'hvritual/biz'
        self.api = MainFacts(self.receipt)

    def verify(self):
        return d.verify_main(self.api, CONTRACT, 'contract', 'hvritual/biz', 'd' * 40, ['full-01-one'])

    def blocked(self, code):
        with self.assertRaises(d.Blocked) as result:
            self.verify()
        self.assertEqual(result.exception.code, code)

    def test_main_exact_chain_passes(self):
        self.assertEqual(self.verify()['state'], 'MAIN_VERIFIED')

    def test_direct_main_push_is_rejected(self):
        self.api.pull['merged_at'] = None
        self.blocked('MAIN_MERGED_PR_BINDING_MISSING')

    def test_changed_main_tip_is_rejected(self):
        self.api.tip = 'e' * 40
        self.blocked('MAIN_TIP_CHANGED')

    def test_merge_tree_must_equal_tested_candidate_tree(self):
        self.api.tree = 'e' * 40
        self.blocked('MAIN_CANDIDATE_TREE_MISMATCH')

    def test_old_success_cannot_hide_new_main_proof_failure(self):
        self.api.items.append(run(21, 'merge_workflow', conclusion='failure'))
        self.blocked('LATEST_RUN_NOT_SUCCESS')

    def test_expired_receipt_is_not_main_acceptance(self):
        self.api.artifact['expired'] = True
        self.blocked('MERGE_RECEIPT_MISSING_OR_EXPIRED')

    def test_wrong_artifact_name_is_rejected(self):
        self.api.artifact['name'] = 'delivery-execution-19-1'
        self.blocked('MERGE_RECEIPT_MISSING_OR_EXPIRED')

    def test_foreign_repository_receipt_is_rejected(self):
        self.receipt['repository'] = 'somewhere/else'
        self.blocked('RECEIPT_REPOSITORY_MISMATCH')

    def test_green_workflow_without_matching_contract_is_rejected(self):
        self.receipt['contract_sha256'] = 'old-contract'
        self.blocked('RECEIPT_BINDING_MISMATCH')


class MergedRunBindingTests(unittest.TestCase):
    def refs(self, pr=PR):
        sha = 'e' * 40
        return [{'path': f'hvritual/biz/.github/workflows/role.yml@{sha}',
                 'ref': f'refs/pull/{pr}/merge', 'sha': sha}]

    def merged_run(self, **overrides):
        return run(pull_requests=[], referenced_workflows=self.refs(), **overrides)

    def test_merged_run_uses_immutable_pr_ref(self):
        self.assertEqual(d.latest_success([self.merged_run()], CONTRACT['qualification_workflow'], CANDIDATE, PR)['id'], 10)

    def test_branch_title_and_actor_cannot_replace_pr_proof(self):
        item = run(pull_requests=[], head_branch='feat/enterprise-179-role-grant-tree',
                   display_title='#213', actor={'login': 'hvritual'})
        self.assertFalse(d.run_binds_pr(item, PR))

    def test_wrong_or_mixed_pr_refs_are_rejected(self):
        for references in [self.refs(PR + 1), self.refs() + self.refs(PR + 1)]:
            self.assertFalse(d.run_binds_pr(run(pull_requests=[], referenced_workflows=references), PR))

    def test_live_conflicting_pr_cannot_use_fallback(self):
        self.assertFalse(d.run_binds_pr(run(pull_requests=[{'number': PR+1}], referenced_workflows=self.refs()), PR))

    def test_unpinned_or_mismatched_reference_is_rejected(self):
        for patch_value in [{'sha': 'main'}, {'path': 'hvritual/biz/workflow.yml@main'}, {'ref': f'refs/pull/{PR}/head'}]:
            references = self.refs()
            references[0].update(patch_value)
            self.assertFalse(d.run_binds_pr(run(pull_requests=[], referenced_workflows=references), PR))

    def test_merged_reference_does_not_relax_head_or_event(self):
        for fields in [{'head_sha': 'f'*40}, {'event': 'workflow_dispatch'}]:
            self.assertEqual(d.matching_runs([self.merged_run(**fields)], CONTRACT['qualification_workflow'], CANDIDATE, PR), [])

    def test_newer_merged_failure_is_not_hidden(self):
        with self.assertRaises(d.Blocked) as caught:
            d.latest_success([run(), self.merged_run(number=11, conclusion='failure')], CONTRACT['qualification_workflow'], CANDIDATE, PR)
        self.assertEqual(caught.exception.code, 'LATEST_RUN_NOT_SUCCESS')

    def test_main_proof_accepts_detached_pr_with_exact_receipt(self):
        receipt = ControlTests().receipt()
        receipt['repository'] = 'hvritual/biz'
        api = MainFacts(receipt)
        for item in api.items:
            item['pull_requests'] = []
            item['referenced_workflows'] = self.refs()
        result = d.verify_main(api, CONTRACT, 'contract', 'hvritual/biz', 'd'*40, ['full-01-one'])
        self.assertEqual(result['state'], 'MAIN_VERIFIED')


if __name__ == '__main__':
    unittest.main(verbosity=2)
