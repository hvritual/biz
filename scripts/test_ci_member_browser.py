"""Bounded command probes for CI browser setup, not rendered UI evidence."""
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class MemberBrowserSetupTest(unittest.TestCase):
    def run_probe(self, **overrides):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'web').mkdir()
            bindir = root / 'bin'
            bindir.mkdir()
            font = root / 'font'
            font.write_text('ready')
            if overrides.pop('missing_font', False):
                font.unlink()
            commands = {
                'npx': 'echo "npx $*" >> "$TRACE"; exit "${INSTALL_EXIT:-0}"',
                'fc-list': '[[ ! -f "$FONT" ]] || echo "fixture-font"',
                'sudo': 'echo "sudo $*" >> "$TRACE"; [[ "${APT_EXIT:-0}" == 0 ]] || exit "$APT_EXIT"; '
                        '[[ "${NO_FONT_AFTER_INSTALL:-0}" == 1 ]] || touch "$FONT"',
                'node': 'cat > "$SMOKE_SOURCE"; echo "node $*" >> "$TRACE"; exit "${SMOKE_EXIT:-0}"',
            }
            for name, body in commands.items():
                file = bindir / name
                file.write_text('#!/usr/bin/env bash\nset -euo pipefail\n' + body + '\n')
                file.chmod(0o755)
            env = {**os.environ, 'PATH': str(bindir) + os.pathsep + os.environ['PATH'],
                   'GITHUB_ACTIONS': 'true', 'GITHUB_WORKSPACE': str(root),
                   'TRACE': str(root / 'trace'), 'FONT': str(font),
                   'SMOKE_SOURCE': str(root / 'smoke'), **overrides}
            result = subprocess.run(['bash', str(ROOT / 'scripts/ci_member_browser.sh')],
                                    env=env, capture_output=True, text=True, timeout=10)
            trace = (root / 'trace').read_text() if (root / 'trace').exists() else ''
            smoke = (root / 'smoke').read_text() if (root / 'smoke').exists() else ''
            return result, trace, smoke

    def test_ready_runner_does_not_upgrade_os(self):
        result, trace, smoke = self.run_probe()
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertIn('playwright install --only-shell chromium', trace)
        self.assertNotIn('sudo', trace)
        self.assertIn('chromium.launch({ headless: true })', smoke)
        self.assertIn('finally', smoke)
        self.assertIn('MEMBER_BROWSER_PREREQUISITES=PASS', result.stdout)

    def test_browser_download_and_missing_font_install_are_parallelized(self):
        source = (ROOT / 'scripts/ci_member_browser.sh').read_text()
        self.assertIn('playwright install --only-shell chromium &', source)
        self.assertIn('font_pid=$!', source)
        self.assertIn('wait "$browser_pid"', source)
        self.assertIn('wait "$font_pid"', source)
        self.assertIn('(( browser_status == 0 )) || exit "$browser_status"', source)
        self.assertIn('(( font_status == 0 )) || exit "$font_status"', source)

    def test_success_receipt_exposes_bounded_elapsed_seconds(self):
        result, _, _ = self.run_probe()
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertRegex(result.stdout, r'^MEMBER_BROWSER_PREREQUISITES=PASS elapsed_seconds=\d+\n$')

    def test_missing_chinese_font_installs_only_required_font(self):
        result, trace, _ = self.run_probe(missing_font=True)
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertIn('apt-get install -y --no-install-recommends fonts-wqy-zenhei', trace)
        self.assertNotIn('--with-deps', trace)

    def test_failed_download_does_not_reach_browser(self):
        result, trace, _ = self.run_probe(INSTALL_EXIT='17')
        self.assertEqual(17, result.returncode)
        self.assertNotIn('node ', trace)

    def test_failed_font_install_does_not_reach_browser(self):
        result, trace, _ = self.run_probe(missing_font=True, APT_EXIT='19')
        self.assertEqual(19, result.returncode)
        self.assertNotIn('node ', trace)

    def test_missing_font_after_successful_install_is_rejected(self):
        result, trace, _ = self.run_probe(missing_font=True, NO_FONT_AFTER_INSTALL='1')
        self.assertNotEqual(0, result.returncode)
        self.assertIn('CHINESE_FONT_UNAVAILABLE', result.stderr)
        self.assertNotIn('node ', trace)

    def test_real_launch_failure_stays_failure(self):
        result, _, _ = self.run_probe(SMOKE_EXIT='23')
        self.assertEqual(23, result.returncode)
        self.assertNotIn('=PASS', result.stdout)

    def test_non_ci_use_is_rejected_before_setup(self):
        result, trace, _ = self.run_probe(GITHUB_ACTIONS='false')
        self.assertEqual(2, result.returncode)
        self.assertEqual('', trace)

    def test_workflow_keeps_member_suite_and_setup_probe(self):
        source = (ROOT / '.github/workflows/enterprise-177-member-lifecycle.yml').read_text()
        self.assertIn('python3 -B scripts/test_ci_member_browser.py', source)
        self.assertIn('bash scripts/ci_member_browser.sh', source)
        self.assertIn('ENTERPRISE_MEMBER_REAL_E2E=1 npx playwright test e2e/enterprise-members-real.spec.ts', source)
        self.assertNotRegex(source, r'playwright\s+install\s+--with-deps\s+chromium')

    def test_coffeelink_reuses_bounded_browser_setup_without_weakening_evidence(self):
        source = (ROOT / '.github/workflows/coffeelink-web.yml').read_text()
        self.assertEqual(2, source.count('bash scripts/ci_member_browser.sh'))
        self.assertIn('python3 -B scripts/test_ci_member_browser.py', source)
        self.assertNotIn('playwright install --with-deps', source)
        self.assertNotIn('fonts-noto-cjk', source)
        self.assertIn('timeout-minutes: 6', source)
        self.assertIn('default: 180', source)
        self.assertIn('default: 240', source)
        self.assertIn('Run CoffeeLink core E2E', source)
        self.assertIn('Verify visual contract evidence', source)
        self.assertIn('Run serial site-rental E2E', source)
        self.assertIn('Verify site-rental evidence', source)

    def test_organization_keeps_e2e_and_four_viewport_evidence(self):
        source = (ROOT / '.github/workflows/ec-ri-04-web-qualification.yml').read_text()
        self.assertIn('python3 -B scripts/test_ci_member_browser.py', source)
        self.assertIn('bash scripts/ci_member_browser.sh', source)
        self.assertIn('ENTERPRISE_ORGANIZATION_REAL_E2E=1 npx playwright test e2e/enterprise-organization-real.spec.ts', source)
        self.assertIn('for width, height in [(1366, 768), (1440, 900), (1536, 1024), (390, 844)]', source)
        self.assertIn('assert got == (width, height)', source)
        self.assertNotIn('playwright install --with-deps', source)
        self.assertNotIn('apt-get install -y fonts-noto-cjk', source)


    def test_plan_read_reuses_setup_without_weakening_api_evidence(self):
        source = (ROOT / '.github/workflows/ec-ri-06-plan-read-web.yml').read_text()
        self.assertIn('python3 -B scripts/test_ci_member_browser.py', source)
        self.assertIn('bash scripts/ci_member_browser.sh', source)
        self.assertNotIn('playwright install --with-deps', source)
        self.assertNotIn('apt-get install -y fonts-noto-cjk', source)
        self.assertIn('VITE_DATA_MODE=api npm run build', source)
        self.assertIn('ENTERPRISE_PLAN_REAL_E2E=1 npx playwright test e2e/enterprise-plan-real.spec.ts', source)
        self.assertIn("assert stats.get('unexpected', 0) == 0", source)
        self.assertIn('for width, height in [(1366, 768), (1440, 900), (1536, 1024), (390, 844)]', source)
        self.assertIn('assert got == (width, height)', source)


    def full_chromium_probe(self, install_exit='0'):
        source = (ROOT / '.github/workflows/ec-ri-06-plan-read-web.yml').read_text()
        pattern = (r'^      - name: Install Chromium channel for native browser zoom\n'
                   r'        working-directory: web\n'
                   r'        shell: bash\n'
                   r'        run: ([^\n]+)\n')
        steps = list(re.finditer(pattern, source, re.M))
        self.assertEqual(1, len(steps), 'native zoom needs an executed full Chromium install step')
        self.assertLess(source.index('bash scripts/ci_member_browser.sh'), steps[0].start())
        self.assertLessEqual(steps[0].end(), source.index('      - name: Run tenant plan and usage API-mode E2E'))
        self.assertIn("channel: 'chromium'", (ROOT / 'web/e2e/commercial-state-zoom.helpers.ts').read_text())
        self.assertNotIn('continue-on-error:', source)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'web').mkdir()
            bindir = root / 'bin'
            bindir.mkdir()
            npx = bindir / 'npx'
            npx.write_text('#!/usr/bin/env bash\nset -euo pipefail\nprintf "%s\\n" "$*" > "$TRACE"\nexit "$INSTALL_EXIT"\n')
            npx.chmod(0o755)
            env = {**os.environ, 'PATH': str(bindir) + os.pathsep + os.environ['PATH'],
                   'TRACE': str(root / 'trace'), 'INSTALL_EXIT': install_exit}
            result = subprocess.run(['bash', '-e', '-o', 'pipefail', '-c', steps[0].group(1)],
                                    cwd=root / 'web', env=env, capture_output=True, text=True, timeout=10)
            trace = (root / 'trace').read_text() if (root / 'trace').exists() else ''
        return result, trace

    def test_plan_read_installs_full_chromium_before_native_zoom(self):
        result, trace = self.full_chromium_probe()
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertEqual(['--no-install', 'playwright', 'install', 'chromium'], trace.split())
        self.assertNotIn('--with-deps', trace)
        self.assertNotIn('--only-shell', trace)

    def test_plan_read_full_chromium_install_failure_stays_failure(self):
        result, _ = self.full_chromium_probe(install_exit='23')
        self.assertEqual(23, result.returncode)


if __name__ == '__main__':
    unittest.main()
