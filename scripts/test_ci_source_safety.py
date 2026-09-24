#!/usr/bin/env python3
import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch
from check_ci_source_safety import check, PROTECTED, validate_workflow


def workflow(script):
    return 'name: test\non: [push]\njobs:\n  check:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n' + '\n'.join('          ' + line for line in script.splitlines()) + '\n'


class SourceTests(unittest.TestCase):
    def test_valid_quoted_selector(self):
        self.assertEqual(validate_workflow(workflow("go test -run '^TestExample$' | tee log")), 1)

    def test_missing_quote_is_blocked_before_go(self):
        with self.assertRaisesRegex(ValueError, 'SHELL_SYNTAX'):
            validate_workflow(workflow("go test -run '^TestExample | tee log"))

    def test_duplicate_yaml_key_rejected(self):
        with self.assertRaisesRegex(ValueError, 'DUPLICATE_YAML_KEY'):
            validate_workflow('jobs:\n  x: {}\n  x: {}\n')

    def test_heredoc_supported(self):
        self.assertEqual(validate_workflow(workflow("python3 - <<'PYTHON'\nprint('ok')\nPYTHON")), 1)

    def test_github_expressions_supported(self):
        self.assertEqual(validate_workflow(workflow('echo "${{ github.sha }}"')), 1)

    def test_empty_run_rejected(self):
        with self.assertRaisesRegex(ValueError, 'EMPTY_RUN'):
            validate_workflow(workflow(''))

    def test_registry_routes_to_access_without_workflow_changes(self):
        from ci_changed_files_router import classify_path
        self.assertIn('access', classify_path('scripts/ci_access_tests.json'))

    def test_business_control_mutation_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / PROTECTED[0]
            path.parent.mkdir(parents=True)
            path.write_text('changed')
            with patch('check_ci_source_safety.original', return_value=b'approved'), self.assertRaisesRegex(ValueError, 'REQUIRES_GOVERNANCE'):
                check(root, 'base', 'feat/business')


if __name__ == '__main__':
    unittest.main()
