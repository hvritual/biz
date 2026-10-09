#!/usr/bin/env python3
"""Environment-only negative fixtures; not browser, database or approval evidence."""
from pathlib import Path
import tempfile
import unittest

import check_ci_ce13 as checks

ROOT = Path(__file__).resolve().parents[1]


class EnvironmentPinTests(unittest.TestCase):
    def mutated(self, path, transform):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for item in (
                ".github/workflows/ce13-plan-catalog-qualification.yml",
                ".github/workflows/ce13-platform-web-session.yml",
                ".github/ci/ce13.node-version",
            ):
                target = root / item
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes((ROOT / item).read_bytes())
            target = root / path
            before = target.read_text()
            changed = transform(before)
            self.assertNotEqual(before, changed, "fixture must inject a real change")
            target.write_text(changed)
            if path.startswith(".github/workflows/"):
                affected = [path]
            else:
                affected = [".github/workflows/ce13-plan-catalog-qualification.yml",
                            ".github/workflows/ce13-platform-web-session.yml"]
            for workflow in affected:
                with self.subTest(workflow=workflow), self.assertRaises(ValueError):
                    checks.check_environment(root, (root / workflow).read_text(), workflow)

    def test_node_floating_major_rejected(self):
        self.mutated(".github/ci/ce13.node-version", lambda _: "22\n")

    def test_node_range_rejected(self):
        self.mutated(".github/ci/ce13.node-version", lambda _: ">=22.12.0\n")

    def test_node_duplicate_lines_rejected(self):
        self.mutated(".github/ci/ce13.node-version", lambda s: s + s)

    def test_runner_latest_rejected(self):
        self.mutated(".github/workflows/ce13-plan-catalog-qualification.yml",
                     lambda s: s.replace("runs-on: ubuntu-24.04", "runs-on: ubuntu-latest"))

    def test_runner_pin_in_comment_rejected(self):
        self.mutated(".github/workflows/ce13-platform-web-session.yml",
                     lambda s: s.replace("runs-on: ubuntu-24.04",
                                         "runs-on: ubuntu-latest # runs-on: ubuntu-24.04"))

    def test_node_inline_override_rejected(self):
        self.mutated(".github/workflows/ce13-platform-web-session.yml",
                     lambda s: s.replace("node-version-file: biz/.github/ci/ce13.node-version",
                                         "node-version-file: biz/.github/ci/ce13.node-version\n          node-version: '22'"))

    def test_node_profile_path_override_rejected(self):
        self.mutated(".github/workflows/ce13-plan-catalog-qualification.yml",
                     lambda s: s.replace("node-version-file: biz/.github/ci/ce13.node-version",
                                         "node-version-file: biz/web/package.json"))

    def test_node_duplicate_yaml_rejected(self):
        self.mutated(".github/workflows/ce13-platform-web-session.yml",
                     lambda s: s.replace("node-version-file: biz/.github/ci/ce13.node-version",
                                         "node-version-file: biz/.github/ci/ce13.node-version\n          node-version-file: biz/web/package.json"))

    def test_node_action_unpinned_rejected(self):
        self.mutated(".github/workflows/ce13-platform-web-session.yml",
                     lambda s: s.replace("actions/setup-node@49933ea5288caeca8642d1e84afbd3f7d6820020",
                                         "actions/setup-node@v4"))

    def test_node_npm_cache_key_drift_rejected(self):
        self.mutated(".github/workflows/ce13-plan-catalog-qualification.yml",
                     lambda s: s.replace("cache-dependency-path: biz/web/package-lock.json",
                                         "cache-dependency-path: biz/web/package.json"))

    def test_node_npm_cache_disabled_rejected(self):
        self.mutated(".github/workflows/ce13-platform-web-session.yml",
                     lambda s: s.replace("cache: npm", "cache: ''"))

    def test_node_check_latest_rejected(self):
        self.mutated(".github/workflows/ce13-platform-web-session.yml",
                     lambda s: s.replace("node-version-file: biz/.github/ci/ce13.node-version",
                                         "node-version-file: biz/.github/ci/ce13.node-version\n          check-latest: true"))


if __name__ == "__main__":
    unittest.main()
