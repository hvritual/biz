"""Count executed Go tests without treating no-test packages as skipped tests."""
from __future__ import annotations
from typing import Any


def summarize(rows: list[dict[str, Any]]) -> dict[str, int]:
    passed = {(r['Package'], r['Test']) for r in rows
              if r.get('Action') == 'pass' and r.get('Test')}
    failures = [r for r in rows if r.get('Action') == 'fail']
    skipped_tests = [r for r in rows if r.get('Action') == 'skip' and r.get('Test')]
    no_test_packages = {r.get('Package') for r in rows
                        if r.get('Action') == 'output' and not r.get('Test')
                        and '[no test files]' in r.get('Output', '')}
    unexplained_skips = [r for r in rows if r.get('Action') == 'skip'
                         and not r.get('Test') and r.get('Package') not in no_test_packages]
    if not passed or failures or skipped_tests or unexplained_skips:
        raise ValueError(f'CE03-TEST-EVIDENCE passed={len(passed)} failures={failures} '
                         f'skipped_tests={skipped_tests} unexplained_package_skips={unexplained_skips}')
    leaves = [(pkg, test) for pkg, test in passed
              if not any(p == pkg and t.startswith(test + '/') for p, t in passed)]
    return {'leaf_tests': len(leaves), 'pass_events': len(passed), 'failed': 0,
            'skipped': 0, 'no_test_packages': len(no_test_packages)}
