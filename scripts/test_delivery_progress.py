#!/usr/bin/env python3
import unittest
import copy
import io
import json
import zipfile
import delivery_progress as progress
from delivery_progress import derive, QUALIFICATION, MERGE, main_run_state
from delivery_execution import Blocked


class ProgressTests(unittest.TestCase):
    def setUp(self):
        self.pull = {'number': 272, 'body': 'Refs #182', 'head': {'sha': 'a' * 40}, 'base': {'sha': 'b' * 40}}
        self.q = {'id': 10, 'run_attempt': 1, 'event': 'pull_request', 'path': QUALIFICATION,
                  'head_sha': 'a' * 40, 'pull_requests': [{'number': 272}], 'status': 'completed', 'conclusion': 'success'}
        self.files = [{'filename': 'web/src/features/enterprise/PersonalSecurity.vue', 'status': 'modified'},
                      {'filename': 'web/e2e/personal-security.spec.ts', 'status': 'added'}]
        self.jobs = {10: [{'id': 1, 'name': 'backend', 'conclusion': 'success'}]}

    def test_other_issue_cannot_reuse_report(self):
        with self.assertRaises(Blocked):
            derive(self.pull, self.files, [self.q], self.jobs, 181)

    def test_existing_main_ui_not_new_task_ui(self):
        result = derive(self.pull, [], [self.q], self.jobs, 182)
        self.assertEqual(result['ui_state'], 'NO_INCREMENTAL_UI_COMMIT')
        self.assertEqual(result['next_action'], 'implement_missing_ui_and_acceptance')

    def test_ui_source_does_not_imply_acceptance(self):
        result = derive(self.pull, self.files[:1], [self.q], self.jobs, 182)
        self.assertEqual(result['ui_state'], 'COMMITTED_UNVERIFIED')
        self.assertEqual(result['state'], 'BACKEND_QUALIFIED_UI_INCOMPLETE')

    def test_backend_green_advances_without_polling(self):
        result = derive(self.pull, self.files, [self.q], self.jobs, 182)
        self.assertEqual(result['next_action'], 'freeze_exact_candidate_and_mark_ready')

    def test_docs_only_uses_lightweight_gate_without_ui_requirement(self):
        self.files = [{'filename': 'docs/design-notes.md', 'status': 'modified'}]
        result = derive(self.pull, self.files, [self.q], self.jobs, 182)
        self.assertEqual(result['change_class'], 'docs_only')
        self.assertFalse(result['merge_gate_required'])
        self.assertEqual(result['state'], 'LIGHTWEIGHT_QUALIFIED')

    def test_removed_runtime_plus_skill_escalates_to_product_change(self):
        self.files = [
            {'filename': '.agents/skills/b2b-product-ux/SKILL.md', 'status': 'modified'},
            {'filename': 'internal/access/application/runtime.go', 'status': 'removed'},
        ]
        result = derive(self.pull, self.files, [self.q], self.jobs, 182, ui_required=False)
        self.assertEqual(result['change_class'], 'product_change')
        self.assertTrue(result['merge_gate_required'])
        self.assertEqual(result['state'], 'QUALIFIED')

    def test_renamed_runtime_into_skill_uses_previous_filename(self):
        self.files = [
            {'filename': '.agents/skills/b2b-product-ux/runtime.go', 'previous_filename': 'Makefile',
             'status': 'renamed'},
        ]
        result = derive(self.pull, self.files, [self.q], self.jobs, 182, ui_required=False)
        self.assertEqual(result['change_class'], 'product_change')
        self.assertTrue(result['merge_gate_required'])

    def test_skill_only_uses_lightweight_gate_without_ui_requirement(self):
        self.files = [
            {'filename': '.agents/skills/b2b-product-ux/SKILL.md', 'status': 'modified'},
            {'filename': 'docs/design/B2B-PRODUCT-UX.md', 'status': 'modified'},
        ]
        result = derive(self.pull, self.files, [self.q], self.jobs, 182)
        self.assertEqual(result['change_class'], 'skill_only')
        self.assertFalse(result['merge_gate_required'])
        self.assertEqual(result['state'], 'LIGHTWEIGHT_QUALIFIED')
        self.assertEqual(result['next_action'], 'mark_ready_for_lightweight_gate')

    def test_skill_lightweight_green_can_merge_without_full_receipt(self):
        self.files = [{'filename': '.agents/skills/b2b-product-ux/SKILL.md', 'status': 'modified'}]
        merge = {**self.q, 'id': 20, 'path': MERGE}
        jobs = {**self.jobs, 20: [{'id': 2, 'name': 'lightweight-ready', 'conclusion': 'success'}]}
        result = derive(self.pull, self.files, [self.q, merge], jobs, 182)
        # No Full Gate receipt is required, but canonical lightweight evidence still is.
        self.assertEqual(result['state'], 'LIGHTWEIGHT_GATE_SUCCEEDED_PENDING_RECEIPT')
        self.assertEqual(result['next_action'], 'verify_exact_lightweight_receipt')

    def test_design_governance_uses_lightweight_gate(self):
        self.files = [{'filename': 'web/ui-contracts.json', 'status': 'modified'}]
        result = derive(self.pull, self.files, [self.q], self.jobs, 182)
        self.assertEqual(result['change_class'], 'design_governance')
        self.assertEqual(result['state'], 'LIGHTWEIGHT_QUALIFIED')

    def test_skill_plus_runtime_escalates_to_full_product_gate(self):
        self.files = [
            {'filename': '.agents/skills/b2b-product-ux/SKILL.md', 'status': 'modified'},
            {'filename': 'internal/access/application/tenant_member_lifecycle.go', 'status': 'modified'},
        ]
        result = derive(self.pull, self.files, [self.q], self.jobs, 182, ui_required=False)
        self.assertEqual(result['change_class'], 'product_change')
        self.assertTrue(result['merge_gate_required'])
        self.assertEqual(result['state'], 'QUALIFIED')

    def test_terminal_failure_never_waits(self):
        self.q['conclusion'] = 'failure'
        self.assertEqual(derive(self.pull, self.files, [self.q], self.jobs, 182)['next_action'], 'collect_terminal_failure_and_repair')

    def test_new_failed_attempt_beats_old_green(self):
        latest = {**self.q, 'run_attempt': 2, 'conclusion': 'failure'}
        self.assertEqual(derive(self.pull, self.files, [self.q, latest], self.jobs, 182)['state'], 'QUALIFICATION_FAILED')

    def test_old_sha_green_ignored(self):
        self.q['head_sha'] = 'c' * 40
        self.assertEqual(derive(self.pull, self.files, [self.q], self.jobs, 182)['state'], 'NEEDS_QUALIFICATION')

    def test_zero_jobs_not_success(self):
        self.assertEqual(derive(self.pull, self.files, [self.q], {}, 182)['state'], 'QUALIFICATION_EVIDENCE_MISSING')

    def test_full_green_requests_receipt_not_wait(self):
        full = {**self.q, 'id': 20, 'path': MERGE}
        result = derive(self.pull, self.files, [self.q, full], self.jobs, 182)
        self.assertEqual(result['next_action'], 'verify_exact_merge_receipt')
        self.assertNotEqual(result['state'], 'MAIN_VERIFIED')

    def test_merged_is_not_done(self):
        self.pull['merged'] = True
        result = derive(self.pull, self.files, [self.q], self.jobs, 182)
        self.assertEqual(result['next_action'], 'verify_exact_main_receipt')
        self.assertNotEqual(result['state'], 'MAIN_VERIFIED')


    def test_main_missing_cannot_be_verified(self):
        self.assertEqual(main_run_state([], 'm')[0], 'MAIN_QUALIFICATION_MISSING')

    def test_main_running_not_done(self):
        run = {'id': 40, 'head_sha': 'm', 'path': '.github/workflows/main-receipt.yml', 'event': 'push', 'status': 'in_progress'}
        self.assertEqual(main_run_state([run], 'm')[0], 'MAIN_QUALIFICATION_RUNNING')

    def test_main_failed_latest_attempt_not_old_success(self):
        run = {'id': 40, 'head_sha': 'm', 'path': '.github/workflows/main-receipt.yml', 'event': 'push', 'status': 'completed', 'conclusion': 'success', 'run_attempt': 1}
        newer = {**run, 'run_attempt': 2, 'conclusion': 'failure'}
        self.assertEqual(main_run_state([run, newer], 'm')[0], 'MAIN_QUALIFICATION_FAILED')

    def test_main_wrong_sha_ignored(self):
        run = {'id': 40, 'head_sha': 'old', 'path': '.github/workflows/main-receipt.yml', 'event': 'push', 'status': 'completed', 'conclusion': 'success'}
        self.assertEqual(main_run_state([run], 'm')[0], 'MAIN_QUALIFICATION_MISSING')



class ObservationFacts:
    """Offline GitHub-shaped facts; ZIP fixtures are not live approval evidence."""
    prefix = '/repos/hvritual/biz'

    def __init__(self):
        self.candidate, self.tree, self.main_sha = 'a' * 40, 'b' * 40, 'c' * 40
        self.pull = {'number': 321, 'body': 'Refs #320', 'state': 'open', 'draft': False,
                     'head': {'sha': self.candidate}, 'base': {'sha': self.main_sha, 'ref': 'main'}}
        common = {'run_attempt': 1, 'event': 'pull_request', 'head_sha': self.candidate,
                  'pull_requests': [{'number': 321}], 'status': 'completed', 'conclusion': 'success'}
        self.qualification = {**common, 'id': 10, 'path': QUALIFICATION}
        self.merge = {**common, 'id': 20, 'path': MERGE}
        self.workflow_runs = [self.qualification, self.merge]
        self.files = [{'filename': '.agents/skills/b2b-product-ux/SKILL.md', 'status': 'modified'}]
        self.job_items = {
            10: [{'id': 1, 'name': 'skill-governance', 'status': 'completed', 'conclusion': 'success'}],
            20: [{'id': 2, 'name': 'route', 'status': 'completed', 'conclusion': 'success'},
                 {'id': 3, 'name': 'lightweight-ready', 'status': 'completed', 'conclusion': 'success'},
                 {'id': 4, 'name': 'full-01-example', 'status': 'completed', 'conclusion': 'skipped'}],
        }
        contract = (progress.ROOT / 'scripts/delivery_execution_contract.json').read_bytes()
        self.receipt = {'schema_version': 1, 'state': 'LIGHTWEIGHT_MERGE_READY',
                        'contract_sha256': progress.digest(contract), 'repository': 'hvritual/biz',
                        'candidate_sha': self.candidate, 'candidate_tree': self.tree, 'pr_number': 321,
                        'merge_run_id': '20', 'merge_run_attempt': 1, 'change_class': 'skill_only',
                        'merge_gate_required': False, 'full_results': {}, 'root_cause_signatures': [],
                        'qualification_run': progress.run_ref(self.qualification),
                        'frozen_main_sha': self.main_sha}
        self.artifacts = [{'id': 30, 'name': 'delivery-execution-20-1',
                           'expired': False, 'size_in_bytes': 1024}]
        self.calls = []
        self.final_pull = None
        self.main_reads = []

    def get(self, path):
        self.calls.append(('get', path))
        if path == '/pulls/321':
            count = self.calls.count(('get', path))
            return copy.deepcopy(self.final_pull if count > 1 and self.final_pull else self.pull)
        if path == '/git/commits/' + self.candidate:
            return {'tree': {'sha': self.tree}}
        if path == '/git/ref/heads/main':
            return {'object': {'sha': self.main_reads.pop(0) if self.main_reads else self.main_sha}}
        raise AssertionError('unexpected API GET: ' + path)

    def runs(self, candidate):
        self.calls.append(('runs', candidate))
        return copy.deepcopy(self.workflow_runs)

    def jobs(self, run):
        self.calls.append(('jobs', str(run['id'])))
        return copy.deepcopy(self.job_items.get(run['id'], []))

    def pages(self, path, key=None):
        self.calls.append(('pages', path))
        if path == '/pulls/321/files':
            return copy.deepcopy(self.files)
        if path == '/actions/runs/20/artifacts':
            return copy.deepcopy(self.artifacts)
        raise AssertionError('unexpected API pages: ' + path)

    def raw(self, path, cap):
        self.calls.append(('raw', path))
        if path != self.prefix + '/actions/artifacts/30/zip':
            raise AssertionError('unexpected artifact: ' + path)
        archive = io.BytesIO()
        with zipfile.ZipFile(archive, 'w') as handle:
            handle.writestr('delivery-execution.json', json.dumps(self.receipt))
        return archive.getvalue()


class LightweightObservationTests(unittest.TestCase):
    def setUp(self):
        self.api = ObservationFacts()

    def observe(self):
        return progress.observe(self.api, 320, 321, ui_required=False)

    def blocked(self, code):
        with self.assertRaises(Blocked) as caught:
            self.observe()
        self.assertEqual(code, caught.exception.code)

    def test_green_summary_is_not_merge_authority(self):
        report = derive(self.api.pull, self.api.files, self.api.workflow_runs,
                        self.api.job_items, 320, ui_required=False)
        self.assertEqual(report['state'], 'LIGHTWEIGHT_GATE_SUCCEEDED_PENDING_RECEIPT')
        self.assertEqual(report['next_action'], 'verify_exact_lightweight_receipt')

    def test_exact_lightweight_receipt_and_current_main_allow_merge(self):
        report = self.observe()
        self.assertEqual(report['state'], 'LIGHTWEIGHT_MERGE_READY')
        self.assertEqual(report['next_action'], 'merge_exact_candidate')
        self.assertEqual(report['expected_head_sha'], self.api.candidate)
        self.assertEqual(report['merge_receipt_artifact'], 30)
        self.assertIn(('raw', self.api.prefix + '/actions/artifacts/30/zip'), self.api.calls)
        self.assertIn(('get', '/git/ref/heads/main'), self.api.calls)

    def test_docs_and_design_require_their_own_matching_lightweight_receipt(self):
        for change_class, path in [('docs_only', 'docs/notes.md'),
                                   ('design_governance', 'web/ui-contracts.json')]:
            with self.subTest(change_class=change_class):
                self.api = ObservationFacts()
                self.api.files = [{'filename': path, 'status': 'modified'}]
                self.api.receipt['change_class'] = change_class
                self.assertEqual(self.observe()['state'], 'LIGHTWEIGHT_MERGE_READY')

    def test_advanced_main_rejects_old_green_receipt(self):
        self.api.main_sha = 'd' * 40
        self.blocked('MAIN_CHANGED_REQUALIFY')

    def test_final_main_read_is_after_pr_recheck(self):
        self.observe()
        self.assertEqual(self.api.calls[-1], ('get', '/git/ref/heads/main'))
        self.assertEqual(self.api.calls[-2], ('get', '/pulls/321'))

    def test_missing_expired_duplicate_or_wrong_attempt_artifacts_fail_closed(self):
        for kind in ('missing', 'expired', 'duplicate', 'old_attempt'):
            with self.subTest(kind=kind):
                self.api = ObservationFacts()
                if kind == 'missing':
                    self.api.artifacts = []
                elif kind == 'expired':
                    self.api.artifacts[0]['expired'] = True
                elif kind == 'duplicate':
                    self.api.artifacts += copy.deepcopy(self.api.artifacts)
                else:
                    self.api.artifacts[0]['name'] = 'delivery-execution-20-0'
                self.blocked('MERGE_RECEIPT_MISSING')

    def test_receipt_bindings_cannot_be_replayed(self):
        for field, value in [('candidate_sha', 'd' * 40), ('candidate_tree', 'd' * 40),
                             ('pr_number', 999), ('merge_run_id', '999'), ('merge_run_attempt', 2),
                             ('contract_sha256', 'old'), ('change_class', 'design_governance'),
                             ('state', 'DOMAIN_QUALIFIED'), ('merge_gate_required', True),
                             ('full_results', {'full-01-example': 'success'})]:
            with self.subTest(field=field):
                self.api = ObservationFacts()
                self.api.receipt[field] = value
                self.blocked('LIGHTWEIGHT_RECEIPT_BINDING_MISMATCH')

    def test_foreign_repository_receipt_is_rejected(self):
        self.api.receipt['repository'] = 'other/biz'
        self.blocked('RECEIPT_REPOSITORY_MISMATCH')

    def test_missing_frozen_main_is_rejected(self):
        del self.api.receipt['frozen_main_sha']
        self.blocked('MAIN_CHANGED_REQUALIFY')

    def test_qualification_receipt_must_bind_latest_attempt(self):
        newer = {**self.api.qualification, 'run_attempt': 2}
        self.api.workflow_runs.append(newer)
        self.blocked('QUALIFICATION_PROOF_SUPERSEDED')

    def test_latest_failed_qualification_is_not_hidden(self):
        self.api.workflow_runs.append({**self.api.qualification, 'run_attempt': 2, 'conclusion': 'failure'})
        self.assertEqual(self.observe()['state'], 'QUALIFICATION_FAILED')

    def test_actual_lightweight_job_must_succeed(self):
        for mutation in ('missing', 'failed', 'running', 'duplicate'):
            with self.subTest(mutation=mutation):
                self.api = ObservationFacts()
                if mutation == 'missing':
                    self.api.job_items[20] = [self.api.job_items[20][0]]
                elif mutation == 'failed':
                    self.api.job_items[20][1]['conclusion'] = 'failure'
                elif mutation == 'running':
                    self.api.job_items[20][1]['status'] = 'in_progress'
                else:
                    self.api.job_items[20].append(copy.deepcopy(self.api.job_items[20][1]))
                self.blocked('LIGHTWEIGHT_JOB_NOT_SUCCESS')

    def test_executed_full_job_cannot_claim_lightweight(self):
        self.api.job_items[20][2]['conclusion'] = 'success'
        self.blocked('LIGHTWEIGHT_FULL_JOB_EXECUTED')

    def test_wrong_pr_or_wrong_head_run_does_not_grant_merge(self):
        for field, value in [('head_sha', 'e' * 40), ('pull_requests', [{'number': 999}])]:
            with self.subTest(field=field):
                self.api = ObservationFacts()
                self.api.merge[field] = value
                self.assertEqual(self.observe()['state'], 'LIGHTWEIGHT_QUALIFIED')

    def test_head_changed_during_observation_fails_closed(self):
        self.api.final_pull = copy.deepcopy(self.api.pull)
        self.api.final_pull['head']['sha'] = 'e' * 40
        self.blocked('HEAD_CHANGED_DURING_OBSERVATION')

    def test_draft_closed_or_retargeted_pr_cannot_be_recommended_for_merge(self):
        for mutation in ('draft', 'closed', 'base'):
            with self.subTest(mutation=mutation):
                self.api = ObservationFacts()
                self.api.final_pull = copy.deepcopy(self.api.pull)
                if mutation == 'draft':
                    self.api.final_pull['draft'] = True
                elif mutation == 'closed':
                    self.api.final_pull['state'] = 'closed'
                else:
                    self.api.final_pull['base']['ref'] = 'develop'
                self.blocked('PR_NOT_READY_TO_MERGE')

    def test_multiple_merge_runs_cannot_be_collapsed_to_one_green(self):
        self.api.workflow_runs.append({**self.api.merge, 'id': 19})
        self.blocked('FULL_RUN_BUDGET_EXCEEDED')

    def test_rerun_merge_attempt_does_not_reuse_old_receipt(self):
        self.api.merge['run_attempt'] = 2
        self.blocked('FULL_ATTEMPT_BUDGET_EXCEEDED')

    def test_archive_name_must_be_canonical(self):
        def invalid_archive(path, cap):
            out = io.BytesIO()
            with zipfile.ZipFile(out, 'w') as archive:
                archive.writestr('other.json', json.dumps(self.api.receipt))
            return out.getvalue()
        self.api.raw = invalid_archive
        self.blocked('RECEIPT_ARCHIVE_INVALID')

    def test_receipt_size_limit_remains_enforced(self):
        self.api.receipt['extra'] = 'x' * (2 * 1024 * 1024)
        self.blocked('RECEIPT_TOO_LARGE')

    def test_product_change_still_rejects_a_lightweight_receipt(self):
        self.api.files.append({'filename': 'internal/access/application/runtime.go', 'status': 'removed'})
        self.blocked('RECEIPT_BINDING_MISMATCH')

    def test_full_receipt_path_is_preserved(self):
        self.api.files = [{'filename': 'scripts/delivery_progress.py', 'status': 'modified'}]
        expected = progress.expected_jobs(json.loads((progress.ROOT / 'scripts/ci_topology_contract.json').read_text()))
        self.api.receipt.update(state='MERGE_READY', change_class='product_change',
                                merge_gate_required=True, active_full_jobs=0,
                                full_results={name: 'success' for name in expected})
        report = self.observe()
        self.assertEqual(report['state'], 'MERGE_READY')
        self.assertEqual(report['expected_head_sha'], self.api.candidate)
        self.assertEqual(self.api.calls[-1], ('get', '/git/ref/heads/main'))

    def test_full_receipt_also_rejects_stale_main(self):
        self.api.files = [{'filename': 'scripts/delivery_progress.py', 'status': 'modified'}]
        expected = progress.expected_jobs(json.loads((progress.ROOT / 'scripts/ci_topology_contract.json').read_text()))
        self.api.receipt.update(state='MERGE_READY', active_full_jobs=0,
                                full_results={name: 'success' for name in expected})
        self.api.main_sha = 'd' * 40
        self.blocked('MAIN_CHANGED_REQUALIFY')


if __name__ == '__main__':
    unittest.main()
