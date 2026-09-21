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
