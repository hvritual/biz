#!/usr/bin/env python3
"""Serialize fixture groups in one existing local database; back up and restore it.

No containers or databases are created. Stop application access while this
explicit fixture-reset verification runs. Physical DB restart certification
remains in the existing task-specific qualification workflows.
"""
import fcntl
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time

root = Path(__file__).resolve().parents[1]
container = os.environ['EVOLUTION_MYSQL_CONTAINER']
password = os.environ['EVOLUTION_MYSQL_PASSWORD']
dsn = os.environ['YUNKA_TEST_MYSQL_DSN']
if os.environ.get('YUNKA_TEST_RESET_FIXTURES') != '1':
    raise SystemExit('Explicit YUNKA_TEST_RESET_FIXTURES=1 required; stop application access first')
if '@tcp(127.0.0.1:' not in dsn or ')/' not in dsn:
    raise SystemExit('Existing loopback MySQL DSN required')
database = dsn.split(')/', 1)[1].split('?', 1)[0]
port = dsn.split('@tcp(127.0.0.1:', 1)[1].split(')', 1)[0]
ci = os.environ.get('GITHUB_ACTIONS') == 'true'
if not ci and (container, port, database) != ('biz-evolution-mysql-20260912', '13316', 'biz_evolution'):
    raise SystemExit('Local verification must use the one configured Docker instance and database')
ports = json.loads(subprocess.check_output(['docker', 'inspect', '--format', '{{json .NetworkSettings.Ports}}', container], text=True))
if not any(p['HostPort'] == port and (ci or p['HostIp'] == '127.0.0.1') for p in ports.get('3306/tcp', []) or []):
    raise SystemExit('Container port mapping does not match the test DSN')
if not re.fullmatch(r'[A-Za-z0-9_]+', database):
    raise SystemExit('Invalid database name')
output = Path(os.environ.get('EVOLUTION_EVIDENCE_DIR', tempfile.mkdtemp(prefix='biz-evolution-'))).resolve()
output.mkdir(parents=True, exist_ok=True)
admin_env = dict(os.environ, MYSQL_PWD=password)
admin = ['docker', 'exec', '-e', 'MYSQL_PWD', container, 'mysql', '-uroot', '-N', '-B', database]


def reset_fixtures():
    raw = subprocess.check_output(admin + ['-e', "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'"], env=admin_env, text=True)
    names = [name for name in raw.splitlines() if name.startswith('biz_') and re.fullmatch(r'[A-Za-z0-9_]+', name)]
    sql = 'SET FOREIGN_KEY_CHECKS=0;' + ''.join(f'DROP TABLE `{name}`;' for name in names) + 'SET FOREIGN_KEY_CHECKS=1;'
    subprocess.run(admin + ['-e', sql], env=admin_env, check=True, capture_output=True)


common = Path(subprocess.check_output(['git', 'rev-parse', '--git-common-dir'], cwd=root, text=True).strip())
if not common.is_absolute():
    common = (root / common).resolve()
backup_dir = common / 'local-db-backups'
backup_dir.mkdir(exist_ok=True)
lock = (backup_dir / 'verification.lock').open('a')
try:
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
except BlockingIOError:
    raise SystemExit('Another shared-database verification is running')
marker = backup_dir / 'recovery-required.json'
if marker.exists():
    raise SystemExit('Restore the preserved database backup before retrying; see ' + str(marker))
clients = int(subprocess.check_output(admin + ['-e', 'SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE DB=DATABASE() AND ID<>CONNECTION_ID()'], env=admin_env, text=True).strip())
if clients:
    raise SystemExit('Stop application/database clients before fixture verification')
backup = backup_dir / ('database-before-tests-' + str(time.time_ns()) + '.sql')
with backup.open('xb') as stream:
    backup.chmod(0o600)
    subprocess.run(['docker', 'exec', '-e', 'MYSQL_PWD', container, 'mysqldump', '-uroot', '--single-transaction', '--no-tablespaces', '--set-gtid-purged=OFF', database], env=admin_env, stdout=stream, check=True)
with marker.open('x') as stream:
    marker.chmod(0o600)
    json.dump({'container': container, 'database': database, 'backup': str(backup)}, stream)
results = []
selected = set(filter(None, os.environ.get('EVOLUTION_GROUPS', '').split(',')))
after_reset_command = os.environ.get('EVOLUTION_AFTER_RESET_COMMAND_JSON', '')
after_reset_result = os.environ.get('EVOLUTION_AFTER_RESET_RESULT_JSON', '')
try:
    # Browser acceptance has to keep its seeded fixture alive while the IdP,
    # BFF and Chromium run. A JSON argv (rather than a shell fragment) keeps
    # the same backup/reset/restore envelope without introducing another local
    # database workflow.
    if after_reset_command:
        if selected:
            raise SystemExit('EVOLUTION_AFTER_RESET_COMMAND_JSON cannot be combined with EVOLUTION_GROUPS')
        if not after_reset_result:
            raise SystemExit('EVOLUTION_AFTER_RESET_RESULT_JSON must name the Playwright JSON result produced by the command')
        try:
            command = json.loads(after_reset_command)
        except json.JSONDecodeError as exc:
            raise SystemExit('EVOLUTION_AFTER_RESET_COMMAND_JSON must be a JSON argv array') from exc
        if not isinstance(command, list) or not command or not all(isinstance(part, str) and part for part in command):
            raise SystemExit('EVOLUTION_AFTER_RESET_COMMAND_JSON must be a non-empty JSON argv array')
        reset_fixtures()
        log = output / 'after-reset-command.log'
        started = time.monotonic()
        with log.open('w') as stream:
            result = subprocess.run(command, cwd=root, env=dict(os.environ), stdout=stream, stderr=subprocess.STDOUT)
        report = Path(after_reset_result).resolve()
        try:
            stats = json.loads(report.read_text())['stats']
            expected = int(stats.get('expected', 0))
            unexpected = int(stats.get('unexpected', 0))
            skipped = int(stats.get('skipped', 0))
            flaky = int(stats.get('flaky', 0))
        except (OSError, ValueError, TypeError, KeyError, json.JSONDecodeError) as exc:
            raise SystemExit('after-reset command did not produce a readable Playwright JSON result: ' + str(report)) from exc
        passed = expected if result.returncode == 0 and expected > 0 and not unexpected and not skipped and not flaky else 0
        entry = {'group': 'after-reset-command', 'package': '.', 'database': database, 'expected_tests': expected, 'passed': passed, 'skipped': skipped, 'flaky': flaky, 'unexpected': unexpected, 'command_exit_code': result.returncode, 'exit_code': 0 if passed == expected and expected else 1, 'seconds': round(time.monotonic() - started, 2), 'log': str(log), 'playwright_result': str(report)}
        results.append(entry)
        print(json.dumps(entry), flush=True)
        (output / 'summary.json').write_text(json.dumps(results, indent=2) + '\n')
    else:
        for package, label in [('./integration', 'business'), ('./integration/b13delegation', 'delegation')]:
            binary = output / (label + '.test')
            subprocess.run(['go', 'test', '-c', '-tags=integration', '-o', str(binary), package], cwd=root, check=True)
            names = subprocess.check_output([str(binary), '-test.list=^Test'], cwd=root, text=True).splitlines()
            names = [name for name in names if name.startswith('Test')]
            if not names:
                raise SystemExit('Empty integration test inventory: ' + package)
            groups = {}
            for name in names:
                match = re.match(r'Test(CE\d+)', name)
                group = match[1] if match else label
                if 'PersistenceBeforeRestart' in name or 'PersistenceAfterRestart' in name:
                    group += '-restart'
                if name.endswith('Seed'):
                    group = name
                groups.setdefault(group, []).append(name)
            for group, tests in groups.items():
                if selected and group not in selected:
                    continue
                reset_fixtures()
                env = dict(os.environ)
                for key in ['CE08_RESTART_RECEIPT', 'CE09_RESTART_RECEIPT', 'CE10_RESTART_RECEIPT', 'CE12_E2E_ENV_FILE', 'CE13_PLATFORM_E2E_ENV_FILE']:
                    env[key] = str(output / (group + '-' + key.lower() + '.json'))
                log = output / (group + '.log')
                started = time.monotonic()
                commands = []
                if group.endswith('-restart'):
                    # A process restart is part of the persistence assertion:
                    # run the two helpers in distinct OS processes while
                    # intentionally retaining the same reset fixture state.
                    commands = [[str(binary), '-test.v', '-test.count=1', '-test.timeout=15m', '-test.run=^' + test + '$'] for test in tests]
                else:
                    commands = [[str(binary), '-test.v', '-test.count=1', '-test.timeout=15m', '-test.run=^(' + '|'.join(tests) + ')$']]
                exit_code = 0
                with log.open('w') as stream:
                    for command in commands:
                        result = subprocess.run(command, cwd=root / package.removeprefix('./'), env=env, stdout=stream, stderr=subprocess.STDOUT)
                        exit_code = exit_code or result.returncode
                text = log.read_text()
                passed = len(re.findall(r'^--- PASS:', text, re.M))
                skipped = len(re.findall(r'^--- SKIP:', text, re.M))
                entry = {'group': group, 'package': package, 'database': database, 'expected_tests': len(tests), 'passed': passed, 'skipped': skipped, 'exit_code': exit_code, 'seconds': round(time.monotonic() - started, 2), 'log': str(log)}
                results.append(entry)
                print(json.dumps(entry), flush=True)
                (output / 'summary.json').write_text(json.dumps(results, indent=2) + '\n')
finally:
    reset_fixtures()
    with backup.open('rb') as stream:
        subprocess.run(['docker', 'exec', '-i', '-e', 'MYSQL_PWD', container, 'mysql', '-uroot', database], env=admin_env, stdin=stream, check=True, stdout=subprocess.DEVNULL)
    marker.unlink()
    print('Original database contents restored: ' + database, flush=True)
if not results or (selected and selected != {r['group'] for r in results}) or any(r['exit_code'] or r['skipped'] or r['passed'] != r['expected_tests'] for r in results):
    raise SystemExit(1)
