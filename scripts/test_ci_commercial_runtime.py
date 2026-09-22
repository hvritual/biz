#!/usr/bin/env python3
"""Offline adversarial coverage/runner checks, never live CI qualification."""
import copy
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stdout, redirect_stderr
from unittest.mock import patch

import ci_commercial_runtime as c
import check_ci_commercial as checks

ROOT = Path(__file__).resolve().parents[1]


def events():
    return [{'Action': 'run', 'Package': 'sample', 'Test': 'TestOne'},
            {'Action': 'pass', 'Package': 'sample', 'Test': 'TestOne'},
            {'Action': 'pass', 'Package': 'sample'}]


class EvidenceTests(unittest.TestCase):
    def setUp(self):
        self.baseline = {'leaf_tests': 1, 'passed': {'sample': ['TestOne']}}

    def test_complete_identity(self):
        result = c.verify_suite(events(), self.baseline)
        self.assertEqual(result['baseline_identities_preserved'], 1)

    def test_same_count_different_identity_rejected(self):
        rows = events(); rows[1]['Test'] = 'TestImposter'
        with self.assertRaisesRegex(ValueError, 'IDENTITY_MISSING'):
            c.verify_suite(rows, self.baseline)

    def test_failed_rejected(self):
        with self.assertRaises(ValueError):
            c.verify_suite(events()+[{'Action':'fail','Package':'sample'}], self.baseline)

    def test_skipped_rejected(self):
        with self.assertRaises(ValueError):
            c.verify_suite(events()+[{'Action':'skip','Package':'sample','Test':'Other'}], self.baseline)

    def test_zero_execution_rejected(self):
        with self.assertRaises(ValueError): c.verify_suite([], self.baseline)

    def test_missing_package_completion_rejected(self):
        with self.assertRaisesRegex(ValueError, 'COMPLETION'):
            c.verify_suite(events()[:-1], self.baseline)

    def test_additional_tests_allowed(self):
        rows = events()+[{'Action':'pass','Package':'sample','Test':'TestNew'}]
        self.assertEqual(c.verify_suite(rows,self.baseline)['leaf_tests'], 2)

    def test_another_package_cannot_replace_identity(self):
        rows = [{**row,'Package':'other'} for row in events()]
        with self.assertRaisesRegex(ValueError, 'IDENTITY_MISSING'):
            c.verify_suite(rows,self.baseline)

    def test_leaf_shrink_rejected(self):
        self.baseline['leaf_tests']=2
        with self.assertRaisesRegex(ValueError,'LEAF_COUNT'):
            c.verify_suite(events(),self.baseline)

    def test_metadata_does_not_invent_tests(self):
        with self.assertRaises(ValueError):
            c.verify_suite([{'Action':'output','Package':'sample','Output':'PASS'}],self.baseline)


class RunnerTests(unittest.TestCase):
    def run_command(self, argv, timeout=2):
        with tempfile.TemporaryDirectory() as d, redirect_stdout(io.StringIO()), redirect_stderr(io.StringIO()):
            out=Path(d)/'phase.jsonl'
            rc=c.run_phase(out, argv, timeout=timeout, interval=0.05)
            return rc,out.read_text(),out.with_suffix('.stderr').read_text(),out.with_suffix('.exit').read_text()

    def test_actual_stdout_and_stderr_separate(self):
        rc,out,err,exit_code=self.run_command([sys.executable,'-c','import sys;print("stdout");print("stderr",file=sys.stderr)'])
        self.assertEqual((rc,out,err,exit_code),(0,'stdout\n','stderr\n','0\n'))

    def test_failure_propagates(self):
        rc,_,_,exit_code=self.run_command([sys.executable,'-c','raise SystemExit(7)'])
        self.assertEqual((rc,exit_code),(7,'7\n'))

    def test_timeout_is_failure_and_retained(self):
        with tempfile.TemporaryDirectory() as d, redirect_stdout(io.StringIO()):
            out=Path(d)/'timeout.jsonl'
            with self.assertRaises(TimeoutError):
                c.run_phase(out,[sys.executable,'-c','import time;time.sleep(60)'],timeout=0.15,interval=0.05)
            self.assertEqual(out.with_suffix('.exit').read_text(),'124\n')

    def test_argv_not_shell_evaluated(self):
        _,out,_,_=self.run_command([sys.executable,'-c','import sys;print(sys.argv[1])','$(printf injected)'])
        self.assertEqual(out,'$(printf injected)\n')

    def test_empty_command_refused(self):
        with self.assertRaises(ValueError): c.run_phase(Path('/tmp/unused'),[])

    def test_ci_only_before_docker(self):
        result=subprocess.run(['bash',str(ROOT/'scripts/ci_commercial_mysql.sh'),'start','ce04'],
            env={**os.environ,'GITHUB_ACTIONS':'false'},capture_output=True,text=True)
        self.assertEqual(result.returncode,2)
        self.assertIn('CI_OWNED_DATABASE_ONLY',result.stderr)

    def test_invalid_gate_refused(self):
        result=subprocess.run(['bash',str(ROOT/'scripts/ci_commercial_mysql.sh'),'start','../ce04'],
            env={**os.environ,'GITHUB_ACTIONS':'true','GITHUB_RUN_ID':'1','GITHUB_RUN_ATTEMPT':'1'},capture_output=True,text=True)
        self.assertEqual(result.returncode,2)


class SourceTests(unittest.TestCase):
    def mutated(self, path, transform):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d)
            paths=['scripts/ci_commercial_baseline.json','scripts/ci_commercial_mysql.sh','.github/workflows/pr-qualification.yml']
            for n in range(4,8):
                paths += [f'scripts/ce0{n}_qualify.sh',f'.github/workflows/ce0{n}-qualification.yml']
            for p in paths:
                target=root/p;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((ROOT/p).read_bytes())
            p=root/path;p.write_text(transform(p.read_text()))
            with self.assertRaises(ValueError): checks.check_sources(root)

    def test_current_sources(self):
        with redirect_stdout(io.StringIO()): checks.check_sources()

    def test_race_removed(self):
        self.mutated('scripts/ce06_qualify.sh',lambda s:s.replace('go test -race','go test'))

    def test_normal_selector_changed(self):
        self.mutated('scripts/ce07_qualify.sh',lambda s:s.replace('^TestCE07MySQL','^TestCE07Nothing'))

    def test_restart_removed(self):
        self.mutated('scripts/ce04_qualify.sh',lambda s:s.replace('docker restart "$MYSQL_CONTAINER_ID"','echo skipped'))

    def test_evidence_hook_removed(self):
        self.mutated('scripts/ce05_qualify.sh',lambda s:s.replace('verify_output(','fake_output('))

    def test_db_parallelism_rejected(self):
        self.mutated('scripts/ce07_qualify.sh',lambda s:s+'true &\n')

    def test_tmpfs_cannot_replace_durable_restart(self):
        self.mutated('scripts/ci_commercial_mysql.sh',lambda s:s.replace('docker run','docker run --tmpfs /var/lib/mysql'))

    def test_cache_disabled(self):
        self.mutated('.github/workflows/ce04-qualification.yml',lambda s:s.replace('cache: true','cache: false'))

    def test_targeted_hook_removed(self):
        self.mutated('.github/workflows/pr-qualification.yml',lambda s:s.replace('ci_commercial_runtime.py check','echo disabled'))

    def test_timeout_relaxed(self):
        self.mutated('.github/workflows/ce06-qualification.yml',lambda s:s.replace('timeout-minutes: 5','timeout-minutes: 30'))


if __name__=='__main__':
    unittest.main()
