#!/usr/bin/env python3
import copy
import tempfile
from pathlib import Path
import unittest
from ci_access_qualification import PACKAGE, load_json, test_argv, validate, verify_events


def events(name='TestExample'):
    return [{'Action': 'run', 'Test': name, 'Package': PACKAGE},
            {'Action': 'pass', 'Test': name, 'Package': PACKAGE}, {'Action': 'pass', 'Package': PACKAGE}]


class EvidenceTests(unittest.TestCase):
    def test_exact_pass(self):
        self.assertEqual(verify_events(events(), {'TestExample'}, 0), ['TestExample'])

    def test_no_tests_is_not_success(self):
        with self.assertRaises(ValueError):
            verify_events([{'Action': 'pass', 'Package': PACKAGE}], {'TestExample'}, 0)

    def test_printed_pass_is_not_execution(self):
        with self.assertRaises(ValueError):
            verify_events([{'Action': 'output', 'Output': '--- PASS: TestExample', 'Package': PACKAGE}], {'TestExample'}, 0)

    def test_skip_is_not_pass(self):
        for name in ['TestExample', 'TestExample/subcase']:
            with self.subTest(name=name), self.assertRaises(ValueError):
                verify_events(events() + [{'Action': 'skip', 'Test': name, 'Package': PACKAGE}], {'TestExample'}, 0)

    def test_duplicate_identity_is_rejected(self):
        with self.assertRaises(ValueError):
            verify_events(events() + events(), {'TestExample'}, 0)

    def test_wrong_package_is_rejected(self):
        changed = events()
        changed[0]['Package'] = 'other/package'
        with self.assertRaises(ValueError):
            verify_events(changed, {'TestExample'}, 0)

    def test_failure_exit_not_overridden_by_pass(self):
        with self.assertRaises(ValueError):
            verify_events(events(), {'TestExample'}, 1)

    def test_failure_event_not_overridden_by_pass(self):
        with self.assertRaises(ValueError):
            verify_events(events() + [{'Action': 'fail', 'Package': PACKAGE}], {'TestExample'}, 0)

    def test_selector_is_an_argv_item_not_shell(self):
        self.assertEqual(test_argv({'tests': [{'name': 'TestOne'}, {'name': 'TestTwo'}]})[-1], '^(TestOne|TestTwo)$')

    def test_duplicate_json_key(self):
        with self.assertRaises(ValueError):
            load_json('{"schema_version":1,"schema_version":2}')


class RegistryTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        (self.root / 'integration').mkdir()
        (self.root / 'integration/example_test.go').write_text('package integration\nfunc TestExample(t *testing.T) {}\nfunc TestAdded(t *testing.T) {}\n')
        self.data = {'schema_version': 1, 'suites': [{'id': 'example', 'tests': [{'name': 'TestExample', 'file': 'integration/example_test.go'}]}]}

    def test_existing_source_passes(self):
        self.assertEqual(len(validate(self.data, self.root)), 1)

    def test_append_is_allowed(self):
        changed = copy.deepcopy(self.data)
        changed['suites'][0]['tests'].append({'name': 'TestAdded', 'file': 'integration/example_test.go'})
        self.assertEqual(len(validate(changed, self.root, self.data)), 2)

    def test_removal_is_rejected(self):
        changed = copy.deepcopy(self.data)
        changed['suites'][0]['tests'][0]['name'] = 'TestAdded'
        with self.assertRaisesRegex(ValueError, 'HISTORICAL_TEST'):
            validate(changed, self.root, self.data)

    def test_changed_owner_is_rejected(self):
        changed = copy.deepcopy(self.data)
        changed['suites'][0]['id'] = 'other-owner'
        with self.assertRaisesRegex(ValueError, 'HISTORICAL_TEST'):
            validate(changed, self.root, self.data)

    def test_missing_test_is_rejected(self):
        self.data['suites'][0]['tests'][0]['name'] = 'TestAbsent'
        with self.assertRaisesRegex(ValueError, 'DECLARATION'):
            validate(self.data, self.root)

    def test_comment_is_not_test(self):
        (self.root / 'integration/example_test.go').write_text('/*\nfunc TestExample(t *testing.T) {}\n*/')
        with self.assertRaises(ValueError):
            validate(self.data, self.root)

    def test_malicious_selector_is_rejected(self):
        self.data['suites'][0]['tests'][0]['name'] = 'TestExample; touch bad'
        with self.assertRaisesRegex(ValueError, 'TEST_NAME'):
            validate(self.data, self.root)

    def test_duplicate_test_is_rejected(self):
        self.data['suites'][0]['tests'] *= 2
        with self.assertRaisesRegex(ValueError, 'DUPLICATE_TEST'):
            validate(self.data, self.root)

    def test_escape_is_rejected(self):
        self.data['suites'][0]['tests'][0]['file'] = '../example_test.go'
        with self.assertRaisesRegex(ValueError, 'TEST_PATH'):
            validate(self.data, self.root)


if __name__ == '__main__':
    unittest.main()
