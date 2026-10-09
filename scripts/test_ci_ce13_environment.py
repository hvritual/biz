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
            for workflow in (".github/workflows/ce13-plan-catalog-qualification.yml",
                             ".github/workflows/ce13-platform-web-session.yml"):
                checks.check_environment(root, (root / workflow).read_text(), workflow)
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


class EnvironmentProtectionTests(unittest.TestCase):
    """Test execution dependencies are protected; fixtures cannot excuse defects."""

    def test_clean_environment_sources(self):
        for name in ('ce13-plan-catalog-qualification.yml', 'ce13-platform-web-session.yml'):
            checks.check_environment(ROOT, (ROOT / '.github/workflows' / name).read_text(), name)

    def test_disabled_original_detector_is_exposed(self):
        from unittest.mock import patch
        from test_ci_ce13_runtime import SourceContractTests
        original = checks.require
        def bypass_one(ok, reason):
            if not reason.startswith('CE13_SERVICE_MYSQL_REINTRODUCED:'):
                original(ok, reason)
        with patch.object(checks, 'require', side_effect=bypass_one):
            with self.assertRaises(AssertionError):
                SourceContractTests('test_service_mysql_rejected').test_service_mysql_rejected()

    def test_environment_inputs_reject_business_mutation_in_both_guards(self):
        import subprocess
        from unittest.mock import patch
        import check_ci_source_safety as source
        import ci_proof_governance as proof
        for name in ('scripts/test_ci_ce13_environment.py', '.github/ci/ce13.node-version'):
            with self.subTest(path=name), tempfile.TemporaryDirectory() as directory:
                self.assertIn(name, source.PROTECTED)
                self.assertIn(name, proof.PROTECTED)
                root = Path(directory)
                changed = root / name
                changed.parent.mkdir(parents=True, exist_ok=True)
                changed.write_bytes(b'changed')
                def original(_root, _ref, path):
                    return b'approved' if path == name else None
                with patch.object(source, 'original', side_effect=original):
                    with self.assertRaisesRegex(ValueError, 'CI_CONTROL_REQUIRES_GOVERNANCE:' + name):
                        source.check(root, 'a' * 40, 'feat/business')
                for path in proof.PROTECTED:
                    target = root / path
                    target.parent.mkdir(parents=True, exist_ok=True)
                    if path != name:
                        target.write_bytes((ROOT / path).read_bytes())
                contract = proof.strict_json((ROOT / proof.CONTRACT).read_bytes())
                def git_show(args, **_kwargs):
                    path = args[-1].split(':', 1)[1]
                    raw = b'approved' if path == name else (root / path).read_bytes()
                    return subprocess.CompletedProcess(args, 0,
                        raw.decode() if _kwargs.get('text') else raw, b'')
                with patch.object(proof.subprocess, 'run', side_effect=git_show), \
                        patch.dict('os.environ', {'GITHUB_HEAD_REF': 'feat/business'}):
                    with self.assertRaisesRegex(proof.Violation, 'PROOF_CONTROL_CHANGE_REQUIRES_GOVERNANCE:' + name):
                        proof.check_base(root, 'a' * 40, contract)


class ReadOnlyWarmupTests(unittest.TestCase):
    """Execute actual workflow shell with bounded command doubles, not a speed claim."""

    def run_step(self, lane, fail=''):
        import os
        import subprocess
        import textwrap
        import yaml
        from check_ci_source_safety import StrictLoader
        workflow = 'ce13-platform-web-session.yml' if lane == 'session' else 'ce13-plan-catalog-qualification.yml'
        doc = yaml.load((ROOT / '.github/workflows' / workflow).read_text(), Loader=StrictLoader)
        steps = doc['jobs']['qualify']['steps']
        source = next(s['run'] for s in steps if s['name'].startswith('Verify ') and 'make -C biz generate' in s.get('run', ''))
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            fake_bin = root / 'bin'
            fake_bin.mkdir()
            command = fake_bin / 'command-double'
            command.write_text(textwrap.dedent('''\
                #!/bin/bash
                set -euo pipefail
                name="${0##*/}"
                root="$PROBE_ROOT"
                record() { printf '%s\\n' "$1" >> "$root/trace"; }
                await_file() {
                  for (( attempt=0; attempt<200; attempt++ )); do
                    if [[ -f "$root/$1" ]]; then return; fi
                    sleep .01
                  done
                  exit 90
                }
                case "$name" in
                  git)
                    if [[ "$*" == *rev-parse* ]]; then printf 'snapshot\\n';
                    elif [[ "$PROBE_FAIL" == dirty ]]; then printf ' M source.go\\n'; fi
                    ;;
                  go)
                    [[ "$*" == '-C biz build ./internal/bizruntime' ]]
                    record compile-start
                    touch "$root/compile-start"
                    await_file check-start
                    sleep .02
                    record compile-end
                    touch "$root/compile-end"
                    if [[ "$PROBE_FAIL" == compile ]]; then exit 17; fi
                    ;;
                  make)
                    if [[ " $* " == *' check '* ]]; then
                      record check-start
                      touch "$root/check-start"
                      await_file compile-start
                      record check-end
                      touch "$root/check-end"
                      if [[ "$PROBE_FAIL" == check ]]; then exit 19; fi
                    elif [[ " $* " == *' generate '* ]]; then
                      [[ -f "$root/compile-end" && -f "$root/check-end" ]]
                      record generate
                      if [[ "$PROBE_FAIL" == generate ]]; then exit 23; fi
                    else exit 91; fi
                    ;;
                  gofmt|protoc) : ;;
                  *) exit 92 ;;
                esac
                '''))
            command.chmod(0o755)
            for name in ('git', 'go', 'make', 'gofmt', 'protoc'):
                (fake_bin / name).symlink_to(command)
            env = {**os.environ, 'PATH': str(fake_bin) + os.pathsep + os.environ['PATH'],
                   'PROBE_ROOT': str(root), 'PROBE_FAIL': fail,
                   'BIZ_SHA': 'snapshot', 'YUNKA_SHA': 'snapshot', 'GITHUB_WORKSPACE': str(root)}
            result = subprocess.run(['bash', '-c', source], cwd=root, env=env,
                                    capture_output=True, text=True, timeout=8)
            trace = (root / 'trace').read_text().splitlines() if (root / 'trace').exists() else []
            return result, trace

    def test_cold_compile_overlaps_check_but_finishes_before_generation(self):
        for lane in ('plan', 'session'):
            with self.subTest(lane=lane):
                result, trace = self.run_step(lane)
                self.assertEqual(result.returncode, 0, result.stderr + repr(trace))
                self.assertEqual(set(trace), {'compile-start', 'compile-end', 'check-start', 'check-end', 'generate'})
                self.assertEqual(trace.count('generate'), 1)
                self.assertLess(trace.index('check-start'), trace.index('compile-end'))
                self.assertLess(trace.index('compile-end'), trace.index('generate'))
                self.assertLess(trace.index('check-end'), trace.index('generate'))

    def test_compile_or_check_failure_drains_both_and_never_generates(self):
        for lane in ('plan', 'session'):
            for fail in ('compile', 'check'):
                with self.subTest(lane=lane, fail=fail):
                    result, trace = self.run_step(lane, fail)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn('CE13_READONLY_PREPARATION_FAILED', result.stderr)
                    self.assertIn('compile-end', trace)
                    self.assertIn('check-end', trace)
                    self.assertNotIn('generate', trace)

    def test_dirty_sources_reject_before_generation(self):
        for lane in ('plan', 'session'):
            with self.subTest(lane=lane):
                result, trace = self.run_step(lane, 'dirty')
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('compile-end', trace)
                self.assertNotIn('generate', trace)

    def test_original_generation_failure_still_fails(self):
        for lane in ('plan', 'session'):
            with self.subTest(lane=lane):
                result, trace = self.run_step(lane, 'generate')
                self.assertEqual(result.returncode, 23)
                self.assertEqual(trace.count('generate'), 1)


if __name__ == "__main__":
    unittest.main()
