"""Execute bounded browser-preparation command probes; these are not UI evidence."""
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import tempfile
import textwrap
import time
import unittest

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / '.github/workflows/ec-ri-06-plan-change-web.yml'


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


class PlanChangeBrowserPreparationTests(unittest.TestCase):
    def probe(self, missing_font=False, workflow_step=False, **overrides):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'web').mkdir()
            lock = root / 'web/package-lock.json'
            lock.write_text('fixture lock; preparation must not change dependencies\n')
            bindir = root / 'bin'
            bindir.mkdir()
            font = root / 'font'
            if not missing_font:
                font.touch()
            commands = {
                'npx': 'printf "npx %s cwd=%s\\n" "$*" "$PWD" >> "$TRACE"; '
                       'touch "$BROWSER_STARTED"; '
                       'if [[ "${CHECK_OVERLAP:-0}" == 1 ]]; then '
                       'for attempt in $(seq 1 80); do [[ -f "$FONT_STARTED" ]] && break; sleep 0.02; done; '
                       '[[ -f "$FONT_STARTED" ]] || exit 57; fi; '
                       'if [[ "${HANG_STAGE:-}" == browser ]]; then sleep 30 & child=$!; echo "$child" > "$CHILD_PID"; wait "$child"; fi; '
                       'exit "${INSTALL_EXIT:-0}"',
                'fc-match': 'if [[ -f "$FONT" ]]; then echo "Noto Sans CJK SC"; else echo "DejaVu Sans"; fi',
                'sudo': 'printf "sudo %s\\n" "$*" >> "$TRACE"; '
                        'if [[ "$*" == *"update"* ]]; then '
                        'touch "$FONT_STARTED"; '
                        'if [[ "${CHECK_OVERLAP:-0}" == 1 ]]; then '
                        'for attempt in $(seq 1 80); do [[ -f "$BROWSER_STARTED" ]] && break; sleep 0.02; done; '
                        '[[ -f "$BROWSER_STARTED" ]] || exit 58; fi; '
                        'exit "${APT_UPDATE_EXIT:-0}"; fi; '
                        '[[ "${APT_INSTALL_EXIT:-0}" == 0 ]] || exit "$APT_INSTALL_EXIT"; '
                        'if [[ "${HANG_STAGE:-}" == font ]]; then '
                        '(while :; do echo "font-download-progress"; sleep 0.02; done) & '
                        'child=$!; echo "$child" > "$CHILD_PID"; wait "$child"; fi; '
                        '[[ "${NO_FONT_AFTER_INSTALL:-0}" == 1 ]] || touch "$FONT"',
                'node': 'cat > "$SMOKE_SOURCE"; printf "node %s cwd=%s\\n" "$*" "$PWD" >> "$TRACE"; '
                        'if [[ "${HANG_STAGE:-}" == launch ]]; then sleep 30 & child=$!; echo "$child" > "$CHILD_PID"; wait "$child"; fi; '
                        'exit "${SMOKE_EXIT:-0}"',
                # Use GNU timeout itself, shortening only the clock in hang probes.
                'timeout': 'printf "timeout %s\\n" "$*" >> "$TRACE"; '
                           '[[ "$1" == --signal=TERM && "$2" == --kill-after=5s ]] || exit 81; '
                           'duration="$3"; shift 3; [[ "$duration" == 90s || "$duration" == 30s ]] || exit 82; '
                           'if [[ "${ACCELERATE_TIMEOUTS:-0}" == 1 ]]; then '
                           'exec "$REAL_TIMEOUT" --signal=TERM --kill-after=0.1s 0.5s "$@"; fi; '
                           'exec "$REAL_TIMEOUT" --signal=TERM --kill-after=5s "$duration" "$@"',
                'python3': 'printf "python3 %s\\n" "$*" >> "$TRACE"; exit "${GUARD_EXIT:-0}"',
            }
            for name, body in commands.items():
                target = bindir / name
                target.write_text('#!/usr/bin/env bash\nset -euo pipefail\n' + body + '\n')
                target.chmod(0o755)
            real_timeout = shutil.which('timeout')
            self.assertIsNotNone(real_timeout, 'the CI timeout executable must be available')
            env = {**os.environ, 'PATH': str(bindir) + os.pathsep + os.environ['PATH'],
                   'GITHUB_ACTIONS': 'true', 'GITHUB_WORKSPACE': str(root),
                   'TRACE': str(root / 'trace'), 'FONT': str(font),
                   'BROWSER_STARTED': str(root / 'browser-started'), 'FONT_STARTED': str(root / 'font-started'),
                   'SMOKE_SOURCE': str(root / 'smoke'), 'CHILD_PID': str(root / 'child-pid'),
                   'REAL_TIMEOUT': real_timeout, **overrides}
            command = ['bash', str(ROOT / 'scripts/ci_plan_change_browser.sh')]
            cwd = root
            if workflow_step:
                step = preparation_step()
                self.assertEqual('.', step['working-directory'])
                command = ['bash', '-e', '-o', 'pipefail', '-c', step['run']]
                cwd = ROOT
            started = time.monotonic()
            process = subprocess.Popen(command, cwd=cwd, env=env, stdout=subprocess.PIPE,
                                       stderr=subprocess.PIPE, text=True, start_new_session=True)
            try:
                stdout, stderr = process.communicate(timeout=8)
                result = subprocess.CompletedProcess(command, process.returncode, stdout, stderr)
                child_alive = False
                if (root / 'child-pid').exists():
                    child = int((root / 'child-pid').read_text())
                    stat = Path('/proc') / str(child) / 'stat'
                    child_alive = stat.exists() and stat.read_text().split()[2] != 'Z'
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.communicate()
                if (root / 'child-pid').exists():
                    try:
                        os.kill(int((root / 'child-pid').read_text()), signal.SIGKILL)
                    except ProcessLookupError:
                        pass
            elapsed = time.monotonic() - started
            trace = (root / 'trace').read_text() if (root / 'trace').exists() else ''
            smoke = (root / 'smoke').read_text() if (root / 'smoke').exists() else ''
            self.assertEqual('fixture lock; preparation must not change dependencies\n', lock.read_text())
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
        self.assertRegex(result.stdout, r'PLAN_CHANGE_BROWSER_PREREQUISITES=PASS elapsed_seconds=\d+\n$')

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
        self.assertIn('PLAN_CHANGE_BROWSER_PREP_RESULT=browser exit=17', result.stdout)
        self.assertIn('PLAN_CHANGE_BROWSER_PREP_RESULT=font exit=0', result.stdout)
        self.assertNotIn('node ', trace)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_font_index_and_install_failures_each_stay_failure(self):
        for phase, code in [('APT_UPDATE_EXIT', '19'), ('APT_INSTALL_EXIT', '21')]:
            with self.subTest(phase=phase):
                result, trace, _, _, _ = self.probe(missing_font=True, **{phase: code})
                self.assertEqual(int(code), result.returncode)
                self.assertIn('PLAN_CHANGE_BROWSER_PREP_RESULT=browser exit=0', result.stdout)
                self.assertIn('PLAN_CHANGE_BROWSER_PREP_RESULT=font exit=' + code, result.stdout)
                self.assertNotIn('node ', trace)
                self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_successful_apt_without_noto_is_rejected(self):
        result, trace, _, _, _ = self.probe(missing_font=True, NO_FONT_AFTER_INSTALL='1')
        self.assertNotEqual(0, result.returncode)
        self.assertIn('PLAN_CHANGE_NOTO_CJK_UNAVAILABLE', result.stderr)
        self.assertNotIn('node ', trace)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_both_installation_failures_remain_visible(self):
        result, _, _, _, _ = self.probe(missing_font=True, INSTALL_EXIT='17', APT_INSTALL_EXIT='21')
        self.assertEqual(17, result.returncode)
        self.assertIn('PLAN_CHANGE_BROWSER_PREP_RESULT=browser exit=17', result.stdout)
        self.assertIn('PLAN_CHANGE_BROWSER_PREP_RESULT=font exit=21', result.stdout)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_launch_failure_cannot_emit_success(self):
        result, _, _, _, _ = self.probe(SMOKE_EXIT='23')
        self.assertEqual(23, result.returncode)
        self.assertIn('PLAN_CHANGE_BROWSER_PREP_RESULT=launch exit=23', result.stdout)
        self.assertNotIn('PREREQUISITES=PASS', result.stdout)

    def test_blocked_download_font_and_launch_are_terminated(self):
        for stage in ['browser', 'font', 'launch']:
            with self.subTest(stage=stage):
                result, trace, _, child_alive, elapsed = self.probe(
                    missing_font=stage == 'font', HANG_STAGE=stage, ACCELERATE_TIMEOUTS='1')
                self.assertEqual(124, result.returncode, result.stderr)
                self.assertIn('PLAN_CHANGE_BROWSER_PREP_RESULT=' + stage + ' exit=124', result.stdout)
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


if __name__ == '__main__':
    unittest.main()
