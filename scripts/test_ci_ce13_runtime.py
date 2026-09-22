#!/usr/bin/env python3
import tempfile
import unittest
from pathlib import Path
import subprocess
import os

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

    def test_ce07_overlap_rejected(self):
        self.mutated(".github/workflows/ce13-plan-catalog-qualification.yml",
                     lambda s: s.replace("go -C biz vet", "go -C biz test -count=1 -tags=integration ./integration -run '^TestCE07MySQL'\n          go -C biz vet", 1))

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

if __name__ == "__main__":
    unittest.main()
