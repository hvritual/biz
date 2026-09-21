#!/usr/bin/env python3
"""Static governance check for #205 PR qualification topology."""
from __future__ import annotations

import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
WORKFLOWS = ROOT / ".github" / "workflows"
MANIFEST = ROOT / "scripts" / "ci_qualification_manifest.json"
PR_ENTRYPOINTS = {"pr-qualification.yml", "pr-merge-gate.yml"}
FAST_GATE = "candidate-qualification.yml"

DOMAIN_GATE_UNITS = {
    "b12-4-tenant-role-mysql.yml",
    "b12-multitenant-access-pressure.yml",
    "enterprise-178-role-lifecycle.yml",
    "ce03-qualification.yml",
    "ce10-qualification.yml",
    "coffeelink-web.yml",
    "c9-pressure.yml",
    "b12-8-framework-pressure-disposition.yml",
    "evolution-qualification.yml",
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

    merge_path = workflows / "pr-merge-gate.yml"
    if merge_path.exists():
        text = merge_path.read_text(encoding="utf-8")
        if "github.event.pull_request.draft == false" not in text:
            errors.append("pr-merge-gate.yml: Full Merge Gate must be disabled for Draft PRs")
        if "Require matching PR Qualification success" not in text:
            errors.append("pr-merge-gate.yml: must wait for matching-head PR Qualification")
        for name in manifest_names:
            call = reusable_call(name)
            if text.count(call) != 1:
                errors.append(f"pr-merge-gate.yml: expected exactly one full-regression call for {name}")

    # Only the governed heavy units and the Fast Gate are forbidden from
    # self-triggering. Other repository workflows may have independent scopes.
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
