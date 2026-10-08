#!/usr/bin/env python3
import tempfile
import unittest
from pathlib import Path
import subprocess
import os
import textwrap
import re

import check_ci_ce13 as checks

ROOT = Path(__file__).resolve().parents[1]

class SourceContractTests(unittest.TestCase):
    def mutated(self, path, transform):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            paths = [
                ".github/workflows/ce13-plan-catalog-qualification.yml",
                ".github/workflows/ce13-platform-web-session.yml",
                ".github/workflows/pr-qualification.yml",
                "scripts/ci_ce13_mysql.sh",
                "scripts/ci_ce13_browser.sh",
                "scripts/ci_proof_contract.json",
                "web/playwright.ce13-plan-catalog.config.ts",
                "web/playwright.ce13-platform.config.ts",
            ]
            for item in paths:
                target = root / item
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes((ROOT / item).read_bytes())
            target = root / path
            target.write_text(transform(target.read_text()))
            with self.assertRaises(ValueError):
                checks.check(root)

    def test_current_sources(self):
        checks.check()

    def test_service_mysql_rejected(self):
        self.mutated(".github/workflows/ce13-plan-catalog-qualification.yml",
                     lambda s: s.replace("permissions:\n", "services:\n  mysql:\n    image: mysql:8.4\npermissions:\n"))

    def test_cache_disabled_rejected(self):
        self.mutated(".github/workflows/ce13-platform-web-session.yml",
                     lambda s: s.replace("cache: true", "cache: false"))

    def test_headed_chromium_rejected(self):
        self.mutated("scripts/ci_ce13_browser.sh",
                     lambda s: s.replace("--only-shell ", ""))

    def test_full_os_bootstrap_rejected(self):
        self.mutated("scripts/ci_ce13_browser.sh",
                     lambda s: s.replace("--only-shell chromium", "--with-deps --only-shell chromium"))

    def test_browser_launch_probe_removal_rejected(self):
        self.mutated("scripts/ci_ce13_browser.sh",
                     lambda s: s.replace("chromium.launch({ headless: true, timeout: 20000 })", "Promise.resolve(null)"))

    def test_font_readiness_guard_removal_rejected(self):
        self.mutated("scripts/ci_ce13_browser.sh",
                     lambda s: s.replace("CE13_CHINESE_FONT_UNAVAILABLE", "FONT_WARNING"))

    def test_ce07_overlap_rejected(self):
        self.mutated(".github/workflows/ce13-plan-catalog-qualification.yml",
                     lambda s: s.replace("go -C biz vet", "go -C biz test -count=1 -tags=integration ./integration -run '^TestCE07MySQL'\n          go -C biz vet", 1))


    def test_plan_browser_prep_after_generation_rejected(self):
        block = (
            "      - name: Start CE13 browser dependency preparation\n"
            "        run: bash biz/scripts/ci_ce13_browser.sh start plan\n\n"
        )
        self.mutated(
            ".github/workflows/ce13-plan-catalog-qualification.yml",
            lambda s: s.replace(block, "", 1).replace(
                "      - name: Wait for workflow-owned CE13 MySQL\n",
                block + "      - name: Wait for workflow-owned CE13 MySQL\n",
                1,
            ),
        )

    def test_session_source_web_working_directory_rejected(self):
        self.mutated(
            ".github/workflows/ce13-platform-web-session.yml",
            lambda s: s.replace(
                'cd "$RUNNER_TEMP/ce13-session/web"',
                'cd "$GITHUB_WORKSPACE/biz/web"',
            ),
        )

    def test_browser_helper_source_tree_write_rejected(self):
        self.mutated(
            "scripts/ci_ce13_browser.sh",
            lambda s: s.replace(
                'cd "$isolated_web"',
                'cd "${GITHUB_WORKSPACE:?}/biz/web"',
                1,
            ),
        )

    def test_go_run_recompile_rejected(self):
        self.mutated(".github/workflows/ce13-platform-web-session.yml",
                     lambda s: s.replace('nohup "$RUNNER_TEMP/ce13-session-idp"', "nohup go -C biz run ./cmd/biz-idp"))

    def test_serial_browser_contract_rejected(self):
        self.mutated("web/playwright.ce13-platform.config.ts",
                     lambda s: s.replace("workers: 1", "workers: 4"))

    def test_tmpfs_rejected(self):
        self.mutated("scripts/ci_ce13_mysql.sh",
                     lambda s: s.replace("docker run --name", "docker run --tmpfs /var/lib/mysql --name"))

    def test_budget_relaxation_rejected(self):
        self.mutated("scripts/ci_proof_contract.json",
                     lambda s: s.replace('"ce13-plan-catalog-qualification.yml": {"owns": ["ce13-plan-catalog-qualification.qualification"], "delegates": ["ce07-qualification.qualification"], "runtime": "mixed", "jobs": ["qualify"], "target_seconds": 120, "hard_seconds": 180',
                                                 '"ce13-plan-catalog-qualification.yml": {"owns": ["ce13-plan-catalog-qualification.qualification"], "delegates": ["ce07-qualification.qualification"], "runtime": "mixed", "jobs": ["qualify"], "target_seconds": 120, "hard_seconds": 240'))

    def test_legacy_debt_reintroduction_rejected(self):
        self.mutated("scripts/ci_proof_contract.json",
                     lambda s: s.replace('"ce13-platform-web-session.yml": {"owns": ["ce13-platform-web-session.qualification"], "delegates": [], "runtime": "mixed", "jobs": ["qualify"], "target_seconds": 120, "hard_seconds": 180, "race_commands": [], "restart": "none", "legacy_cost_ceiling": {}',
                                                 '"ce13-platform-web-session.yml": {"owns": ["ce13-platform-web-session.qualification"], "delegates": [], "runtime": "mixed", "jobs": ["qualify"], "target_seconds": 120, "hard_seconds": 180, "race_commands": [], "restart": "none", "legacy_cost_ceiling": {"bootstrap.service-mysql": 1}'))

    def test_helper_ci_only(self):
        result = subprocess.run(
            ["bash", str(ROOT / "scripts/ci_ce13_mysql.sh"), "start", "plan"],
            env={**os.environ, "GITHUB_ACTIONS": "false"},
            capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        self.assertIn("CI_OWNED_DATABASE_ONLY", result.stderr)

    def test_browser_helper_ci_only(self):
        result = subprocess.run(
            ["bash", str(ROOT / "scripts/ci_ce13_browser.sh"), "start", "plan"],
            env={**os.environ, "GITHUB_ACTIONS": "false"},
            capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        self.assertIn("CI_OWNED_BROWSER_ONLY", result.stderr)



class BrowserPreparationTests(unittest.TestCase):
    """Exercise the real shell helper with bounded command doubles, not UI evidence."""

    def probe(self, lane="plan", missing_font=False, **overrides):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            source = root / "checkout/biz/web"
            source.mkdir(parents=True)
            (source / "package-lock.json").write_text("fixture lock\n")
            for excluded in ("node_modules", "test-results", "playwright-report", "dist"):
                (source / excluded).mkdir()
                (source / excluded / "sentinel").write_text("must not copy\n")
            bindir = root / "bin"
            bindir.mkdir()
            font = root / "font"
            if not missing_font:
                font.write_text("ready")
            commands = {
                "npm": 'printf "npm %s cwd=%s\\n" "$*" "$PWD" >> "$TRACE"; '
                       'mkdir -p node_modules; exit "${NPM_EXIT:-0}"',
                "npx": 'printf "npx %s cwd=%s\\n" "$*" "$PWD" >> "$TRACE"; '
                       '[[ "$*" != *--with-deps* ]] || exit 88; exit "${INSTALL_EXIT:-0}"',
                "fc-match": 'if [[ -f "$FONT" ]]; then echo "Noto Sans CJK SC"; '
                            'else echo "DejaVu Sans"; fi',
                "sudo": 'printf "sudo %s\\n" "$*" >> "$TRACE"; '
                        '[[ "${APT_EXIT:-0}" == 0 ]] || exit "$APT_EXIT"; '
                        'if [[ "$*" == *"install "* && "${NO_FONT_AFTER_INSTALL:-0}" != 1 ]]; '
                        'then touch "$FONT"; fi',
                "node": 'cat > "$SMOKE_SOURCE"; printf "node %s cwd=%s\\n" "$*" "$PWD" >> "$TRACE"; '
                        'exit "${SMOKE_EXIT:-0}"',
            }
            for name, body in commands.items():
                target = bindir / name
                target.write_text("#!/usr/bin/env bash\nset -euo pipefail\n" + body + "\n")
                target.chmod(0o755)
            env = {**os.environ, "PATH": str(bindir) + os.pathsep + os.environ["PATH"],
                   "GITHUB_ACTIONS": "true", "GITHUB_RUN_ID": "123", "RUNNER_TEMP": str(root / "temp"),
                   "GITHUB_WORKSPACE": str(root / "checkout"), "TRACE": str(root / "trace"),
                   "FONT": str(font), "SMOKE_SOURCE": str(root / "smoke"), **overrides}
            command = ["bash", str(ROOT / "scripts/ci_ce13_browser.sh")]
            result = subprocess.run([*command, "prepare", lane], env=env,
                                    capture_output=True, text=True, timeout=10)
            out = root / "temp" / ("ce13-" + lane)
            ready = (out / "browser-prep.ready").exists()
            exit_code = (out / "browser-prep.exit").read_text().strip() if (out / "browser-prep.exit").exists() else None
            trace = (root / "trace").read_text() if (root / "trace").exists() else ""
            smoke = (root / "smoke").read_text() if (root / "smoke").exists() else ""
            self.assertEqual((source / "package-lock.json").read_text(), "fixture lock\n")
            for excluded in ("node_modules", "test-results", "playwright-report", "dist"):
                self.assertTrue((source / excluded / "sentinel").is_file())
                self.assertFalse((out / "web" / excluded / "sentinel").exists())
            self.assertNotIn("cwd=" + str(source), trace)
            if result.returncode == 0:
                (out / "browser-prep.started").write_text((out / "browser-prep.ready").read_text())
                wait = subprocess.run([*command, "wait", lane], env=env,
                                      capture_output=True, text=True, timeout=5)
                self.assertEqual(wait.returncode, 0, wait.stderr)
            return result, trace, smoke, ready, exit_code

    def test_ready_runner_both_lanes_skip_os_bootstrap_and_probe_browser(self):
        for lane in ("plan", "session"):
            with self.subTest(lane=lane):
                result, trace, smoke, ready, exit_code = self.probe(lane)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("npx --no-install playwright install --only-shell chromium", trace)
                self.assertNotIn("sudo", trace)
                self.assertIn("chromium.launch({ headless: true", smoke)
                self.assertIn("finally", smoke)
                self.assertIn("browser.close()", smoke)
                self.assertIn("CE13_BROWSER_PREP_STAGE=launch", result.stdout)
                self.assertTrue(ready)
                self.assertEqual(exit_code, "0")

    def test_missing_noto_installs_only_noto_then_verifies(self):
        result, trace, _, ready, exit_code = self.probe(missing_font=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("apt-get update", trace)
        self.assertIn("--no-install-recommends fonts-noto-cjk", trace)
        self.assertNotIn("--with-deps", trace)
        self.assertTrue(ready)
        self.assertEqual(exit_code, "0")

    def test_npm_failure_does_not_publish_readiness(self):
        result, trace, _, ready, exit_code = self.probe(NPM_EXIT="17")
        self.assertEqual(result.returncode, 17)
        self.assertNotIn("npx", trace)
        self.assertFalse(ready)
        self.assertEqual(exit_code, "17")

    def test_browser_download_failure_does_not_launch(self):
        result, trace, _, ready, exit_code = self.probe(INSTALL_EXIT="19")
        self.assertEqual(result.returncode, 19)
        self.assertNotIn("node ", trace)
        self.assertFalse(ready)
        self.assertEqual(exit_code, "19")

    def test_font_install_failure_does_not_launch(self):
        result, trace, _, ready, exit_code = self.probe(missing_font=True, APT_EXIT="23")
        self.assertEqual(result.returncode, 23)
        self.assertNotIn("node ", trace)
        self.assertFalse(ready)
        self.assertEqual(exit_code, "23")

    def test_successful_apt_without_required_font_stays_failure(self):
        result, trace, _, ready, _ = self.probe(missing_font=True, NO_FONT_AFTER_INSTALL="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("CE13_CHINESE_FONT_UNAVAILABLE", result.stderr)
        self.assertNotIn("node ", trace)
        self.assertFalse(ready)

    def test_missing_browser_library_fails_launch_not_silently_passes(self):
        result, _, _, ready, exit_code = self.probe(SMOKE_EXIT="29")
        self.assertEqual(result.returncode, 29)
        self.assertFalse(ready)
        self.assertEqual(exit_code, "29")

    def test_prepare_non_ci_rejected(self):
        result, trace, _, ready, _ = self.probe(GITHUB_ACTIONS="false")
        self.assertEqual(result.returncode, 2)
        self.assertEqual(trace, "")
        self.assertFalse(ready)




class SessionParallelQualificationTests(unittest.TestCase):
    """Exercise the actual workflow shell, not a duplicate command model."""

    @staticmethod
    def run_script(fail_match=""):
        workflow = (ROOT / ".github/workflows/ce13-platform-web-session.yml").read_text()
        match = re.search(
            r"(?ms)^      - name: Verify CE-13 authentication allowlist and scoped regressions\n"
            r"        shell: bash\n        run: \|\n(?P<commands>.*?)"
            r"^      - name: Seed platform, denied-platform and tenant-only browser identities",
            workflow,
        )
        if match is None:
            raise AssertionError("Missing scoped validation step")
        commands = textwrap.dedent(match.group("commands")).strip()
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            bin_root = root / "fake-bin"
            bin_root.mkdir()
            go = bin_root / "go"
            go.write_text(
                '#!/bin/bash\n'
                'printf "%s\\n" "$*" >> "$CE13_COMMANDS"\n'
                'if [[ -n "$CE13_FAIL_MATCH" && "$*" == *"$CE13_FAIL_MATCH"* ]]; then exit 17; fi\n'
            )
            go.chmod(0o755)
            env = {**os.environ, "PATH": str(bin_root) + os.pathsep + os.environ["PATH"],
                   "RUNNER_TEMP": str(root), "CE13_COMMANDS": str(root / "commands.log"),
                   "CE13_FAIL_MATCH": fail_match}
            result = subprocess.run(["bash", "-c", commands], env=env,
                                    cwd=ROOT, capture_output=True, text=True)
            lines = (root / "commands.log").read_text().splitlines()
            return result, lines

    def test_parallel_success_still_runs_all_tests_vet_and_three_builds(self):
        result, commands = self.run_script()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("CE13_SCOPED_VALIDATION=PASS", result.stdout)
        self.assertEqual(len(commands), 6, commands)
        self.assertEqual(sum(" build " in " " + x + " " for x in commands), 3)
        self.assertEqual(sum(" test " in " " + x + " " for x in commands), 2)
        self.assertEqual(sum(" vet " in " " + x + " " for x in commands), 1)

    def test_parallel_unit_test_failure_is_not_hidden_by_successful_builds(self):
        result, commands = self.run_script("-run ^TestCE13Platform")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("CE13_SCOPED_VALIDATION_FAILED", result.stderr)
        self.assertNotIn("CE13_SCOPED_VALIDATION=PASS", result.stdout)
        self.assertEqual(len(commands), 6, commands)
        self.assertEqual(sum(" build " in " " + x + " " for x in commands), 3)
        self.assertEqual(sum(" test " in " " + x + " " for x in commands), 2)
        self.assertEqual(sum(" vet " in " " + x + " " for x in commands), 1)

    def test_parallel_binary_failure_is_not_hidden_by_successful_tests(self):
        result, commands = self.run_script("ce13-session-idp")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("CE13_SCOPED_VALIDATION_FAILED", result.stderr)
        self.assertNotIn("CE13_SCOPED_VALIDATION=PASS", result.stdout)
        self.assertEqual(len(commands), 6, commands)
        self.assertEqual(sum(" test " in " " + x + " " for x in commands), 2)

    def test_parallel_vet_failure_runs_all_six_before_rejecting(self):
        result, commands = self.run_script(" vet ")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("CE13_SCOPED_VALIDATION_FAILED", result.stderr)
        self.assertEqual(len(commands), 6, commands)
        self.assertEqual(sum(" vet " in " " + x + " " for x in commands), 1)

    def test_parallel_second_test_failure_preserves_first_and_vet_evidence(self):
        result, commands = self.run_script("access/infrastructure/persistence")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("CE13_SCOPED_VALIDATION_FAILED", result.stderr)
        self.assertEqual(len(commands), 6, commands)
        self.assertEqual(sum(" vet " in " " + x + " " for x in commands), 1)

if __name__ == "__main__":
    unittest.main()
