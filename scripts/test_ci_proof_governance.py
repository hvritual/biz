#!/usr/bin/env python3
"""Adversarial/offline tests; fixtures can never certify a GitHub candidate."""
from __future__ import annotations

import copy
from datetime import datetime, timedelta, timezone
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import ci_proof_governance as g

ROOT = Path(__file__).resolve().parents[1]
C = g.strict_json((ROOT / g.CONTRACT).read_text())
T = g.strict_json((ROOT / g.TOPOLOGY).read_text())
SHA = 'a' * 40


def jobs_fixture():
    return [{'id': i, 'name': name, 'head_sha': SHA, 'run_id': 101, 'run_attempt': 1,
             'status': 'completed', 'conclusion': 'success',
             'created_at': '2026-09-22T09:00:00Z', 'started_at': '2026-09-22T09:00:10Z',
             'completed_at': '2026-09-22T09:01:50Z'}
            for i, name in enumerate(g.expected_full(C, T), 1)]


class ContractTests(unittest.TestCase):
    def reject(self, mutate, contains):
        c = copy.deepcopy(C)
        mutate(c)
        with self.assertRaisesRegex(g.Violation, contains):
            g.validate(c, T, ROOT)

    def test_all_35_units_registered(self):
        owners, obs = g.validate(C, T, ROOT)
        self.assertEqual(len(obs), 35)
        self.assertEqual(len(g.expected_full(C, T)), 42)
        self.assertIn('repository.go.regression', owners)

    def test_missing_gate(self):
        self.reject(lambda c: c['gates'].pop('ce06-qualification.yml'), 'GATE_CONTRACT_SET')

    def test_unknown_gate(self):
        self.reject(lambda c: c['gates'].update({'unregistered.yml': {}}), 'GATE_CONTRACT_SET')

    def test_duplicate_owner(self):
        self.reject(lambda c: c['gates']['ce07-qualification.yml']['owns'].append('repository.go.regression'), 'DUPLICATE_PROOF_OWNER')

    def test_missing_terminal_owner(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml']['delegates'].append('ghost.proof'), 'TERMINAL_OWNER')

    def test_self_delegation(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml']['delegates'].append('ce06-qualification.qualification'), 'TERMINAL_OWNER')

    def test_empty_ownership(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml'].update(owns=[]), 'OWNER_EMPTY')

    def test_unknown_fields(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml'].update(skip=True), 'GATE_FIELDS')

    def test_zero_budget(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml'].update(target_seconds=0), 'BUDGET_INVALID')

    def test_boolean_budget(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml'].update(target_seconds=True), 'BUDGET_INVALID')

    def test_inverted_budget(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml'].update(target_seconds=250, hard_seconds=120), 'BUDGET_INVALID')

    def test_receipt_metric_cannot_replace_wall(self):
        self.reject(lambda c: c.update(metric='test_only'), 'METRIC_INVALID')

    def test_race_narrowing_requires_manifest_review(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml'].update(race_commands=[]), 'RACE_SCOPE_DRIFT')

    def test_race_manifest_cannot_be_faked(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml']['race_commands'].append('go test -race ./ghost'), 'RACE_SCOPE_DRIFT')

    def test_restart_removal(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml'].update(restart='none'), 'RESTART_SCOPE_DRIFT')

    def test_declared_jobs_cannot_shrink(self):
        self.reject(lambda c: c['gates']['enterprise-role-qualification.yml'].update(jobs=['admission']), 'JOB_SET_DRIFT')

    def test_expired_debt(self):
        self.reject(lambda c: c.update(legacy_debt_expires='2000-01-01'), 'LEGACY_DEBT_EXPIRED')

    def test_unregistered_cost_is_blocked(self):
        self.reject(lambda c: c['gates']['ce06-qualification.yml'].update(legacy_cost_ceiling={}), 'NEW_BOOTSTRAP_OR_DUPLICATE_COST')

    def test_duplicate_json_keys(self):
        with self.assertRaisesRegex(g.Violation, 'DUPLICATE_JSON_KEY'):
            g.strict_json('{"owns": [], "owns": ["fabricated"]}')

    def test_nonfinite_json(self):
        with self.assertRaises(g.Violation):
            g.strict_json('{"target": NaN}')

    def test_parser_does_not_silently_accept_matrix(self):
        with self.assertRaises(g.Violation):
            g.job_ids('jobs:\n  test:\n    strategy:\n      matrix: {}\n')

    def test_parser_rejects_duplicate_job_ids(self):
        with self.assertRaises(g.Violation):
            g.job_ids('jobs:\n  test:\n    runs-on: ubuntu-latest\n  test:\n')

    def test_source_path_escape(self):
        with self.assertRaises(g.Violation):
            g.read(ROOT, '../not-a-source')

    def test_symlink_escape(self):
        with tempfile.TemporaryDirectory() as d, tempfile.TemporaryDirectory() as outside:
            Path(outside, 'file').write_text('not repository data')
            Path(d, 'file').symlink_to(Path(outside, 'file'))
            with self.assertRaises(g.Violation):
                g.read(Path(d), 'file')

    def test_current_hooks(self):
        g.hook_check(ROOT)

    def test_new_nested_duplicate_detected(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            (root / '.github/workflows').mkdir(parents=True)
            (root / 'scripts').mkdir()
            (root / '.github/workflows/ce07-qualification.yml').write_text('jobs:\n  qualify:\n    steps:\n      - run: bash scripts/outer.sh\n')
            (root / 'scripts/outer.sh').write_text('bash scripts/inner.sh\n')
            (root / 'scripts/inner.sh').write_text("go test -tags=integration ./integration -run '^TestCE06MySQL'\n")
            obs = g.inventory(root, 'ce07-qualification.yml')
            self.assertEqual(obs['cost_candidates']['overlap.ce06.mysql'], 1)
            self.assertIn('scripts/inner.sh', obs['source_sha256'])

    def test_comment_is_not_execution(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d, '.github/workflows'); p.mkdir(parents=True)
            (p / 'a.yml').write_text('jobs:\n  qualify:\n    # go test ./...\n')
            self.assertEqual(g.inventory(Path(d), 'a.yml')['cost_candidates'], {})


class RuntimeTests(unittest.TestCase):
    def audit(self, jobs):
        return g.audit_jobs(C, T, jobs, 101, 1, SHA)

    def test_exact_complete_execution(self):
        rows = self.audit(jobs_fixture())
        self.assertEqual(len(rows), 42)
        self.assertTrue(all(r['job_wall_seconds'] == 100 and r['queue_seconds'] == 10 for r in rows))

    def test_missing_job(self):
        with self.assertRaisesRegex(g.Violation, 'EXPANDED_JOB_SET'):
            self.audit(jobs_fixture()[:-1])

    def test_duplicated_job(self):
        jobs = jobs_fixture();jobs[0] = jobs[1]
        with self.assertRaises(g.Violation): self.audit(jobs)

    def test_extra_job(self):
        jobs = jobs_fixture();jobs.append({**jobs[0], 'name':'full-36-extra / ghost'})
        with self.assertRaises(g.Violation): self.audit(jobs)

    def test_skipped_cancelled_failed_running_rejected(self):
        for status, result in [('completed','skipped'),('completed','cancelled'),('completed','failure'),('in_progress',None)]:
            with self.subTest(result=result):
                jobs=jobs_fixture();jobs[0].update(status=status, conclusion=result)
                with self.assertRaises(g.Violation): self.audit(jobs)

    def test_wrong_sha_run_attempt_rejected(self):
        for key, value in [('head_sha','b'*40),('run_id',999),('run_attempt',2)]:
            with self.subTest(key=key):
                jobs=jobs_fixture();jobs[0][key]=value
                with self.assertRaises(g.Violation):self.audit(jobs)

    def test_negative_or_missing_time(self):
        for end in [None, '2020-01-01T00:00:00Z', '2026-09-22T09:02:00']:
            jobs=jobs_fixture();jobs[0]['completed_at']=end
            with self.assertRaises(g.Violation):self.audit(jobs)

    def test_hard_budget_not_hidden_by_test_time(self):
        jobs=jobs_fixture();jobs[0]['completed_at']='2026-09-22T09:10:00Z'
        rows=self.audit(jobs)
        self.assertEqual(rows[0]['performance'],'HARD_EXCEEDED')
        self.assertEqual(rows[0]['job_wall_seconds'],590)

    def test_target_breach_reported_separately(self):
        jobs=jobs_fixture();jobs[0]['completed_at']='2026-09-22T09:02:05Z'
        row=next(r for r in self.audit(jobs) if r['job_id']==jobs[0]['id'])
        self.assertEqual(row['performance'],'TARGET_EXCEEDED')

    def test_control_jobs_not_counted_as_coverage(self):
        jobs=jobs_fixture();jobs.append({'name':'merge-ready','status':'in_progress'})
        self.assertEqual(len(self.audit(jobs)),42)

    def test_fast_gate_skip_does_not_prove_delegation(self):
        class API:
            def jobs(self, run):
                return [{'name':'fast-web / qualify','run_id':3,'run_attempt':1,'head_sha':SHA,
                         'status':'completed','conclusion':'skipped'}]
        with self.assertRaises(g.Violation):g.check_fast_gate(API(), {'id':3,'run_attempt':1}, C, SHA)

    def test_receipt_identity_is_exact(self):
        run={'id':101,'run_attempt':1};q={'id':100,'run_attempt':1}
        receipt={'schema_version':1,'state':'VERIFIED','evidence_source':'github_api','repository':'hvritual/biz',
                 'contract_sha256':'c','topology_sha256':'t','candidate_sha':SHA,'candidate_tree':'tree',
                 'pr_number':180,'run_id':'101','run_attempt':1,'qualification_run_id':'100','qualification_run_attempt':1}
        def verify(r):g.verify_binding(r,'hvritual/biz','c','t',SHA,'tree',180,run,q)
        verify(receipt)
        for key,value in [('state','PASS'),('evidence_source','fixture'),('candidate_tree','old'),
                          ('contract_sha256','old'),('qualification_run_attempt',2),('run_attempt',True)]:
            with self.subTest(key=key):
                with self.assertRaises(g.Violation):verify({**receipt,key:value})

    def test_zip_requires_single_named_receipt(self):
        for names in [[g.RECEIPT],[g.RECEIPT,'extra'],['../'+g.RECEIPT]]:
            data=io.BytesIO()
            with zipfile.ZipFile(data,'w') as z:
                for name in names:z.writestr(name,'{}')
            if names==[g.RECEIPT]:self.assertEqual(g.receipt_zip(data.getvalue()),{})
            else:
                with self.assertRaises(g.Violation):g.receipt_zip(data.getvalue())


class BasePolicyTests(unittest.TestCase):
    def test_bootstrap_requires_exact_base_and_governance_branch(self):
        from types import SimpleNamespace
        with patch.object(g.subprocess, 'run', return_value=SimpleNamespace(returncode=1, stdout='')):
            with patch.dict(os.environ, {'GITHUB_HEAD_REF':'feat/enterprise-180'}):
                with self.assertRaisesRegex(g.Violation,'UNAPPROVED'):
                    g.check_base(ROOT,C['baseline_main'],C)
            with patch.dict(os.environ, {'GITHUB_HEAD_REF':'chore/ci-proof-production'}):
                g.check_base(ROOT,C['baseline_main'],C)
                with self.assertRaises(g.Violation):g.check_base(ROOT,'b'*40,C)

    def test_business_branch_cannot_edit_contract(self):
        from types import SimpleNamespace
        calls=[SimpleNamespace(returncode=0,stdout=json.dumps(C)), SimpleNamespace(returncode=0,stdout=b'altered')]
        with patch.object(g.subprocess,'run',side_effect=calls), patch.dict(os.environ,{'GITHUB_HEAD_REF':'chore/ci-enterprise-policy-contract'}):
            with self.assertRaisesRegex(g.Violation,'REQUIRES_GOVERNANCE'):
                g.check_base(ROOT,'a'*40,C)

    def test_governance_cannot_raise_hard_budget(self):
        from types import SimpleNamespace
        changed=copy.deepcopy(C);changed['gates']['ce06-qualification.yml']['hard_seconds']+=1
        with patch.object(g.subprocess,'run',return_value=SimpleNamespace(returncode=0,stdout=json.dumps(C))), patch.dict(os.environ,{'GITHUB_HEAD_REF':'chore/ci-proof-production'}):
            with self.assertRaisesRegex(g.Violation,'HARD_BUDGET_INCREASE'):
                g.check_base(ROOT,'a'*40,changed)

    def test_governance_cannot_raise_debt_ceiling(self):
        from types import SimpleNamespace
        changed=copy.deepcopy(C);changed['gates']['ce06-qualification.yml']['legacy_cost_ceiling']['go.all.test']=1
        with patch.object(g.subprocess,'run',return_value=SimpleNamespace(returncode=0,stdout=json.dumps(C))), patch.dict(os.environ,{'GITHUB_HEAD_REF':'chore/ci-proof-production'}):
            with self.assertRaisesRegex(g.Violation,'LEGACY_DEBT_INCREASE'):
                g.check_base(ROOT,'a'*40,changed)


if __name__ == '__main__':
    unittest.main()
