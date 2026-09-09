#!/usr/bin/env python3
"""Reject an incomplete CE delivery. This gate checks evidence structure and Git
ancestry; it does not replace execution or independently authenticate CI logs.
"""
from __future__ import annotations
import argparse
import json
import re
import subprocess
import sys
from pathlib import Path


def check_completion(task: dict, proof: dict, repo: Path, main_ref: str) -> list[str]:
    errors = []
    if task.get('status') != 'DONE':
        errors.append('task must be DONE, not merely submitted or VERIFYING')
    sha = task.get('integration_commit') or ''
    if not re.fullmatch(r'[0-9a-f]{40}', sha):
        errors.append('full integration commit required')
    if proof.get('task_id') != task.get('id'):
        errors.append('proof task_id mismatch')
    if proof.get('verified_commit') != sha or proof.get('result') != 'PASS':
        errors.append('PASS proof must target the integrated commit')
    checks = proof.get('checks')
    if not isinstance(checks, list) or not checks:
        errors.append('actual command results required')
        checks = []
    tests = 0
    for check in checks:
        if not isinstance(check, dict):
            errors.append('invalid check result')
            continue
        if not check.get('command') or check.get('exit_code') != 0 or not check.get('evidence'):
            errors.append('every command must have a zero exit code and evidence locator')
        count = check.get('tests_passed', 0)
        if isinstance(count, bool) or not isinstance(count, int) or count < 0:
            errors.append('invalid nonnegative test count')
        else:
            tests += count
    if tests == 0:
        errors.append('zero executed tests cannot complete a round')
    if re.fullmatch(r'[0-9a-f]{40}', sha):
        for target in ('HEAD', main_ref):
            result = subprocess.run(['git', '-C', str(repo), 'merge-base', '--is-ancestor', sha, target], capture_output=True, text=True, timeout=30)
            if result.returncode != 0:
                errors.append(f'integration commit is not an ancestor of {target}')
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--task', required=True)
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument('--main-ref', default='origin/main')
    parser.add_argument('--require-main', action='store_true')
    args = parser.parse_args()
    root = args.root.resolve()
    repo = root.parents[1]
    try:
        from check_plan import validate
        errors, _ = validate(root)
        plan = json.loads((root / 'tasks.json').read_text(encoding='utf-8'))
        matches = [task for task in plan['tasks'] if task['id'] == args.task]
        if len(matches) != 1:
            raise ValueError('exactly one known task required')
        task = matches[0]
        locator = task.get('verification')
        if not isinstance(locator, str) or not locator:
            raise ValueError('task verification file required')
        path = (root / locator).resolve()
        if not path.is_relative_to(root):
            raise ValueError('verification path escapes the plan')
        proof = json.loads(path.read_text(encoding='utf-8'))
        errors.extend(check_completion(task, proof, repo, args.main_ref))
        if args.require_main:
            def revision(ref: str) -> str:
                return subprocess.check_output(['git', '-C', str(repo), 'rev-parse', '--verify', ref], text=True, timeout=30).strip()
            if revision('HEAD') != revision(args.main_ref):
                errors.append('HEAD is not the fetched main head')
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as exc:
        print(f'ROUND_CHECK=FAIL: {exc}', file=sys.stderr)
        return 1
    for error in errors:
        print(f'ERROR: {error}', file=sys.stderr)
    print(f"ROUND_CHECK={'FAIL' if errors else 'PASS'} task={args.task}")
    return 1 if errors else 0


if __name__ == '__main__':
    raise SystemExit(main())
