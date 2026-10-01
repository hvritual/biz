#!/usr/bin/env python3
"""Isolated negative fixtures; no browser, API, reviewer or user-test evidence is fabricated."""
from __future__ import annotations

from contextlib import redirect_stdout
import copy
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import yaml

import check_b2b_ux_skill as ux

ROOT = Path(__file__).resolve().parents[1]
PRODUCT = 'a' * 40
OTHER = 'b' * 40
TEMPLATE = ROOT / ux.SKILL / 'templates/ux-contract.template.yaml'


class ParserTests(unittest.TestCase):
    def test_current_template_parses(self):
        self.assertEqual(ux.parse(TEMPLATE.read_bytes())['artifact_kind'], 'template')

    def test_nested_duplicate_keys_are_rejected(self):
        with self.assertRaisesRegex(ux.Invalid, 'DUPLICATE_KEY'):
            ux.parse(b'context:\n  role: one\n  role: two\n')

    def test_json_duplicate_keys_are_rejected(self):
        with self.assertRaisesRegex(ux.Invalid, 'DUPLICATE_KEY'):
            ux.parse(b'{"a":1,"a":2}')

    def test_aliases_and_explicit_tags_are_rejected(self):
        for raw in (b'a: &a [1]\nb: *a', b'a: !!str 1', b'!!python/object:evil {}'):
            with self.subTest(raw=raw), self.assertRaises(ux.Invalid):
                ux.parse(raw)

    def test_merge_keys_and_non_string_keys_are_rejected(self):
        for raw in (b'{<<: {x: 1}}', b'{1: value}', b'{true: value}'):
            with self.subTest(raw=raw), self.assertRaises((ux.Invalid, yaml.YAMLError)):
                ux.parse(raw)

    def test_depth_and_size_are_bounded(self):
        for raw in (b'[' * 41 + b'0' + b']' * 41, b' ' * (ux.MAX_BYTES + 1)):
            with self.subTest(size=len(raw)), self.assertRaises(ux.Invalid):
                ux.parse(raw)

    def test_non_finite_numbers_fail(self):
        for raw in (b'a: .nan', b'a: .inf', b'a: -.inf'):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                ux.parse(raw)

    def test_boolean_one_and_float_are_distinct_payloads(self):
        values = [ux.payload({'x': x}) for x in (True, 1, 1.0)]
        self.assertEqual(len(set(values)), 3)

    def test_mapping_order_is_ignored_but_list_order_is_not(self):
        self.assertEqual(ux.payload({'a': 1, 'b': 2}), ux.payload({'b': 2, 'a': 1}))
        self.assertNotEqual(ux.payload({'a': [1, 2]}), ux.payload({'a': [2, 1]}))

    def test_only_top_level_review_and_status_are_excluded(self):
        original = {'review': None, 'status': 'draft', 'context': {'status': 'old'}}
        revised = copy.deepcopy(original)
        revised.update(review={'reviewer': 'fixture'}, status='reviewed')
        self.assertEqual(ux.payload(original), ux.payload(revised))
        revised['context']['status'] = 'new'
        self.assertNotEqual(ux.payload(original), ux.payload(revised))

    def test_dates_and_yes_are_not_implicitly_coerced(self):
        self.assertEqual(ux.parse(b'a: 2026-10-01\nb: yes\nc: true'),
                         {'a': '2026-10-01', 'b': 'yes', 'c': True})

    def test_global_yaml_loader_is_not_modified(self):
        self.assertIs(yaml.safe_load('yes'), True)


class AnalysisTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='ux-check-fixture-')
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        (self.root / 'docs').mkdir()
        (self.root / 'docs/policy.md').write_text('# Fixture policy\nNot production evidence.\n')
        (self.root / 'web').mkdir()
        (self.root / 'web/ui-contracts.json').write_text(json.dumps({'patterns': {
            'ListPage': {'status': 'implemented'}, 'MetricsPage': {'status': 'reserved'}}}))
        self.path = 'docs/analysis.yaml'
        self.doc = ux.parse(TEMPLATE.read_bytes())
        self.doc.update(artifact_kind='analysis', status='draft')
        self.doc['task_ref'].update(issue='#unit-fixture', name='Unit fixture only', candidate_commit=PRODUCT)
        self.doc['classification'].update(type='ui_interaction', business_state_mutation=False,
                                          authorization_sensitive=False, asynchronous_effect=False)
        self.doc['sources'] = [{'id': 'policy', 'kind': 'repository_contract', 'path': 'docs/policy.md',
                               'commit': PRODUCT, 'locator': 'Fixture policy', 'claim': 'Design obligation only, not API evidence'}]
        for item in self.doc['context'].values():
            item.update(summary='Unit fixture only', source_refs=['policy'])
        self.doc['page_pattern'].update(name='ListPage', reason='Read a fixture collection', contract_ref='web/ui-contracts.json')
        self.doc['outcome'].update(definition='Fixture task target', result_contract_refs=['policy'], recovery_contract_refs=['policy'])
        for item in self.doc['humanized_ux'].values():
            item.update(applicability='not_applicable', reason='Bounded fixture excludes this concern; review is still required',
                        verification='not_applicable')
        self.doc['open_questions'] = []
        self.active()
        self.write()

    def write(self):
        (self.root / self.path).write_text(yaml.safe_dump(self.doc, allow_unicode=True, sort_keys=False))

    def active(self, dimension='time_to_information'):
        self.doc['humanized_ux'][dimension].update(applicability='applies', reason='Specific fixture obligation',
                requirements=['Read fixture identity'], evidence_types=['browser_test'], verification='not_verified', evidence=[])
        return self.doc['humanized_ux'][dimension]

    def evidence(self, dimension='time_to_information', kind='browser_test'):
        item = self.active(dimension)
        item.update(verification='pass', evidence_types=[kind], evidence=[{
            'type': kind, 'reference': 'https://example.invalid/fixture-not-real-evidence',
            'candidate_sha': PRODUCT, 'scope': dimension, 'result': 'pass'}])
        return item

    def check(self, expected=PRODUCT):
        return ux.analyze(self.root, self.path, self.doc, expected)

    def rejected(self, code):
        with self.assertRaisesRegex(ux.Invalid, code):
            self.check()

    def git(self, *args):
        return subprocess.check_output(['git', '-c', 'user.name=Unit Fixture',
                                       '-c', 'user.email=fixture@example.invalid', *args], cwd=self.root).decode().strip()

    def reviewed(self, scope='task_experience', decision='approved'):
        self.write()
        self.git('init', '-q')
        self.git('add', '.')
        self.git('commit', '-qm', 'Synthetic analysis fixture; not review evidence')
        commit = self.git('rev-parse', 'HEAD')
        blob = self.git('rev-parse', 'HEAD:' + self.path)
        self.doc.update(status='reviewed')
        self.doc['review'].update(reviewer='fictional-reviewer', actor_kind='human', scope=scope,
                reference='https://example.invalid/not-an-actual-review', reviewed_at='2026-10-01T00:00:00Z',
                candidate_sha=self.doc['task_ref']['candidate_commit'], decision=decision,
                analysis_ref={'path': self.path, 'commit': commit, 'blob_sha': blob})
        return self.doc['review']

    def test_valid_draft_and_justified_na_remain_unverified(self):
        pending = self.check()
        self.assertIn('DIMENSION_NOT_VERIFIED', {p['code'] for p in pending})
        self.assertIn('NA_REASON_REQUIRES_REVIEW', {p['code'] for p in pending})

    def test_missing_context_dimension_fails(self):
        del self.doc['context']['risk']
        self.rejected('FIELDS_INVALID:context')

    def test_missing_ux_dimension_fails(self):
        del self.doc['humanized_ux']['how_to_guidance']
        self.rejected('FIELDS_INVALID:humanized_ux')

    def test_unknown_fields_fail(self):
        self.doc['auto_approve'] = True
        self.rejected('FIELDS_INVALID')

    def test_boolean_schema_version_is_not_one(self):
        self.doc['schema_version'] = True
        self.rejected('SCHEMA_VERSION')

    def test_unknown_cannot_be_pass(self):
        self.active().update(applicability='unknown', verification='pass')
        self.rejected('UNKNOWN_VERIFIED')

    def test_unknown_requires_question_and_stays_pending(self):
        self.doc['classification']['type'] = 'unknown'
        self.rejected('UNKNOWN_WITHOUT_QUESTION')
        self.doc['open_questions'] = [{'question': 'Unknown role', 'blocking': True, 'owner_role': 'Product'}]
        self.assertIn('CLASSIFICATION_UNKNOWN', {p['code'] for p in self.check()})

    def test_na_requires_nonempty_reason(self):
        self.doc['humanized_ux']['how_to_guidance']['reason'] = ' '
        self.rejected('TEXT_REQUIRED')

    def test_na_cannot_pass(self):
        self.doc['humanized_ux']['how_to_guidance']['verification'] = 'pass'
        self.rejected('NA_STATE_INVALID')

    def test_duplicate_source_ids_fail(self):
        self.doc['sources'].append(copy.deepcopy(self.doc['sources'][0]))
        self.rejected('DUPLICATE_SOURCE_ID')

    def test_context_and_outcome_refs_must_resolve(self):
        for path in ('context', 'result_contract_refs', 'recovery_contract_refs'):
            with self.subTest(path=path):
                original = copy.deepcopy(self.doc)
                if path == 'context':
                    self.doc['context']['role']['source_refs'] = ['missing']
                else:
                    self.doc['outcome'][path] = ['missing']
                self.rejected('SOURCE_REF_UNRESOLVED')
                self.doc = original

    def test_missing_source_path_fails(self):
        self.doc['sources'][0]['path'] = 'docs/missing.md'
        self.rejected('FILE_MISSING')

    def test_symlink_source_and_ancestor_fail(self):
        (self.root / 'link').symlink_to(self.root / 'docs', target_is_directory=True)
        self.doc['sources'][0]['path'] = 'link/policy.md'
        self.rejected('SYMLINK_FORBIDDEN')

    def test_paths_cannot_escape_or_use_unsafe_schemes(self):
        for value in ('../secret', '/etc/passwd', 'file:///etc/passwd', 'javascript:alert(1)',
                      'https://user:secret@example.invalid/path', '..\\secret', '%2e%2e/secret'):
            with self.subTest(value=value):
                self.doc['sources'][0]['path'] = value
                with self.assertRaises(ux.Invalid):
                    self.check()

    def test_risk_cannot_be_downgraded_by_small_diff(self):
        self.doc['classification']['authorization_sensitive'] = True
        self.rejected('RISK_DOWNGRADED')

    def test_boolean_string_is_not_authorization_flag(self):
        self.doc['classification']['authorization_sensitive'] = 'false'
        self.rejected('BOOLEAN_OR_UNKNOWN')

    def test_reserved_pattern_requires_gap(self):
        self.doc['page_pattern']['name'] = 'MetricsPage'
        self.rejected('RESERVED_PATTERN_WITHOUT_GAP')
        self.doc['page_pattern']['gaps'] = ['No implemented consumer']
        self.assertIn('PAGE_PATTERN_NOT_IMPLEMENTED', {p['code'] for p in self.check()})

    def test_no_second_page_contract_authority(self):
        self.doc['page_pattern']['contract_ref'] = 'web/ux-contracts.json'
        self.rejected('PAGE_CONTRACT_AUTHORITY')

    def test_pass_cannot_use_missing_failed_unrun_or_wrong_type_evidence(self):
        item = self.evidence()
        good = copy.deepcopy(item)
        for value in ([], [dict(good['evidence'][0], result='fail')], [dict(good['evidence'][0], result='not_run')],
                      [dict(good['evidence'][0], type='static_check')]):
            with self.subTest(value=value):
                item['evidence'] = value
                self.rejected('PASS_WITHOUT_REQUIRED_EVIDENCE')

    def test_passing_retest_preserves_failed_history_for_review(self):
        item = self.evidence()
        current = copy.deepcopy(item['evidence'][0])
        failed = dict(current, result='fail', reference='https://example.invalid/failed-run')
        for entries in ([failed, current], [current, failed]):
            with self.subTest(order=[e['result'] for e in entries]):
                item['evidence'] = entries
                before = copy.deepcopy(self.doc)
                pending = self.check()
                self.assertIn({'code': 'NONPASS_HISTORY_REQUIRES_REVIEW',
                               'location': 'time_to_information.evidence:' + failed['reference']}, pending)
                self.assertIn('EXTERNAL_EVIDENCE_NOT_VERIFIED', {p['code'] for p in pending})
                self.assertEqual(self.doc, before)

    def test_unrun_history_is_retained_without_counting_as_passing_evidence(self):
        item = self.evidence()
        unrun = dict(item['evidence'][0], result='not_run', reference='https://example.invalid/unrun')
        item['evidence'].append(unrun)
        pending = self.check()
        self.assertIn({'code': 'NONPASS_HISTORY_REQUIRES_REVIEW',
                       'location': 'time_to_information.evidence:' + unrun['reference']}, pending)
        self.assertEqual(item['evidence'][1]['result'], 'not_run')

    def test_failure_cannot_supply_a_missing_required_evidence_type(self):
        item = self.evidence()
        item['evidence_types'].append('api_test')
        item['evidence'].append(dict(item['evidence'][0], type='api_test', result='fail',
                                     reference='https://example.invalid/failed-api'))
        self.rejected('PASS_WITHOUT_REQUIRED_EVIDENCE')

    def test_unrun_cannot_supply_a_missing_required_evidence_type(self):
        item = self.evidence()
        item['evidence_types'].append('user_test')
        item['evidence'].append(dict(item['evidence'][0], type='user_test', result='not_run',
                                     reference='https://example.invalid/unrun-user-test'))
        self.rejected('PASS_WITHOUT_REQUIRED_EVIDENCE')

    def test_each_required_type_needs_a_pass_despite_retained_failure(self):
        item = self.evidence()
        item['evidence_types'].append('api_test')
        api_pass = dict(item['evidence'][0], type='api_test', reference='https://example.invalid/api-retest')
        item['evidence'].extend([
            dict(api_pass, result='fail', reference='https://example.invalid/failed-api'), api_pass])
        self.assertIn('NONPASS_HISTORY_REQUIRES_REVIEW', {p['code'] for p in self.check()})
        item['evidence'].remove(api_pass)
        self.rejected('PASS_WITHOUT_REQUIRED_EVIDENCE')

    def test_retained_failure_does_not_bypass_current_candidate_binding(self):
        item = self.evidence()
        item['evidence'].append(dict(item['evidence'][0], result='fail', candidate_sha=OTHER,
                                     reference='https://example.invalid/other-candidate-failure'))
        self.rejected('EVIDENCE_CANDIDATE_MISMATCH')

    def test_retained_failure_does_not_bypass_dimension_scope_binding(self):
        item = self.evidence()
        item['evidence'].append(dict(item['evidence'][0], result='fail', scope='unrelated_build',
                                     reference='https://example.invalid/unrelated-failure'))
        self.rejected('EVIDENCE_SCOPE_MISMATCH')

    def test_same_run_cannot_be_relabelled_as_both_failure_and_pass(self):
        item = self.evidence()
        item['evidence'].append(dict(item['evidence'][0], result='fail'))
        self.rejected('DUPLICATE_EVIDENCE')

    def test_old_candidate_and_wrong_scope_cannot_pass(self):
        item = self.evidence()
        item['evidence'][0]['candidate_sha'] = OTHER
        self.rejected('EVIDENCE_CANDIDATE_MISMATCH')
        item['evidence'][0].update(candidate_sha=PRODUCT, scope='unrelated_build')
        self.rejected('EVIDENCE_SCOPE_MISMATCH')

    def test_duplicate_evidence_fails(self):
        item = self.evidence()
        item['evidence'].append(copy.deepcopy(item['evidence'][0]))
        self.rejected('DUPLICATE_EVIDENCE')

    def test_plausible_evidence_url_is_not_verified(self):
        self.evidence()
        self.assertIn('EXTERNAL_EVIDENCE_NOT_VERIFIED', {p['code'] for p in self.check()})

    def test_fail_requires_failure_evidence(self):
        self.active()['verification'] = 'fail'
        self.rejected('FAIL_WITHOUT_EVIDENCE')

    def test_historical_failure_can_be_kept_but_not_reused(self):
        item = self.evidence()
        item['verification'] = 'not_verified'
        item['evidence'][0].update(candidate_sha=OTHER, result='fail')
        self.assertIn('HISTORICAL_EVIDENCE_NOT_CURRENT', {p['code'] for p in self.check()})

    def test_illustrations_cannot_record_execution(self):
        self.doc['artifact_kind'] = 'example'
        self.evidence()
        self.rejected('ILLUSTRATION_CANNOT_CERTIFY')

    def metric(self):
        self.doc['acceptance']['metrics'] = [{'id': 'time_to_information', 'definition': 'Human identifies fixture object',
                'conditions': 'Fixture participant and device', 'baseline': None, 'target': None, 'observed': None,
                'unit': 'seconds', 'sample_size': 0, 'failure_policy': 'Keep abandoned and failed attempts', 'evidence_ref': None}]
        return self.doc['acceptance']['metrics'][0]

    def test_unmeasured_metric_is_valid(self):
        self.metric()
        self.assertIn('METRIC_NOT_MEASURED', {p['code'] for p in self.check()})

    def test_observed_zero_without_sample_is_not_unmeasured(self):
        self.metric()['observed'] = 0
        self.rejected('OBSERVED_WITHOUT_SAMPLE')

    def test_boolean_sample_size_is_not_integer_one(self):
        self.metric().update(observed=1, sample_size=True)
        self.rejected('SAMPLE_SIZE_INVALID')

    def test_observed_requires_matching_user_measurement_not_browser_speed(self):
        item = self.evidence()
        self.metric().update(observed=1, sample_size=1, evidence_ref=item['evidence'][0]['reference'])
        self.rejected('METRIC_EVIDENCE_UNRESOLVED')
        item['evidence_types'] = ['user_test']
        item['evidence'][0]['type'] = 'user_test'
        self.assertIn('MEASUREMENT_SAMPLE_AND_CONDITIONS_NOT_VERIFIED', {p['code'] for p in self.check()})

    def test_duplicate_metric_and_gate_ids_fail(self):
        self.metric()
        self.doc['acceptance']['metrics'] *= 2
        self.rejected('DUPLICATE_METRIC_ID')
        self.doc['acceptance']['metrics'] = []
        self.doc['handoff']['gates'] = [{'name': 'same', 'status': 'not_run'}] * 2
        self.rejected('DUPLICATE_GATE_ID')

    def test_write_cannot_use_page_declaration_as_backend_truth(self):
        self.doc['classification'].update(type='business_task', business_state_mutation=True)
        self.doc['sources'][0]['path'] = 'web/ui-contracts.json'
        self.rejected('WRITE_AUTHORITY_MISSING')

    def test_write_missing_capability_is_a_blocker_not_fake_implementation(self):
        self.doc['classification'].update(type='business_task', business_state_mutation=True)
        self.doc['sources'][0]['kind'] = 'hypothesis'
        self.doc['open_questions'] = [{'question': 'Missing backend', 'blocking': True, 'owner_role': 'Backend'}]
        self.active('error_recoverability')
        self.active('operation_effect_transparency')
        self.assertIn('WRITE_AUTHORITY_MISSING', {p['code'] for p in self.check()})

    def test_write_cannot_waive_recovery_or_effects(self):
        self.doc['classification'].update(type='business_task', business_state_mutation=True)
        self.rejected('WRITE_OBLIGATION_WAIVED')

    def test_timeout_and_guarantees_need_semantic_review_even_with_negation(self):
        self.doc['outcome']['definition'] = '超时即失败，重新提交；不会重复扣款；页面声明就是API真相。'
        codes = {p['code'] for p in self.check()}
        self.assertTrue({'TIMEOUT_RECOVERY_REQUIRES_REVIEW', 'HIGH_RISK_PROMISE_REQUIRES_REVIEW', 'PAGE_IS_NOT_API_EVIDENCE'} <= codes)
        self.doc['outcome']['definition'] = '禁止承诺不会重复扣款；超时后先核实原操作状态。'
        self.assertIn('HIGH_RISK_PROMISE_REQUIRES_REVIEW', {p['code'] for p in self.check()})

    def test_unreviewed_metadata_cannot_claim_human_name(self):
        self.doc['review']['reviewer'] = 'invented'
        self.rejected('UNREVIEWED_METADATA_CONTRADICTION')

    def test_reviewed_does_not_mean_approved(self):
        self.doc['open_questions'] = [{'question': 'Still blocked', 'blocking': True, 'owner_role': 'Product'}]
        self.reviewed(decision='changes_requested')
        self.assertIn('EXTERNAL_REVIEW_NOT_VERIFIED', {p['code'] for p in self.check()})
        self.doc['review']['decision'] = 'approved'
        self.rejected('APPROVED_WITH_BLOCKER')

    def test_reviewed_without_decision_fails(self):
        self.doc['status'] = 'reviewed'
        self.rejected('REVIEWED_WITHOUT_DECISION')

    def test_expected_product_candidate_is_external(self):
        with self.assertRaisesRegex(ux.Invalid, 'EXPECTED_CANDIDATE_MISMATCH'):
            self.check(OTHER)
        self.assertIn('EXTERNAL_CANDIDATE_NOT_VERIFIED', {p['code'] for p in self.check(None)})

    def test_document_design_can_have_no_product_candidate(self):
        self.doc['task_ref']['candidate_commit'] = None
        self.reviewed(scope='document_design')
        self.assertIn('EXTERNAL_REVIEW_NOT_VERIFIED', {p['code'] for p in self.check(None)})

    def test_experience_review_requires_known_candidate(self):
        self.doc['task_ref']['candidate_commit'] = None
        self.reviewed()
        with self.assertRaisesRegex(ux.Invalid, 'EXPERIENCE_CANDIDATE_UNKNOWN'):
            self.check(None)

    def test_review_product_binding_cannot_disagree(self):
        self.reviewed()['candidate_sha'] = OTHER
        self.rejected('REVIEW_CANDIDATE_MISMATCH')

    def test_review_must_bind_actual_analysis_path(self):
        self.evidence()  # Synthetic fixture, not externally verified evidence.
        self.reviewed()['analysis_ref']['path'] = 'docs/another.yaml'
        self.rejected('ANALYSIS_PATH_MISMATCH')

    def test_review_must_bind_real_git_blob(self):
        self.evidence()  # Synthetic fixture, not externally verified evidence.
        self.reviewed()['analysis_ref']['blob_sha'] = OTHER
        self.rejected('ANALYSIS_BLOB_MISMATCH')

    def test_review_backfill_avoids_self_sha_loop_but_does_not_grant_approval(self):
        self.evidence()  # Synthetic fixture, not externally verified evidence.
        self.reviewed()
        self.assertIn('EXTERNAL_REVIEW_NOT_VERIFIED', {p['code'] for p in self.check()})

    def test_analysis_payload_changes_invalidate_review(self):
        self.evidence()  # Synthetic fixture, not externally verified evidence.
        self.reviewed()
        for key in ('task_ref', 'sources', 'context', 'acceptance', 'open_questions', 'handoff'):
            with self.subTest(key=key):
                original = copy.deepcopy(self.doc)
                if key == 'task_ref': self.doc[key]['name'] += ' changed'
                elif key == 'sources': self.doc[key][0]['claim'] += ' changed'
                elif key == 'context': self.doc[key]['task']['summary'] += ' changed'
                elif key == 'acceptance': self.doc[key]['scenarios'].append('changed')
                elif key == 'open_questions': self.doc[key].append({'question': 'Changed', 'blocking': False, 'owner_role': 'Product'})
                else: self.doc[key]['allowed_paths'].append('changed')
                self.rejected('ANALYSIS_PAYLOAD_CHANGED')
                self.doc = original

    def test_recomputing_git_blob_still_cannot_verify_old_external_review(self):
        self.evidence()  # Synthetic fixture, not externally verified evidence.
        review = self.reviewed()
        self.doc['context']['task']['summary'] += ' revised'
        self.doc['status'] = 'draft'
        self.doc['review'] = ux.parse(TEMPLATE.read_bytes())['review']
        self.write()
        self.git('add', '.')
        self.git('commit', '-qm', 'Revised fixture; external review did not happen')
        review['analysis_ref'].update(commit=self.git('rev-parse', 'HEAD'), blob_sha=self.git('rev-parse', 'HEAD:' + self.path))
        self.doc.update(status='reviewed', review=review)
        self.assertIn('EXTERNAL_REVIEW_NOT_VERIFIED', {p['code'] for p in self.check()})

    def test_unavailable_commit_cannot_approve(self):
        self.evidence()  # Synthetic fixture, not externally verified evidence.
        self.reviewed()['analysis_ref']['commit'] = OTHER
        self.assertIn('ANALYSIS_GIT_OBJECT_NOT_AVAILABLE', {p['code'] for p in self.check()})

    def test_tree_object_cannot_impersonate_review_commit(self):
        self.evidence()  # Synthetic fixture, not externally verified evidence.
        review = self.reviewed()
        review['analysis_ref']['commit'] = self.git('rev-parse', 'HEAD^{tree}')
        self.rejected('GIT_NOT_COMMIT')

    def test_experience_approval_rejects_unverified_dimension(self):
        self.reviewed()
        self.rejected('EXPERIENCE_DIMENSION_NOT_PASSED:time_to_information')

    def test_nonblocking_label_cannot_waive_experience_verification(self):
        self.doc['open_questions'] = [{'question': 'Test not executed', 'blocking': False, 'owner_role': 'QA'}]
        self.reviewed()
        self.rejected('EXPERIENCE_DIMENSION_NOT_PASSED')

    def test_experience_approval_rejects_unknown_in_any_dimension(self):
        self.evidence()
        self.doc['open_questions'] = [{'question': 'Applicability unknown', 'blocking': False, 'owner_role': 'Product'}]
        original = copy.deepcopy(self.doc)
        for dimension in ux.DIMENSIONS:
            with self.subTest(dimension=dimension):
                self.doc = copy.deepcopy(original)
                self.doc['humanized_ux'][dimension].update(applicability='unknown', verification='not_verified', evidence=[])
                self.reviewed()
                self.rejected('EXPERIENCE_APPLICABILITY_UNKNOWN:' + dimension)

    def test_experience_approval_requires_every_applicable_dimension(self):
        for dimension in ux.DIMENSIONS:
            self.evidence(dimension)
        original = copy.deepcopy(self.doc)
        for dimension in ux.DIMENSIONS:
            with self.subTest(dimension=dimension):
                self.doc = copy.deepcopy(original)
                self.active(dimension)
                self.reviewed()
                self.rejected('EXPERIENCE_DIMENSION_NOT_PASSED:' + dimension)

    def test_experience_approval_rejects_a_failed_dimension(self):
        item = self.evidence()
        item['verification'] = 'fail'
        item['evidence'][0]['result'] = 'fail'
        self.reviewed()
        self.rejected('EXPERIENCE_DIMENSION_NOT_PASSED')

    def test_experience_pass_cannot_omit_required_evidence(self):
        self.evidence()['evidence'] = []
        self.reviewed()
        self.rejected('PASS_WITHOUT_REQUIRED_EVIDENCE')

    def test_experience_pass_cannot_omit_declared_obligation(self):
        self.evidence()['requirements'] = []
        self.reviewed()
        self.rejected('APPLICABLE_OBLIGATION_EMPTY')

    def test_experience_pass_cannot_omit_required_evidence_types(self):
        self.evidence()['evidence_types'] = []
        self.reviewed()
        self.rejected('APPLICABLE_OBLIGATION_EMPTY')

    def test_experience_approval_with_current_pass_remains_pending_external_review(self):
        self.evidence()
        self.reviewed()
        before = copy.deepcopy(self.doc)
        codes = {p['code'] for p in self.check()}
        self.assertTrue({'EXTERNAL_REVIEW_NOT_VERIFIED', 'EXTERNAL_EVIDENCE_NOT_VERIFIED',
                         'NA_REASON_REQUIRES_REVIEW'} <= codes)
        self.assertEqual(self.doc, before)

    def test_experience_changes_requested_can_retain_unverified_dimension(self):
        self.reviewed(decision='changes_requested')
        codes = {p['code'] for p in self.check()}
        self.assertTrue({'DIMENSION_NOT_VERIFIED', 'EXTERNAL_REVIEW_NOT_VERIFIED'} <= codes)

    def test_experience_changes_requested_can_retain_unknown_dimension(self):
        self.active().update(applicability='unknown')
        self.doc['open_questions'] = [{'question': 'Unknown', 'blocking': True, 'owner_role': 'Product'}]
        self.reviewed(decision='changes_requested')
        self.assertIn('APPLICABILITY_UNKNOWN', {p['code'] for p in self.check()})

    def test_document_design_approval_can_retain_unknown_and_unexecuted_validation(self):
        self.active().update(applicability='unknown')
        self.doc['open_questions'] = [{'question': 'Future task validation', 'blocking': False, 'owner_role': 'QA'}]
        self.reviewed(scope='document_design')
        codes = {p['code'] for p in self.check()}
        self.assertTrue({'APPLICABILITY_UNKNOWN', 'DIMENSION_NOT_VERIFIED', 'EXTERNAL_REVIEW_NOT_VERIFIED'} <= codes)

    def test_changing_design_scope_cannot_replay_unverified_experience_approval(self):
        self.reviewed(scope='document_design')
        self.assertIn('DIMENSION_NOT_VERIFIED', {p['code'] for p in self.check()})
        self.doc['review']['scope'] = 'task_experience'
        self.rejected('EXPERIENCE_DIMENSION_NOT_PASSED')

    def test_experience_approval_preserves_failed_history_as_pending_not_resolved(self):
        item = self.evidence()
        failed = dict(item['evidence'][0], result='fail', reference='https://example.invalid/unresolved-old-run')
        item['evidence'].append(failed)
        self.reviewed()
        before = copy.deepcopy(self.doc)
        codes = {p['code'] for p in self.check()}
        self.assertTrue({'NONPASS_HISTORY_REQUIRES_REVIEW', 'EXTERNAL_REVIEW_NOT_VERIFIED'} <= codes)
        self.assertEqual(self.doc, before)

    def test_git_uses_literal_paths_and_no_shell_or_network(self):
        self.evidence()  # Synthetic fixture, not externally verified evidence.
        self.reviewed()
        original = subprocess.run
        def bounded(*args, **kwargs):
            self.assertFalse(kwargs.get('shell', False))
            self.assertEqual(args[0][0], 'git')
            self.assertNotIn('fetch', args[0])
            self.assertIn('timeout', kwargs)
            return original(*args, **kwargs)
        with patch.object(ux.subprocess, 'run', side_effect=bounded):
            self.check()


class RepositoryTests(unittest.TestCase):
    def test_actual_skill_and_wiring(self):
        result = ux.validate(ROOT)
        self.assertEqual(result['structure_result'], 'PASS', result['errors'])
        self.assertEqual(result['result'], 'NEEDS_REVIEW')
        self.assertFalse(result['approval_granted'])
        self.assertFalse(result['delivery_granted'])
        self.assertGreater(result['local_links_checked'], 40)

    def test_read_only_gate_does_not_change_tracked_source(self):
        paths = [ROOT / name for name in ('AGENTS.md', 'web/AGENTS.md', 'Makefile')]
        paths.extend(p for p in (ROOT / ux.SKILL).rglob('*') if p.is_file())
        before = {str(p): p.read_bytes() for p in paths}
        ux.validate(ROOT)
        self.assertEqual(before, {str(p): p.read_bytes() for p in paths})

    def test_repo_mutations_fail_without_editing_worktree(self):
        original_read = ux.read
        mutations = [
            ('AGENTS.md', lambda s: s.replace('b2b-product-ux', 'missing-skill'), 'AGENT_ENTRY_NOT_WIRED'),
            ('Makefile', lambda s: s.replace(' ui-skill-check b2b-ux-skill-check', ' ui-skill-check'), 'MAKE_CHECK_NOT_WIRED'),
            ('Makefile', lambda s: s.replace("-p 'test_b2b_ux_skill.py' -v", "-p 'missing.py' -v"), 'B2B_GATE_COMMANDS_MISSING'),
            ('Makefile', lambda s: s.replace("-p 'test_ui_skill_source.py' -v", "-p 'missing.py' -v"), 'TYPOGRAPHY_GATE_CHANGED'),
            (ux.SKILL + '/SKILL.md', lambda s: s.replace('name: b2b-product-ux', 'name: other'), 'SKILL_NAME'),
            (ux.SKILL + '/references/b2b-interaction-patterns.md', lambda s: s.replace('## master-detail —', '## entity-overview —'), 'PATTERN_COVERAGE_OR_DUPLICATE'),
            (ux.SKILL + '/references/b2b-interaction-patterns.md', lambda s: s.replace('**use_when**', '**fake_field**', 1), 'PATTERN_FIELDS'),
            (ux.SKILL + '/examples/device-incident.md', lambda s: s.replace('| risk |', '| wrong |'), 'EXAMPLE_CONTEXT'),
            (ux.SKILL + '/examples/device-incident.md', lambda s: s.replace('not_verified', 'pass'), 'EXAMPLE_STATUS'),
            (ux.SKILL + '/examples/device-incident.md', lambda s: s.replace('源码阅读基线：`9bbef8f', '源码阅读基线：`BROKEN'), 'EXAMPLE_BASELINE_MISSING'),
        ]
        for target, mutation, expected in mutations:
            def changed(root, path):
                raw = original_read(root, path)
                return mutation(raw.decode()).encode() if path == target else raw
            with self.subTest(target=target, expected=expected), patch.object(ux, 'read', side_effect=changed):
                result = ux.validate(ROOT)
                self.assertEqual(result['structure_result'], 'FAIL')
                self.assertTrue(any(expected in e for e in result['errors']), result['errors'])

    def test_require_verified_cannot_turn_pending_into_approval(self):
        with redirect_stdout(io.StringIO()):
            self.assertEqual(ux.main(['--root', str(ROOT), '--require-verified']), 2)

    def test_missing_skill_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            result = ux.validate(Path(directory))
        self.assertEqual(result['result'], 'FAIL')

    def test_symlink_root_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            link = Path(directory) / 'root'
            link.symlink_to(ROOT, target_is_directory=True)
            self.assertEqual(ux.validate(link)['result'], 'FAIL')

    def test_markdown_links_reject_broken_escape_and_script_urls(self):
        for target in ('missing.md', '../../etc/passwd', 'javascript:alert(1)', '/etc/passwd'):
            with self.subTest(target=target), self.assertRaises(ux.Invalid):
                ux.markdown_links(ROOT, 'AGENTS.md', '[bad](' + target + ')')

    def test_markdown_reference_links_and_anchors(self):
        self.assertEqual(ux.markdown_links(ROOT, 'AGENTS.md', '[entry][one]\n[one]: AGENTS.md'), 1)
        for content in ('[entry][unknown]', '[entry](AGENTS.md#not-a-heading)'):
            with self.subTest(content=content), self.assertRaises(ux.Invalid):
                ux.markdown_links(ROOT, 'AGENTS.md', content)

    def test_cli_outputs_structure_scope_not_approval(self):
        output = io.StringIO()
        with redirect_stdout(output):
            code = ux.main(['--root', str(ROOT)])
        result = json.loads(output.getvalue())
        self.assertEqual(code, 0)
        self.assertEqual(result['external_records'], 'NOT_VERIFIED')
        self.assertEqual(result['result'], 'NEEDS_REVIEW')


if __name__ == '__main__':
    unittest.main()
