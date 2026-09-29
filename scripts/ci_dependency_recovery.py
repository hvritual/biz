#!/usr/bin/env python3
"""Recover locked dependency transport inside one job, never rerun qualification.

Only the fixed Go download command is executable. The current candidate's go.sum
supplies versions and checksums; source policy on frozen main authorizes a second
transport attempt. This does not authorize a second GitHub workflow attempt.
"""
from __future__ import annotations

from datetime import datetime, timezone
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time
import zipfile

from delivery_execution import API, Blocked, require

ROOT = Path(__file__).resolve().parents[1]
POLICY = 'scripts/ci_dependency_recovery.json'
RECEIPT = 'dependency-recovery.json'
SHA = r'[0-9a-f]{40}'
SUM = r'h1:[A-Za-z0-9+/]{43}='


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def strict_json(raw):
    def pairs(items):
        value = {}
        for k, v in items:
            require(k not in value, 'RECOVERY_DUPLICATE_JSON_KEY', k)
            value[k] = v
        return value
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda v: (_ for _ in ()).throw(Blocked('RECOVERY_NONFINITE_JSON')))


def policy(root=ROOT):
    value = strict_json((root / POLICY).read_bytes())
    expected = {
        'schema_version': 1, 'strategy': 'same_job_locked_download',
        'workflow': '.github/workflows/ce03-qualification.yml', 'job': 'qualify',
        'step': 'Prepare locked Go dependencies', 'artifact_prefix': 'ci-dependency-ce03',
        'max_attempts': 2, 'window_seconds': 90, 'process_seconds': 60,
        'max_output_bytes': 2097152, 'proxy': 'https://proxy.golang.org',
        'lock_files': ['go.mod', 'go.sum', 'go.work', 'go.work.sum', '.yunka/source.env'],
        'authorization_sources': [POLICY, 'scripts/ci_dependency_recovery.py',
                                  '.github/workflows/ce03-qualification.yml']}
    require(json.dumps(value, sort_keys=True) == json.dumps(expected, sort_keys=True), 'RECOVERY_POLICY_INVALID')
    return value


def source_hashes(root, spec):
    paths = spec['lock_files'] + spec['authorization_sources']
    out = {}
    for name in paths:
        path = root / name
        require(path.is_file() and not path.is_symlink() and
                path.resolve().is_relative_to(root.resolve()), 'RECOVERY_SOURCE_MISSING_OR_ESCAPED', name)
        out[name] = digest(path.read_bytes())
    return out


def locked_modules(raw):
    modules = {}
    for line in raw.splitlines():
        parts = line.split()
        require(len(parts) == 3 and re.fullmatch(SUM, parts[2]), 'RECOVERY_INVALID_GO_SUM')
        path, version, checksum = parts
        require(re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._~!+/-]+', path) and '..' not in path.split('/') and
                re.fullmatch(r'v[0-9][A-Za-z0-9.+-]*(?:/go.mod)?', version), 'RECOVERY_INVALID_MODULE')
        if version.endswith('/go.mod'):
            continue
        key = path + '@' + version
        require(key not in modules, 'RECOVERY_DUPLICATE_MODULE', key)
        modules[key] = checksum
    require(modules and len(modules) <= 1024, 'RECOVERY_MODULE_SET_INVALID')
    return dict(sorted(modules.items()))


def go_escape(text):
    return re.sub(r'[A-Z]', lambda m: '!' + m.group().lower(), text)


def classify(error, module, spec):
    """A diagnostic in arbitrary workflow output is never recovery authority."""
    path, version = module.rsplit('@', 1)
    endpoint = spec['proxy'] + '/' + go_escape(path) + '/@v/' + go_escape(version) + '.zip'
    prefix = '(?:' + re.escape(module) + ': )?(?:read|Get) "' + re.escape(endpoint) + '": '
    if re.fullmatch(prefix + r'stream error: stream ID [1-9][0-9]*; INTERNAL_ERROR; received from peer', error):
        return 'GO_PROXY_HTTP2_INTERNAL_ERROR'
    if re.fullmatch(prefix + r'(?:502 Bad Gateway|503 Service Unavailable|504 Gateway Timeout)', error):
        return 'GO_PROXY_TRANSIENT_HTTP'
    raise Blocked('RECOVERY_NON_TRANSPORT_FAILURE', {'module': module})


def download_results(raw, stderr, code, requested, modules, spec):
    text = raw.decode('utf-8')
    decoder, at, records = json.JSONDecoder(), 0, []
    while at < len(text):
        if text[at].isspace():
            at += 1
            continue
        item, end = decoder.raw_decode(text, at)
        records.append(strict_json(text[at:end]))
        at = end
    require(records, 'RECOVERY_DOWNLOAD_RESULT_MISSING')
    seen, failed = set(), {}
    for item in records:
        require(isinstance(item, dict), 'RECOVERY_DOWNLOAD_RESULT_INVALID')
        key = str(item.get('Path')) + '@' + str(item.get('Version'))
        require(key in requested and key in modules and key not in seen,
                'RECOVERY_UNEXPECTED_MODULE', key)
        seen.add(key)
        if item.get('Error'):
            failed[key] = classify(item['Error'], key, spec)
        else:
            require(item.get('Sum') == modules[key], 'RECOVERY_CHECKSUM_MISMATCH', key)
    require(seen == set(requested), 'RECOVERY_MODULE_RESULT_SET_MISMATCH')
    require(type(code) is int and ((code == 0 and not failed) or (code != 0 and failed)), 'RECOVERY_PROCESS_RESULT_MISMATCH')
    # Native Go diagnostics are retained; none can turn a checksum/build/test
    # failure into a recoverable download merely by containing a proxy URL.
    require(not re.search(rb'(?i)checksum mismatch|SECURITY ERROR|--- FAIL:|build failed|updates to go.mod', stderr),
            'RECOVERY_NON_TRANSPORT_STDERR')
    return failed


def git(root, *args):
    return subprocess.check_output(['git', *args], cwd=root, text=True, timeout=20).strip()


def authorize_retry(root, spec, binding):
    """A candidate cannot grant itself recovery by editing policy or a workflow."""
    for name in spec['authorization_sources']:
        old = subprocess.run(['git', 'show', binding['frozen_main_sha'] + ':' + name],
                             cwd=root, capture_output=True, timeout=20)
        require(old.returncode == 0 and old.stdout == (root / name).read_bytes(),
                'RECOVERY_NOT_AUTHORIZED_BY_FROZEN_MAIN', name)


def live_binding(api, spec, root, environment):
    run_id, attempt = int(environment['GITHUB_RUN_ID']), int(environment['GITHUB_RUN_ATTEMPT'])
    require(attempt == 1, 'RECOVERY_CANNOT_AUTHORIZE_WORKFLOW_RERUN')
    event = strict_json(Path(environment['GITHUB_EVENT_PATH']).read_bytes())
    run = api.get('/actions/runs/' + str(run_id))
    repository = environment['GITHUB_REPOSITORY']
    require(run.get('repository', {}).get('full_name') == repository and run.get('run_attempt') == attempt,
            'RECOVERY_RUN_BINDING_MISMATCH')
    candidate = run['head_sha']
    require(re.fullmatch(SHA, candidate), 'RECOVERY_CANDIDATE_INVALID')
    main = api.get('/git/ref/heads/main')['object']['sha']
    pull = event.get('pull_request')
    if pull:
        pr = event['number']
        current = api.get('/pulls/' + str(pr))
        require(run.get('event') == 'pull_request' and current.get('state') == 'open' and
                current['base']['ref'] == 'main' and current['head']['sha'] == candidate and
                pull['head']['sha'] == candidate and pull['base']['sha'] == main and
                current['base']['sha'] == main, 'RECOVERY_PR_OR_BASE_CHANGED')
    else:
        pr = None
        require(run.get('event') == 'workflow_dispatch' and run.get('path') == spec['workflow'],
                'RECOVERY_CALLER_NOT_SUPPORTED')
    checkout = git(root, 'rev-parse', 'HEAD')
    tree = git(root, 'rev-parse', 'HEAD^{tree}')
    require(checkout == environment['GITHUB_SHA'] and
            api.get('/git/commits/' + candidate)['tree']['sha'] == tree, 'RECOVERY_CHECKOUT_MISMATCH')
    require(subprocess.run(['git', 'merge-base', '--is-ancestor', main, candidate], cwd=root,
                           capture_output=True, timeout=20).returncode == 0, 'RECOVERY_STALE_MAIN')
    jobs = api.jobs({'id': run_id, 'run_attempt': attempt})
    matches = [(j, s) for j in jobs for s in j.get('steps', []) if s.get('name') == spec['step'] and
               s.get('status') == 'in_progress' and j.get('status') == 'in_progress']
    require(len(matches) == 1, 'RECOVERY_JOB_STEP_NOT_UNIQUE')
    job, step = matches[0]
    require(job.get('head_sha') == candidate and job.get('run_id') == run_id and
            job.get('run_attempt') == attempt and environment['GITHUB_JOB'] == spec['job'],
            'RECOVERY_JOB_BINDING_MISMATCH')
    return {'repository': repository, 'pr_number': pr, 'candidate_sha': candidate,
            'candidate_tree': tree, 'frozen_main_sha': main, 'checkout_sha': checkout,
            'run_id': str(run_id), 'run_attempt': attempt, 'job_id': job['id'],
            'job_name': job['name'], 'step_name': spec['step'], 'step_number': step['number']}


def run_go(root, requested, spec, timeout):
    # Do not accept shell commands, user-selected URLs or newer module versions.
    # Tokens are never passed to the child process or retained in evidence.
    keep = ('PATH', 'HOME', 'GOPATH', 'GOCACHE', 'GOMODCACHE', 'GOROOT', 'TMPDIR', 'SSL_CERT_FILE', 'SSL_CERT_DIR')
    env = {k: os.environ[k] for k in keep if k in os.environ}
    env.update(GOWORK='off', GOTOOLCHAIN='local', GOPROXY=spec['proxy'], GOSUMDB='sum.golang.org',
               GOFLAGS='', GONOPROXY='', GONOSUMDB='', GOPRIVATE='')
    try:
        result = subprocess.run(['go', 'mod', 'download', '-json', *requested], cwd=root, env=env,
                                capture_output=True, timeout=timeout)
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired as exc:
        return 124, exc.output or b'', exc.stderr or b''


def prepare(root, output, spec, binding_reader, executor=run_go, clock=time.monotonic):
    require(not output.exists(), 'RECOVERY_OUTPUT_ALREADY_EXISTS')
    output.mkdir(parents=True)
    started = clock()
    report = {'schema_version': 1, 'state': 'BLOCKED', 'business_certification': 'NOT_PERFORMED',
              'strategy': spec['strategy'], 'attempts': []}
    try:
        bound = binding_reader()
        hashes = source_hashes(root, spec)
        modules = locked_modules((root / 'go.sum').read_text())
        requested = list(modules)
        report.update(binding=bound, source_sha256=hashes, policy_sha256=digest((root / POLICY).read_bytes()))
        for ordinal in range(1, spec['max_attempts'] + 1):
            require(clock() - started < spec['window_seconds'], 'RECOVERY_LEASE_EXPIRED')
            require(binding_reader() == bound, 'RECOVERY_BINDING_CHANGED')
            require(source_hashes(root, spec) == hashes, 'RECOVERY_SOURCE_CHANGED')
            if ordinal > 1:
                authorize_retry(root, spec, bound)
            stamp = datetime.now(timezone.utc).isoformat()
            process_started = clock()
            code, stdout, stderr = executor(root, requested, spec,
                min(spec['process_seconds'], spec['window_seconds'] - (clock() - started)))
            stdout_name, stderr_name = f'download-{ordinal}.json', f'download-{ordinal}.stderr'
            require(len(stdout) <= spec['max_output_bytes'] and len(stderr) <= spec['max_output_bytes'],
                    'RECOVERY_OUTPUT_TOO_LARGE')
            (output / stdout_name).write_bytes(stdout)
            (output / stderr_name).write_bytes(stderr)
            row = {'ordinal': ordinal, 'requested': requested, 'exit_code': code,
                   'started_at': stamp, 'completed_at': datetime.now(timezone.utc).isoformat(),
                   'process_seconds': round(clock() - process_started, 6),
                   'stdout': stdout_name, 'stderr': stderr_name,
                   'stdout_sha256': digest(stdout), 'stderr_sha256': digest(stderr)}
            report['attempts'].append(row)
            require(source_hashes(root, spec) == hashes, 'RECOVERY_SOURCE_CHANGED')
            require(code != 124, 'RECOVERY_PROCESS_TIMEOUT')
            failed = download_results(stdout, stderr, code, requested, modules, spec)
            row['transport_failures'] = failed
            if not failed:
                require(binding_reader() == bound, 'RECOVERY_BINDING_CHANGED')
                require(clock() - started <= spec['window_seconds'], 'RECOVERY_LEASE_EXPIRED')
                report['state'] = 'RECOVERED' if ordinal > 1 else 'READY'
                return report
            requested = sorted(failed)
        raise Blocked('RECOVERY_TRANSPORT_BUDGET_EXHAUSTED')
    except Blocked as exc:
        report.update(reason=exc.code, evidence=exc.evidence)
        raise
    except Exception as exc:
        report.update(reason='RECOVERY_PREPARATION_ERROR', evidence=type(exc).__name__)
        raise
    finally:
        report['total_seconds'] = round(clock() - started, 6)
        (output / RECEIPT).write_text(json.dumps(report, indent=2, sort_keys=True) + '\n')
        print('DEPENDENCY_PREPARATION=' + report['state'], flush=True)


def verify_bundle(data, spec, root, expected, job):
    """Shared by PR and main proof auditors; replay evidence rather than a reason."""
    limit = spec['max_output_bytes']
    require(len(data) <= 6 * limit, 'RECOVERY_ARCHIVE_TOO_LARGE')
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        names = archive.namelist()
        require(len(names) == len(set(names)) and RECEIPT in names and
                sum(f.file_size for f in archive.infolist()) <= 6 * limit, 'RECOVERY_ARCHIVE_INVALID')
        raw = {name: archive.read(name) for name in names}
    report = strict_json(raw[RECEIPT])
    require(report.get('schema_version') == 1 and report.get('strategy') == spec['strategy'] and
            report.get('business_certification') == 'NOT_PERFORMED', 'RECOVERY_RECEIPT_SCHEMA_INVALID')
    require(json.dumps(report.get('binding'), sort_keys=True) == json.dumps(expected, sort_keys=True),
            'RECOVERY_RECEIPT_BINDING_MISMATCH')
    require(report.get('source_sha256') == source_hashes(root, spec) and
            report.get('policy_sha256') == digest((root / POLICY).read_bytes()), 'RECOVERY_SOURCE_PROOF_MISMATCH')
    total = report.get('total_seconds')
    require(type(total) in {int, float} and 0 <= total <= spec['window_seconds'], 'RECOVERY_LEASE_EXPIRED')
    attempts = report.get('attempts', [])
    require(1 <= len(attempts) <= spec['max_attempts'], 'RECOVERY_ATTEMPTS_INVALID')
    require(report.get('state') == ('READY' if len(attempts) == 1 else 'RECOVERED'), 'RECOVERY_NOT_READY')
    steps = [s for s in job.get('steps', []) if s.get('name') == spec['step']]
    require(len(steps) == 1 and steps[0]['number'] == expected['step_number'] and
            steps[0].get('conclusion') == 'success' and steps[0].get('status') == 'completed',
            'RECOVERY_STEP_NOT_SUCCESS')
    stamp = lambda text: datetime.fromisoformat(text.replace('Z', '+00:00')).timestamp()
    lower, upper = stamp(steps[0]['started_at']), stamp(steps[0]['completed_at'])
    modules, allowed_names = locked_modules((root / 'go.sum').read_text()), {RECEIPT}
    requested, previous_end = list(modules), lower - 1
    for ordinal, row in enumerate(attempts, 1):
        require(row.get('ordinal') == ordinal and row.get('requested') == requested,
                'RECOVERY_RETRY_SCOPE_MISMATCH')
        stdout, stderr = f'download-{ordinal}.json', f'download-{ordinal}.stderr'
        allowed_names.update([stdout, stderr])
        require(row.get('stdout') == stdout and row.get('stderr') == stderr and stdout in raw and stderr in raw,
                'RECOVERY_LOG_MISSING')
        require(len(raw[stdout]) <= limit and len(raw[stderr]) <= limit and
                row.get('stdout_sha256') == digest(raw[stdout]) and row.get('stderr_sha256') == digest(raw[stderr]),
                'RECOVERY_LOG_DIGEST_MISMATCH')
        a, b = stamp(row['started_at']), stamp(row['completed_at'])
        require(previous_end <= a <= b <= upper + 1 and a >= lower - 1 and b - a <= spec['process_seconds'] + 1,
                'RECOVERY_TIME_BINDING_MISMATCH')
        previous_end = b
        failed = download_results(raw[stdout], raw[stderr], row['exit_code'], requested, modules, spec)
        require(row.get('transport_failures') == failed, 'RECOVERY_FAILURE_CLASSIFICATION_MISMATCH')
        require(bool(failed) == (ordinal < len(attempts)), 'RECOVERY_RESULT_SEQUENCE_INVALID')
        requested = sorted(failed)
    require(stamp(attempts[-1]['completed_at']) - stamp(attempts[0]['started_at']) <= spec['window_seconds'],
            'RECOVERY_LEASE_EXPIRED')
    require(set(names) == allowed_names, 'RECOVERY_UNEXPECTED_ARCHIVE_CONTENT')
    if len(attempts) > 1:
        authorize_retry(root, spec, expected)
    return {'artifact_state': report['state'], 'binding': expected, 'source_sha256': report['source_sha256'],
            'attempts': len(attempts), 'total_seconds': total}


def verify_run(api, root, repository, candidate, tree, pr, run, frozen_main):
    spec = policy(root)
    require(run['run_attempt'] == 1, 'RECOVERY_CANNOT_AUTHORIZE_WORKFLOW_RERUN')
    jobs = [j for j in api.jobs(run) if j.get('name') == 'full-08-ce03-qualification / qualify']
    require(len(jobs) == 1 and jobs[0].get('head_sha') == candidate and jobs[0].get('run_id') == run['id'] and
            jobs[0].get('run_attempt') == 1 and jobs[0].get('conclusion') == 'success', 'RECOVERY_JOB_NOT_QUALIFIED')
    job = jobs[0]
    steps = [s for s in job.get('steps', []) if s.get('name') == spec['step']]
    require(len(steps) == 1, 'RECOVERY_STEP_MISSING')
    name = f"{spec['artifact_prefix']}-{run['id']}-{run['run_attempt']}"
    artifacts = [a for a in api.pages(f"/actions/runs/{run['id']}/artifacts", 'artifacts') if a.get('name') == name]
    require(len(artifacts) == 1 and not artifacts[0].get('expired'), 'RECOVERY_ARTIFACT_MISSING_OR_EXPIRED')
    artifact = artifacts[0]
    data = api.raw(api.prefix + f"/actions/artifacts/{artifact['id']}/zip", cap=6 * spec['max_output_bytes'])
    require(artifact.get('digest') == 'sha256:' + digest(data), 'RECOVERY_ARTIFACT_DIGEST_MISMATCH')
    # CE03 intentionally checks out the synthetic merge ref; it must be a pinned
    # reference reported by GitHub and have exactly the candidate's source tree.
    refs = {r['sha'] for r in run.get('referenced_workflows', [])}
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        require(archive.getinfo(RECEIPT).file_size <= spec['max_output_bytes'], 'RECOVERY_RECEIPT_TOO_LARGE')
        checkout = strict_json(archive.read(RECEIPT)).get('binding', {}).get('checkout_sha')
    require(checkout in refs and api.get('/git/commits/' + checkout)['tree']['sha'] == tree,
            'RECOVERY_WORKFLOW_SOURCE_MISMATCH')
    expected = {'repository': repository, 'pr_number': pr, 'candidate_sha': candidate,
                'candidate_tree': tree, 'frozen_main_sha': frozen_main, 'checkout_sha': checkout,
                'run_id': str(run['id']), 'run_attempt': run['run_attempt'], 'job_id': job['id'],
                'job_name': job['name'], 'step_name': spec['step'], 'step_number': steps[0]['number']}
    result = verify_bundle(data, spec, root, expected, job)
    return {**result, 'artifact_id': artifact['id'], 'archive_sha256': digest(data)}


class PreparationAPI(API):
    def __init__(self, repository, token, limits, window):
        super().__init__(repository, token, dict(limits))
        self.deadline = time.monotonic() + window
        self.limits['transport_attempts'] = 1

    def raw(self, path, cap=8 * 1024 * 1024):
        remaining = self.deadline - time.monotonic()
        require(remaining > 0, 'RECOVERY_LEASE_EXPIRED')
        self.limits['request_seconds'] = min(5, remaining)
        result = super().raw(path, cap)
        require(time.monotonic() <= self.deadline, 'RECOVERY_LEASE_EXPIRED')
        return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    try:
        spec = policy()
        limits = strict_json((ROOT / 'scripts/delivery_execution_contract.json').read_bytes())['limits']
        api = PreparationAPI(os.getenv('GITHUB_REPOSITORY'), os.getenv('GH_TOKEN') or os.getenv('GITHUB_TOKEN'), limits, spec['window_seconds'])
        prepare(ROOT, args.output, spec, lambda: live_binding(api, spec, ROOT, os.environ))
        return 0
    except Exception as exc:
        print('DEPENDENCY_PREPARATION_BLOCKED=' + str(exc), file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
