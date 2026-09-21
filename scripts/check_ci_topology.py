#!/usr/bin/env python3
"""Repository CI topology contract.

This checker protects the control-plane shape. Business PRs may extend tests inside
canonical reusable gates, but may not create new direct event entrypoints, grow the
Full Merge Gate, or raise CI budgets.
"""
from __future__ import annotations

import argparse
import json
import os
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
WORKFLOWS = ROOT / ".github" / "workflows"
CONTRACT = ROOT / "scripts" / "ci_topology_contract.json"
MANIFEST = ROOT / "scripts" / "ci_qualification_manifest.json"
BUDGET = ROOT / "scripts" / "ci_qualification_budget.json"


def on_children(text: str) -> set[str]:
    result: set[str] = set()
    lines = text.splitlines()
    in_on = False
    for line in lines:
        if line == "on:":
            in_on = True
            continue
        if in_on and line and not line.startswith((" ", "\t")):
            break
        if in_on:
            match = re.match(r"^  ([A-Za-z0-9_-]+)\s*:", line)
            if match:
                result.add(match.group(1))
    return result


def push_targets_main(text: str) -> bool:
    lines = text.splitlines()
    start = None
    in_on = False
    for index, line in enumerate(lines):
        if line == "on:":
            in_on = True
            continue
        if in_on and line and not line.startswith((" ", "\t")):
            break
        if in_on and line == "  push:":
            start = index
            break
    if start is None:
        return False

    block: list[str] = []
    for line in lines[start + 1:]:
        if re.match(r"^  [A-Za-z0-9_-]+\s*:", line):
            break
        if line and not line.startswith((" ", "\t")):
            break
        block.append(line)

    for line in block:
        inline = re.match(r"^    branches:\s*\[(.*)\]\s*$", line)
        if inline:
            values = [item.strip().strip("'\"") for item in inline.group(1).split(",") if item.strip()]
            return "main" in values

    for index, line in enumerate(block):
        if line == "    branches:":
            values: list[str] = []
            for child in block[index + 1:]:
                match = re.match(r"^      -\s+(.+?)\s*$", child)
                if not match:
                    break
                values.append(match.group(1).strip().strip("'\""))
            return "main" in values

    # An unscoped push includes main and is therefore a main entrypoint.
    return True


def workflow_uses(text: str) -> list[str]:
    return re.findall(r"uses:\s+\./\.github/workflows/([^\s]+)", text)


def git_show(ref: str, path: str) -> str | None:
    proc = subprocess.run(
        ["git", "show", f"{ref}:{path}"],
        cwd=ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    return proc.stdout if proc.returncode == 0 else None


def governance_branch() -> bool:
    head = os.getenv("GITHUB_HEAD_REF", "")
    return head.startswith("chore/ci-")


def validate(base_ref: str | None = None) -> list[str]:
    errors: list[str] = []
    contract = json.loads(CONTRACT.read_text(encoding="utf-8"))
    manifest = json.loads(MANIFEST.read_text(encoding="utf-8"))
    budget = json.loads(BUDGET.read_text(encoding="utf-8"))

    if contract.get("schema_version") != 1:
        errors.append("CI topology contract schema_version must be 1")

    expected_full = contract["full_merge_gate"]["expected_workflows"]
    actual_full = [item["file"] for item in manifest.get("workflows", [])]
    expected_units = int(contract["full_merge_gate"]["expected_units"])
    if actual_full != expected_full:
        errors.append("qualification manifest order/set drifted from CI topology contract")
    if len(actual_full) != expected_units:
        errors.append(f"Full Merge Gate manifest has {len(actual_full)} units; expected {expected_units}")

    for key, expected in contract["budgets"].items():
        actual = budget.get(key)
        if actual != expected:
            errors.append(f"CI budget {key}={actual!r}; topology contract requires {expected!r}")

    workflows = {path.name: path for path in WORKFLOWS.glob("*.yml")}

    mysql_budget = contract.get("performance_budgets", {}).get("mysql_runtime_qualification")
    if mysql_budget:
        mysql_workflow = mysql_budget["workflow"]
        mysql_path = workflows.get(mysql_workflow)
        if mysql_path is None:
            errors.append(f"MySQL performance workflow missing: {mysql_workflow}")
        else:
            mysql_text = mysql_path.read_text(encoding="utf-8")
            timeouts = [
                int(value)
                for value in re.findall(r"^    timeout-minutes:\s*(\d+)\s*$", mysql_text, re.MULTILINE)
            ]
            max_minutes = int(mysql_budget["max_job_minutes"])
            if len(timeouts) != 1:
                errors.append(
                    f"{mysql_workflow}: expected exactly one timed qualification job; found {len(timeouts)}"
                )
            elif timeouts[0] > max_minutes:
                errors.append(
                    f"{mysql_workflow}: timeout {timeouts[0]}m exceeds MySQL budget {max_minutes}m"
                )
            image = mysql_budget.get("image")
            if image and f"mysql:{image.split(':', 1)[-1]}" not in mysql_text:
                errors.append(f"{mysql_workflow}: MySQL image drifted from {image}")
            if expected_full.count(mysql_workflow) != 1:
                errors.append(
                    f"Full Merge Gate must contain exactly one consolidated MySQL workflow {mysql_workflow}"
                )
    ce08_budget = contract.get("performance_budgets", {}).get("ce08_qualification")
    if ce08_budget:
        ce08_workflow = ce08_budget["workflow"]
        ce08_path = workflows.get(ce08_workflow)
        shard_manifest_path = ROOT / ce08_budget["shard_manifest"]
        if ce08_path is None:
            errors.append(f"CE08 performance workflow missing: {ce08_workflow}")
        else:
            ce08_text = ce08_path.read_text(encoding="utf-8")
            timeouts = [
                int(value)
                for value in re.findall(r"^    timeout-minutes:\s*(\d+)\s*$", ce08_text, re.MULTILINE)
            ]
            max_minutes = int(ce08_budget["max_job_minutes"])
            if len(timeouts) != 1:
                errors.append(
                    f"{ce08_workflow}: expected exactly one timed qualification job; found {len(timeouts)}"
                )
            elif timeouts[0] > max_minutes:
                errors.append(
                    f"{ce08_workflow}: timeout {timeouts[0]}m exceeds CE08 budget {max_minutes}m"
                )
        if not shard_manifest_path.exists():
            errors.append(f"CE08 shard manifest missing: {shard_manifest_path.relative_to(ROOT)}")
        else:
            shard_manifest = json.loads(shard_manifest_path.read_text(encoding="utf-8"))
            manifest_tests = [
                test
                for shard in shard_manifest.get("shards", [])
                for test in shard.get("tests", [])
            ]
            if len(manifest_tests) != len(set(manifest_tests)):
                errors.append("CE08 shard manifest contains duplicate tests")
            source_tests: set[str] = set()
            for source in (ROOT / "integration").glob("*.go"):
                text = source.read_text(encoding="utf-8")
                source_tests.update(
                    re.findall(r"^func (TestCE08MySQL[A-Za-z0-9_]+)\(t \*testing\.T\)", text, re.MULTILINE)
                )
            if set(manifest_tests) != source_tests:
                errors.append(
                    "CE08 shard manifest exact-set drifted from integration tests: "
                    f"missing={sorted(source_tests - set(manifest_tests))} "
                    f"stale={sorted(set(manifest_tests) - source_tests)}"
                )
            expected_race = set(ce08_budget.get("expected_race_tests", []))
            actual_race = set(shard_manifest.get("race_tests", []))
            if actual_race != expected_race:
                errors.append(
                    f"CE08 race set drifted: {sorted(actual_race)}; expected {sorted(expected_race)}"
                )
            if not actual_race.issubset(source_tests):
                errors.append("CE08 race set references tests outside CE08 MySQL coverage")
            delegated = shard_manifest.get("delegated_full_gate_coverage", [])
            expected_delegated = ce08_budget.get("delegated_full_gate_coverage", [])
            if delegated != expected_delegated:
                errors.append("CE08 delegated Full Gate coverage drifted from topology contract")
            for workflow in delegated:
                if workflow not in expected_full:
                    errors.append(f"CE08 delegated workflow missing from Full Merge Gate: {workflow}")

        ce08_script_path = ROOT / "scripts" / "ce08_qualify.sh"
        ce08_script = ce08_script_path.read_text(encoding="utf-8")
        syntax = subprocess.run(
            ["bash", "-n", str(ce08_script_path)],
            cwd=ROOT,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )
        if syntax.returncode != 0:
            errors.append(
                "CE08 qualification shell syntax invalid: "
                + (syntax.stderr.strip() or syntax.stdout.strip())
            )
        if ce08_path is not None:
            ce08_text = ce08_path.read_text(encoding="utf-8")
            if "--tmpfs /var/lib/mysql:rw,nosuid,size=1g" not in ce08_text:
                errors.append("CE08 shard runtime lost tmpfs acceleration")
        restart_markers = [
            'restart_container="ce08-restart-',
            "3307:3306",
            'docker restart "$restart_container"',
            "biz_ce08_restart",
        ]
        for marker in restart_markers:
            if marker not in ce08_script:
                errors.append(f"CE08 restart persistence proof lost isolated durable runtime marker: {marker}")
        if 'docker restart "$MYSQL_CONTAINER_ID"' in ce08_script:
            errors.append("CE08 restart proof must not restart the tmpfs shard service")
        forbidden_ce08_duplicates = [
            "^TestCE07MySQL",
            "^TestCE06MySQL",
            "^TestCE05MySQL",
            "^TestCE04MySQL",
            "^TestCE02",
            "TestB122TenantLifecycleRESTAndGRPCUseUnifiedExecutor",
            "go test -count=1 -json ./...",
            "go vet ./...",
            "go build ./...",
            "-race -count=1 -tags=integration -json ./integration -run '^TestCE08MySQL'",
        ]
        for marker in forbidden_ce08_duplicates:
            if marker in ce08_script:
                errors.append(f"CE08 qualification reintroduced delegated duplicate coverage: {marker}")

    b127_resilience = contract.get("resilience_contracts", {}).get("b12_7_runtime_evidence_upload")
    if b127_resilience:
        workflow = b127_resilience["workflow"]
        path = workflows.get(workflow)
        if path is None:
            errors.append(f"B12.7 evidence resilience workflow missing: {workflow}")
        else:
            text = path.read_text(encoding="utf-8")
            action = "uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a"
            if text.count(action) != int(b127_resilience["attempts"]):
                errors.append(
                    f"{workflow}: B12.7 evidence upload attempts drifted; "
                    f"found {text.count(action)} expected {b127_resilience['attempts']}"
                )
            required_markers = [
                "id: upload-runtime-evidence",
                "continue-on-error: true",
                "steps.upload-runtime-evidence.outcome == 'failure'",
                "if-no-files-found: error",
                "overwrite: true",
            ]
            for marker in required_markers:
                if marker not in text:
                    errors.append(f"{workflow}: B12.7 evidence resilience marker missing: {marker}")
            if text.count("if-no-files-found: error") != int(b127_resilience["attempts"]):
                errors.append(f"{workflow}: every B12.7 upload attempt must fail on missing evidence")

    web_budget = contract.get("performance_budgets", {}).get("coffeelink_web")
    if web_budget:
        workflow = web_budget["workflow"]
        path = workflows.get(workflow)
        if path is None:
            errors.append(f"CoffeeLink Web performance workflow missing: {workflow}")
        else:
            text = path.read_text(encoding="utf-8")
            timeouts = [
                int(value)
                for value in re.findall(r"^    timeout-minutes:\s*(\d+)\s*$", text, re.MULTILINE)
            ]
            max_minutes = int(web_budget["max_job_minutes"])
            if len(timeouts) != 1:
                errors.append(
                    f"{workflow}: expected exactly one timed web qualification job; found {len(timeouts)}"
                )
            elif timeouts[0] > max_minutes:
                errors.append(
                    f"{workflow}: timeout {timeouts[0]}m exceeds CoffeeLink budget {max_minutes}m"
                )

            required_markers = [
                "skip_fast_check:",
                "default: false",
                "if: ${{ !inputs.skip_fast_check }}",
                "Build default CoffeeLink bundle",
                "run: npx vite build",
                f"npx playwright test --workers={int(web_budget['default_e2e_workers'])}",
                "npx playwright install --with-deps chromium",
                "Verify visual contract evidence",
            ]
            for marker in required_markers:
                if marker not in text:
                    errors.append(f"{workflow}: CoffeeLink performance/coverage marker missing: {marker}")

            forbidden_direct = [
                "VITE_DATA_MODE=api",
                "ENTERPRISE_MEMBER_REAL_E2E",
                "ENTERPRISE_ROLE_REAL_E2E",
                "e2e/enterprise-members-real.spec.ts",
                "e2e/enterprise-roles-real.spec.ts",
                "npm run test:e2e",
                "enterprise_required",
            ]
            for marker in forbidden_direct:
                if marker in text:
                    errors.append(f"{workflow}: CoffeeLink delegated/long-tail regression reintroduced: {marker}")

        for spec_path in web_budget.get("parallel_specs", []):
            spec = ROOT / spec_path
            if not spec.exists():
                errors.append(f"CoffeeLink parallel spec missing: {spec_path}")
                continue
            spec_text = spec.read_text(encoding="utf-8")
            if "test.describe.configure({ mode: 'parallel' })" not in spec_text:
                errors.append(f"CoffeeLink parallel spec lost file-level parallelism: {spec_path}")

        for delegated_workflow, spec_path in web_budget.get("delegated_api_e2e", {}).items():
            if delegated_workflow not in expected_full:
                errors.append(f"CoffeeLink delegated API workflow missing from Full Gate: {delegated_workflow}")
                continue
            delegated_path = workflows.get(delegated_workflow)
            if delegated_path is None:
                errors.append(f"CoffeeLink delegated API workflow file missing: {delegated_workflow}")
                continue
            delegated_text = delegated_path.read_text(encoding="utf-8")
            required = ["VITE_DATA_MODE=api", spec_path]
            if spec_path.endswith("enterprise-members-real.spec.ts"):
                required.append("ENTERPRISE_MEMBER_REAL_E2E=1")
            if spec_path.endswith("enterprise-roles-real.spec.ts"):
                required.append("ENTERPRISE_ROLE_REAL_E2E=1")
            for marker in required:
                if marker not in delegated_text:
                    errors.append(
                        f"{delegated_workflow}: delegated CoffeeLink API proof marker missing: {marker}"
                    )

        merge_text = workflows["pr-merge-gate.yml"].read_text(encoding="utf-8")
        merge_block_match = re.search(
            r"(?ms)^  full-20-coffeelink-web:\n(?P<body>.*?)(?=^  [A-Za-z0-9_-]+:|\Z)",
            merge_text,
        )
        if not merge_block_match:
            errors.append("PR Merge Gate CoffeeLink Web job missing")
        else:
            merge_block = merge_block_match.group(0)
            required_merge = [
                "needs: [route, wait-qualification]",
                "needs.wait-qualification.result == 'success'",
                "uses: ./.github/workflows/coffeelink-web.yml",
                "skip_fast_check: true",
            ]
            for marker in required_merge:
                if marker not in merge_block:
                    errors.append(f"PR Merge Gate CoffeeLink proof-reuse marker missing: {marker}")

    pull_entrypoints = sorted(
        name for name, path in workflows.items()
        if "pull_request" in on_children(path.read_text(encoding="utf-8"))
    )
    expected_pull = sorted(contract["control_plane"]["pull_request_entrypoints"])
    if pull_entrypoints != expected_pull:
        errors.append(f"pull_request entrypoints={pull_entrypoints}; expected {expected_pull}")

    main_push = sorted(
        name for name, path in workflows.items()
        if push_targets_main(path.read_text(encoding="utf-8"))
    )
    expected_main = sorted(contract["control_plane"]["main_push_entrypoints"])
    if main_push != expected_main:
        errors.append(f"main push entrypoints={main_push}; expected {expected_main}")

    allowed_reusable_events = set(contract["control_plane"]["reusable_gate_allowed_events"])
    for name in contract["reusable_workflows"]:
        path = workflows.get(name)
        if path is None:
            errors.append(f"reusable workflow missing: {name}")
            continue
        events = on_children(path.read_text(encoding="utf-8"))
        if "workflow_call" not in events:
            errors.append(f"{name}: reusable gate missing workflow_call")
        illegal = events - allowed_reusable_events
        if illegal:
            errors.append(f"{name}: reusable gate self-triggers via {sorted(illegal)}")

    for name in contract.get("forbidden_workflows", []):
        if name in workflows:
            errors.append(f"obsolete issue-specific workflow must be removed: {name}")

    prq = workflows["pr-qualification.yml"].read_text(encoding="utf-8")
    role_gate = contract["canonical_domain_gates"]["enterprise_role"]
    if f"uses: ./.github/workflows/{role_gate}" not in prq:
        errors.append("PR Qualification does not route role changes through canonical role gate")

    legacy_allow = set(contract.get("legacy_issue_named_pr_gate_allowlist", []))
    for name in workflow_uses(prq):
        if re.match(r"^enterprise-\d+-", name) and name not in legacy_allow:
            errors.append(f"PR Qualification references issue-numbered permanent gate: {name}")

    merge = workflows["pr-merge-gate.yml"].read_text(encoding="utf-8")
    full_jobs = [
        match.group(1)
        for match in re.finditer(r"^  (full-[^:]+):", merge, re.MULTILINE)
    ]
    if len(full_jobs) != expected_units:
        errors.append(f"PR Merge Gate declares {len(full_jobs)} full jobs; expected {expected_units}")
    expected_full_jobs = [
        f"full-{index:02d}-{name.removesuffix('.yml')}"
        for index, name in enumerate(expected_full, start=1)
    ]
    if full_jobs != expected_full_jobs:
        errors.append(
            f"PR Merge Gate full job names/order drifted: {full_jobs}; expected {expected_full_jobs}"
        )
    for name in expected_full:
        count = merge.count(f"uses: ./.github/workflows/{name}")
        if count != 1:
            errors.append(f"PR Merge Gate must call {name} exactly once; found {count}")
    for name in contract.get("forbidden_workflows", []):
        if name in merge or name in prq:
            errors.append(f"control plane still references obsolete workflow: {name}")

    if base_ref:
        base_contract_raw = git_show(base_ref, "scripts/ci_topology_contract.json")
        current_contract_raw = CONTRACT.read_text(encoding="utf-8")
        if base_contract_raw is not None and json.loads(base_contract_raw) != json.loads(current_contract_raw):
            if not governance_branch():
                errors.append(
                    "CI topology contract changed outside chore/ci-* governance branch"
                )

        base_budget_raw = git_show(base_ref, "scripts/ci_qualification_budget.json")
        if base_budget_raw is not None:
            base_budget = json.loads(base_budget_raw)
            for key, current in contract["budgets"].items():
                before = base_budget.get(key)
                if isinstance(before, int) and isinstance(current, int) and current > before and not governance_branch():
                    errors.append(
                        f"CI budget increase {key}: {before}->{current} requires chore/ci-* governance branch"
                    )

    return errors


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-ref", default="")
    args = parser.parse_args()
    errors = validate(args.base_ref or None)
    if errors:
        for error in errors:
            print(f"CI-TOPOLOGY: {error}", file=sys.stderr)
        return 1
    print("CI-TOPOLOGY: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
