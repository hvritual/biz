#!/usr/bin/env python3
"""Offline fault injection and evidence-tampering tests, not production receipts."""
from __future__ import annotations
import copy
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import ci_dependency_recovery as r
import delivery_execution as d

ROOT = Path(__file__).resolve().parents[1]
SPEC = r.policy(ROOT)
MODULE = 'github.com/aliyun/credentials-go@v1.1.2'
OTHER = 'example.com/library@v1.0.0'
CHECKSUM = 'h1:' + 'a' * 43 + '='
ERROR = MODULE + ': read "https://proxy.golang.org/github.com/aliyun/credentials-go/@v/v1.1.2.zip": stream error: stream ID 55; INTERNAL_ERROR; received from peer'


def response(requested, failures=(), error=ERROR):
    records = []
    for key in requested:
        path, version = key.split('@')
        item = {'Path': path, 'Version': version}
        item.update({'Error': error} if key in failures else {'Sum': CHECKSUM})
        records.append(json.dumps(item))
    return ('\n'.join(records) + '\n').encode()


class RecoveryFixture(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'source'
        self.root.mkdir()
        for name in SPEC['lock_files'] + SPEC['authorization_sources']:
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes((ROOT / name).read_bytes())
        (self.root / 'go.sum').write_text('\n'.join(key.replace('@', ' ') + ' ' + CHECKSUM for key in [MODULE, OTHER]) + '\n')
        for args in [['init', '-q'], ['add', '.'], ['-c', 'user.name=fixture', '-c', 'user.email=fixture@invalid', 'commit', '-qm', 'test fixture only']]:
            subprocess.run(['git', *args], cwd=self.root, check=True, capture_output=True)
        self.bound = {'repository': 'hvritual/biz', 'pr_number': 298, 'candidate_sha': 'a'*40,
            'candidate_tree': 'b'*40, 'frozen_main_sha': r.git(self.root, 'rev-parse', 'HEAD'),
            'checkout_sha': 'c'*40, 'run_id': '10', 'run_attempt': 1, 'job_id': 20,
            'job_name': 'full-08-ce03-qualification / qualify', 'step_name': SPEC['step'], 'step_number': 9}
        self.output = Path(self.temp.name) / 'evidence'
        self.calls = []
        now = datetime.now(timezone.utc)
        self.job = {'id': 20, 'run_id': 10, 'run_attempt': 1, 'head_sha': 'a'*40,
            'name': self.bound['job_name'], 'status': 'completed', 'conclusion': 'success',
            'steps': [{'name': SPEC['step'], 'number': 9, 'status': 'completed', 'conclusion': 'success',
                       'started_at': (now-timedelta(seconds=2)).isoformat(),
                       'completed_at': (now+timedelta(seconds=30)).isoformat()}]}

    def execute(self, failing_first=False, always_fail=False, error=ERROR, mutate=None):
        def fake(root, requested, spec, timeout):
            self.calls.append(list(requested))
            if mutate:
                mutate()
            failed = [MODULE] if (always_fail or failing_first and len(self.calls)==1) else []
            return (1 if failed else 0), response(requested, failed, error), b''
        return r.prepare(self.root, self.output, SPEC, lambda: dict(self.bound), executor=fake)

    def bundle(self, mutate=None, mutate_files=None):
        values = {p.name: p.read_bytes() for p in self.output.iterdir()}
        report = json.loads(values[r.RECEIPT])
        if mutate:
            mutate(report)
        values[r.RECEIPT] = json.dumps(report).encode()
        if mutate_files:
            mutate_files(values)
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, 'w') as archive:
            for name, raw in values.items(): archive.writestr(name, raw)
        return buffer.getvalue()

    def verify(self, **kwargs):
        return r.verify_bundle(self.bundle(**kwargs), SPEC, self.root, self.bound, self.job)

    def rejected(self, code, fn):
        with self.assertRaises(d.Blocked) as caught: fn()
        self.assertEqual(caught.exception.code, code)


class RecoveryTests(RecoveryFixture):
    def test_first_download_does_not_consume_recovery_or_claim_business_success(self):
        report = self.execute()
        self.assertEqual(report['state'], 'READY')
        self.assertEqual(len(self.calls), 1)
        self.assertEqual(report['business_certification'], 'NOT_PERFORMED')
        self.assertEqual(self.verify()['artifact_state'], 'READY')

    def test_only_failed_locked_module_is_retried(self):
        self.assertEqual(self.execute(failing_first=True)['state'], 'RECOVERED')
        self.assertEqual(self.calls, [sorted([MODULE, OTHER]), [MODULE]])
        self.assertEqual(self.verify()['attempts'], 2)

    def test_transport_budget_exhaustion_is_terminal(self):
        self.rejected('RECOVERY_TRANSPORT_BUDGET_EXHAUSTED', lambda: self.execute(always_fail=True))
        self.assertEqual(len(self.calls), 2)
        self.assertEqual(json.loads((self.output/r.RECEIPT).read_text())['state'], 'BLOCKED')

    def test_business_failure_cannot_use_download_recovery(self):
        self.rejected('RECOVERY_NON_TRANSPORT_FAILURE', lambda: self.execute(failing_first=True, error='--- FAIL: TestAuthorization'))
        self.assertEqual(len(self.calls), 1)

    def test_unknown_failure_cannot_retry(self):
        self.rejected('RECOVERY_NON_TRANSPORT_FAILURE', lambda: self.execute(failing_first=True, error='unknown network error'))
        self.assertEqual(len(self.calls), 1)

    def test_failed_source_snapshot_stops_before_retry(self):
        self.rejected('RECOVERY_SOURCE_CHANGED', lambda: self.execute(failing_first=True,
            mutate=lambda: (self.root/'go.mod').write_text('module tampered')))
        self.assertEqual(len(self.calls), 1)

    def test_candidate_policy_cannot_self_authorize(self):
        (self.root/'scripts/ci_dependency_recovery.py').write_text('# changed candidate runner\n')
        self.rejected('RECOVERY_NOT_AUTHORIZED_BY_FROZEN_MAIN', lambda: self.execute(failing_first=True))
        self.assertEqual(len(self.calls), 1)

    def test_live_candidate_or_main_change_stops_next_attempt(self):
        binding = copy.deepcopy(self.bound)
        def fake(*args):
            self.calls.append(1)
            binding['frozen_main_sha'] = 'd'*40
            return 1, response(args[1], [MODULE]), b''
        self.rejected('RECOVERY_BINDING_CHANGED', lambda: r.prepare(self.root, self.output, SPEC,
            lambda: dict(binding), executor=fake))
        self.assertEqual(len(self.calls), 1)

    def test_expired_recovery_lease_does_not_execute(self):
        times = iter([0, 91, 92])
        self.rejected('RECOVERY_LEASE_EXPIRED', lambda: r.prepare(self.root, self.output, SPEC,
            lambda: self.bound, executor=lambda *args: self.fail('expired lease executed'), clock=lambda: next(times)))

    def test_existing_output_cannot_be_reused(self):
        self.execute()
        self.rejected('RECOVERY_OUTPUT_ALREADY_EXISTS', lambda: self.execute())

    def test_receipt_repository_candidate_tree_base_job_attempt_cannot_change(self):
        self.execute()
        for field in self.bound:
            with self.subTest(field=field):
                self.rejected('RECOVERY_RECEIPT_BINDING_MISMATCH', lambda: self.verify(
                    mutate=lambda v: v['binding'].update({field: 'forged'})))

    def test_log_tampering_is_rejected(self):
        self.execute(failing_first=True)
        self.rejected('RECOVERY_LOG_DIGEST_MISMATCH', lambda: self.verify(
            mutate_files=lambda v: v.update({'download-1.json': b'forged'})))

    def test_forged_log_and_recomputed_hash_still_require_real_failure_grammar(self):
        self.execute(failing_first=True)
        raw = response(self.calls[0], [MODULE], 'business assertion failed')
        self.rejected('RECOVERY_NON_TRANSPORT_FAILURE', lambda: self.verify(
            mutate=lambda v: v['attempts'][0].update(stdout_sha256=r.digest(raw)),
            mutate_files=lambda v: v.update({'download-1.json': raw})))

    def test_widening_retry_to_successful_modules_is_rejected(self):
        self.execute(failing_first=True)
        self.rejected('RECOVERY_RETRY_SCOPE_MISMATCH', lambda: self.verify(
            mutate=lambda v: v['attempts'][1].update(requested=self.calls[0])))

    def test_source_proof_cannot_be_replaced(self):
        self.execute()
        self.rejected('RECOVERY_SOURCE_PROOF_MISMATCH', lambda: self.verify(mutate=lambda v: v.update(source_sha256={})))

    def test_additional_archive_content_is_rejected(self):
        self.execute()
        self.rejected('RECOVERY_UNEXPECTED_ARCHIVE_CONTENT', lambda: self.verify(
            mutate_files=lambda v: v.update({'../outside': b'x'})))

    def test_old_or_future_timestamps_are_rejected(self):
        self.execute()
        self.rejected('RECOVERY_TIME_BINDING_MISMATCH', lambda: self.verify(
            mutate=lambda v: v['attempts'][0].update(started_at='2000-01-01T00:00:00Z')))

    def test_green_report_does_not_override_failed_github_step(self):
        self.execute()
        self.job['steps'][0]['conclusion'] = 'failure'
        self.rejected('RECOVERY_STEP_NOT_SUCCESS', self.verify)

    def test_same_verifier_is_required_by_pr_and_both_main_receipts(self):
        proof = (ROOT/'scripts/ci_proof_governance.py').read_text()
        delivery = (ROOT/'scripts/delivery_execution.py').read_text()
        self.assertIn('recovery = verify_run(api, ROOT, repository, candidate, bound[', proof)
        self.assertIn("recovery = verify_run(api, ROOT, repository, candidate, tree, pr, run, receipt['frozen_main_sha'])", proof)
        self.assertIn('recovery = verify_run(api, ROOT, repository, candidate, tree, pr, merge_run, receipt["frozen_main_sha"])', delivery)

    def test_fixture_cannot_certify_actual_workflow(self):
        self.execute()
        self.rejected('RECOVERY_RECEIPT_SCHEMA_INVALID', lambda: self.verify(
            mutate=lambda v: v.update(business_certification='PASS')))


class ParserTests(unittest.TestCase):
    def test_original_pr298_failure_is_preserved_and_diagnosable_not_authorized(self):
        root = ROOT/'scripts/fixtures'
        source = json.loads((root/'ce03-transport-failure.source.json').read_text())
        raw = (root/'ce03-transport-failure.log').read_bytes()
        self.assertEqual(hashlib.sha256(raw).hexdigest(), source['log_sha256'])
        self.assertIn(ERROR, raw.decode())
        self.assertEqual(r.classify(ERROR, MODULE, SPEC), 'GO_PROXY_HTTP2_INTERNAL_ERROR')
        with self.assertRaises((ValueError, d.Blocked)):
            r.download_results(raw, b'', 1, [MODULE], {MODULE:CHECKSUM}, SPEC)
        contract = json.loads((ROOT/'scripts/delivery_execution_contract.json').read_text())
        full = {'id':36577101214, 'run_attempt':2, 'path':contract['merge_workflow'],
                'head_sha':source['candidate_sha'],'event':'pull_request','pull_requests':[{'number':298}]}
        with self.assertRaisesRegex(d.Blocked, 'FULL_ATTEMPT_BUDGET_EXCEEDED'):
            d.assert_single_full([full], contract, source['candidate_sha'], 298, full['id'], 2)

    def test_wrong_host_module_version_or_unclassified_failure_is_rejected(self):
        for error in [ERROR.replace('proxy.golang.org','evil.example'), ERROR.replace('v1.1.2','v9.9.9'),
                      ERROR+'\n--- FAIL: TestSecret', 'checksum mismatch', 'HTTP 404 Not Found']:
            with self.subTest(error=error):
                with self.assertRaises(d.Blocked): r.classify(error,MODULE,SPEC)

    def test_checksum_mismatch_never_retries(self):
        with self.assertRaisesRegex(d.Blocked,'RECOVERY_CHECKSUM_MISMATCH'):
            r.download_results(response([MODULE]),b'',0,[MODULE],{MODULE:'h1:wrong'},SPEC)

    def test_missing_duplicate_or_extra_module_rejected(self):
        for raw in [response([MODULE])+response([MODULE]),response([OTHER]),b'']:
            with self.assertRaises(d.Blocked): r.download_results(raw,b'',0,[MODULE],{MODULE:CHECKSUM},SPEC)

    def test_success_exit_code_cannot_hide_download_error(self):
        with self.assertRaisesRegex(d.Blocked,'RECOVERY_PROCESS_RESULT_MISMATCH'):
            r.download_results(response([MODULE],[MODULE]),b'',0,[MODULE],{MODULE:CHECKSUM},SPEC)

    def test_unknown_stderr_cannot_hide_checksum_error(self):
        with self.assertRaisesRegex(d.Blocked,'RECOVERY_NON_TRANSPORT_STDERR'):
            r.download_results(response([MODULE],[MODULE]),b'SECURITY ERROR',1,[MODULE],{MODULE:CHECKSUM},SPEC)

    def test_duplicate_json_keys_are_rejected(self):
        with self.assertRaisesRegex(d.Blocked,'RECOVERY_DUPLICATE_JSON_KEY'):
            r.strict_json('{"x":1,"x":2}')

    def test_fixed_command_keeps_tokens_out_of_subprocess(self):
        with patch.dict('os.environ', {'GH_TOKEN':'not-a-real-token'},clear=False), patch.object(r.subprocess,'run') as process:
            process.return_value = subprocess.CompletedProcess([],0,b'{}',b'')
            r.run_go(ROOT,[MODULE],SPEC,1)
            args,kwargs = process.call_args
            self.assertEqual(args[0],['go','mod','download','-json',MODULE])
            self.assertNotIn('GH_TOKEN', kwargs['env'])
            self.assertEqual(kwargs['env']['GOPROXY'],'https://proxy.golang.org')


class TerminalizationTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.path=Path(self.temp.name)/'proof.json'
        self.bound={'candidate_sha':'a'*40,'candidate_tree':'b'*40,'frozen_main_sha':'c'*40,'pr_number':298}

    def require(self):
        return d.require_proof_receipt(self.path,self.bound,'10',1,'hvritual/biz')

    def test_missing_upstream_proof_blocks(self):
        with self.assertRaisesRegex(d.Blocked,'PROOF_RECEIPT_MISSING'):self.require()

    def test_failed_audit_cannot_create_merge_ready(self):
        self.path.write_text(json.dumps({'state':'BLOCKED','reason':'FULL_ATTEMPT_BUDGET_EXCEEDED'}))
        with self.assertRaisesRegex(d.Blocked,'PROOF_AUDIT_BLOCKED'):self.require()

    def test_workflow_terminalizer_is_separate_and_always_runs(self):
        import yaml
        doc=yaml.safe_load((ROOT/'.github/workflows/pr-merge-gate.yml').read_text())
        steps=doc['jobs']['merge-ready']['steps']
        proof=next(i for i,s in enumerate(steps) if 'ci_proof_governance.py audit-run' in s.get('run',''))
        receipt=next(i for i,s in enumerate(steps) if 'delivery_execution.py merge-ready' in s.get('run',''))
        self.assertGreater(receipt,proof)
        self.assertEqual(steps[receipt]['if'],'always()')
        self.assertIn('--proof-receipt',steps[receipt]['run'])

    def test_blocked_delivery_cli_still_writes_exact_terminal_receipt(self):
        env={'GITHUB_REPOSITORY':'hvritual/biz','GITHUB_ACTOR':'fixture','GH_TOKEN':'fixture',
             'PR_NUMBER':'298','CANDIDATE_SHA':'a'*40,'GITHUB_RUN_ID':'10','GITHUB_RUN_ATTEMPT':'1'}
        output=Path(self.temp.name)/'terminal.json'
        with patch.dict('os.environ',env), patch.object(d,'bound_refs',return_value=self.bound), \
             patch.object(sys,'argv',['delivery_execution.py','merge-ready','--proof-receipt',str(self.path),'--output',str(output)]):
            self.assertEqual(d.main(),1)
        result=json.loads(output.read_text())
        self.assertEqual(result['state'],'BLOCKED')
        self.assertEqual(result['reason'],'PROOF_RECEIPT_MISSING')
        self.assertEqual(result['candidate_sha'],'a'*40)


class ApiEvidenceTests(RecoveryFixture):
    """Exercise artifact/API binding; the fake API is explicitly non-production."""
    def api(self, data):
        owner = self
        class FakeAPI:
            prefix = '/repos/hvritual/biz'
            artifact = {'id': 77, 'name': 'ci-dependency-ce03-10-1', 'expired': False,
                        'digest': 'sha256:' + r.digest(data)}
            job = copy.deepcopy(owner.job)
            def jobs(self, run): return [self.job]
            def pages(self, path, key): return [self.artifact]
            def get(self, path): return {'tree': {'sha': 'b'*40}}
            def raw(self, path, cap): return data
        return FakeAPI()

    def verify_api(self, api, attempt=1):
        return r.verify_run(api, self.root, 'hvritual/biz', 'a'*40, 'b'*40, 298,
            {'id':10,'run_attempt':attempt,'referenced_workflows':[{'sha':'c'*40}]}, self.bound['frozen_main_sha'])

    def test_api_bound_current_recovery_is_verifiable(self):
        self.execute(failing_first=True)
        self.assertEqual(self.verify_api(self.api(self.bundle()))['artifact_state'], 'RECOVERED')

    def test_expired_github_artifact_is_rejected(self):
        self.execute();api=self.api(self.bundle());api.artifact['expired']=True
        self.rejected('RECOVERY_ARTIFACT_MISSING_OR_EXPIRED', lambda: self.verify_api(api))

    def test_forged_archive_digest_is_rejected(self):
        self.execute();api=self.api(self.bundle());api.artifact['digest']='sha256:forged'
        self.rejected('RECOVERY_ARTIFACT_DIGEST_MISMATCH', lambda: self.verify_api(api))

    def test_old_job_or_other_candidate_is_rejected(self):
        self.execute();api=self.api(self.bundle());api.job['head_sha']='d'*40
        self.rejected('RECOVERY_JOB_NOT_QUALIFIED', lambda: self.verify_api(api))

    def test_second_workflow_attempt_is_not_recovery(self):
        self.execute();api=self.api(self.bundle())
        self.rejected('RECOVERY_CANNOT_AUTHORIZE_WORKFLOW_RERUN', lambda: self.verify_api(api,2))

    def test_boolean_attempt_cannot_masquerade_as_integer(self):
        self.execute()
        self.rejected('RECOVERY_RECEIPT_BINDING_MISMATCH', lambda: self.verify(
            mutate=lambda v: v['binding'].update(run_attempt=True)))

    def test_process_timeout_is_terminal_with_partial_logs(self):
        with self.assertRaisesRegex(d.Blocked,'RECOVERY_PROCESS_TIMEOUT'):
            r.prepare(self.root,self.output,SPEC,lambda:self.bound,
                      executor=lambda *args:(124,b'partial',b'download in progress'))
        report=json.loads((self.output/r.RECEIPT).read_text())
        self.assertEqual(report['reason'],'RECOVERY_PROCESS_TIMEOUT')
        self.assertEqual((self.output/'download-1.json').read_bytes(),b'partial')


if __name__=='__main__':
    import sys
    unittest.main(verbosity=2)
