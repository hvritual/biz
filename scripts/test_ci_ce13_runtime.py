#!/usr/bin/env python3
import tempfile
import unittest
from pathlib import Path
import subprocess
import os
import textwrap
import re
import shutil
import signal
import sys
import time

import check_ci_ce13 as checks

ROOT = Path(__file__).resolve().parents[1]
APT_SOURCE = ('# Preserve Ubuntu archive policy while selecting its official HTTPS endpoint.\n'
              'Types: deb\nURIs: mirror+file:/etc/apt/apt-mirrors.txt\n'
              'Suites: noble noble-updates noble-backports\n'
              'Components: main restricted universe multiverse\nArchitectures: amd64\n'
              'Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\n\n'
              'Types: deb\nURIs: https://security.ubuntu.com/ubuntu/\n'
              'Suites: noble-security\nComponents: main restricted universe multiverse\n'
              'Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\n')


def recorded_child_alive(root):
    if not (root / 'child-pid').exists():
        return False
    # The child records /proc/self because a nested PID namespace can share an outer /proc.
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

    def probe(self, lane="plan", missing_font=False, apt_source=APT_SOURCE, lifecycle=False, **overrides):
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
            (root / "temp").mkdir()
            source_fixture = root / "ubuntu.sources"
            source_fixture.write_text(apt_source)
            capture = root / "apt-capture"
            capture.mkdir()
            commands = {
                "mktemp": 'printf "mktemp %s\\n" "$*" >> "$TRACE"; '
                          '[[ "$#" == 2 && "$1" == -d && "$2" == /tmp/headless-noto-apt.XXXXXX ]] || exit 86; '
                          'exec "$REAL_MKTEMP" -d "$RUNNER_TEMP/headless-noto-apt.XXXXXX"',
                "cat": 'if [[ "$#" == 1 && "$1" == /etc/apt/sources.list.d/ubuntu.sources ]]; then '
                       'echo read-ubuntu-source >> "$TRACE"; '
                       '[[ "${APT_SOURCE_MISSING:-0}" == 0 ]] || exit 27; '
                       'if [[ "${APT_HANG_STAGE:-}" == source ]]; then stage-progress source >&2; fi; '
                       'exec "$REAL_CAT" "$APT_SOURCE_FIXTURE"; fi; exec "$REAL_CAT" "$@"',
                "stage-progress": '(read -r child_stat < /proc/self/stat; '
                                  'printf "%s\\n" "$child_stat" > "$CHILD_STAT"; '
                                  'while :; do echo "$1-download-progress"; sleep 0.02; done) & '
                                  'child=$!; echo "$child" > "$CHILD_PID"; wait "$child"',
                "npm": 'printf "npm %s cwd=%s\\n" "$*" "$PWD" >> "$TRACE"; '
                       'if [[ "${CHECK_PREP_OVERLAP:-0}" == 1 ]]; then '
                       'for attempt in $(seq 1 80); do [[ -f "$FONT_PIPELINE_STARTED" ]] && break; sleep 0.02; done; '
                       '[[ -f "$FONT_PIPELINE_STARTED" ]] || exit 57; fi; '
                       'mkdir -p node_modules; exit "${NPM_EXIT:-0}"',
                "npx": 'printf "npx %s cwd=%s\\n" "$*" "$PWD" >> "$TRACE"; '
                       '[[ "$*" != *--with-deps* ]] || exit 88; exit "${INSTALL_EXIT:-0}"',
                "fc-match": 'if [[ -f "$FONT" ]]; then echo "Noto Sans CJK SC"; '
                            'else echo "DejaVu Sans"; fi',
                "sudo": 'printf "sudo %s\\n" "$*" >> "$TRACE"; '
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
                        'if [[ "$phase" == update ]]; then touch "$FONT_PIPELINE_STARTED"; '
                        'if [[ "${APT_HANG_STAGE:-}" == index ]]; then stage-progress index; fi; '
                        'exit "${APT_UPDATE_EXIT:-${APT_EXIT:-0}}"; fi; '
                        'if [[ "${APT_HANG_STAGE:-}" == install ]]; then stage-progress install; fi; '
                        '[[ "${APT_INSTALL_EXIT:-${APT_EXIT:-0}}" == 0 ]] || exit "${APT_INSTALL_EXIT:-$APT_EXIT}"; '
                        '[[ "${NO_FONT_AFTER_INSTALL:-0}" == 1 ]] || touch "$FONT"',
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
                   "FONT": str(font), "SMOKE_SOURCE": str(root / "smoke"),
                   "FONT_PIPELINE_STARTED": str(root / "font-pipeline-started"),
                   "REAL_CAT": shutil.which("cat"), "REAL_RM": shutil.which("rm"),
                   "REAL_MKTEMP": shutil.which("mktemp"), "APT_SOURCE_FIXTURE": str(source_fixture),
                   "APT_CAPTURE": str(capture), "CHILD_PID": str(root / "child-pid"),
                   "CHILD_STAT": str(root / "child-proc-stat"), **overrides}
            command = ["bash", str(ROOT / "scripts/ci_ce13_browser.sh")]
            out = root / "temp" / ("ce13-" + lane)
            if lifecycle:
                begin = time.monotonic()
                start = subprocess.run([*command, "start", lane], env=env,
                                       capture_output=True, text=True, timeout=5)
                self.assertEqual(0, start.returncode, start.stderr)
                try:
                    limit = time.monotonic() + 3
                    while not ((root / "child-pid").exists() and
                               (root / "child-proc-stat").exists() and
                               (root / "child-proc-stat").stat().st_size > 0) and time.monotonic() < limit:
                        time.sleep(0.01)
                    self.assertTrue(recorded_child_alive(root), "the original start must own a live font child")
                    self.assertEqual(os.getpgid(int((root / "child-pid").read_text())),
                                     int((out / "browser-prep.pid").read_text()),
                                     "font children must remain in the original setsid process group")
                    original_started = int((out / "browser-prep.started").read_text())
                    self.assertLessEqual(original_started, int(time.time()))
                    # Age only the fixture clock: the actual unchanged 150-second wait condition runs.
                    (out / "browser-prep.started").write_text(str(int(time.time()) - 150))
                    result = subprocess.run([*command, "wait", lane], env=env,
                                            capture_output=True, text=True, timeout=5)
                    stop = subprocess.run([*command, "stop", lane], env=env,
                                          capture_output=True, text=True, timeout=5)
                    result.stop_returncode = stop.returncode
                    limit = time.monotonic() + 2
                    while recorded_child_alive(root) and time.monotonic() < limit:
                        time.sleep(0.01)
                    result.child_alive = recorded_child_alive(root)
                    post_stop_wait = subprocess.run([*command, "wait", lane], env=env,
                                                    capture_output=True, text=True, timeout=5)
                    result.post_stop_wait_returncode = post_stop_wait.returncode
                    result.post_stop_wait_stdout = post_stop_wait.stdout
                    result.lifecycle_seconds = time.monotonic() - begin
                finally:
                    subprocess.run([*command, "stop", lane], env=env,
                                   capture_output=True, text=True, timeout=5)
                    terminate_recorded_child(root)
            else:
                result = subprocess.run([*command, "prepare", lane], env=env,
                                        capture_output=True, text=True, timeout=10)
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
            result.apt_calls = {}
            for phase in ["update", "install"]:
                arguments_file = capture / (phase + ".args")
                if arguments_file.exists():
                    arguments = arguments_file.read_bytes().decode().rstrip("\0").split("\0")
                    result.apt_calls[phase] = {
                        "arguments": arguments,
                        "options": dict(item.split("=", 1) for item in arguments if "=" in item),
                        "source": (capture / (phase + ".sources")).read_text()
                        if (capture / (phase + ".sources")).exists() else None,
                        "modes": (capture / (phase + ".modes")).read_text().splitlines()
                        if (capture / (phase + ".modes")).exists() else [],
                    }
            result.fixture_root = str(root)
            result.apt_temp_remaining = list((root / "temp").glob("headless-noto-apt.*"))
            self.assertEqual(apt_source, source_fixture.read_text())
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

    def test_cold_font_bootstrap_starts_while_npm_runs(self):
        result, trace, _, ready, exit_code = self.probe(missing_font=True, CHECK_PREP_OVERLAP="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("npm ci", trace)
        self.assertIn("apt-get update", trace)
        self.assertIn("apt-get install", trace)
        self.assertTrue(ready)
        self.assertEqual(exit_code, "0")

    def test_npm_failure_drains_font_before_returning(self):
        result, trace, _, ready, exit_code = self.probe(missing_font=True, NPM_EXIT="17")
        self.assertEqual(result.returncode, 17, result.stderr)
        self.assertIn("apt-get install", trace)
        self.assertNotIn("npx", trace)
        self.assertNotIn("node ", trace)
        self.assertFalse(ready)
        self.assertEqual(exit_code, "17")

    def test_browser_failure_drains_font_before_returning(self):
        result, trace, _, ready, exit_code = self.probe(missing_font=True, INSTALL_EXIT="19")
        self.assertEqual(result.returncode, 19, result.stderr)
        self.assertIn("apt-get install", trace)
        self.assertNotIn("node ", trace)
        self.assertFalse(ready)
        self.assertEqual(exit_code, "19")

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




    def test_private_noto_https_source_is_shared_by_update_install_and_preserves_fields(self):
        for uri in ["mirror+file:/etc/apt/apt-mirrors.txt", "http://azure.archive.ubuntu.com/ubuntu/",
                    "https://archive.ubuntu.com/ubuntu/"]:
            with self.subTest(uri=uri):
                source = APT_SOURCE.replace("mirror+file:/etc/apt/apt-mirrors.txt", uri)
                result, trace, _, ready, exit_code = self.probe(missing_font=True, apt_source=source)
                self.assertEqual(0, result.returncode, result.stderr)
                self.assertTrue(ready)
                self.assertEqual("0", exit_code)
                self.assertEqual({"update", "install"}, set(result.apt_calls))
                update, install = result.apt_calls["update"], result.apt_calls["install"]
                self.assertEqual(source.replace(uri, "https://archive.ubuntu.com/ubuntu/"), update["source"])
                self.assertEqual(update["source"], install["source"])
                self.assertEqual(update["options"], install["options"])
                for name in ["Dir::Etc::SourceList", "Dir::Etc::SourceParts", "Dir::State::Lists", "Dir::Cache::Archives"]:
                    self.assertTrue(update["options"][name].startswith(result.fixture_root + "/temp/headless-noto-apt."))
                self.assertEqual("", update["options"]["Dir::Cache::pkgcache"])
                self.assertEqual("", update["options"]["Dir::Cache::srcpkgcache"])
                self.assertEqual(["755"] * 4, update["modes"])
                self.assertIn("sudo -u _apt test -r", trace)
                self.assertIn("--no-install-recommends fonts-noto-cjk", trace)
                self.assertNotIn("--allow-unauthenticated", trace)
                for name in ["RootDir", "Dir::State::status", "Dir::Etc::Trusted", "APT::Sandbox::User",
                             "Acquire::https::Verify-Peer", "Acquire::https::Verify-Host"]:
                    self.assertNotIn(name, update["options"])
                self.assertEqual([], result.apt_temp_remaining)

    def test_private_noto_invalid_source_or_permissions_fail_before_apt(self):
        cases = [(APT_SOURCE, {"APT_SOURCE_MISSING": "1"}),
                 (APT_SOURCE, {"APT_PERMISSION_EXIT": "33"}),
                 (APT_SOURCE.replace("mirror+file:/etc/apt/apt-mirrors.txt", "https://unknown.invalid/ubuntu/"), {}),
                 (APT_SOURCE.replace("Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\n", ""), {}),
                 (APT_SOURCE.replace("URIs: mirror+file:/etc/apt/apt-mirrors.txt",
                                     "URIs: mirror+file:/etc/apt/apt-mirrors.txt\n# interrupted continuation\n https://unknown.invalid/ubuntu/"), {}),
                 (APT_SOURCE.replace("URIs: mirror+file:/etc/apt/apt-mirrors.txt",
                                     "URIs: mirror+file:/etc/apt/apt-mirrors.txt\nURIs: https://unknown.invalid/ubuntu/"), {})]
        for source, overrides in cases:
            with self.subTest(source=source, overrides=overrides):
                result, trace, _, ready, _ = self.probe(missing_font=True, apt_source=source, **overrides)
                self.assertNotEqual(0, result.returncode)
                self.assertEqual({}, result.apt_calls)
                self.assertNotIn("node ", trace)
                self.assertFalse(ready)
                self.assertEqual([], result.apt_temp_remaining)

    def test_private_noto_error_codes_and_final_font_check_precede_cleanup_error(self):
        cases = [({"APT_UPDATE_EXIT": "19"}, 19), ({"APT_INSTALL_EXIT": "21"}, 21),
                 ({"CLEANUP_EXIT": "29"}, 29),
                 ({"APT_INSTALL_EXIT": "21", "CLEANUP_EXIT": "29"}, 21),
                 ({"NO_FONT_AFTER_INSTALL": "1", "CLEANUP_EXIT": "29"}, 1)]
        for overrides, expected in cases:
            with self.subTest(overrides=overrides):
                result, trace, _, ready, exit_code = self.probe(missing_font=True, **overrides)
                self.assertEqual(expected, result.returncode, result.stderr)
                self.assertEqual(str(expected), exit_code)
                self.assertNotIn("node ", trace)
                self.assertFalse(ready)
                if overrides.get("NO_FONT_AFTER_INSTALL"):
                    self.assertIn("CE13_CHINESE_FONT_UNAVAILABLE", result.stderr)
                if overrides.get("CLEANUP_EXIT"):
                    self.assertIn("CE13_NOTO_APT_CLEANUP_FAILED exit=29", result.stderr)
                if "APT_INSTALL_EXIT" in overrides:
                    self.assertEqual({"update", "install"}, set(result.apt_calls))

    def test_private_noto_keeps_original_deadline_and_stop_process_group_cleanup(self):
        for stage in ["source", "index", "install"]:
            with self.subTest(stage=stage):
                result, trace, _, ready, exit_code = self.probe(
                    missing_font=True, lifecycle=True, APT_HANG_STAGE=stage)
                self.assertNotEqual(0, result.returncode)
                self.assertIn("CE13_BROWSER_PREP_TIMEOUT", result.stderr)
                self.assertIn(stage + "-download-progress", result.stderr)
                self.assertEqual(0, result.stop_returncode)
                self.assertFalse(result.child_alive)
                self.assertFalse(ready)
                # The existing EXIT receipt can be zero after TERM. Readiness is also mandatory.
                self.assertNotEqual(0, result.post_stop_wait_returncode)
                self.assertNotIn("CE13_BROWSER_PREP_SECONDS=", result.post_stop_wait_stdout)
                self.assertEqual([], result.apt_temp_remaining)
                self.assertNotIn("node ", trace)
                self.assertLess(result.lifecycle_seconds, 5)

    def test_private_noto_ready_font_never_reads_source_or_calls_apt(self):
        result, trace, _, ready, exit_code = self.probe(APT_SOURCE_MISSING="1", CLEANUP_EXIT="29")
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertTrue(ready)
        self.assertEqual("0", exit_code)
        self.assertNotIn("read-ubuntu-source", trace)
        self.assertNotIn("sudo", trace)
        self.assertEqual({}, result.apt_calls)
        self.assertEqual([], result.apt_temp_remaining)


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
