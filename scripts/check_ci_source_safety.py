#!/usr/bin/env python3
"""Validate workflow source before execution; protect the Access runner from business patches."""
from __future__ import annotations
import argparse
import os
from pathlib import Path
import re
import subprocess
import sys
import yaml

PROTECTED = (
    '.github/workflows/b12-multitenant-access-pressure.yml',
    '.github/workflows/pr-qualification.yml',
    'scripts/check_ci_source_safety.py', 'scripts/test_ci_source_safety.py',
    'scripts/ci_access_qualification.py', 'scripts/test_ci_access_qualification.py',
    'scripts/ci_safety_requirements.txt',
    'scripts/literal_patch.py', 'scripts/test_literal_patch.py',
    'scripts/delivery_progress.py', 'scripts/test_delivery_progress.py',
)
REGISTRY = 'scripts/ci_access_tests.json'


class StrictLoader(yaml.SafeLoader):
    pass


def strict_mapping(loader, node, deep=False):
    out = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        if key in out:
            raise ValueError('DUPLICATE_YAML_KEY:' + str(key))
        out[key] = loader.construct_object(value_node, deep=deep)
    return out


StrictLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, strict_mapping)


def validate_workflow(text, name='workflow'):
    doc = yaml.load(text, Loader=StrictLoader)
    if not isinstance(doc, dict) or not isinstance(doc.get('jobs'), dict):
        raise ValueError('WORKFLOW_JOBS_INVALID:' + name)
    count = 0
    for job_id, job in doc['jobs'].items():
        for index, step in enumerate(job.get('steps', [])):
            if 'run' not in step:
                continue
            script = step['run']
            if not isinstance(script, str) or not script.strip():
                raise ValueError('EMPTY_RUN:' + name + ':' + str(index))
            shell = step.get('shell') or job.get('defaults', {}).get('run', {}).get('shell') or doc.get('defaults', {}).get('run', {}).get('shell') or 'bash'
            if shell not in {'bash', 'sh'}:
                raise ValueError('UNVALIDATED_SHELL:' + str(shell))
            # GitHub expressions are substitutions, not shell syntax. The actual
            # executable bash (including heredocs) must still parse independently.
            rendered = re.sub(r'\$\{\{.*?\}\}', 'CI_EXPRESSION', script, flags=re.S)
            result = subprocess.run([shell, '-n'], input=rendered, text=True, capture_output=True, timeout=10)
            if result.returncode:
                raise ValueError(f'SHELL_SYNTAX:{name}:{job_id}:{index}: {result.stderr.strip()}')
            count += 1
    return count


def original(root, ref, path):
    result = subprocess.run(['git', 'show', f'{ref}:{path}'], cwd=root, capture_output=True, timeout=20)
    return result.stdout if result.returncode == 0 else None


def check(root, base_ref='', branch=''):
    root = root.resolve()
    if base_ref and not branch.startswith('chore/ci-proof-'):
        for path in PROTECTED:
            before = original(root, base_ref, path)
            if before is not None and (not (root / path).is_file() or (root / path).is_symlink() or (root / path).read_bytes() != before):
                raise ValueError('CI_CONTROL_REQUIRES_GOVERNANCE:' + path)
    # Check source, not exit codes from a workflow that may never have parsed.
    count = 0
    for path in sorted((root / '.github/workflows').glob('*.yml')):
        if path.is_symlink():
            raise ValueError('WORKFLOW_SYMLINK:' + str(path))
        count += validate_workflow(path.read_text(), path.name)
    sys.path.insert(0, str(root / 'scripts'))
    from ci_access_qualification import load_json, validate
    data = load_json((root / REGISTRY).read_bytes())
    raw = original(root, base_ref, REGISTRY) if base_ref else None
    validate(data, root, load_json(raw) if raw is not None else None)
    workflow = (root / PROTECTED[0]).read_text()
    if workflow.count('python3 biz/scripts/ci_access_qualification.py') != 1 or "-run '^TestEnterprise" in workflow:
        raise ValueError('ACCESS_RUNNER_BINDING_DRIFT')
    print(f'CI_SOURCE_SAFETY=PASS bash_blocks={count}')
    return count


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path.cwd())
    parser.add_argument('--base-ref', default='')
    args = parser.parse_args()
    try:
        check(args.root, args.base_ref, os.getenv('GITHUB_HEAD_REF', ''))
    except Exception as error:
        print('CI_SOURCE_SAFETY=BLOCKED ' + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
