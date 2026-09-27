#!/usr/bin/env python3
"""CE04-07 phase execution and baseline coverage evidence (no test selection)."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import signal
import subprocess
import sys
import time

from ce03_test_events import summarize

ROOT = Path(__file__).resolve().parents[1]
BASELINE = ROOT / 'scripts/ci_commercial_baseline.json'


def require(ok, message):
    if not ok:
        raise ValueError(message)


def run_phase(output, command, timeout=180, interval=10):
    """Execute actual argv, preserve stdout/stderr separately, reap on timeout."""
    require(command and timeout > 0 and interval > 0, 'INVALID_PHASE')
    output = Path(output)
    output.parent.mkdir(parents=True, exist_ok=True)
    stem = output.with_suffix('')
    stem.with_suffix('.command').write_text(shlex.join(command) + '\n')
    started = time.monotonic()
    print(f'CE_PHASE_STARTED={stem.name}', flush=True)
    with output.open('wb') as stdout, stem.with_suffix('.stderr').open('wb') as stderr:
        process = subprocess.Popen(command, stdout=stdout, stderr=stderr, start_new_session=True)
        rc = None
        try:
            while rc is None:
                remaining = timeout - (time.monotonic() - started)
                if remaining <= 0:
                    raise TimeoutError(f'PHASE_TIMEOUT:{stem.name}')
                try:
                    rc = process.wait(timeout=min(interval, remaining))
                except subprocess.TimeoutExpired:
                    print(f'CE_PHASE_RUNNING={stem.name} elapsed={time.monotonic()-started:.1f}s '
                          f'log_bytes={output.stat().st_size}', flush=True)
        except BaseException:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=5)
            rc = 124
            raise
        finally:
            stem.with_suffix('.exit').write_text(str(rc if rc is not None else 125) + '\n')
            stem.with_suffix('.seconds').write_text(f'{time.monotonic()-started:.3f}\n')
            print(f'CE_PHASE_FINISHED={stem.name} exit={rc} elapsed={time.monotonic()-started:.1f}s', flush=True)
    if rc:
        for path in (output, stem.with_suffix('.stderr')):
            print(path.read_text(errors='replace')[-8000:], file=sys.stderr)
    return rc


def passed_ids(rows):
    return {r['Package'] + '::' + r['Test'] for r in rows
            if r.get('Action') == 'pass' and r.get('Test')}


def verify_suite(rows, baseline):
    counts = summarize(rows)
    packages = {r['Package'] for r in rows if r.get('Action') == 'pass' and r.get('Test')}
    completed = {r['Package'] for r in rows if r.get('Action') == 'pass' and not r.get('Test')}
    require(packages <= completed, 'PACKAGE_COMPLETION_MISSING')
    actual = passed_ids(rows)
    wanted = {pkg + '::' + test for pkg, tests in baseline['passed'].items() for test in tests}
    require(wanted <= actual, 'BASELINE_TEST_IDENTITY_MISSING:' + ','.join(sorted(wanted - actual)))
    require(counts['leaf_tests'] >= baseline['leaf_tests'], 'BASELINE_LEAF_COUNT_SHRANK')
    return {**counts, 'baseline_identities_preserved': len(wanted),
            'test_identity_sha256': hashlib.sha256('\n'.join(sorted(actual)).encode()).hexdigest()}


def verify_output(code, directory):
    spec = json.loads(BASELINE.read_text())['gates'][code]
    out = Path(directory)
    suites = {}
    for name, baseline in spec['suites'].items():
        rows = [json.loads(line) for line in (out / (name + '.jsonl')).read_text().splitlines()]
        require((out / (name + '.exit')).read_text().strip() == '0', 'SUITE_NONZERO:' + name)
        require((out / (name + '.command')).read_text().strip() == spec['commands'][name], 'COMMAND_SCOPE_DRIFT:' + name)
        suites[name] = verify_suite(rows, baseline)
    return suites



def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='operation', required=True)
    run = sub.add_parser('run')
    run.add_argument('output', type=Path)
    run.add_argument('command', nargs=argparse.REMAINDER)
    sub.add_parser('check')
    args = parser.parse_args()
    if args.operation == 'check':
        from check_ci_commercial import check_sources
        check_sources()
        return 0
    argv = args.command[1:] if args.command[:1] == ['--'] else args.command
    return run_phase(args.output, argv)


if __name__ == '__main__':
    raise SystemExit(main())
