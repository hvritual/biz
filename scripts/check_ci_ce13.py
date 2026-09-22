#!/usr/bin/env python3
"""Static fail-closed contract checks for CE13 Batch B."""
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]

def require(ok, reason):
    if not ok:
        raise ValueError(reason)

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
        require("npx playwright install --with-deps --only-shell chromium" in text,
                "CE13_HEADLESS_ONLY_REQUIRED:" + lane)
        require('fc-match "Noto Sans CJK SC"' in text,
                "CE13_FONT_PROBE_REQUIRED:" + lane)
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

    print("CE13_BATCH_B_SOURCE_CONTRACT=PASS")

if __name__ == "__main__":
    check()
