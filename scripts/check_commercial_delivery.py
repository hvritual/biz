#!/usr/bin/env python3
"""Check commercial evidence references, never certify product/runtime success.

Status authority: docs/commercial-entitlements/tasks.json. delivery-links.json
is a reference index only. No GitHub write, network, database or UI mutation.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys

PLAN = Path('docs/commercial-entitlements')
SHA = re.compile(r'[0-9a-f]{40}')
STATES = {'PLANNED', 'IN_PROGRESS', 'VERIFYING', 'BLOCKED', 'DONE'}
KINDS = {'api_implementation', 'design_preview'}
TEST_KINDS = {'ui_mock', 'unit_source', 'mysql_source', 'runtime_receipt'}


def strict_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('DUPLICATE_JSON_KEY: ' + key)
        result[key] = value
    return result


def read_json(path):
    return json.loads(path.read_text(encoding='utf-8'), object_pairs_hook=strict_object)


def git(root, *args):
    process = subprocess.run(['git', '-C', str(root), *args], capture_output=True,
                             text=True, timeout=20, check=False)
    if process.returncode:
        raise ValueError('GIT_OBJECT_UNAVAILABLE: ' + ' '.join(args) +
                         '; use a complete checkout (fetch-depth: 0)')
    return process.stdout.strip()


def validate(root: Path) -> dict:
    root = root.resolve()
    errors, observations, inventory, inputs = [], [], [], {}

    def require(ok, code, context):
        if not ok:
            errors.append({'code': code, 'context': str(context)})
        return bool(ok)

    def source(value):
        if not isinstance(value, str) or not value or '\\' in value:
            raise ValueError('INVALID_REFERENCE_PATH: ' + str(value))
        path = PurePosixPath(value)
        if path.is_absolute() or '..' in path.parts or ':' in value:
            raise ValueError('UNSAFE_REFERENCE_PATH: ' + value)
        current = root
        for part in path.parts:
            current /= part
            if current.is_symlink():
                raise ValueError('SYMLINK_REFERENCE: ' + value)
        if not current.resolve().is_relative_to(root) or not current.is_file():
            raise ValueError('MISSING_REFERENCE: ' + value)
        inputs[value] = hashlib.sha256(current.read_bytes()).hexdigest()
        return current

    def load(value):
        return read_json(source(value))

    def commit(value, context):
        if not require(isinstance(value, str) and SHA.fullmatch(value),
                       'FULL_COMMIT_REQUIRED', context):
            return
        try:
            git(root, 'cat-file', '-e', value + '^{commit}')
        except ValueError as exc:
            require(False, 'COMMIT_UNRESOLVED', str(context) + ': ' + str(exc))

    def anchor(reference):
        if not isinstance(reference, dict) or set(reference) != {'path', 'symbol'}:
            raise ValueError('SOURCE_ANCHOR_REQUIRED')
        text = source(reference['path']).read_text(encoding='utf-8')
        symbol = reference['symbol']
        require(isinstance(symbol, str) and bool(symbol.strip()) and symbol in text,
                'SOURCE_SYMBOL_MISSING', reference['path'] + ':' + str(symbol))

    def test_reference(reference):
        kind = reference.get('kind')
        require(kind in TEST_KINDS, 'TEST_KIND_UNKNOWN', kind)
        path = reference.get('path')
        text = source(path).read_text(encoding='utf-8')
        if kind == 'runtime_receipt':
            receipt = read_json(source(path))
            require(receipt.get('evidence_kind') == 'runtime_receipt' and
                    receipt.get('result') == 'PASS' and
                    type(receipt.get('executed_tests')) is int and receipt['executed_tests'] > 0 and
                    receipt.get('required_skips') == 0,
                    'INVALID_RUNTIME_RECEIPT', path)
            commit(receipt.get('candidate_commit'), path)
            require(receipt.get('candidate_commit') == candidate,
                    'RUNTIME_RECEIPT_STALE', path)
            raw = receipt.get('artifacts', [])
            require(isinstance(raw, list) and bool(raw), 'RAW_ARTIFACTS_REQUIRED', path)
            for artifact in raw:
                file = source(artifact['path'])
                require(hashlib.sha256(file.read_bytes()).hexdigest() == artifact.get('sha256'),
                        'ARTIFACT_HASH_MISMATCH', artifact['path'])
        elif kind != 'ui_mock' and re.search(r'\b(?:page|context)\.route\s*\(', text):
            require(False, 'MOCK_AS_INTEGRATION', path)
        return {'path': path, 'kind': kind, 'execution_claimed': kind == 'runtime_receipt'}

    try:
        candidate = git(root, 'rev-parse', 'HEAD')
        plan = load(str(PLAN / 'tasks.json'))
        index = load(str(PLAN / 'delivery-links.json'))
        require(index.get('schema_version') == 1, 'INDEX_SCHEMA', 'delivery-links.json')
        require(index.get('status_authority') == str(PLAN / 'tasks.json'),
                'STATUS_AUTHORITY_DRIFT', 'delivery-links.json')
        require('status' not in index, 'DUPLICATE_STATUS_AUTHORITY', 'index')
        commit(index.get('observed_at_commit'), 'observation baseline')
        tasks = plan['tasks']
        by_id = {task['id']: task for task in tasks}
        require(len(by_id) == len(tasks) and bool(tasks), 'TASK_ID_DUPLICATE_OR_EMPTY', 'tasks.json')
        links = index['task_links']
        require(set(links) == set(by_id), 'TASK_LINK_COVERAGE', 'all canonical CE tasks required')
        for tid, task in by_id.items():
            require(task.get('status') in STATES, 'UNKNOWN_TASK_STATUS', tid)
            link = links.get(tid, {})
            require('status' not in link, 'DUPLICATE_STATUS_AUTHORITY', tid)
            issues = link.get('tracking_issues', [])
            require(isinstance(issues, list) and bool(issues) and
                    all(type(number) is int and number > 0 for number in issues),
                    'TRACKING_ISSUE_REQUIRED', tid)
            if task.get('status') == 'DONE':
                require(not link.get('unresolved_scope'), 'DONE_WITH_UNRESOLVED_SCOPE', tid)
                for key in ('evidence', 'verification'):
                    value = task.get(key)
                    if require(bool(value), 'DONE_EVIDENCE_REQUIRED', tid + ':' + key):
                        file = source(str(PLAN / str(value)))
                        if key == 'verification':
                            read_json(file)
                commit(task.get('integration_commit'), tid)
            else:
                require(bool(link.get('unresolved_scope')), 'UNRESOLVED_SCOPE_REQUIRED', tid)
            for ref in link.get('sources', []):
                anchor(ref)
            tests = [test_reference(test) for test in link.get('tests', [])]
            if task.get('status') != 'DONE' and link.get('sources'):
                observations.append({'code': 'PARTIAL_IMPLEMENTATION_NOT_ACCEPTANCE', 'task': tid,
                                     'tracking_issues': issues, 'unresolved_scope': link['unresolved_scope']})
            inventory.append({'task': tid, 'status_from_authority': task.get('status'),
                              'tracking_issues': issues, 'tests': tests,
                              'integration_commit': task.get('integration_commit'),
                              'unresolved_scope': link.get('unresolved_scope', [])})
        operations_doc = load('contracts/generated/operation-plans.json')
        operations = {entry['operationId'] for entry in operations_doc['operations']}
        owners = {}
        for owner in index['authority_owners']:
            op, tid = owner['operation'], owner['task']
            require(op in operations, 'UNKNOWN_AUTHORITY_OPERATION', op)
            require(tid in by_id, 'UNKNOWN_AUTHORITY_TASK', tid)
            require(op not in owners, 'MULTIPLE_AUTHORITY_OWNERS', op)
            owners[op] = tid
            anchor(owner['source'])
        ui = load('web/ui-contracts.json')
        routes = {entry['path']: entry for entry in ui['routes']}
        scoped = {path for path in routes if path.startswith('/platform/commercial/') or path == '/enterprise/plan'}
        require(set(index['consumers']) == scoped, 'CONSUMER_COVERAGE', sorted(scoped))
        for route, consumer in index['consumers'].items():
            require('status' not in consumer, 'DUPLICATE_STATUS_AUTHORITY', route)
            if not require(route in routes, 'CONSUMER_ROUTE_MISSING', route):
                continue
            require(consumer.get('task') in by_id, 'CONSUMER_TASK_UNKNOWN', route)
            kind = consumer.get('kind')
            require(kind in KINDS, 'CONSUMER_KIND_UNKNOWN', route)
            component = routes[route]['component']
            file = source('web/src/' + component[2:]) if component.startswith('@/') else source(component)
            # This uses the canonical Page Contract, whose executable Vue binding is
            # independently checked by the existing AST-based web UI gate.
            preview = file.name == 'LifecycleManagementView.vue' or 'platformLifecyclePages' in file.read_text(encoding='utf-8')
            refs = consumer.get('operations', [])
            if kind == 'api_implementation':
                require(not preview, 'PREVIEW_AS_REAL_AUTHORITY', route)
                require(isinstance(refs, list) and bool(refs), 'REAL_ROUTE_AUTHORITY_REQUIRED', route)
                for operation in refs:
                    require(operation in owners, 'UNOWNED_ROUTE_AUTHORITY', route + ':' + operation)
            elif kind == 'design_preview':
                require(not refs, 'PREVIEW_CANNOT_OWN_API_EVIDENCE', route)
                observations.append({'code': 'DESIGN_PREVIEW_NOT_RUNTIME', 'route': route,
                                     'remediation_issue': 289})
            require(not consumer.get('production_verified'), 'STATIC_INDEX_CANNOT_CERTIFY_PRODUCTION', route)
        return {'schema_version': 1, 'check': 'commercial_delivery_references',
                'candidate_commit': candidate, 'result': 'FAIL' if errors else 'PASS',
                'runtime_certification': 'NOT_PERFORMED', 'issue_state_verification': 'NOT_PERFORMED',
                'errors': errors, 'observations': observations, 'tasks': inventory,
                'input_sha256': dict(sorted(inputs.items()))}
    except (OSError, ValueError, TypeError, KeyError, AttributeError, subprocess.SubprocessError) as exc:
        errors.append({'code': 'INVALID_INPUT', 'context': str(exc)})
        return {'schema_version': 1, 'result': 'FAIL', 'runtime_certification': 'NOT_PERFORMED',
                'errors': errors, 'observations': observations}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument('--report', type=Path)
    args = parser.parse_args()
    report = validate(args.root)
    text = json.dumps(report, ensure_ascii=False, indent=2) + '\n'
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(text, encoding='utf-8')
    print(text, end='')
    return 0 if report['result'] == 'PASS' else 1


if __name__ == '__main__':
    raise SystemExit(main())
