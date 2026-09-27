#!/usr/bin/env python3
import unittest
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


if __name__ == '__main__':
    unittest.main()
