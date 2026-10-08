"""Execute bounded browser-preparation command probes; these are not UI evidence."""
import json
import os
from pathlib import Path
import re
import select
import shutil
import signal
import subprocess
import sys
import tempfile
import textwrap
import time
import unittest

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / '.github/workflows/ec-ri-06-plan-change-web.yml'
CE12_WORKFLOW = ROOT / '.github/workflows/ce12-browser-e2e.yml'
APT_SOURCE = ('# Hosted Ubuntu source fixture; preserve all fields except known URI tokens.\n'
              'Types: deb\nURIs: mirror+file:/etc/apt/apt-mirrors.txt\n'
              'Suites: noble noble-updates noble-backports\n'
              'Components: main restricted universe multiverse\nArchitectures: amd64\n'
              'Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\n\n'
              'Types: deb\nURIs: https://security.ubuntu.com/ubuntu/\n'
              'Suites: noble-security\nComponents: main restricted universe multiverse\n'
              'Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\n')


def preparation_step():
    pattern = (r'^      - name: Prepare headless Chromium and Noto CJK\n'
               r'        working-directory: (?P<directory>[^\n]+)\n'
               r'        shell: bash\n'
               r'        run: \|\n'
               r'(?P<run>(?:          [^\n]*\n)+)')
    matches = list(re.finditer(pattern, WORKFLOW.read_text(), re.M))
    if len(matches) != 1:
        raise AssertionError('one executed, fail-closed preparation step is required')
    return {'working-directory': matches[0]['directory'],
            'run': textwrap.dedent(matches[0]['run'])}


def ce12_preparation_step():
    source = CE12_WORKFLOW.read_text()
    start = '      - name: Build CE12 browser runtime binaries\n        shell: bash\n        run: |\n'
    end = '\n      - name: Install browser E2E dependencies\n'
    if source.count(start) != 1 or source.count(end) != 1:
        raise AssertionError('one executed CE12 parallel preparation step is required')
    return textwrap.dedent(source.split(start, 1)[1].split(end, 1)[0])


def recorded_child_alive(root):
    if not (root / 'child-pid').exists():
        return False
    # A nested PID namespace may share an outer /proc mount. The child records
    # its own /proc/self identity; the shell PID is only used for local signals.
    original = (root / 'child-proc-stat').read_text()
    proc_pid = original.split(' ', 1)[0]
    original_fields = original.rsplit(') ', 1)[1].split()
    try:
        current = (Path('/proc') / proc_pid / 'stat').read_text()
    except FileNotFoundError:
        return False
    current_fields = current.rsplit(') ', 1)[1].split()
    return current_fields[19] == original_fields[19] and current_fields[0] != 'Z'


def terminate_recorded_child(root):
    if recorded_child_alive(root):
        try:
            os.kill(int((root / 'child-pid').read_text()), signal.SIGKILL)
        except ProcessLookupError:
            pass


class PlanChangeBrowserPreparationTests(unittest.TestCase):
    def probe(self, missing_font=False, workflow_step=False, layout="root", invalid_web=None,
              apt_source=APT_SOURCE, **overrides):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            web = root / ('biz/web' if layout == 'nested' else 'web')
            web.mkdir(parents=True)
            lock = web / 'package-lock.json'
            lock.write_text('fixture lock; preparation must not change dependencies\n')
            (web / 'package.json').write_text('{"name":"browser-preparation-fixture"}\n')
            browser_cli = web / 'node_modules/.bin/playwright'
            browser_cli.parent.mkdir(parents=True)
            browser_cli.write_text('#!/usr/bin/env bash\nexit 99\n')
            browser_cli.chmod(0o755)
            (root / 'temp').mkdir()
            source_fixture = root / 'ubuntu.sources'
            source_fixture.write_text(apt_source)
            capture = root / 'apt-capture'
            capture.mkdir()
            if layout == 'nested':
                (root / 'biz/scripts').symlink_to(ROOT / 'scripts', target_is_directory=True)
            bindir = root / 'bin'
            bindir.mkdir()
            font = root / 'font'
            if not missing_font:
                font.touch()
            commands = {
                'mktemp': 'printf "mktemp %s\\n" "$*" >> "$TRACE"; '
                          '[[ "$#" == 2 && "$1" == -d && "$2" == /tmp/headless-noto-apt.XXXXXX ]] || exit 86; '
                          'exec "$REAL_MKTEMP" -d "$RUNNER_TEMP/headless-noto-apt.XXXXXX"',
                # Only this public runner-source read is redirected; source conversion runs for real.
                'cat': 'if [[ "$#" == 1 && "$1" == /etc/apt/sources.list.d/ubuntu.sources ]]; then '
                       'printf "read-ubuntu-source\\n" >> "$TRACE"; '
                       '[[ "${APT_SOURCE_MISSING:-0}" == 0 ]] || exit 27; '
                       'if [[ "${APT_HANG_STAGE:-}" == source ]]; then stage-progress source >&2; fi; '
                       'exec "$REAL_CAT" "$APT_SOURCE_FIXTURE"; fi; exec "$REAL_CAT" "$@"',
                'stage-progress': '(record_child_identity; while :; do '
                                  'if [[ "$1" == font-check ]]; then echo "$1-download-progress" >&2; '
                                  'else echo "$1-download-progress"; fi; sleep 0.02; done) & '
                                  'child=$!; echo "$child" > "$CHILD_PID"; wait "$child"',
                'npx': 'printf "npx %s cwd=%s workspace=%s\\n" "$*" "$PWD" "$GITHUB_WORKSPACE" >> "$TRACE"; '
                       'touch "$BROWSER_STARTED"; '
                       'if [[ "${CE12_HANG_STAGE:-}" == os-browser ]]; then stage-progress os-browser; fi; '
                       'if [[ "${CHECK_OVERLAP:-0}" == 1 ]]; then '
                       'for attempt in $(seq 1 80); do [[ -f "$FONT_STARTED" ]] && break; sleep 0.02; done; '
                       '[[ -f "$FONT_STARTED" ]] || exit 57; fi; '
                       'if [[ "${HANG_STAGE:-}" == browser ]]; then '
                       '(record_child_identity; exec sleep 30) & child=$!; echo "$child" > "$CHILD_PID"; wait "$child"; fi; '
                       'exit "${INSTALL_EXIT:-0}"',
                'fc-match': 'if [[ "${CE12_HANG_STAGE:-}" == font-check ]]; then stage-progress font-check; fi; '
                            'if [[ -f "$FONT" ]]; then echo "Noto Sans CJK SC"; else echo "DejaVu Sans"; fi',
                'sudo': 'printf "sudo %s\\n" "$*" >> "$TRACE"; '
                        'if [[ "${1:-}" == rm ]]; then shift; '
                        '[[ "${CLEANUP_EXIT:-0}" == 0 ]] || exit "$CLEANUP_EXIT"; '
                        'exec "$REAL_RM" "$@"; fi; '
                        'if [[ "${1:-}" == -u ]]; then '
                        '[[ "$2" == _apt && "$3" == test && "$4" == -r ]] || exit 85; '
                        '[[ "${APT_PERMISSION_EXIT:-0}" == 0 ]] || exit "$APT_PERMISSION_EXIT"; '
                        'shift 2; "$@"; exit; fi; '
                        '[[ "${1:-}" == apt-get ]] || exit 83; '
                        'phase=install; [[ "$*" != *"update"* ]] || phase=update; '
                        'printf "%s\\0" "$@" > "$APT_CAPTURE/$phase.args"; '
                        'source_list=""; lists=""; archives=""; parts=""; '
                        'for option in "$@"; do case "$option" in '
                        'Dir::Etc::SourceList=*) source_list="${option#*=}" ;; '
                        'Dir::Etc::SourceParts=*) parts="${option#*=}" ;; '
                        'Dir::State::Lists=*) lists="${option#*=}" ;; '
                        'Dir::Cache::Archives=*) archives="${option#*=}" ;; esac; done; '
                        'if [[ -n "$source_list" ]]; then '
                        '"$REAL_CAT" "$source_list" > "$APT_CAPTURE/$phase.sources"; '
                        '[[ -d "$parts" && -z "$(ls -A "$parts")" ]] || exit 84; '
                        'stat -c "%a" "${source_list%/*}" "$parts" "$lists" "$archives" '
                        '> "$APT_CAPTURE/$phase.modes"; '
                        'mkdir -p "$lists/partial" "$archives/partial"; '
                        'touch "$lists/partial/index-fixture" "$archives/partial/package-fixture"; fi; '
                        'if [[ "$*" == *"update"* ]]; then '
                        'touch "$FONT_STARTED"; '
                        'if [[ "${APT_HANG_STAGE:-}" == index ]]; then stage-progress index; fi; '
                        'if [[ "${CHECK_OVERLAP:-0}" == 1 ]]; then '
                        'for attempt in $(seq 1 80); do [[ -f "$BROWSER_STARTED" ]] && break; sleep 0.02; done; '
                        '[[ -f "$BROWSER_STARTED" ]] || exit 58; fi; '
                        'exit "${APT_UPDATE_EXIT:-0}"; fi; '
                        '[[ "${APT_INSTALL_EXIT:-0}" == 0 ]] || exit "$APT_INSTALL_EXIT"; '
                        'if [[ "${HANG_STAGE:-}" == font ]]; then '
                        '(record_child_identity; while :; do echo "font-download-progress"; sleep 0.02; done) & '
                        'child=$!; echo "$child" > "$CHILD_PID"; wait "$child"; fi; '
                        'if [[ "${CE12_HANG_STAGE:-}" == noto ]]; then stage-progress noto; fi; '
                        '[[ "${NO_FONT_AFTER_INSTALL:-0}" == 1 ]] || touch "$FONT"',
                'node': 'cat > "$SMOKE_SOURCE"; printf "node %s cwd=%s\\n" "$*" "$PWD" >> "$TRACE"; '
                        'if [[ "${HANG_STAGE:-}" == launch ]]; then '
                        '(record_child_identity; exec sleep 30) & child=$!; echo "$child" > "$CHILD_PID"; wait "$child"; fi; '
                        'if [[ "${CE12_HANG_STAGE:-}" == launch ]]; then stage-progress launch; fi; '
                        'exit "${SMOKE_EXIT:-0}"',
                # Use GNU timeout itself, shortening only the clock in hang probes.
                'timeout': 'printf "timeout %s\\n" "$*" >> "$TRACE"; '
                           '[[ "$1" == --signal=TERM && "$2" == --kill-after=5s ]] || exit 81; '
                           'duration="$3"; shift 3; [[ "$duration" == 90s || "$duration" == 30s ]] || exit 82; '
                           'if [[ "${ACCELERATE_TIMEOUTS:-0}" == 1 ]]; then '
                           'exec "$REAL_TIMEOUT" --signal=TERM --kill-after=0.1s 0.5s "$@"; fi; '
                           'exec "$REAL_TIMEOUT" --signal=TERM --kill-after=5s "$duration" "$@"',
                'python3': 'if [[ "${1:-}" == - ]]; then exec "$REAL_PYTHON" "$@"; fi; '
                           'printf "python3 %s\\n" "$*" >> "$TRACE"; exit "${GUARD_EXIT:-0}"',
                'npm': 'printf "npm %s cwd=%s\\n" "$*" "$PWD" >> "$TRACE"; echo "npm-fixture-output"; '
                       'touch "$NPM_STARTED"; '
                       'if [[ "${CHECK_BUILD_OVERLAP:-0}" == 1 ]]; then '
                       'for attempt in $(seq 1 80); do [[ -f "$BUILD_STARTED" ]] && break; sleep 0.02; done; '
                       '[[ -f "$BUILD_STARTED" ]] || exit 59; fi; '
                       'if [[ "${CE12_HANG_STAGE:-}" == npm ]]; then stage-progress npm; fi; '
                       'exit "${NPM_EXIT:-0}"',
                'go': 'printf "go %s\\n" "$*" >> "$TRACE"; touch "$BUILD_STARTED"; '
                      'if [[ "${CHECK_BUILD_OVERLAP:-0}" == 1 ]]; then '
                      'for attempt in $(seq 1 80); do [[ -f "$NPM_STARTED" ]] && break; sleep 0.02; done; '
                      '[[ -f "$NPM_STARTED" ]] || exit 60; fi; '
                      'sleep "${BUILD_DELAY_SECONDS:-0}"; exit "${BUILD_EXIT:-0}"',
                'tee': 'if [[ "${TEE_EXIT:-0}" != 0 ]]; then cat > /dev/null; exit "$TEE_EXIT"; fi; '
                       'exec "$REAL_TEE" "$@"',
            }
            child_identity_function = (
                'record_child_identity() {\n'
                '  read -r child_stat < /proc/self/stat\n'
                '  printf "%s\\n" "$child_stat" > "$CHILD_STAT"\n'
                '}\n')
            for name, body in commands.items():
                target = bindir / name
                target.write_text('#!/usr/bin/env bash\nset -euo pipefail\n' + child_identity_function + body + '\n')
                target.chmod(0o755)
            real_timeout = shutil.which('timeout')
            self.assertIsNotNone(real_timeout, 'the CI timeout executable must be available')
            env = {**os.environ, 'PATH': str(bindir) + os.pathsep + os.environ['PATH'],
                   'GITHUB_ACTIONS': 'true', 'GITHUB_WORKSPACE': str(root),
                   'TRACE': str(root / 'trace'), 'FONT': str(font),
                   'BROWSER_STARTED': str(root / 'browser-started'), 'FONT_STARTED': str(root / 'font-started'),
                   'SMOKE_SOURCE': str(root / 'smoke'), 'CHILD_PID': str(root / 'child-pid'),
                   'CHILD_STAT': str(root / 'child-proc-stat'),
                   'REAL_TIMEOUT': real_timeout, 'REAL_TEE': shutil.which('tee'),
                   'REAL_CAT': shutil.which('cat'), 'REAL_RM': shutil.which('rm'),
                   'REAL_MKTEMP': shutil.which('mktemp'),
                   'REAL_PYTHON': sys.executable, 'APT_SOURCE_FIXTURE': str(source_fixture),
                   'APT_CAPTURE': str(capture),
                   'RUNNER_TEMP': str(root / 'temp'), 'BUILD_STARTED': str(root / 'build-started'),
                   'NPM_STARTED': str(root / 'npm-started'), **overrides}
            arguments = [str(web)]
            if invalid_web == 'omitted':
                arguments = []
            elif invalid_web == 'relative':
                arguments = ['web']
            elif invalid_web == 'outside_layout':
                outside = root / 'unowned-web'
                outside.mkdir()
                arguments = [str(outside)]
            elif invalid_web == 'symlink_escape':
                original = root / 'unowned-web'
                web.rename(original)
                web.symlink_to(original, target_is_directory=True)
            elif invalid_web == 'missing_lock':
                lock.unlink()
            elif invalid_web == 'missing_browser':
                browser_cli.unlink()
            command = ['bash', str(ROOT / 'scripts/ci_plan_change_browser.sh'), *arguments]
            cwd = root
            if workflow_step == 'ce12':
                self.assertEqual('nested', layout)
                command = ['bash', '-e', '-o', 'pipefail', '-c', ce12_preparation_step()]
            elif workflow_step:
                step = preparation_step()
                self.assertEqual('.', step['working-directory'])
                command = ['bash', '-e', '-o', 'pipefail', '-c', step['run']]
                cwd = ROOT
            started = time.monotonic()
            process = subprocess.Popen(command, cwd=cwd, env=env, stdout=subprocess.PIPE,
                                       stderr=subprocess.PIPE, text=True, start_new_session=True)
            try:
                if overrides.get('PROBE_SIGNAL'):
                    deadline = time.monotonic() + 2
                    while not (root / 'child-pid').exists() and time.monotonic() < deadline:
                        if process.poll() is not None:
                            break
                        time.sleep(0.01)
                    self.assertTrue((root / 'child-pid').exists(), 'interrupt must reach a running font child')
                    os.kill(process.pid, getattr(signal, 'SIG' + overrides['PROBE_SIGNAL']))
                stdout, stderr = process.communicate(timeout=8)
                result = subprocess.CompletedProcess(command, process.returncode, stdout, stderr)
                child_alive = recorded_child_alive(root)
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGKILL)
                terminate_recorded_child(root)
                if process.poll() is None:
                    process.communicate(timeout=2)
            elapsed = time.monotonic() - started
            trace = (root / 'trace').read_text() if (root / 'trace').exists() else ''
            smoke = (root / 'smoke').read_text() if (root / 'smoke').exists() else ''
            if 'read-ubuntu-source' in trace:
                self.assertIn('mktemp -d /tmp/headless-noto-apt.XXXXXX', trace)
            if invalid_web != 'missing_lock':
                self.assertEqual('fixture lock; preparation must not change dependencies\n', lock.read_text())
            result.workspace = str(root)
            result.web_dir = str(web)
            deps_log = root / 'temp/ce12-browser-deps.log'
            result.deps_log = deps_log.read_text() if deps_log.exists() else ''
            result.deps_timing_exists = (root / 'temp/ce12-browser-deps-timing.log').exists()
            result.build_timing_exists = (root / 'temp/ce12-runtime-build-timing.log').exists()
            result.apt_calls = {}
            for phase in ['update', 'install']:
                arguments_file = capture / (phase + '.args')
                if arguments_file.exists():
                    arguments = arguments_file.read_bytes().decode().rstrip('\0').split('\0')
                    result.apt_calls[phase] = {
                        'arguments': arguments,
                        'options': dict(item.split('=', 1) for item in arguments if '=' in item),
                        'source': (capture / (phase + '.sources')).read_text()
                        if (capture / (phase + '.sources')).exists() else None,
                        'modes': (capture / (phase + '.modes')).read_text().splitlines()
                        if (capture / (phase + '.modes')).exists() else [],
                    }
            self.assertEqual(apt_source, source_fixture.read_text(), 'the runner source must remain unchanged')
            result.apt_temp_remaining = list((root / 'temp').glob('headless-noto-apt.*'))
            return result, trace, smoke, child_alive, elapsed

    def test_ready_noto_uses_locked_shell_and_real_launch_before_success(self):
        result, trace, smoke, _, _ = self.probe()
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertIn('npx --no-install playwright install --only-shell chromium cwd=', trace)
        self.assertNotIn('sudo ', trace)
        self.assertIn('chromium.launch({ headless: true, timeout: 20000 })', smoke)
        self.assertIn('finally', smoke)
        self.assertIn('await browser.close()', smoke)
        self.assertIn('套餐变更', smoke)
        self.assertRegex(result.stdout, r'HEADLESS_NOTO_BROWSER_PREREQUISITES=PASS elapsed_seconds=\d+\n$')

    def test_missing_noto_installs_only_noto_and_overlaps_browser_download(self):
        result, trace, _, _, _ = self.probe(missing_font=True, CHECK_OVERLAP='1')
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertIn('apt-get update -qq', trace)
        self.assertIn('--no-install-recommends', trace)
        self.assertIn('fonts-noto-cjk', trace)
        self.assertNotIn('fonts-wqy', trace)
        self.assertNotIn('--with-deps', trace)

    def test_download_failure_is_returned_after_both_branches_finish(self):
        result, trace, _, _, _ = self.probe(missing_font=True, INSTALL_EXIT='17')
        self.assertEqual(17, result.returncode)
        self.assertIn('fonts-noto-cjk', trace)
        self.assertIn('HEADLESS_NOTO_BROWSER_PREP_RESULT=browser exit=17', result.stdout)
        self.assertIn('HEADLESS_NOTO_BROWSER_PREP_RESULT=font exit=0', result.stdout)
        self.assertNotIn('node ', trace)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_font_index_and_install_failures_each_stay_failure(self):
        for phase, code in [('APT_UPDATE_EXIT', '19'), ('APT_INSTALL_EXIT', '21')]:
            with self.subTest(phase=phase):
                result, trace, _, _, _ = self.probe(missing_font=True, **{phase: code})
                self.assertEqual(int(code), result.returncode)
                self.assertIn('HEADLESS_NOTO_BROWSER_PREP_RESULT=browser exit=0', result.stdout)
                self.assertIn('HEADLESS_NOTO_BROWSER_PREP_RESULT=font exit=' + code, result.stdout)
                self.assertNotIn('node ', trace)
                self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_successful_apt_without_noto_is_rejected(self):
        result, trace, _, _, _ = self.probe(missing_font=True, NO_FONT_AFTER_INSTALL='1')
        self.assertNotEqual(0, result.returncode)
        self.assertIn('HEADLESS_NOTO_FONT_UNAVAILABLE', result.stderr)
        self.assertNotIn('node ', trace)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_both_installation_failures_remain_visible(self):
        result, _, _, _, _ = self.probe(missing_font=True, INSTALL_EXIT='17', APT_INSTALL_EXIT='21')
        self.assertEqual(17, result.returncode)
        self.assertIn('HEADLESS_NOTO_BROWSER_PREP_RESULT=browser exit=17', result.stdout)
        self.assertIn('HEADLESS_NOTO_BROWSER_PREP_RESULT=font exit=21', result.stdout)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_launch_failure_cannot_emit_success(self):
        result, _, _, _, _ = self.probe(SMOKE_EXIT='23')
        self.assertEqual(23, result.returncode)
        self.assertIn('HEADLESS_NOTO_BROWSER_PREP_RESULT=launch exit=23', result.stdout)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_blocked_download_font_and_launch_are_terminated(self):
        for stage in ['browser', 'font', 'launch']:
            with self.subTest(stage=stage):
                result, trace, _, child_alive, elapsed = self.probe(
                    missing_font=stage == 'font', HANG_STAGE=stage, ACCELERATE_TIMEOUTS='1')
                self.assertEqual(124, result.returncode, result.stderr)
                self.assertIn('HEADLESS_NOTO_BROWSER_PREP_RESULT=' + stage + ' exit=124', result.stdout)
                self.assertFalse(child_alive, 'timed-out preparation must not leave its command running')
                self.assertLess(elapsed, 4)
                self.assertNotIn('PREREQUISITES=PASS', result.stdout)
                if stage != 'launch':
                    self.assertNotIn('node ', trace)
                if stage == 'font':
                    self.assertIn('fonts-noto-cjk', trace)
                    self.assertIn('font-download-progress', result.stdout)

    def test_non_ci_call_stops_before_any_preparation(self):
        result, trace, _, _, _ = self.probe(GITHUB_ACTIONS='false')
        self.assertEqual(2, result.returncode)
        self.assertIn('CI_OWNED_BROWSER_ONLY', result.stderr)
        self.assertEqual('', trace)

    def test_workflow_executes_guard_and_helper_from_repository_root(self):
        result, trace, _, _, _ = self.probe(workflow_step=True)
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertIn('python3 -B scripts/test_ci_plan_change_browser.py', trace)
        self.assertIn('npx --no-install playwright install --only-shell chromium', trace)

    def test_workflow_guard_failure_prevents_installation(self):
        result, trace, _, _, _ = self.probe(workflow_step=True, GUARD_EXIT='31')
        self.assertEqual(31, result.returncode)
        self.assertNotIn('npx ', trace)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_workflow_preserves_lifecycle_screenshots_and_existing_budgets(self):
        source = WORKFLOW.read_text()
        self.assertIn('    timeout-minutes: 25\n', source)
        self.assertIn('    defaults:\n      run:\n        working-directory: web\n', source)
        self.assertIn('      - run: npm ci\n', source)
        self.assertIn('      - name: Check frontend contracts\n        run: npm run check\n', source)
        lifecycle = ("      - name: Run tenant plan change lifecycle E2E\n"
                     "        env:\n          ENTERPRISE_PLAN_CHANGE_E2E: '1'\n"
                     "        run: |\n          set -euo pipefail\n"
                     "          VITE_DATA_MODE=api npm run build\n"
                     "          npx playwright test e2e/enterprise-plan-change-real.spec.ts\n")
        self.assertIn(lifecycle, source)
        self.assertLess(source.index('name: Check frontend contracts'),
                        source.index('name: Prepare headless Chromium and Noto CJK'))
        self.assertLess(source.index('name: Prepare headless Chromium and Noto CJK'), source.index(lifecycle))
        screenshots = source.split('      - name: Verify screenshot evidence\n', 1)[1].split(
            '      - name: Upload lifecycle screenshots\n', 1)[0]
        for state in ['targets', 'preview', 'external-approval', 'applied', 'scheduled', 'provisioning']:
            self.assertIn("'enterprise-plan-change-" + state + "-1440'", screenshots)
        self.assertIn("assert data[:8] == b'\\x89PNG\\r\\n\\x1a\\n'", screenshots)
        self.assertIn("assert struct.unpack('>II', data[16:24]) == (1440, 900)", screenshots)
        upload = source.split('      - name: Upload lifecycle screenshots\n', 1)[1]
        self.assertIn('          name: ec-ri-06-plan-change-screenshots\n', upload)
        self.assertIn('          path: web/screenshots/enterprise-plan-change-*.png\n', upload)
        self.assertIn('          retention-days: 14\n', upload)
        self.assertIn('          if-no-files-found: error\n', upload)
        self.assertNotIn('continue-on-error:', WORKFLOW.read_text())
        gate = json.loads((ROOT / 'scripts/ci_proof_contract.json').read_text())['gates'][WORKFLOW.name]
        self.assertEqual((150, 240), (gate['target_seconds'], gate['hard_seconds']))
        self.assertEqual(['repository.web.fast'], gate['delegates'])


    def test_child_identity_detects_live_process_and_refuses_mismatched_starttime(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            identity = root / 'child-proc-stat'
            command = ('read -r child_stat < /proc/self/stat; '
                       'printf "%s\\n" "$child_stat" > "$CHILD_STAT"; '
                       'printf "READY\\n"; exec sleep 30')
            child = subprocess.Popen(
                ['bash', '-euo', 'pipefail', '-c', command],
                env={**os.environ, 'CHILD_STAT': str(identity)},
                stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
            try:
                (root / 'child-pid').write_text(str(child.pid))
                self.assertTrue(select.select([child.stdout], [], [], 2)[0], 'child must record its own identity')
                self.assertEqual('READY\n', child.stdout.readline())
                self.assertTrue(recorded_child_alive(root))
                original = identity.read_text()
                identity.unlink()
                with self.assertRaises(FileNotFoundError):
                    recorded_child_alive(root)
                prefix, fields = original.rsplit(') ', 1)
                changed = fields.split()
                changed[19] = str(int(changed[19]) + 1)
                identity.write_text(prefix + ') ' + ' '.join(changed) + '\n')
                self.assertFalse(recorded_child_alive(root))
                terminate_recorded_child(root)
                with self.assertRaises(subprocess.TimeoutExpired):
                    child.wait(timeout=0.05)
                identity.write_text(original)
                self.assertTrue(recorded_child_alive(root))
                terminate_recorded_child(root)
                child.wait(timeout=2)
                self.assertFalse(recorded_child_alive(root))
            finally:
                if child.poll() is None:
                    child.kill()
                    child.wait(timeout=2)
                child.stdout.close()
                child.stderr.close()

    def test_explicit_nested_web_layout_keeps_workspace_and_locked_directory(self):
        result, trace, _, _, _ = self.probe(layout='nested')
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertIn('cwd=' + result.web_dir + ' workspace=' + result.workspace, trace)
        self.assertTrue(result.web_dir.endswith('/biz/web'))

    def test_invalid_or_unprepared_explicit_paths_are_rejected_before_setup(self):
        for case in ['omitted', 'relative', 'outside_layout', 'symlink_escape', 'missing_lock', 'missing_browser']:
            with self.subTest(case=case):
                result, trace, _, _, _ = self.probe(invalid_web=case)
                self.assertEqual(2, result.returncode, result.stderr)
                self.assertEqual('', trace)
                self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_ce12_production_step_overlaps_go_and_npm_then_keeps_dependency_logs(self):
        result, trace, smoke, _, _ = self.probe(
            layout='nested', workflow_step='ce12', missing_font=True, CHECK_BUILD_OVERLAP='1')
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertIn('python3 -B biz/scripts/test_ci_plan_change_browser.py', trace)
        self.assertIn('go -C biz build -tags=qualification', trace)
        self.assertIn('go -C biz build -o', trace)
        npm = 'npm ci cwd=' + result.web_dir
        browser = ('npx playwright install --with-deps --only-shell chromium cwd=' +
                   result.web_dir + ' workspace=' + result.workspace)
        noto = 'sudo apt-get install -y fonts-noto-cjk'
        launch = 'node --input-type=module cwd=' + result.web_dir
        for command in [npm, browser, noto, launch]:
            self.assertIn(command, trace)
        self.assertLess(trace.index(npm), trace.index(browser))
        self.assertLess(trace.index(browser), trace.index(noto))
        self.assertLess(trace.index(noto), trace.index(launch))
        self.assertEqual(1, len([line for line in trace.splitlines() if line.startswith('npx ')]))
        self.assertEqual(
            ['timeout --signal=TERM --kill-after=5s 90s bash -euo pipefail'],
            [line for line in trace.splitlines() if line.startswith('timeout ')])
        self.assertIn('chromium.launch({ headless: true, timeout: 20000 })', smoke)
        self.assertIn('finally', smoke)
        self.assertIn('await browser.close()', smoke)
        self.assertIn('npm-fixture-output', result.deps_log)
        for stage in ['npm', 'os-browser', 'noto', 'font-check', 'launch']:
            self.assertIn('CE12_BROWSER_PREP_STAGE=' + stage, result.deps_log)
        self.assertIn('CE12_BROWSER_PREREQUISITES=PASS', result.deps_log)
        self.assertIn('CE12_PARALLEL_PREP_SECONDS=', result.stdout)
        self.assertTrue(result.deps_timing_exists)
        self.assertTrue(result.build_timing_exists)

    def test_ce12_npm_failure_does_not_start_browser_installation(self):
        result, trace, _, _, _ = self.probe(
            layout='nested', workflow_step='ce12', NPM_EXIT='19', BUILD_DELAY_SECONDS='0.05')
        self.assertEqual(1, result.returncode)
        self.assertIn('CE12 preparation failed: build=0 browser_deps=19', result.stderr)
        self.assertIn('npm-fixture-output', result.deps_log)
        self.assertNotIn('npx ', trace)
        self.assertNotIn('CE12_BROWSER_PREP_STAGE=os-browser', result.deps_log)
        self.assertNotIn('CE12_PARALLEL_PREP_SECONDS=', result.stdout)
        self.assertFalse(result.deps_timing_exists)
        self.assertTrue(result.build_timing_exists)

    def test_ce12_canonical_install_failure_is_not_hidden_by_tee(self):
        result, trace, _, _, _ = self.probe(layout='nested', workflow_step='ce12', INSTALL_EXIT='17')
        self.assertEqual(1, result.returncode)
        self.assertIn('CE12 preparation failed: build=0 browser_deps=17', result.stderr)
        self.assertIn('CE12_BROWSER_PREP_STAGE=os-browser', result.deps_log)
        self.assertNotIn('sudo ', trace)
        self.assertNotIn('node ', trace)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)
        self.assertNotIn('CE12_PARALLEL_PREP_SECONDS=', result.stdout)
        self.assertFalse(result.deps_timing_exists)

    def test_ce12_noto_readiness_and_launch_failures_remain_failures(self):
        cases = [('APT_INSTALL_EXIT', '21', 21),
                 ('NO_FONT_AFTER_INSTALL', '1', 1),
                 ('SMOKE_EXIT', '23', 23)]
        for variable, value, code in cases:
            with self.subTest(variable=variable):
                result, trace, _, _, _ = self.probe(
                    layout='nested', workflow_step='ce12', missing_font=True, **{variable: value})
                self.assertEqual(1, result.returncode)
                self.assertIn('CE12 preparation failed: build=0 browser_deps=' + str(code), result.stderr)
                self.assertIn('sudo apt-get install -y fonts-noto-cjk', trace)
                if variable != 'SMOKE_EXIT':
                    self.assertNotIn('node ', trace)
                if variable == 'NO_FONT_AFTER_INSTALL':
                    self.assertIn('CE12_NOTO_CJK_UNAVAILABLE', result.deps_log)
                self.assertNotIn('PREREQUISITES=PASS', result.stdout)
                self.assertNotIn('CE12_PARALLEL_PREP_SECONDS=', result.stdout)
                self.assertFalse(result.deps_timing_exists)

    def test_ce12_go_failure_is_checked_after_browser_branch_finishes(self):
        result, _, _, _, _ = self.probe(layout='nested', workflow_step='ce12', BUILD_EXIT='23')
        self.assertEqual(1, result.returncode)
        self.assertIn('CE12 preparation failed: build=23 browser_deps=0', result.stderr)
        self.assertIn('CE12_BROWSER_PREREQUISITES=PASS', result.deps_log)
        self.assertTrue(result.deps_timing_exists)
        self.assertNotIn('CE12_PARALLEL_PREP_SECONDS=', result.stdout)

    def test_ce12_log_writer_failure_stays_failure(self):
        result, _, _, _, _ = self.probe(layout='nested', workflow_step='ce12', TEE_EXIT='29')
        self.assertEqual(1, result.returncode)
        self.assertIn('CE12 preparation failed: build=0 browser_deps=29', result.stderr)
        self.assertNotIn('CE12_PARALLEL_PREP_SECONDS=', result.stdout)
        self.assertFalse(result.deps_timing_exists)

    def test_ce12_complete_phase_deadline_stops_progress_without_killing_go_or_tee(self):
        for stage in ['npm', 'os-browser', 'noto', 'font-check', 'launch']:
            with self.subTest(stage=stage):
                result, trace, _, child_alive, elapsed = self.probe(
                    layout='nested', workflow_step='ce12', missing_font=True,
                    CE12_HANG_STAGE=stage, ACCELERATE_TIMEOUTS='1', BUILD_DELAY_SECONDS='0.3')
                self.assertEqual(1, result.returncode, result.stderr)
                self.assertIn('CE12 preparation failed: build=0 browser_deps=124', result.stderr)
                self.assertIn(stage + '-download-progress', result.deps_log)
                self.assertFalse(child_alive, 'the dependency deadline must terminate its descendants')
                self.assertFalse(result.deps_timing_exists)
                self.assertTrue(result.build_timing_exists, 'the deadline must not kill the parallel Go build')
                self.assertIn('CE12_RUNTIME_BUILD_SECONDS=', result.stdout)
                self.assertNotIn('PREREQUISITES=PASS', result.stdout)
                self.assertNotIn('CE12_PARALLEL_PREP_SECONDS=', result.stdout)
                self.assertLess(elapsed, 4)
                self.assertEqual(
                    ['timeout --signal=TERM --kill-after=5s 90s bash -euo pipefail'],
                    [line for line in trace.splitlines() if line.startswith('timeout ')])
                if stage == 'npm':
                    self.assertNotIn('npx ', trace)
                if stage in ['npm', 'os-browser']:
                    self.assertNotIn('sudo ', trace)
                if stage != 'launch':
                    self.assertNotIn('node ', trace)

    def test_ce12_guard_failure_prevents_both_preparation_branches(self):
        result, trace, _, _, _ = self.probe(layout='nested', workflow_step='ce12', GUARD_EXIT='31')
        self.assertEqual(31, result.returncode)
        self.assertNotIn('go ', trace)
        self.assertNotIn('npm ', trace)
        self.assertNotIn('npx ', trace)

    def test_ce12_preserves_trust_chain_and_existing_budgets(self):
        source = CE12_WORKFLOW.read_text()
        self.assertIn('    timeout-minutes: 4\n', source)
        # The canonical CE12 topology requires full OS dependencies; the earlier
        # CE12-only prohibition conflicted with that independently owned contract.
        self.assertIn('npx playwright install --with-deps --only-shell chromium', source)
        self.assertNotIn('continue-on-error:', source)
        self.assertIn('go -C biz test -count=1 -tags=integration ./integration -run', source)
        self.assertIn("'^TestCE12BrowserSeed$'", source)
        self.assertIn('test "${#core_specs[@]}" -gt 0', source)
        for name in ['ce12-core-results', 'ce12-aux-results', 'ce12-189-authorization-results',
                     'ce12-189-notification-results', 'ce12-189-rate-limit-results']:
            self.assertIn('test-results/' + name + '.json', source)
        for field in ['unexpected', 'flaky', 'skipped']:
            self.assertIn("assert sum(item.get('" + field + "', 0) for item in stats) == 0", source)
        self.assertIn("assert sum(item.get('expected', 0) for item in stats) >= len(required)", source)
        self.assertIn("for title in required:", source)
        self.assertIn('VITE_DATA_MODE=api', source)
        self.assertIn('CE12_BROWSER_AUTHORITY_CHAIN=PASS', source)
        self.assertIn('CE12_THEME_APPEARANCE_AUTHORITY=PASS', source)
        self.assertIn('CE12_ENTERPRISE189_FULL_FLOW=PASS', source)
        gate = json.loads((ROOT / 'scripts/ci_proof_contract.json').read_text())['gates'][CE12_WORKFLOW.name]
        self.assertEqual((150, 180), (gate['target_seconds'], gate['hard_seconds']))


    def test_noto_source_conversion_preserves_trust_and_uses_one_private_https_configuration(self):
        for uri in ['mirror+file:/etc/apt/apt-mirrors.txt',
                    'http://azure.archive.ubuntu.com/ubuntu/',
                    'http://azure.archive.ubuntu.com/ubuntu',
                    'https://archive.ubuntu.com/ubuntu/']:
            with self.subTest(uri=uri):
                source = APT_SOURCE.replace('mirror+file:/etc/apt/apt-mirrors.txt', uri)
                result, trace, _, _, _ = self.probe(missing_font=True, apt_source=source)
                self.assertEqual(0, result.returncode, result.stderr)
                self.assertEqual({'update', 'install'}, set(result.apt_calls))
                expected = source.replace(uri, 'https://archive.ubuntu.com/ubuntu/')
                update, install = result.apt_calls['update'], result.apt_calls['install']
                self.assertEqual(expected, update['source'])
                self.assertEqual(expected, install['source'])
                self.assertEqual(update['options'], install['options'])
                options = update['options']
                self.assertEqual('', options['Dir::Cache::pkgcache'])
                self.assertEqual('', options['Dir::Cache::srcpkgcache'])
                for name in ['Dir::Etc::SourceList', 'Dir::Etc::SourceParts',
                             'Dir::State::Lists', 'Dir::Cache::Archives']:
                    self.assertTrue(options[name].startswith(result.workspace + '/temp/headless-noto-apt.'))
                self.assertTrue(options['Dir::Etc::SourceList'].endswith('/ubuntu.sources'))
                self.assertEqual(['755'] * 4, update['modes'], '_apt must traverse private public-data directories')
                self.assertEqual(['755'] * 4, install['modes'])
                for forbidden in ['RootDir', 'Dir::State::status', 'Dir::Etc::Trusted',
                                  'APT::Sandbox::User', 'Acquire::https::Verify-Peer',
                                  'Acquire::https::Verify-Host', 'Acquire::AllowInsecureRepositories']:
                    self.assertNotIn(forbidden, options)
                self.assertNotIn('--allow-unauthenticated', trace)
                self.assertEqual([], result.apt_temp_remaining)

    def test_noto_unknown_or_missing_source_is_rejected_before_apt(self):
        cases = [({'APT_SOURCE_MISSING': '1'}, APT_SOURCE),
                 ({}, APT_SOURCE.replace('mirror+file:/etc/apt/apt-mirrors.txt', 'https://unknown.invalid/ubuntu/')),
                 ({}, APT_SOURCE.replace('Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\n', '')),
                 ({}, APT_SOURCE.replace('Suites: noble noble-updates noble-backports\n', '')),
                 ({}, APT_SOURCE.replace('URIs: mirror+file:/etc/apt/apt-mirrors.txt',
                                         'URIs: mirror+file:/etc/apt/apt-mirrors.txt\n https://unknown.invalid/ubuntu/')),
                 ({}, APT_SOURCE.replace('URIs: mirror+file:/etc/apt/apt-mirrors.txt',
                                         'URIs: mirror+file:/etc/apt/apt-mirrors.txt\n'
                                         '# interrupted continuation\n https://unknown.invalid/ubuntu/')),
                 ({}, APT_SOURCE.replace('URIs: mirror+file:/etc/apt/apt-mirrors.txt',
                                         'URIs: mirror+file:/etc/apt/apt-mirrors.txt\n'
                                         'URIs: https://unknown.invalid/ubuntu/'))]
        for overrides, source in cases:
            with self.subTest(overrides=overrides, source=source):
                result, trace, _, _, _ = self.probe(missing_font=True, apt_source=source, **overrides)
                self.assertNotEqual(0, result.returncode)
                self.assertEqual({}, result.apt_calls)
                self.assertNotIn('node ', trace)
                self.assertNotIn('PREREQUISITES=PASS', result.stdout)
                self.assertEqual([], result.apt_temp_remaining)

    def test_ready_noto_does_not_read_or_prepare_apt_sources(self):
        result, trace, _, _, _ = self.probe(APT_SOURCE_MISSING='1', CLEANUP_EXIT='29')
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertNotIn('read-ubuntu-source', trace)
        self.assertNotIn('sudo ', trace)
        self.assertEqual({}, result.apt_calls)
        self.assertEqual([], result.apt_temp_remaining)

    def test_noto_unreadable_to_apt_sandbox_is_rejected_before_index_or_install(self):
        result, trace, _, _, _ = self.probe(missing_font=True, APT_PERMISSION_EXIT='33')
        self.assertEqual(33, result.returncode, result.stderr)
        self.assertIn('sudo -u _apt test -r ', trace)
        self.assertEqual({}, result.apt_calls)
        self.assertEqual([], result.apt_temp_remaining)
        self.assertNotIn('node ', trace)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_noto_cleanup_failure_cannot_pass_or_replace_an_apt_error(self):
        for apt_code, expected in [('0', 29), ('21', 21)]:
            with self.subTest(apt_code=apt_code):
                result, trace, _, _, _ = self.probe(
                    missing_font=True, APT_INSTALL_EXIT=apt_code, CLEANUP_EXIT='29')
                self.assertEqual(expected, result.returncode, result.stderr)
                self.assertIn('HEADLESS_NOTO_APT_CLEANUP_FAILED exit=29', result.stderr)
                self.assertNotIn('node ', trace)
                self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_noto_source_index_and_install_share_deadline_and_remove_private_data(self):
        for stage in ['source', 'index', 'install']:
            with self.subTest(stage=stage):
                overrides = {'HANG_STAGE': 'font'} if stage == 'install' else {'APT_HANG_STAGE': stage}
                result, trace, _, child_alive, elapsed = self.probe(
                    missing_font=True, ACCELERATE_TIMEOUTS='1', **overrides)
                self.assertEqual(124, result.returncode, result.stderr)
                self.assertIn('HEADLESS_NOTO_BROWSER_PREP_RESULT=font exit=124', result.stdout)
                self.assertIn(('font' if stage == 'install' else stage) + '-download-progress',
                              result.stdout + result.stderr)
                self.assertFalse(child_alive)
                self.assertEqual([], result.apt_temp_remaining)
                self.assertNotIn('node ', trace)
                self.assertNotIn('PREREQUISITES=PASS', result.stdout)
                self.assertLess(elapsed, 4)

    def test_noto_term_and_int_preserve_failure_and_clean_private_data_and_children(self):
        for name, expected in [('TERM', 143), ('INT', 130)]:
            with self.subTest(name=name):
                result, trace, _, child_alive, elapsed = self.probe(
                    missing_font=True, HANG_STAGE='font', PROBE_SIGNAL=name)
                self.assertEqual(expected, result.returncode, result.stderr)
                self.assertFalse(child_alive)
                self.assertEqual([], result.apt_temp_remaining)
                self.assertNotIn('node ', trace)
                self.assertNotIn('PREREQUISITES=PASS', result.stdout)
                self.assertLess(elapsed, 4)


if __name__ == '__main__':
    unittest.main()
