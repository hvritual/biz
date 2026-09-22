#!/usr/bin/env python3
"""Read-only static checks for the CE04-07 migration; never run by domain suites."""
import json
from pathlib import Path
import shlex

ROOT = Path(__file__).resolve().parents[1]

def require(ok, message):
    if not ok:
        raise ValueError(message)

def check_sources(root=ROOT):
    """Check unchanged literal test argv, serial DB work, hooks and bootstrap roles."""
    baseline = json.loads((root / 'scripts/ci_commercial_baseline.json').read_text())
    for code, spec in baseline['gates'].items():
        text = (root / f'scripts/{code}_qualify.sh').read_text()
        commands = []
        for line in text.splitlines():
            if line.startswith('run_suite '):
                tokens = shlex.split(line)
                commands.append((tokens[1], shlex.join(tokens[2:])))
        require(dict(commands) == spec['commands'] and len(commands) == len(spec['commands']), 'RETAINED_COMMANDS_DRIFT:' + code)
        require(' &\n' not in text, 'SHARED_DATABASE_PARALLELISM:' + code)
        require('verify_output(' in text, 'BASELINE_EVIDENCE_HOOK_MISSING:' + code)
        require('ci_commercial_runtime.py' in text, 'PHASE_RUNNER_MISSING:' + code)
        workflow = (root / f'.github/workflows/{code}-qualification.yml').read_text()
        require('services:' not in workflow and 'cache: true' in workflow, 'BOOTSTRAP_REGRESSION:' + code)
        require('github.event.pull_request.head.sha || github.sha' in workflow, 'EXACT_CANDIDATE_MISSING:' + code)
        require('ci_commercial_mysql.sh stop' in workflow and 'if: always()' in workflow, 'CLEANUP_MISSING:' + code)
        require('timeout-minutes: 5' in workflow, 'JOB_TIMEOUT_DRIFT:' + code)
        if code != 'ce05':
            require('docker restart "$MYSQL_CONTAINER_ID"' in text, 'PHYSICAL_RESTART_MISSING:' + code)
    proof = json.loads((root / 'scripts/ci_proof_contract.json').read_text())
    expected_hard = {'ce04': 150, 'ce05': 180, 'ce06': 180, 'ce07': 180}
    expected_debt = {'generation.check': 2, 'generation.generate': 1}
    for code, hard in expected_hard.items():
        gate = proof['gates'][f'{code}-qualification.yml']
        require(gate['target_seconds'] == 120 and gate['hard_seconds'] == hard,
                'PERFORMANCE_RATCHET_DRIFT:' + code)
        require(gate['legacy_cost_ceiling'] == expected_debt,
                'LEGACY_DEBT_RATCHET_DRIFT:' + code)
        workflow = (root / f'.github/workflows/{code}-qualification.yml').read_text()
        require(f'performance_hard_seconds:\n        type: number\n        required: false\n        default: {hard}' in workflow,
                'WORKFLOW_HARD_BUDGET_DRIFT:' + code)
    prq = (root / '.github/workflows/pr-qualification.yml').read_text()
    for code, hard in expected_hard.items():
        block = prq.split(f'  commercial-batch-{code}:', 1)[1].split('\n  ', 1)[0]
        require(f'performance_hard_seconds: {hard}' in block,
                'TARGETED_HARD_BUDGET_DRIFT:' + code)
    helper = (root / 'scripts/ci_commercial_mysql.sh').read_text()
    require('--tmpfs' not in helper, 'DURABLE_DB_REQUIRED')
    require('GITHUB_ACTIONS' in helper and 'ci.batch-a' in helper, 'OWNED_DB_GUARD_MISSING')
    prq = (root / '.github/workflows/pr-qualification.yml').read_text()
    require('ci_commercial_runtime.py check' in prq and 'test_ci_commercial_runtime.py' in prq, 'GOVERNANCE_HOOK_MISSING')
    print('CE_BATCH_A_SOURCE_CONTRACT=PASS')


if __name__ == "__main__":
    check_sources()
