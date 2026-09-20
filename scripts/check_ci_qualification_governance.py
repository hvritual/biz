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


def validate(root: pathlib.Path = ROOT) -> list[str]:
    errors: list[str] = []
    workflows = root / ".github" / "workflows"
    manifest_path = root / "scripts" / "ci_qualification_manifest.json"
    data = json.loads(manifest_path.read_text(encoding="utf-8"))
    units = data.get("workflows", [])
    if len(units) < 1:
        errors.append("qualification manifest is empty")

    manifest_names = set()
    for item in units:
        name = item["file"]
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
        if not has_top_level_key(text, "concurrency") or "cancel-in-progress: true" not in text:
            errors.append(f"{name}: missing cancel-in-progress concurrency")
        if re.search(r"contents\s*:\s*write", text):
            errors.append(f"{name}: qualification may not request contents: write")
        if re.search(r"\bgit\s+push\b", text):
            errors.append(f"{name}: qualification may not push candidate changes")

    for path in sorted(workflows.glob("*.y*ml")):
        text = path.read_text(encoding="utf-8")
        if has_on_child(text, "pull_request") and path.name not in PR_ENTRYPOINTS:
            errors.append(f"{path.name}: direct pull_request trigger is not an approved PR entrypoint")

    for name in PR_ENTRYPOINTS:
        path = workflows / name
        if not path.exists():
            errors.append(f"missing PR entrypoint: {name}")
            continue
        text = path.read_text(encoding="utf-8")
        if not has_on_child(text, "pull_request"):
            errors.append(f"{name}: must trigger from pull_request")
        if not has_top_level_key(text, "concurrency") or "cancel-in-progress: true" not in text:
            errors.append(f"{name}: missing cancel-in-progress concurrency")
        if re.search(r"contents\s*:\s*write", text) or re.search(r"\bgit\s+push\b", text):
            errors.append(f"{name}: PR entrypoint must be read-only")

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
