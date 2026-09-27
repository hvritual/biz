#!/usr/bin/env python3
"""Serial Access qualification with exact go test JSON evidence, never shell selectors."""
from __future__ import annotations
import argparse
from collections import Counter
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
REGISTRY = 'scripts/ci_access_tests.json'
PACKAGE = 'github.com/hvritual/biz/integration'


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def load_json(raw):
    def pairs(items):
        out = {}
        for key, value in items:
            require(key not in out, 'DUPLICATE_KEY:' + key)
            out[key] = value
        return out
    return json.loads(raw, object_pairs_hook=pairs)


def validate(data, root=ROOT, baseline=None):
    require(set(data) == {'schema_version', 'suites'} and type(data['schema_version']) is int
            and data['schema_version'] == 1, 'REGISTRY_SCHEMA')
    require(isinstance(data['suites'], list) and data['suites'], 'EMPTY_SUITES')
    ids, names, identity = set(), set(), {}
    for suite in data['suites']:
        require(set(suite) == {'id', 'tests'} and isinstance(suite['id'], str)
                and re.fullmatch(r'[a-z][a-z0-9-]{1,63}', suite['id']), 'SUITE_SCHEMA')
        require(suite['id'] not in ids and isinstance(suite['tests'], list) and suite['tests'], 'SUITE_DUPLICATE_OR_EMPTY')
        ids.add(suite['id'])
        for test in suite['tests']:
            require(set(test) == {'name', 'file'}, 'TEST_SCHEMA')
            name, source = test['name'], test['file']
            require(isinstance(name, str) and re.fullmatch(r'Test[A-Z][A-Za-z0-9_]*', name), 'TEST_NAME')
            require(name not in names, 'DUPLICATE_TEST:' + name)
            names.add(name)
            require(isinstance(source, str) and re.fullmatch(r'integration/[A-Za-z0-9_]+_test\.go', source), 'TEST_PATH')
            path = root / source
            require(path.is_file() and not path.is_symlink() and path.resolve().is_relative_to(root.resolve()), 'TEST_SOURCE_MISSING:' + source)
            text = re.sub(r'/\*.*?\*/|//[^\n]*', '', path.read_text(), flags=re.S)
            require(re.search(r'^func\s+' + re.escape(name) + r'\s*\(\s*\w+\s+\*testing\.T\s*\)', text, re.M), 'TEST_DECLARATION_MISSING:' + name)
            identity[name] = (suite['id'], source)
    if baseline is not None:
        for suite in baseline['suites']:
            for test in suite['tests']:
                require(identity.get(test['name']) == (suite['id'], test['file']), 'HISTORICAL_TEST_REMOVED_OR_CHANGED:' + test['name'])
    return identity


def test_argv(suite):
    pattern = '^(' + '|'.join(test['name'] for test in suite['tests']) + ')$'
    return ['go', 'test', '-json', '-count=1', '-tags=integration', './integration', '-run', pattern]


def verify_events(events, expected, exit_code):
    require(exit_code == 0, 'GO_TEST_PROCESS_FAILED')
    started, passed, package_passed = Counter(), Counter(), 0
    require(events, 'EMPTY_GO_TEST_EVIDENCE')
    for event in events:
        require(isinstance(event, dict) and isinstance(event.get('Action'), str), 'INVALID_GO_TEST_EVENT')
        action, name, package = event['Action'], event.get('Test'), event.get('Package')
        require(action != 'fail', 'GO_TEST_FAILURE_EVENT')
        if not name:
            if action == 'pass' and package == PACKAGE:
                package_passed += 1
            continue
        top = name.split('/', 1)[0]
        require(package == PACKAGE and top in expected, 'UNEXPECTED_TEST_IDENTITY:' + str(name))
        require(action != 'skip', 'REQUIRED_TEST_OR_SUBTEST_SKIPPED:' + name)
        if '/' not in name:
            if action == 'run':
                started[name] += 1
            if action == 'pass':
                passed[name] += 1
    require(package_passed == 1, 'PACKAGE_PASS_MISSING_OR_DUPLICATE')
    require(dict(started) == {name: 1 for name in expected}, 'TEST_RUN_SET_MISMATCH')
    require(dict(passed) == {name: 1 for name in expected}, 'TEST_PASS_SET_MISMATCH')
    return sorted(passed)


def execute(suite, output, timeout):
    output.mkdir(parents=True, exist_ok=True)
    log = output / (suite['id'] + '.jsonl')
    stderr = output / (suite['id'] + '.stderr')
    with log.open('wb') as stream, stderr.open('wb') as errors:
        process = subprocess.Popen(test_argv(suite), cwd=ROOT, stdout=stream, stderr=errors,
                                   shell=False, start_new_session=True)
        try:
            code = process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait()
            raise ValueError('GO_TEST_TIMEOUT:' + suite['id'])
    # Bound evidence parsing and retain raw logs for diagnosis, not forged grep output.
    require(log.stat().st_size <= 64 * 1024 * 1024, 'GO_TEST_EVIDENCE_TOO_LARGE')
    events = [json.loads(line) for line in log.read_text().splitlines() if line.strip()]
    passed = verify_events(events, {test['name'] for test in suite['tests']}, code)
    return {'suite': suite['id'], 'tests': passed, 'state': 'PASS',
            'log_sha256': hashlib.sha256(log.read_bytes()).hexdigest()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--base-ref', default='')
    parser.add_argument('--output', default=str(Path(os.getenv('RUNNER_TEMP', '.')) / 'access-qualification'))
    args = parser.parse_args()
    report = {'state': 'BLOCKED', 'schema_version': 1, 'suites': [],
              'observed_at': datetime.now(timezone.utc).isoformat()}
    output = Path(args.output)
    try:
        raw = (ROOT / REGISTRY).read_bytes()
        data = load_json(raw)
        baseline = None
        if args.base_ref:
            previous = subprocess.run(['git', 'show', f'{args.base_ref}:{REGISTRY}'], cwd=ROOT, capture_output=True, timeout=20)
            if previous.returncode == 0:
                baseline = load_json(previous.stdout)
        validate(data, ROOT, baseline)
        report['registry_sha256'] = hashlib.sha256(raw).hexdigest()
        if args.check:
            print('ACCESS_REGISTRY=PASS')
            return 0
        require(os.getenv('YUNKA_TEST_MYSQL_DSN') and os.getenv('YUNKA_TEST_RESET_FIXTURES') == '1', 'DATABASE_OPT_IN_REQUIRED')
        # This runner is the existing GitHub-owned B12 lane, never a local reset bypass.
        require(os.getenv('GITHUB_ACTIONS') == 'true', 'USE_QUALIFY_EVOLUTION_MYSQL_FOR_LOCAL_DATABASE')
        report['source_sha'] = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
        report['candidate_tree'] = subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], cwd=ROOT, text=True).strip()
        event_path = Path(os.environ['GITHUB_EVENT_PATH'])
        require(event_path.stat().st_size <= 8 * 1024 * 1024, 'EVENT_TOO_LARGE')
        event = load_json(event_path.read_bytes())
        candidate = event.get('pull_request', {}).get('head', {}).get('sha') or os.environ['GITHUB_SHA']
        require(re.fullmatch(r'[0-9a-f]{40}', candidate), 'CANDIDATE_SHA_INVALID')
        candidate_tree = subprocess.check_output(['git', 'rev-parse', candidate + '^{tree}'], cwd=ROOT, text=True).strip()
        require(candidate_tree == report['candidate_tree'], 'SOURCE_TREE_DIFFERS_FROM_CANDIDATE')
        report['candidate_sha'] = candidate
        report['run_id'] = os.environ['GITHUB_RUN_ID']
        report['run_attempt'] = os.environ['GITHUB_RUN_ATTEMPT']
        for suite in data['suites']:
            print('ACCESS_QUALIFICATION=RUN ' + suite['id'], flush=True)
            report['suites'].append(execute(suite, output, 180))
            print('ACCESS_QUALIFICATION=PASS ' + suite['id'], flush=True)
        report['state'] = 'PASS'
    except Exception as error:
        report['reason'] = str(error)
    output.mkdir(parents=True, exist_ok=True)
    (output / 'receipt.json').write_text(json.dumps(report, indent=2) + '\n')
    print('ACCESS_QUALIFICATION=' + json.dumps(report, sort_keys=True))
    return 0 if report['state'] == 'PASS' else 1


if __name__ == '__main__':
    raise SystemExit(main())
