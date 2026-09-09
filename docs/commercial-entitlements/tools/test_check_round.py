import subprocess
import tempfile
import unittest
from pathlib import Path
from check_round import check_completion


class RoundGateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.git('init', '-b', 'main')
        self.git('config', 'user.name', 'CE01 test fixture')
        self.git('config', 'user.email', 'ce01@example.com')
        (self.repo / 'fixture').write_text('test-only fixture\n')
        self.git('add', 'fixture')
        self.git('commit', '-m', 'test fixture')
        self.sha = self.git('rev-parse', 'HEAD')
        self.task = {'id': 'CE-01', 'status': 'DONE', 'integration_commit': self.sha}
        # These records exercise the validator, not product qualification.
        self.proof = {'task_id': 'CE-01', 'verified_commit': self.sha, 'result': 'PASS', 'checks': [{'command': 'fixture test', 'exit_code': 0, 'tests_passed': 1, 'evidence': 'fixture-log'}]}

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args], text=True, stderr=subprocess.DEVNULL).strip()

    def errors(self):
        return check_completion(self.task, self.proof, self.repo, 'refs/heads/main')

    def test_complete_fixture_accepted(self):
        self.assertEqual(self.errors(), [])

    def test_submitted_only_rejected(self):
        self.task['status'] = 'VERIFYING'
        self.assertTrue(any('DONE' in x for x in self.errors()))

    def test_zero_tests_rejected(self):
        self.proof['checks'][0]['tests_passed'] = 0
        self.assertTrue(any('zero executed' in x for x in self.errors()))

    def test_failed_command_rejected(self):
        self.proof['checks'][0]['exit_code'] = 1
        self.assertTrue(any('zero exit' in x for x in self.errors()))

    def test_wrong_verified_sha_rejected(self):
        self.proof['verified_commit'] = 'f' * 40
        self.assertTrue(any('integrated commit' in x for x in self.errors()))

    def test_missing_log_rejected(self):
        self.proof['checks'][0].pop('evidence')
        self.assertTrue(any('evidence locator' in x for x in self.errors()))

    def test_unmerged_commit_rejected(self):
        self.git('checkout', '-b', 'candidate')
        (self.repo / 'fixture').write_text('candidate\n')
        self.git('commit', '-am', 'unmerged fixture')
        candidate = self.git('rev-parse', 'HEAD')
        self.task['integration_commit'] = candidate
        self.proof['verified_commit'] = candidate
        self.assertTrue(any('refs/heads/main' in x for x in self.errors()))

    def test_unknown_git_commit_rejected(self):
        self.task['integration_commit'] = 'f' * 40
        self.proof['verified_commit'] = 'f' * 40
        self.assertTrue(any('ancestor' in x for x in self.errors()))


if __name__ == '__main__':
    unittest.main()
