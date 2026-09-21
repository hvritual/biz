#!/usr/bin/env python3
"""Static governance check for #205 PR qualification topology."""
from __future__ import annotations

import json
import pathlib
import re
import sys

from check_ci_topology import validate as validate_topology

ROOT = pathlib.Path(__file__).resolve().parents[1]
WORKFLOWS = ROOT / ".github" / "workflows"
MANIFEST = ROOT / "scripts" / "ci_qualification_manifest.json"
BUDGET = ROOT / "scripts" / "ci_qualification_budget.json"
LIFECYCLE = ROOT / "scripts" / "candidate_lifecycle.json"
PR_ENTRYPOINTS = {"pr-qualification.yml", "pr-merge-gate.yml"}
FAST_GATE = "candidate-qualification.yml"
MAIN_ENTRYPOINT = "main-receipt.yml"

DOMAIN_GATE_UNITS = {
    "b12-4-tenant-role-mysql.yml",
    "b12-multitenant-access-pressure.yml",
    "enterprise-role-qualification.yml",
    "ce03-qualification.yml",
    "ce10-qualification.yml",
    "coffeelink-web.yml",
    "c9-pressure.yml",
    "b12-8-framework-pressure-disposition.yml",
    "evolution-qualification.yml",
    "enterprise-172-native-login.yml",
    "delivery-workspace-isolation.yml",
    "web-e2e-harness.yml",
}

REQUIRED_LIFECYCLE_STATES = [
    "WORKING",
    "FAST_VERIFIED",
    "DOMAIN_QUALIFIED",
    "CANDIDATE_FROZEN",
    "MERGE_QUALIFYING",
    "MERGE_READY",
    "MERGED",
    "MAIN_VERIFIED",
]
REQUIRED_LIFECYCLE_INVARIANTS = {
    "candidate_frozen_requires_current_main_ancestor",
    "head_change_invalidates_frozen_candidate",
    "full_merge_gate_requires_non_draft_pr",
    "qualification_is_read_only",
    "full_merge_gate_runs_once_per_candidate_sha",
    "main_verified_requires_exact_remote_main_tip",
}


def has_top_level_key(text: str, key: str) -> bool:
    return re.search(rf"^{re.escape(key)}\s*:", text, re.MULTILINE) is not None


def has_on_child(text: str, key: str) -> bool:
    lines = text.splitlines()
    in_on = False
    for line in lines:
        if line == "on:":
            in_on = True
            continue
        if in_on and line and not line.startswith((" ", "\t")):
            return False
        if in_on and re.match(rf"^  {re.escape(key)}\s*:", line):
            return True
    return False


def push_targets_main(text: str) -> bool:
    lines = text.splitlines()
    in_on = False
    push_index = None
    for index, line in enumerate(lines):
        if line == "on:":
            in_on = True
            continue
        if in_on and line and not line.startswith((" ", "\t")):
            break
        if in_on and line == "  push:":
            push_index = index
            break
    if push_index is None:
        return False

    block: list[str] = []
    for line in lines[push_index + 1:]:
        if re.match(r"^  [A-Za-z0-9_-]+\s*:", line):
            break
        if line and not line.startswith((" ", "\t")):
            break
        block.append(line)

    for line in block:
        inline = re.match(r"^    branches:\s*\[(.*)\]\s*$", line)
        if inline:
            branches = [
                item.strip().strip("'\"")
                for item in inline.group(1).split(",")
                if item.strip()
            ]
            return "main" in branches

    for index, line in enumerate(block):
        if line == "    branches:":
            branches: list[str] = []
            for child in block[index + 1:]:
                match = re.match(r"^      -\s+(.+?)\s*$", child)
                if not match:
                    break
                branches.append(match.group(1).strip().strip("'\""))
            return "main" in branches

    return True


def reusable_call(file: str) -> str:
    return f"uses: ./.github/workflows/{file}"


def validate(root: pathlib.Path = ROOT) -> list[str]:
    errors: list[str] = []
    workflows = root / ".github" / "workflows"
    manifest_path = root / "scripts" / "ci_qualification_manifest.json"
    data = json.loads(manifest_path.read_text(encoding="utf-8"))
    units = data.get("workflows", [])
    if len(units) < 1:
        errors.append("qualification manifest is empty")

    if not BUDGET.exists():
        errors.append("missing CI qualification budget contract")
        budget = {}
    else:
        budget = json.loads(BUDGET.read_text(encoding="utf-8"))
    if not LIFECYCLE.exists():
        errors.append("missing Candidate lifecycle contract")
        lifecycle = {}
    else:
        lifecycle = json.loads(LIFECYCLE.read_text(encoding="utf-8"))
        if lifecycle.get("states") != REQUIRED_LIFECYCLE_STATES:
            errors.append("Candidate lifecycle state ordering drifted")
        invariants = lifecycle.get("invariants", {})
        for invariant in REQUIRED_LIFECYCLE_INVARIANTS:
            if invariants.get(invariant) is not True:
                errors.append(f"Candidate lifecycle invariant disabled: {invariant}")

    max_domain = int(budget.get("max_pr_domain_gate_units", 0) or 0)
    if max_domain < len(DOMAIN_GATE_UNITS):
        errors.append(
            f"PR Domain Gate budget {max_domain} is below configured units {len(DOMAIN_GATE_UNITS)}"
        )

    manifest_names: set[str] = set()
    for item in units:
        name = item["file"]
        if name in manifest_names:
            errors.append(f"duplicate qualification workflow in manifest: {name}")
            continue
        manifest_names.add(name)
        path = workflows / name
        if not path.exists():
            errors.append(f"missing qualification workflow: {name}")
            continue
        text = path.read_text(encoding="utf-8")
        if has_on_child(text, "pull_request"):
            errors.append(f"{name}: reusable qualification unit must not self-trigger on pull_request")
        if not has_on_child(text, "workflow_call"):
            errors.append(f"{name}: missing on.workflow_call")
        # PR stale-run cancellation belongs to the caller. Keeping top-level
        # concurrency in called workflows is unsafe because github.workflow in a
        # reusable workflow resolves to the caller workflow name.
        if has_top_level_key(text, "concurrency"):
            errors.append(f"{name}: reusable unit must not own top-level PR concurrency")
        if re.search(r"contents\s*:\s*write", text):
            errors.append(f"{name}: qualification may not request contents: write")
        if re.search(r"\bgit\s+push\b", text):
            errors.append(f"{name}: qualification may not push candidate changes")

    fast_path = workflows / FAST_GATE
    if not fast_path.exists():
        errors.append(f"missing fast gate: {FAST_GATE}")
    else:
        text = fast_path.read_text(encoding="utf-8")
        if not has_on_child(text, "workflow_call"):
            errors.append(f"{FAST_GATE}: must be reusable through workflow_call")
        if has_on_child(text, "pull_request"):
            errors.append(f"{FAST_GATE}: direct pull_request trigger bypasses the PR control plane")
        if re.search(r"contents\s*:\s*write", text) or re.search(r"\bgit\s+push\b", text):
            errors.append(f"{FAST_GATE}: fast gate must be read-only")

    for path in sorted(workflows.glob("*.yml")):
        if path.name in PR_ENTRYPOINTS:
            continue
        text = path.read_text(encoding="utf-8")
        if has_on_child(text, "pull_request"):
            errors.append(
                f"{path.name}: direct pull_request trigger bypasses the two-entry PR control plane"
            )

    main_push_entrypoints: list[str] = []
    for path in sorted(workflows.glob("*.yml")):
        text = path.read_text(encoding="utf-8")
        if push_targets_main(text):
            main_push_entrypoints.append(path.name)
    if main_push_entrypoints != [MAIN_ENTRYPOINT]:
        errors.append(
            "main push must have exactly one direct entrypoint "
            f"{MAIN_ENTRYPOINT}; found {main_push_entrypoints}"
        )

    expected_main = budget.get("expected_main_push_entrypoint_workflows", [])
    max_main = int(budget.get("max_main_push_entrypoint_workflows", 0) or 0)
    if expected_main != ["Main Qualification"]:
        errors.append("CI budget must declare Main Qualification as the only main push entrypoint")
    if max_main != 1:
        errors.append("CI budget must cap main push entrypoint workflows at 1")

    for name in PR_ENTRYPOINTS:
        path = workflows / name
        if not path.exists():
            errors.append(f"missing PR entrypoint: {name}")
            continue
        text = path.read_text(encoding="utf-8")
        if not has_on_child(text, "pull_request"):
            errors.append(f"{name}: must trigger from pull_request")
        if not has_top_level_key(text, "concurrency") or "cancel-in-progress: true" not in text:
            errors.append(f"{name}: missing PR-level cancel-in-progress concurrency")
        if re.search(r"contents\s*:\s*write", text) or re.search(r"\bgit\s+push\b", text):
            errors.append(f"{name}: PR entrypoint must be read-only")

    qualification_path = workflows / "pr-qualification.yml"
    if qualification_path.exists():
        text = qualification_path.read_text(encoding="utf-8")
        if reusable_call(FAST_GATE) not in text:
            errors.append("pr-qualification.yml: missing Fast Gate")
        for name in DOMAIN_GATE_UNITS:
            if reusable_call(name) not in text:
                errors.append(f"pr-qualification.yml: missing bounded Domain Gate unit {name}")

        domain_calls = sum(text.count(reusable_call(name)) for name in DOMAIN_GATE_UNITS)
        if max_domain and domain_calls > max_domain:
            errors.append(
                f"pr-qualification.yml: Domain Gate fan-out {domain_calls} exceeds budget {max_domain}"
            )
        if "candidate_lifecycle.py validate-contract" not in text or "test_candidate_lifecycle.py" not in text:
            errors.append("pr-qualification.yml: Candidate lifecycle contract/tests are not gated before Domain Gate")
        if "ready_for_review" in text:
            errors.append("pr-qualification.yml: Ready transition must not rerun Draft qualification")
        for bounded in (
            "enterprise-172-native-login.yml",
            "delivery-workspace-isolation.yml",
            "ce-round-receipts.yml",
            "web-e2e-harness.yml",
        ):
            if reusable_call(bounded) not in text:
                errors.append(f"pr-qualification.yml: missing bounded reusable unit {bounded}")
        if "skip_fast_check: true" not in text:
            errors.append("pr-qualification.yml: Product Web Domain Gate must reuse Fast Gate result")

    merge_path = workflows / "pr-merge-gate.yml"
    if merge_path.exists():
        text = merge_path.read_text(encoding="utf-8")
        if "github.event.pull_request.draft == false" not in text:
            errors.append("pr-merge-gate.yml: Full Merge Gate must be disabled for Draft PRs")
        if "types: [ready_for_review]" not in text:
            errors.append("pr-merge-gate.yml: must start only when a PR enters Ready state")
        if "Require matching PR Qualification success" not in text:
            errors.append("pr-merge-gate.yml: must wait for matching-head PR Qualification")
        for name in manifest_names:
            call = reusable_call(name)
            if text.count(call) != 1:
                errors.append(f"pr-merge-gate.yml: expected exactly one full-regression call for {name}")

        if text.count("candidate_lifecycle.py assert-fresh") < 2:
            errors.append("pr-merge-gate.yml: must check current-main freshness at freeze and MERGE_READY")
        if "freeze-candidate:" not in text or "merge-ready:" not in text:
            errors.append("pr-merge-gate.yml: missing CANDIDATE_FROZEN/MERGE_READY lifecycle jobs")
        max_full = int(budget.get("max_full_merge_gate_units", 0) or 0)
        if max_full and len(manifest_names) > max_full:
            errors.append(
                f"Full Merge Gate fan-out {len(manifest_names)} exceeds budget {max_full}"
            )

    receipt_path = workflows / MAIN_ENTRYPOINT
    if not receipt_path.exists():
        errors.append(f"missing {MAIN_ENTRYPOINT}")
    else:
        text = receipt_path.read_text(encoding="utf-8")
        if "name: Main Qualification" not in text:
            errors.append(f"{MAIN_ENTRYPOINT}: workflow name must be Main Qualification")
        if not push_targets_main(text):
            errors.append(f"{MAIN_ENTRYPOINT}: must be the main push entrypoint")
        if "main-qualification.mjs" not in text:
            errors.append(f"{MAIN_ENTRYPOINT}: missing merged-PR proof-chain verifier")
        if "candidate_lifecycle.py verify-main" not in text:
            errors.append(f"{MAIN_ENTRYPOINT}: missing exact MAIN_VERIFIED receipt")
        if "actions: read" not in text or "pull-requests: read" not in text:
            errors.append(f"{MAIN_ENTRYPOINT}: missing read permissions for PR proof reuse")
        if re.search(r"contents\s*:\s*write", text) or re.search(r"\bgit\s+push\b", text):
            errors.append(f"{MAIN_ENTRYPOINT}: qualification must be read-only")

        proof_script = root / ".github" / "scripts" / "main-qualification.mjs"
        proof_test = root / ".github" / "scripts" / "main-qualification.test.mjs"
        if not proof_script.exists() or not proof_test.exists():
            errors.append("Main Qualification proof script/tests are missing")
        else:
            proof = proof_script.read_text(encoding="utf-8")
            if "MAIN_UNTRUSTED_DIRECT_PUSH" not in proof:
                errors.append("Main Qualification must fail closed on direct main push")
            if "PR Merge Gate" not in proof or "MAIN_MERGE_GATE_PROOF_MISSING" not in proof:
                errors.append("Main Qualification must require exact Candidate PR Merge Gate proof")

    helper_path = root / "web" / "e2e" / "ui.helpers.ts"
    if not helper_path.exists():
        errors.append("missing web/e2e/ui.helpers.ts")
    else:
        helper = helper_path.read_text(encoding="utf-8")
        if "installApiFailFast" not in helper or "Unhandled API request" not in helper:
            errors.append("ui.helpers.ts: missing shared API fail-fast boundary")

    e2e_root = root / "web" / "e2e"
    if e2e_root.exists():
        for path in sorted(e2e_root.glob("enterprise-*-real.spec.ts")):
            text = path.read_text(encoding="utf-8")
            if "installApiFailFast(" not in text and "Unhandled API request" not in text:
                errors.append(
                    f"{path.name}: API-mode E2E must fail fast on unmocked auth/api/v1 requests"
                )

    # Topology is a stronger repository-level contract layered above individual
    # workflow governance.
    errors.extend(validate_topology())
    return errors


def main() -> int:
    errors = validate()
    if errors:
        for error in errors:
            print(f"CI-GOVERNANCE: {error}", file=sys.stderr)
        return 1
    print("CI-GOVERNANCE: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
