#!/usr/bin/env python3
"""Static fail-closed contract checks for CE13 Batch B."""
from pathlib import Path
import json
import re
import yaml
from check_ci_source_safety import StrictLoader

ROOT = Path(__file__).resolve().parents[1]

def require(ok, reason):
    if not ok:
        raise ValueError(reason)

def check_environment(root, text, lane):
    """Parse actual job inputs; comments and duplicate YAML cannot satisfy the pin."""
    version_file = root / ".github/ci/ce13.node-version"
    require(version_file.is_file() and not version_file.is_symlink(),
            "CE13_NODE_VERSION_FILE_REQUIRED")
    require(re.fullmatch(r"[1-9][0-9]*\.[0-9]+\.[0-9]+\n", version_file.read_text()) is not None,
            "CE13_NODE_EXACT_VERSION_REQUIRED")
    workflow = yaml.load(text, Loader=StrictLoader)
    job = workflow.get("jobs", {}).get("qualify", {})
    require(job.get("runs-on") == "ubuntu-24.04", "CE13_RUNNER_FAMILY_PIN_REQUIRED:" + lane)
    steps = [step for step in job.get("steps", [])
             if str(step.get("uses", "")).startswith("actions/setup-node@")]
    require(len(steps) == 1 and steps[0]["uses"] ==
            "actions/setup-node@49933ea5288caeca8642d1e84afbd3f7d6820020",
            "CE13_NODE_SETUP_PROVENANCE_REQUIRED:" + lane)
    inputs = steps[0].get("with", {})
    require(inputs.get("node-version-file") == "biz/.github/ci/ce13.node-version"
            and "node-version" not in inputs and inputs.get("check-latest", False) is False,
            "CE13_NODE_VERSION_SOURCE_REQUIRED:" + lane)
    require(inputs.get("cache") == "npm" and
            inputs.get("cache-dependency-path") == "biz/web/package-lock.json",
            "CE13_LOCKED_NPM_CACHE_REQUIRED:" + lane)


def check(root=ROOT):
    specs = {
        "plan": {
            "workflow": "ce13-plan-catalog-qualification.yml",
            "browser_config": "web/playwright.ce13-plan-catalog.config.ts",
            "acceptance": "TestCE13PlanCatalogTrustedPlatformDiscovery",
            "builds": [
                'go -C biz build -o "$RUNNER_TEMP/ce13-plan-biz" ./cmd/biz',
                'go -C biz build -o "$RUNNER_TEMP/ce13-plan-idp" ./cmd/biz-idp',
            ],
        },
        "session": {
            "workflow": "ce13-platform-web-session.yml",
            "browser_config": "web/playwright.ce13-platform.config.ts",
            "acceptance": "TestCE13PlatformCommercialTrustedWebSession",
            "builds": [
                'go -C biz build -o "$RUNNER_TEMP/ce13-session-biz" ./cmd/biz',
                'go -C biz build -o "$RUNNER_TEMP/ce13-session-idp" ./cmd/biz-idp',
                'go -C biz build -o "$RUNNER_TEMP/ce13-session-credential" ./cmd/biz-idp-credential',
            ],
        },
    }
    for lane, spec in specs.items():
        text = (root / ".github/workflows" / spec["workflow"]).read_text()
        require("services:" not in text, "CE13_SERVICE_MYSQL_REINTRODUCED:" + lane)
        require("cache: true" in text and "cache-dependency-path: biz/go.sum" in text,
                "CE13_GO_CACHE_REQUIRED:" + lane)
        require(f"ci_ce13_mysql.sh start {lane}" in text and
                f"ci_ce13_mysql.sh wait {lane}" in text and
                f"ci_ce13_mysql.sh stop {lane}" in text,
                "CE13_OWNED_MYSQL_REQUIRED:" + lane)
        browser_start = f"ci_ce13_browser.sh start {lane}"
        require(browser_start in text and
                f"ci_ce13_browser.sh wait {lane}" in text and
                f"ci_ce13_browser.sh stop {lane}" in text,
                "CE13_BROWSER_PREP_REQUIRED:" + lane)
        require(text.index(browser_start) < text.index("make -C biz generate"),
                "CE13_BROWSER_PREP_PARALLELISM_REQUIRED:" + lane)
        isolated_root = f'cd "$RUNNER_TEMP/ce13-{lane}/web"'
        required_isolated_uses = 2 if lane == "session" else 1
        require(text.count(isolated_root) >= required_isolated_uses,
                "CE13_BROWSER_ISOLATED_WORKSPACE_REQUIRED:" + lane)
        require("go -C biz run" not in text, "CE13_GO_RUN_RECOMPILE_REINTRODUCED:" + lane)
        for marker in spec["builds"]:
            require(marker in text, "CE13_PREBUILT_RUNTIME_MISSING:" + lane + ":" + marker)
        require(spec["acceptance"] in text, "CE13_BROWSER_ACCEPTANCE_MISSING:" + lane)
        require("Start CE13 performance clock" in text and
                "Write CE13 performance receipt" in text and
                "scripts/ci_performance_receipt.py" in text,
                "CE13_PERFORMANCE_RECEIPT_MISSING:" + lane)
        require("timeout-minutes: 5" in text, "CE13_TIMEOUT_DRIFT:" + lane)

        config = (root / spec["browser_config"]).read_text()
        require("fullyParallel: false" in config and "workers: 1" in config,
                "CE13_BROWSER_SERIAL_CONTRACT_DRIFT:" + lane)

    plan = (root / ".github/workflows/ce13-plan-catalog-qualification.yml").read_text()
    require("TestCE07MySQL" not in plan, "CE13_PLAN_CE07_OVERLAP_REINTRODUCED")

    helper = (root / "scripts/ci_ce13_mysql.sh").read_text()
    require("GITHUB_ACTIONS" in helper and "ci.batch-b" in helper,
            "CE13_MYSQL_OWNERSHIP_GUARD_MISSING")
    require("--tmpfs" not in helper, "CE13_DURABLE_DB_REQUIRED")

    browser_helper = (root / "scripts/ci_ce13_browser.sh").read_text()
    require("GITHUB_ACTIONS" in browser_helper,
            "CE13_BROWSER_OWNERSHIP_GUARD_MISSING")
    require("npx --no-install playwright install --only-shell chromium" in browser_helper,
            "CE13_HEADLESS_ONLY_REQUIRED")
    require("--with-deps" not in browser_helper and "playwright install-deps" not in browser_helper,
            "CE13_FULL_OS_BOOTSTRAP_REINTRODUCED")
    require("chromium.launch({ headless: true, timeout: 20000 })" in browser_helper and
            "await browser.close()" in browser_helper and
            "CE13_BROWSER_LAUNCH=PASS" in browser_helper and
            browser_helper.index("CE13_BROWSER_LAUNCH=PASS") < browser_helper.index('date +%s > "$out/browser-prep.ready"'),
            "CE13_REAL_BROWSER_PROBE_REQUIRED")
    require("CE13_CHINESE_FONT_UNAVAILABLE" in browser_helper,
            "CE13_FONT_READINESS_GUARD_MISSING")
    require('source_web="${GITHUB_WORKSPACE:?}/biz/web"' in browser_helper and
            'isolated_web="$out/web"' in browser_helper and
            'tar -C "$source_web"' in browser_helper and
            'cd "$isolated_web"' in browser_helper,
            "CE13_BROWSER_ISOLATION_MISSING")
    require('cd "${GITHUB_WORKSPACE:?}/biz/web"' not in browser_helper,
            "CE13_BROWSER_SOURCE_TREE_WRITE_REINTRODUCED")
    require('fc-match "Noto Sans CJK SC"' in browser_helper,
            "CE13_FONT_PROBE_REQUIRED")

    proof = json.loads((root / "scripts/ci_proof_contract.json").read_text())
    for workflow in [
        "ce13-plan-catalog-qualification.yml",
        "ce13-platform-web-session.yml",
    ]:
        gate = proof["gates"][workflow]
        require(gate["target_seconds"] == 120 and gate["hard_seconds"] == 180,
                "CE13_PERFORMANCE_RATCHET_DRIFT:" + workflow)
        require(gate["legacy_cost_ceiling"] == {},
                "CE13_LEGACY_DEBT_RATCHET_DRIFT:" + workflow)
        workflow_text = (root / ".github/workflows" / workflow).read_text()
        require("performance_target_seconds:\n        type: number\n        required: false\n        default: 120" in workflow_text,
                "CE13_WORKFLOW_TARGET_DRIFT:" + workflow)
        require("performance_hard_seconds:\n        type: number\n        required: false\n        default: 180" in workflow_text,
                "CE13_WORKFLOW_HARD_DRIFT:" + workflow)

    prq = (root / ".github/workflows/pr-qualification.yml").read_text()
    for job, workflow in [
        ("commercial-batch-ce13-plan", "ce13-plan-catalog-qualification.yml"),
        ("commercial-batch-ce13-session", "ce13-platform-web-session.yml"),
    ]:
        m = re.search(rf"(?ms)^  {re.escape(job)}:\n(?P<body>.*?)(?=^  [A-Za-z0-9_-]+:|\Z)", prq)
        require(m is not None, "CE13_TARGET_JOB_MISSING:" + job)
        block = m.group(0)
        for marker in [
            "needs: [route, governance, fast-web]",
            f"uses: ./.github/workflows/{workflow}",
            "enforce_performance: true",
            "performance_target_seconds: 120",
            "performance_hard_seconds: 180",
        ]:
            require(marker in block, "CE13_TARGET_MARKER_MISSING:" + job + ":" + marker)

    for lane, spec in specs.items():
        text = (root / ".github/workflows" / spec["workflow"]).read_text()
        check_environment(root, text, lane)

    print("CE13_BATCH_B_SOURCE_CONTRACT=PASS")

if __name__ == "__main__":
    check()
