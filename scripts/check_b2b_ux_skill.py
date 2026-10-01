#!/usr/bin/env python3
"""Read-only B2B UX structure checks. Never grants evidence, human approval or delivery."""
from __future__ import annotations

import argparse
import copy
from datetime import datetime
import json
import math
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
from urllib.parse import unquote, urlsplit

import yaml

SKILL = '.agents/skills/b2b-product-ux'
CONTEXT = ('role', 'task', 'entity', 'workflow', 'state', 'action', 'risk', 'decision')
DIMENSIONS = ('time_to_information', 'time_to_action', 'context_switching',
              'decision_load', 'interaction_cost', 'error_recoverability',
              'how_to_guidance', 'operation_effect_transparency',
              'problem_resolution_guidance')
LABELS = ('Time to Information', 'Time to Action', 'Context Switching', 'Decision Load',
          'Interaction Cost', 'Error Recoverability', 'How-to Guidance',
          'Operation Effect Transparency', 'Problem Resolution Guidance')
EVIDENCE_TYPES = {'source_review', 'static_check', 'browser_test', 'api_test',
                  'user_test', 'independent_review'}
SOURCE_KINDS = {'repository_contract', 'implementation', 'user_requirement',
                'validated_runbook', 'hypothesis'}
PATTERN_FIELDS = ('use_when', 'avoid_when', 'business_prerequisites', 'minimum_information',
                  'key_states', 'accessibility', 'component_lookup', 'positive_example', 'negative_example')
REQUIRED_FILES = ('SKILL.md', 'PROJECT-INTEGRATION.md', 'UX-CONTRACT.md',
                  'references/business-context.md', 'references/humanized-ux.md',
                  'references/page-pattern-selection.md', 'references/b2b-interaction-patterns.md',
                  'references/guidance-and-recovery.md', 'references/anti-patterns.md',
                  'templates/ux-contract.template.yaml', 'examples/member-context-review.yaml',
                  'examples/customer-workspace.md', 'examples/device-incident.md',
                  'examples/tenant-plan-upgrade.md')
MAX_BYTES = 1024 * 1024
SHA = re.compile(r'[0-9a-f]{40}\Z')


class Invalid(ValueError):
    """An unambiguous structural violation, not a judgment of design quality."""


def require(condition, code, location):
    if not condition:
        raise Invalid(f'{code}:{location}')


def text(value, location, nullable=False):
    require((nullable and value is None) or
            (type(value) is str and bool(value.strip())), 'TEXT_REQUIRED', location)


def fields(value, names, location):
    require(type(value) is dict and set(value) == set(names.split()), 'FIELDS_INVALID', location)


def enum(value, choices, location):
    require(type(value) is str and value in choices, 'ENUM_INVALID', location)


def strings(value, location, unique=True):
    require(type(value) is list, 'LIST_REQUIRED', location)
    for item in value:
        text(item, location)
    require(not unique or len(value) == len(set(value)), 'DUPLICATE_VALUE', location)


def sha(value, location, nullable=False):
    require((nullable and value is None) or
            (type(value) is str and SHA.fullmatch(value) is not None), 'SHA_INVALID', location)


class StrictLoader(yaml.SafeLoader):
    pass


# Do not change PyYAML's global loader (the CI source guard also imports it).
StrictLoader.yaml_implicit_resolvers = copy.deepcopy(yaml.SafeLoader.yaml_implicit_resolvers)
for initial, resolvers in StrictLoader.yaml_implicit_resolvers.items():
    StrictLoader.yaml_implicit_resolvers[initial] = [
        item for item in resolvers if item[0] not in {'tag:yaml.org,2002:bool', 'tag:yaml.org,2002:timestamp'}]
StrictLoader.add_implicit_resolver('tag:yaml.org,2002:bool', re.compile(r'^(?:true|false)$'), list('tf'))


def mapping(loader, node, deep=False):
    out = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        require(type(key) is str and key != '<<', 'MAPPING_KEY_INVALID', str(key))
        require(key not in out, 'DUPLICATE_KEY', key)
        out[key] = loader.construct_object(value_node, deep=deep)
    return out


StrictLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, mapping)


def parse(raw):
    require(type(raw) is bytes and len(raw) <= MAX_BYTES, 'DOCUMENT_TOO_LARGE', 'input')
    decoded = raw.decode('utf-8')
    depth = count = 0
    for event in yaml.parse(decoded):
        count += 1
        require(count <= 30000, 'NODE_LIMIT', 'input')
        require(not isinstance(event, yaml.events.AliasEvent) and not getattr(event, 'anchor', None)
                and not getattr(event, 'tag', None), 'YAML_ALIAS_OR_TAG', 'input')
        if isinstance(event, (yaml.events.MappingStartEvent, yaml.events.SequenceStartEvent)):
            depth += 1
            require(depth <= 40, 'DEPTH_LIMIT', 'input')
        elif isinstance(event, (yaml.events.MappingEndEvent, yaml.events.SequenceEndEvent)):
            depth -= 1
    value = yaml.load(decoded, Loader=StrictLoader)
    # Reject timestamp objects, sets, non-finite numbers and non-string mapping keys.
    json.dumps(value, allow_nan=False)
    return value


def safe_path(root, relative):
    require(type(relative) is str and bool(relative) and '\\' not in relative and
            '\x00' not in relative and not re.search(r'[\x00-\x1f]', relative), 'PATH_INVALID', str(relative))
    parts = PurePosixPath(relative)
    require(not parts.is_absolute() and '..' not in parts.parts and ':' not in relative,
            'PATH_ESCAPE', relative)
    root = Path(os.path.abspath(root))
    path = root / relative
    require(not any(p.is_symlink() for p in (path, *path.parents)), 'SYMLINK_FORBIDDEN', relative)
    require(path.is_file(), 'FILE_MISSING', relative)
    return path


def read(root, relative):
    path = safe_path(root, relative)
    require(path.stat().st_size <= MAX_BYTES, 'DOCUMENT_TOO_LARGE', relative)
    return path.read_bytes()


def reference(root, value, location):
    text(value, location)
    uri = urlsplit(value)
    if uri.scheme or uri.netloc:
        require(uri.scheme == 'https' and bool(uri.hostname) and not uri.username and not uri.password,
                'UNSAFE_REFERENCE', location)
    else:
        safe_path(root, unquote(value.split('#', 1)[0]))


def git_blob(root, commit, path):
    """Inspect literal Git objects only; no fetch, hooks, shell or user-provided command."""
    sha(commit, 'git.commit')
    safe_path(root, path)
    try:
        kind_result = subprocess.run(['git', 'cat-file', '-t', commit], cwd=root,
                                     capture_output=True, timeout=5, check=False)
        if kind_result.returncode:
            return None
        require(kind_result.stdout.strip() == b'commit', 'GIT_NOT_COMMIT', commit)
        result = subprocess.run(['git', '--literal-pathspecs', 'ls-tree', '-z', commit, '--', path],
                                cwd=root, capture_output=True, timeout=5, check=False)
        if result.returncode or not result.stdout:
            return None
        entries = [entry for entry in result.stdout.split(b'\0') if entry]
        require(len(entries) == 1, 'GIT_PATH_AMBIGUOUS', path)
        header, actual_path = entries[0].split(b'\t', 1)
        mode, kind, oid = header.decode('ascii').split()
        require(actual_path.decode('utf-8') == path and mode in {'100644', '100755'} and kind == 'blob',
                'GIT_NOT_REGULAR_FILE', path)
        size = subprocess.run(['git', 'cat-file', '-s', oid], cwd=root,
                              capture_output=True, timeout=5, check=False)
        require(size.returncode == 0 and int(size.stdout) <= MAX_BYTES, 'GIT_BLOB_SIZE', path)
        content = subprocess.run(['git', 'cat-file', 'blob', oid], cwd=root,
                                 capture_output=True, timeout=5, check=False)
        require(content.returncode == 0, 'GIT_BLOB_READ', path)
        return oid, content.stdout
    except (OSError, subprocess.TimeoutExpired):
        return None


def payload(value):
    # JSON retains scalar types: true != 1; 1 != 1.0. Only these two top-level fields may differ.
    return json.dumps({k: v for k, v in value.items() if k not in {'review', 'status'}},
                      ensure_ascii=False, sort_keys=True, separators=(',', ':'), allow_nan=False)


def analyze(root, path, doc, expected_candidate=None):
    pending = []
    def needs(code, location):
        pending.append({'code': code, 'location': location})

    fields(doc, 'schema_version artifact_kind status task_ref classification context sources page_pattern '
                'outcome humanized_ux acceptance open_questions handoff review', path)
    require(type(doc['schema_version']) is int and doc['schema_version'] == 1, 'SCHEMA_VERSION', path)
    enum(doc['artifact_kind'], {'template', 'example', 'analysis'}, 'artifact_kind')
    enum(doc['status'], {'draft', 'ready_for_review', 'reviewed'}, 'status')
    illustrative = doc['artifact_kind'] != 'analysis'
    if illustrative:
        require(doc['status'] == 'draft', 'ILLUSTRATION_NOT_DRAFT', path)
    task = doc['task_ref']
    fields(task, 'issue baseline_commit candidate_commit name', 'task_ref')
    for key in ('issue', 'name'):
        text(task[key], 'task_ref.' + key, nullable=illustrative)
    for key in ('baseline_commit', 'candidate_commit'):
        sha(task[key], 'task_ref.' + key, nullable=True)
    candidate = task['candidate_commit']
    if expected_candidate is not None:
        sha(expected_candidate, 'expected_candidate')
        require(candidate == expected_candidate, 'EXPECTED_CANDIDATE_MISMATCH', path)
    elif candidate is not None:
        needs('EXTERNAL_CANDIDATE_NOT_VERIFIED', path)

    classification = doc['classification']
    fields(classification, 'type reason business_state_mutation authorization_sensitive asynchronous_effect', 'classification')
    enum(classification['type'], {'engineering', 'ui_presentation', 'ui_interaction',
                                 'backend_capability', 'business_task', 'unknown'}, 'classification.type')
    text(classification['reason'], 'classification.reason')
    for key in ('business_state_mutation', 'authorization_sensitive', 'asynchronous_effect'):
        flag = classification[key]
        require(type(flag) is bool or flag == 'unknown', 'BOOLEAN_OR_UNKNOWN', key)
        if flag == 'unknown':
            needs('CLASSIFICATION_UNKNOWN', key)
    if classification['type'] == 'unknown':
        needs('CLASSIFICATION_UNKNOWN', 'classification.type')
    if any(classification[k] is True for k in ('business_state_mutation', 'authorization_sensitive', 'asynchronous_effect')):
        require(classification['type'] in {'business_task', 'backend_capability', 'unknown'},
                'RISK_DOWNGRADED', 'classification.type')

    require(type(doc['sources']) is list, 'LIST_REQUIRED', 'sources')
    sources = {}
    for item in doc['sources']:
        fields(item, 'id kind path commit locator claim', 'sources.item')
        text(item['id'], 'sources.id')
        require(item['id'] not in sources, 'DUPLICATE_SOURCE_ID', item['id'])
        enum(item['kind'], SOURCE_KINDS, 'sources.kind')
        for key in ('locator', 'claim'):
            text(item[key], 'sources.' + key)
        text(item['commit'], 'sources.commit', nullable=True)
        if item['kind'] != 'hypothesis' or item['path'] is not None:
            reference(root, item['path'], 'sources.path')
        if item['kind'] != 'hypothesis':
            needs('SOURCE_CLAIM_REQUIRES_REVIEW', item['id'])
        sources[item['id']] = item

    def refs(items, location):
        strings(items, location)
        require(all(item in sources for item in items), 'SOURCE_REF_UNRESOLVED', location)

    fields(doc['context'], ' '.join(CONTEXT), 'context')
    for key, item in doc['context'].items():
        fields(item, 'summary source_refs', 'context.' + key)
        text(item['summary'], 'context.' + key, nullable=True)
        refs(item['source_refs'], 'context.' + key)
        if item['summary'] is None or not item['source_refs']:
            needs('CONTEXT_INCOMPLETE', 'context.' + key)

    page = doc['page_pattern']
    fields(page, 'name reason contract_ref gaps', 'page_pattern')
    strings(page['gaps'], 'page_pattern.gaps')
    for key in ('name', 'reason', 'contract_ref'):
        text(page[key], 'page_pattern.' + key, nullable=True)
    if page['name'] is not None:
        contract = parse(read(root, 'web/ui-contracts.json'))
        patterns = contract['patterns']
        require(page['name'] in patterns, 'PAGE_PATTERN_UNKNOWN', 'page_pattern.name')
        text(page['reason'], 'page_pattern.reason')
        text(page['contract_ref'], 'page_pattern.contract_ref')
        require(page['contract_ref'].split(':', 1)[0] == 'web/ui-contracts.json',
                'PAGE_CONTRACT_AUTHORITY', 'page_pattern.contract_ref')
        if patterns[page['name']]['status'] != 'implemented':
            require(bool(page['gaps']), 'RESERVED_PATTERN_WITHOUT_GAP', page['name'])
            needs('PAGE_PATTERN_NOT_IMPLEMENTED', page['name'])

    outcome = doc['outcome']
    fields(outcome, 'definition result_contract_refs recovery_contract_refs', 'outcome')
    text(outcome['definition'], 'outcome.definition', nullable=True)
    refs(outcome['result_contract_refs'], 'outcome.result_contract_refs')
    refs(outcome['recovery_contract_refs'], 'outcome.recovery_contract_refs')
    fields(doc['humanized_ux'], ' '.join(DIMENSIONS), 'humanized_ux')
    evidence = []
    for dimension, item in doc['humanized_ux'].items():
        fields(item, 'applicability reason requirements evidence_types verification evidence', dimension)
        enum(item['applicability'], {'applies', 'unknown', 'not_applicable'}, dimension)
        text(item['reason'], dimension + '.reason')
        strings(item['requirements'], dimension + '.requirements')
        strings(item['evidence_types'], dimension + '.evidence_types')
        require(set(item['evidence_types']) <= EVIDENCE_TYPES, 'EVIDENCE_TYPE_INVALID', dimension)
        enum(item['verification'], {'not_verified', 'pass', 'fail', 'not_applicable'}, dimension)
        applicability, verification = item['applicability'], item['verification']
        require(type(item['evidence']) is list, 'LIST_REQUIRED', dimension + '.evidence')
        if applicability == 'unknown':
            require(verification == 'not_verified', 'UNKNOWN_VERIFIED', dimension)
            needs('APPLICABILITY_UNKNOWN', dimension)
        elif applicability == 'not_applicable':
            require(verification == 'not_applicable' and not item['evidence'], 'NA_STATE_INVALID', dimension)
            needs('NA_REASON_REQUIRES_REVIEW', dimension)
        else:
            require(verification != 'not_applicable' and bool(item['requirements']) and bool(item['evidence_types']),
                    'APPLICABLE_OBLIGATION_EMPTY', dimension)
        if illustrative:
            require(verification in {'not_verified', 'not_applicable'} and not item['evidence'],
                    'ILLUSTRATION_CANNOT_CERTIFY', dimension)
        if verification == 'not_verified':
            needs('DIMENSION_NOT_VERIFIED', dimension)
        seen = set()
        for entry in item['evidence']:
            fields(entry, 'type reference candidate_sha scope result', dimension + '.evidence')
            enum(entry['type'], EVIDENCE_TYPES, dimension + '.evidence.type')
            enum(entry['result'], {'pass', 'fail', 'not_run'}, dimension + '.evidence.result')
            sha(entry['candidate_sha'], dimension + '.evidence.candidate_sha')
            text(entry['scope'], dimension + '.evidence.scope')
            reference(root, entry['reference'], dimension + '.evidence.reference')
            identity = (entry['type'], entry['reference'], entry['candidate_sha'], entry['scope'])
            require(identity not in seen, 'DUPLICATE_EVIDENCE', dimension)
            seen.add(identity)
            if verification in {'pass', 'fail'}:
                require(candidate is not None and entry['candidate_sha'] == candidate,
                        'EVIDENCE_CANDIDATE_MISMATCH', dimension)
                require(dimension in entry['scope'], 'EVIDENCE_SCOPE_MISMATCH', dimension)
            elif entry['candidate_sha'] != candidate:
                needs('HISTORICAL_EVIDENCE_NOT_CURRENT', dimension)
            needs('EXTERNAL_EVIDENCE_NOT_VERIFIED', dimension)
            evidence.append(entry)
        if verification == 'pass':
            # Retained failures are history, not proof of a pass. All entries have
            # already passed candidate/scope checks; each required type still
            # needs its own passing record. Never infer retest order from a list.
            passing_types = {e['type'] for e in item['evidence'] if e['result'] == 'pass'}
            require(set(item['evidence_types']) <= passing_types,
                    'PASS_WITHOUT_REQUIRED_EVIDENCE', dimension)
            for entry in item['evidence']:
                if entry['result'] != 'pass':
                    needs('NONPASS_HISTORY_REQUIRES_REVIEW',
                          dimension + '.evidence:' + entry['reference'])
        elif verification == 'fail':
            require(any(e['result'] == 'fail' for e in item['evidence']), 'FAIL_WITHOUT_EVIDENCE', dimension)
            needs('DIMENSION_FAILED', dimension)

    acceptance = doc['acceptance']
    fields(acceptance, 'scenarios automated_checks human_checks metrics', 'acceptance')
    for key in ('scenarios', 'automated_checks', 'human_checks'):
        strings(acceptance[key], 'acceptance.' + key)
    require(type(acceptance['metrics']) is list, 'LIST_REQUIRED', 'acceptance.metrics')
    metric_ids = set()
    for metric in acceptance['metrics']:
        fields(metric, 'id definition conditions baseline target observed unit sample_size failure_policy evidence_ref', 'metric')
        for key in ('id', 'definition', 'conditions', 'unit', 'failure_policy'):
            text(metric[key], 'metric.' + key)
        require(metric['id'] not in metric_ids, 'DUPLICATE_METRIC_ID', metric['id'])
        metric_ids.add(metric['id'])
        require(type(metric['sample_size']) is int and metric['sample_size'] >= 0, 'SAMPLE_SIZE_INVALID', metric['id'])
        for key in ('baseline', 'target', 'observed'):
            value = metric[key]
            require(value is None or (type(value) in {int, float} and math.isfinite(value)), 'METRIC_NUMBER_INVALID', key)
        if metric['observed'] is not None:
            require(not illustrative and candidate is not None and metric['sample_size'] > 0,
                    'OBSERVED_WITHOUT_SAMPLE', metric['id'])
            text(metric['evidence_ref'], 'metric.evidence_ref')
            matches = [e for e in evidence if e['reference'] == metric['evidence_ref'] and e['type'] == 'user_test'
                       and e['candidate_sha'] == candidate and e['result'] == 'pass' and metric['id'] in e['scope']]
            require(bool(matches), 'METRIC_EVIDENCE_UNRESOLVED', metric['id'])
            needs('MEASUREMENT_SAMPLE_AND_CONDITIONS_NOT_VERIFIED', metric['id'])
        else:
            require(metric['sample_size'] == 0 and metric['evidence_ref'] is None, 'UNMEASURED_SAMPLE_INCONSISTENT', metric['id'])
            needs('METRIC_NOT_MEASURED', metric['id'])

    questions = doc['open_questions']
    require(type(questions) is list, 'LIST_REQUIRED', 'open_questions')
    for question in questions:
        fields(question, 'question blocking owner_role', 'open_questions.item')
        text(question['question'], 'open_questions.question')
        text(question['owner_role'], 'open_questions.owner_role')
        require(type(question['blocking']) is bool, 'BLOCKING_NOT_BOOLEAN', 'open_questions')
        if question['blocking']:
            needs('OPEN_BLOCKER', question['question'])
    if not illustrative and any(p['code'] in {'CONTEXT_INCOMPLETE', 'CLASSIFICATION_UNKNOWN', 'APPLICABILITY_UNKNOWN'} for p in pending):
        require(bool(questions), 'UNKNOWN_WITHOUT_QUESTION', path)
    if classification['business_state_mutation'] is True:
        for key in ('result_contract_refs', 'recovery_contract_refs'):
            authoritative = [sources[s] for s in outcome[key]
                             if sources[s]['kind'] in {'implementation', 'validated_runbook', 'repository_contract'}
                             and sources[s]['path'] is not None
                             and not sources[s]['path'].startswith(('web/', 'docs/design/'))]
            if not authoritative:
                require(any(q['blocking'] for q in questions), 'WRITE_AUTHORITY_MISSING', key)
                needs('WRITE_AUTHORITY_MISSING', key)
        for key in ('error_recoverability', 'operation_effect_transparency'):
            require(doc['humanized_ux'][key]['applicability'] != 'not_applicable', 'WRITE_OBLIGATION_WAIVED', key)
        needs('BUSINESS_AUTHORITY_REQUIRES_REVIEW', path)

    handoff = doc['handoff']
    fields(handoff, 'allowed_paths forbidden_paths reuse_refs gates', 'handoff')
    for key in ('allowed_paths', 'forbidden_paths', 'reuse_refs'):
        strings(handoff[key], 'handoff.' + key)
    for ref in handoff['reuse_refs']:
        reference(root, ref, 'handoff.reuse_refs')
    require(type(handoff['gates']) is list, 'LIST_REQUIRED', 'handoff.gates')
    gate_names = set()
    for gate in handoff['gates']:
        fields(gate, 'name status', 'handoff.gates.item')
        text(gate['name'], 'gate.name')
        require(gate['name'] not in gate_names, 'DUPLICATE_GATE_ID', gate['name'])
        gate_names.add(gate['name'])
        enum(gate['status'], {'not_run', 'pass', 'fail', 'not_verified'}, 'gate.status')
        require(not illustrative or gate['status'] in {'not_run', 'not_verified'}, 'ILLUSTRATION_GATE_RESULT', gate['name'])
        needs('GATE_EXECUTION_NOT_VERIFIED', gate['name'])

    review = doc['review']
    fields(review, 'reviewer actor_kind scope reference analysis_ref reviewed_at candidate_sha decision', 'review')
    enum(review['decision'], {'not_reviewed', 'approved', 'changes_requested'}, 'review.decision')
    if doc['status'] != 'reviewed':
        require(review['decision'] == 'not_reviewed' and all(v is None for k, v in review.items() if k != 'decision'),
                'UNREVIEWED_METADATA_CONTRADICTION', path)
        needs('INDEPENDENT_REVIEW_NOT_PERFORMED', path)
    else:
        require(review['decision'] != 'not_reviewed', 'REVIEWED_WITHOUT_DECISION', path)
        text(review['reviewer'], 'review.reviewer')
        enum(review['actor_kind'], {'human', 'automated'}, 'review.actor_kind')
        enum(review['scope'], {'document_design', 'task_experience'}, 'review.scope')
        reference(root, review['reference'], 'review.reference')
        text(review['reviewed_at'], 'review.reviewed_at')
        datetime.fromisoformat(review['reviewed_at'].replace('Z', '+00:00'))
        require(review['candidate_sha'] == candidate, 'REVIEW_CANDIDATE_MISMATCH', path)
        if review['scope'] == 'task_experience':
            require(candidate is not None, 'EXPERIENCE_CANDIDATE_UNKNOWN', path)
        if review['decision'] == 'approved':
            require(not any(q['blocking'] for q in questions), 'APPROVED_WITH_BLOCKER', path)
        ref = review['analysis_ref']
        fields(ref, 'path commit blob_sha', 'review.analysis_ref')
        require(ref['path'] == path, 'ANALYSIS_PATH_MISMATCH', path)
        sha(ref['commit'], 'analysis_ref.commit')
        sha(ref['blob_sha'], 'analysis_ref.blob_sha')
        saved = git_blob(root, ref['commit'], path)
        if saved is None:
            needs('ANALYSIS_GIT_OBJECT_NOT_AVAILABLE', path)
        else:
            require(saved[0] == ref['blob_sha'], 'ANALYSIS_BLOB_MISMATCH', path)
            original = parse(saved[1])
            require(type(original) is dict and payload(original) == payload(doc), 'ANALYSIS_PAYLOAD_CHANGED', path)
        # A Git blob proves bytes, NOT the reviewer identity, scope, independence or external decision.
        needs('EXTERNAL_REVIEW_NOT_VERIFIED', path)

    serialized = json.dumps(doc, ensure_ascii=False)
    hazards = {
        'HIGH_RISK_PROMISE_REQUIRES_REVIEW': r'不会重复扣款|额度已释放|已恢复制作|自动回滚|安全重试|no duplicate charge|safe retry',
        'TIMEOUT_RECOVERY_REQUIRES_REVIEW': r'超时|timeout',
        'PAGE_IS_NOT_API_EVIDENCE': r'API.*(?:已实现|真相)|页面声明|page declaration',
    }
    for code, pattern in hazards.items():
        if re.search(pattern, serialized, re.I):
            # Includes negations intentionally: human review resolves context; a keyword is not a verdict.
            needs(code, path)
    if not illustrative:
        needs('SEMANTIC_REVIEW_REQUIRED', path)
    return pending


def markdown_links(root, path, content):
    """Check local link targets without executing Markdown, fetching URLs or reading outside root."""
    count = 0
    # Fenced examples are not navigation links. Indented code is harmlessly checked conservatively.
    content = re.sub(r'^(`{3,}|~{3,}).*?^\1\s*$', '', content, flags=re.M | re.S)
    targets = re.findall(r'\[[^\]\n]*\]\(([^)\n]+)\)', content)
    definitions = dict(re.findall(r'^\s*\[([^\]]+)\]:\s*(\S+)\s*$', content, re.M))
    for label in re.findall(r'\[[^\]\n]+\]\[([^\]\n]+)\]', content):
        require(label in definitions, 'MARKDOWN_REFERENCE_UNRESOLVED', path + ':' + label)
    targets.extend(definitions.values())
    for target in targets:
        target = target.strip().strip('<>')
        uri = urlsplit(target)
        if uri.scheme or uri.netloc:
            require(uri.scheme == 'https' and bool(uri.hostname) and not uri.username and not uri.password,
                    'UNSAFE_REFERENCE', path)
            continue
        target_path, _, anchor = target.partition('#')
        decoded = unquote(target_path)
        require(not decoded.startswith('/') and '\\' not in decoded, 'PATH_ESCAPE', path)
        absolute = Path(os.path.abspath(root / Path(path).parent / decoded)) if decoded else root / path
        try:
            relative = absolute.relative_to(root).as_posix()
        except ValueError as error:
            raise Invalid('PATH_ESCAPE:' + path) from error
        linked = safe_path(root, relative)
        if anchor and linked.suffix == '.md':
            headings = re.findall(r'^#{1,6}\s+(.+?)\s*#*$', read(root, relative).decode('utf-8'), re.M)
            slugs = [re.sub(r'[^\w\-\s]', '', h.lower()).replace(' ', '-') for h in headings]
            require(unquote(anchor) in slugs, 'MARKDOWN_ANCHOR_MISSING', path + ':' + anchor)
        count += 1
    return count


def validate(root, analyses=(), expected_candidate=None):
    root = Path(os.path.abspath(root))
    errors, pending, checked = [], [], []
    links = 0
    try:
        require(not any(p.is_symlink() for p in (root, *root.parents)), 'SYMLINK_FORBIDDEN', 'root')
        if expected_candidate is not None:
            sha(expected_candidate, 'expected_candidate')
        for name in REQUIRED_FILES:
            safe_path(root, SKILL + '/' + name)
        skill_root = root / SKILL
        files = sorted(skill_root.rglob('*'))
        require(len(files) <= 128, 'SKILL_FILE_LIMIT', SKILL)
        total_bytes = 0
        for file in files:
            require(not file.is_symlink(), 'SYMLINK_FORBIDDEN', file.name)
            if file.is_dir():
                continue
            require(file.suffix in {'.md', '.yaml', '.yml'}, 'UNEXPECTED_SKILL_PAYLOAD', file.name)
            path = file.relative_to(root).as_posix()
            raw = read(root, path)
            total_bytes += len(raw)
            require(total_bytes <= 4 * MAX_BYTES, 'SKILL_SIZE_LIMIT', SKILL)
            if file.suffix == '.md':
                links += markdown_links(root, path, raw.decode('utf-8'))
            else:
                doc = parse(raw)
                require(type(doc) is dict and doc.get('artifact_kind') in {'template', 'example'},
                        'SKILL_DIRECTORY_NOT_TASK_LEDGER', path)
                if path == SKILL + '/templates/ux-contract.template.yaml':
                    require(doc['artifact_kind'] == 'template', 'TEMPLATE_KIND_CHANGED', path)
                elif '/examples/' in path:
                    require(doc['artifact_kind'] == 'example', 'EXAMPLE_KIND_CHANGED', path)
                pending.extend(analyze(root, path, doc))
            checked.append(path)
        entry = read(root, SKILL + '/SKILL.md').decode('utf-8')
        meta = re.match(r'\A---\n(.*?)\n---(?:\n|$)', entry, re.S)
        require(meta is not None, 'SKILL_METADATA_MISSING', SKILL)
        metadata = parse(meta[1].encode())
        fields(metadata, 'name description', 'skill.metadata')
        require(metadata['name'] == 'b2b-product-ux', 'SKILL_NAME', SKILL)
        text(metadata['description'], 'skill.description')
        body = read(root, SKILL + '/references/b2b-interaction-patterns.md').decode('utf-8')
        sections = re.findall(r'^## ([a-z][a-z0-9-]+) —[^\n]*\n(.*?)(?=^## |\Z)', body, re.M | re.S)
        ids = [identifier for identifier, _ in sections]
        require(len(ids) >= 16 and len(ids) == len(set(ids)), 'PATTERN_COVERAGE_OR_DUPLICATE', SKILL)
        for identifier, section in sections:
            keys = re.findall(r'^- \*\*([a-z_]+)\*\*[：:]\s*(.+)$', section, re.M)
            require(len(keys) == len(PATTERN_FIELDS) and {k for k, _ in keys} == set(PATTERN_FIELDS),
                    'PATTERN_FIELDS', identifier)
            for key, value in keys:
                text(value, identifier + '.' + key)
        for filename in ('customer-workspace', 'device-incident', 'tenant-plan-upgrade'):
            path = SKILL + '/examples/' + filename + '.md'
            body = read(root, path).decode('utf-8')
            require(all(re.search(r'^\| ' + key + r' \|', body, re.M) for key in CONTEXT), 'EXAMPLE_CONTEXT', path)
            require(all(label in body for label in LABELS), 'EXAMPLE_DIMENSIONS', path)
            require(all(state in body for state in ('draft', 'not_verified', 'not_reviewed')), 'EXAMPLE_STATUS', path)
            baseline = re.search(r'源码阅读基线：`([0-9a-f]{40})`', body)
            require(baseline is not None, 'EXAMPLE_BASELINE_MISSING', path)
            needs_object = git_blob(root, baseline[1], 'web/ui-contracts.json')
            if needs_object is None:
                pending.append({'code': 'EXAMPLE_BASELINE_NOT_LOCALLY_RESOLVED', 'location': path})
        # Entry wiring is part of the gate, not an assumption made from file existence.
        for path in ('AGENTS.md', 'web/AGENTS.md'):
            content = read(root, path).decode('utf-8')
            require('b2b-product-ux' in content and 'b2b-ux-skill-check' in content, 'AGENT_ENTRY_NOT_WIRED', path)
        makefile = read(root, 'Makefile').decode('utf-8')
        require(re.search(r'^check:.*\bb2b-ux-skill-check\b', makefile, re.M) is not None,
                'MAKE_CHECK_NOT_WIRED', 'Makefile')
        require(re.search(r'^ui-skill-check:\n\t@python3 -B scripts/check_ui_skill_source.py\n'
                          r"\t@python3 -B -m unittest discover -s scripts -p 'test_ui_skill_source.py' -v", makefile, re.M),
                'TYPOGRAPHY_GATE_CHANGED', 'Makefile')
        require(re.search(r'^b2b-ux-skill-check:\n\t@python3 -B scripts/check_b2b_ux_skill.py\n'
                          r"\t@python3 -B -m unittest discover -s scripts -p 'test_b2b_ux_skill.py' -v", makefile, re.M),
                'B2B_GATE_COMMANDS_MISSING', 'Makefile')
        for path in analyses:
            doc = parse(read(root, path))
            require(type(doc) is dict and doc.get('artifact_kind') == 'analysis', 'ANALYSIS_KIND_REQUIRED', path)
            pending.extend(analyze(root, path, doc, expected_candidate))
            checked.append(path)
    except (Invalid, OSError, ValueError, TypeError, KeyError, AttributeError, yaml.YAMLError) as error:
        errors.append(str(error))
    return {'scope': 'source_and_structure_only',
            'result': 'FAIL' if errors else ('NEEDS_REVIEW' if pending else 'PASS'),
            'structure_result': 'FAIL' if errors else 'PASS', 'errors': errors,
            'pending_review': pending, 'checked_files': checked, 'local_links_checked': links,
            'external_records': 'NOT_VERIFIED', 'browser_validation': 'NOT_PERFORMED',
            'human_measurement': 'NOT_PERFORMED', 'approval_granted': False,
            'delivery_granted': False}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument('--analysis', action='append', default=[], metavar='REPO_RELATIVE_PATH')
    parser.add_argument('--expected-candidate', help='Expected product SHA supplied by the task/PR observer, not read from the analysis')
    parser.add_argument('--require-verified', action='store_true',
                        help='Exit 2 when review/evidence is still unverified; this offline checker never grants approval')
    args = parser.parse_args(argv)
    result = validate(args.root, args.analysis, args.expected_candidate)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    if result['errors']:
        return 1
    return 2 if args.require_verified and result['result'] == 'NEEDS_REVIEW' else 0


if __name__ == '__main__':
    raise SystemExit(main())
