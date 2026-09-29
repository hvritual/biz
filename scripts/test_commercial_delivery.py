"""Isolated synthetic Git fixtures; these tests do not certify biz runtime."""
import copy
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

from check_commercial_delivery import PLAN, validate


class DeliveryReferencesTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.write('implementation.go', 'package example\nfunc Execute() {}\n')
        self.write('web/src/Real.vue', '<template>Real fixture</template>')
        self.write('web/src/LifecycleManagementView.vue', '<template>Preview fixture</template>')
        self.write('tests/mock.ts', 'await page.route("**/api", handler)')
        self.write(str(PLAN / 'evidence/a.md'), '# Synthetic acceptance fixture')
        self.dump(str(PLAN / 'evidence/a.json'), {'fixture': True})
        self.dump('contracts/generated/operation-plans.json', {'operations': [{'operationId': 'example.execute'}]})
        self.dump('web/ui-contracts.json', {'routes': [
            {'path': '/enterprise/plan', 'component': '@/Real.vue'},
            {'path': '/platform/commercial/quotas', 'component': '@/LifecycleManagementView.vue'}]})
        self.run_git('init', '-q')
        self.run_git('add', '.')
        self.run_git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'fixture')
        self.sha = self.run_git('rev-parse', 'HEAD')
        self.plan = {'tasks': [
            {'id': 'CE-01', 'status': 'DONE', 'evidence': 'evidence/a.md',
             'verification': 'evidence/a.json', 'integration_commit': self.sha},
            {'id': 'CE-14', 'status': 'PLANNED'}]}
        self.index = {
            'schema_version': 1, 'status_authority': str(PLAN / 'tasks.json'),
            'observed_at_commit': self.sha,
            'task_links': {'CE-01': {'tracking_issues': [112]},
                           'CE-14': {'tracking_issues': [131], 'unresolved_scope': ['full journey'],
                                     'sources': [{'path': 'implementation.go', 'symbol': 'func Execute()'}],
                                     'tests': [{'path': 'tests/mock.ts', 'kind': 'ui_mock'}]}},
            'authority_owners': [{'operation': 'example.execute', 'task': 'CE-01',
                                 'source': {'path': 'implementation.go', 'symbol': 'func Execute()'}}],
            'consumers': {'/enterprise/plan': {'task': 'CE-14', 'kind': 'api_implementation',
                                              'operations': ['example.execute']},
                          '/platform/commercial/quotas': {'task': 'CE-14', 'kind': 'design_preview'}}}

    def write(self, path, text):
        file = self.root / path
        file.parent.mkdir(parents=True, exist_ok=True)
        file.write_text(text, encoding='utf-8')

    def dump(self, path, data):
        self.write(path, json.dumps(data))

    def run_git(self, *args):
        return subprocess.run(['git', '-C', str(self.root), *args], text=True,
                              capture_output=True, check=True).stdout.strip()

    def report(self):
        self.dump(str(PLAN / 'tasks.json'), self.plan)
        self.dump(str(PLAN / 'delivery-links.json'), self.index)
        return validate(self.root)

    def blocked(self, code):
        report = self.report()
        self.assertEqual('FAIL', report['result'], report)
        self.assertIn(code, [error['code'] for error in report['errors']], report)

    def test_valid_references_do_not_certify_runtime(self):
        result = self.report()
        self.assertEqual('PASS', result['result'], result)
        self.assertEqual('NOT_PERFORMED', result['runtime_certification'])
        self.assertEqual('NOT_PERFORMED', result['issue_state_verification'])
        self.assertEqual(2, len(result['tasks']))
        self.assertTrue(result['input_sha256'])

    def test_partial_observation_preserves_status(self):
        result = self.report()
        self.assertEqual('PLANNED', result['tasks'][1]['status_from_authority'])
        self.assertIn('PARTIAL_IMPLEMENTATION_NOT_ACCEPTANCE', [x['code'] for x in result['observations']])
        self.assertFalse(result['tasks'][1]['tests'][0]['execution_claimed'])

    def test_done_without_evidence_is_blocked(self):
        self.plan['tasks'][0]['evidence'] = None
        self.blocked('DONE_EVIDENCE_REQUIRED')

    def test_done_without_verification_is_blocked(self):
        self.plan['tasks'][0]['verification'] = None
        self.blocked('DONE_EVIDENCE_REQUIRED')

    def test_missing_evidence_file_is_blocked(self):
        self.plan['tasks'][0]['evidence'] = 'missing.md'
        self.blocked('INVALID_INPUT')

    def test_full_commit_must_resolve(self):
        self.plan['tasks'][0]['integration_commit'] = '0' * 40
        self.blocked('COMMIT_UNRESOLVED')

    def test_short_commit_rejected(self):
        self.plan['tasks'][0]['integration_commit'] = self.sha[:7]
        self.blocked('FULL_COMMIT_REQUIRED')

    def test_real_route_without_authority_is_blocked(self):
        self.index['consumers']['/enterprise/plan']['operations'] = []
        self.blocked('REAL_ROUTE_AUTHORITY_REQUIRED')

    def test_preview_cannot_be_real(self):
        self.index['consumers']['/platform/commercial/quotas'] = {
            'task': 'CE-14', 'kind': 'api_implementation', 'operations': ['example.execute']}
        self.blocked('PREVIEW_AS_REAL_AUTHORITY')

    def test_static_index_cannot_claim_production(self):
        self.index['consumers']['/enterprise/plan']['production_verified'] = True
        self.blocked('STATIC_INDEX_CANNOT_CERTIFY_PRODUCTION')

    def test_mock_cannot_be_integrated_source(self):
        self.index['task_links']['CE-14']['tests'][0]['kind'] = 'mysql_source'
        self.blocked('MOCK_AS_INTEGRATION')

    def test_duplicate_owner_rejected(self):
        self.index['authority_owners'].append(copy.deepcopy(self.index['authority_owners'][0]))
        self.blocked('MULTIPLE_AUTHORITY_OWNERS')

    def test_missing_symbol_rejected(self):
        self.index['authority_owners'][0]['source']['symbol'] = 'DoesNotExist()'
        self.blocked('SOURCE_SYMBOL_MISSING')

    def test_unknown_operation_rejected(self):
        self.index['authority_owners'][0]['operation'] = 'unknown'
        self.blocked('UNKNOWN_AUTHORITY_OPERATION')

    def test_path_escape_rejected(self):
        self.index['authority_owners'][0]['source']['path'] = '../outside.go'
        self.blocked('INVALID_INPUT')

    def test_symlink_rejected(self):
        file = self.root / 'linked.go'
        file.symlink_to(self.root / 'implementation.go')
        self.index['authority_owners'][0]['source']['path'] = 'linked.go'
        self.blocked('INVALID_INPUT')

    def test_second_status_authority_rejected(self):
        self.index['task_links']['CE-14']['status'] = 'DONE'
        self.blocked('DUPLICATE_STATUS_AUTHORITY')

    def test_unresolved_scope_cannot_close(self):
        self.plan['tasks'][1].update(self.plan['tasks'][0], id='CE-14')
        self.blocked('DONE_WITH_UNRESOLVED_SCOPE')

    def test_missing_task_link_rejected(self):
        del self.index['task_links']['CE-14']
        self.blocked('TASK_LINK_COVERAGE')

    def test_new_unmapped_consumer_rejected(self):
        self.dump('web/ui-contracts.json', {'routes': [
            {'path': '/enterprise/plan', 'component': '@/Real.vue'},
            {'path': '/platform/commercial/quotas', 'component': '@/LifecycleManagementView.vue'},
            {'path': '/platform/commercial/new', 'component': '@/Real.vue'}]})
        self.blocked('CONSUMER_COVERAGE')

    def test_duplicate_json_key_rejected(self):
        self.report()
        self.write(str(PLAN / 'delivery-links.json'), '{"schema_version":1,"schema_version":2}')
        result = validate(self.root)
        self.assertEqual('FAIL', result['result'])
        self.assertIn('DUPLICATE_JSON_KEY', result['errors'][0]['context'])

    def test_invalid_json_is_reported(self):
        self.report()
        self.write(str(PLAN / 'tasks.json'), '{')
        self.assertEqual('FAIL', validate(self.root)['result'])


if __name__ == '__main__':
    unittest.main()
