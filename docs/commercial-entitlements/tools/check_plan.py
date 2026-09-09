#!/usr/bin/env python3
"""Validate the commercial-entitlement plan, not product implementation.

Standard library only. Run from any working directory. The default plan root is
this script's parent plan directory; --root supports isolated negative probes.
A PASS does not certify the truth of evidence or any runtime/business behavior.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from collections import Counter
from pathlib import Path
from urllib.parse import unquote, urlsplit

TASK_ID = r"CE-\d{2}(?:\.\d+)?"
STATES = {"PLANNED", "IN_PROGRESS", "VERIFYING", "BLOCKED", "DONE"}
CARD_SECTIONS = ("目标与输入", "交付要求", "改动边界", "验收", "回滚")
EVIDENCE_SECTIONS = ("范围与结果", "基线与依赖", "变更清单", "验证命令与结果",
                     "反例与故障验证", "风险与回滚", "集成与回读", "自检")


def validate(root: Path) -> tuple[list[str], dict]:
    errors: list[str] = []
    def require(ok: bool, message: str) -> None:
        if not ok:
            errors.append(message)

    def evidence_file(value: object, context: str) -> Path | None:
        if not isinstance(value, str) or not value.strip():
            errors.append(f"{context}: evidence path required")
            return None
        target = (root / value).resolve()
        if not target.is_relative_to(root):
            errors.append(f"{context}: evidence must stay inside the plan directory")
            return None
        if not target.is_file():
            errors.append(f"{context}: evidence file missing: {value}")
            return None
        return target

    plan = json.loads((root / "tasks.json").read_text(encoding="utf-8"))
    require(plan.get("schema_version") == 1, "unsupported schema_version")
    tasks = plan.get("tasks", [])
    require(isinstance(tasks, list) and bool(tasks), "nonempty tasks array required")
    if not isinstance(tasks, list):
        return errors, {}
    ids = [t.get("id", "") for t in tasks]
    duplicates = [key for key, count in Counter(ids).items() if count > 1]
    require(not duplicates, f"duplicate task IDs: {duplicates}")
    by_id = {t.get("id", ""): t for t in tasks}
    routes = plan.get("routes", {})
    gates = plan.get("external_gates", {})
    requirements_doc = (root / "01-requirements-and-contracts.md").read_text(encoding="utf-8")
    requirements = set(re.findall(r"^\| (FR-\d+) \|", requirements_doc, re.M))
    require(bool(requirements), "no requirement definitions found")
    covered: set[str] = set()

    for task in tasks:
        tid = task.get("id", "")
        require(bool(re.fullmatch(TASK_ID, tid)), f"invalid task ID: {tid}")
        require(task.get("route") in routes, f"{tid}: unknown route")
        require(bool(task.get("title")), f"{tid}: title required")
        require(bool(task.get("owner_role")), f"{tid}: owner_role required")
        require(task.get("status") in STATES, f"{tid}: invalid status")
        dependencies = task.get("depends_on", [])
        require(isinstance(dependencies, list), f"{tid}: depends_on must be an array")
        if not isinstance(dependencies, list):
            dependencies = []
        require(len(dependencies) == len(set(dependencies)), f"{tid}: duplicate dependencies")
        for dependency in dependencies:
            require(dependency in by_id, f"{tid}: unknown dependency {dependency}")
            require(dependency != tid, f"{tid}: self dependency")
        task_gates = task.get("external_gates", [])
        for gate in task_gates:
            require(gate in gates, f"{tid}: unknown external gate {gate}")
        reqs = task.get("requirements", [])
        require(bool(reqs), f"{tid}: requirement coverage required")
        for requirement in reqs:
            require(requirement in requirements, f"{tid}: unknown requirement {requirement}")
            covered.add(requirement)
        if task.get("status") == "BLOCKED":
            require(bool(task.get("blocker")), f"{tid}: BLOCKED requires a blocker")
        if task.get("status") in {"IN_PROGRESS", "VERIFYING", "DONE"}:
            for dependency in dependencies:
                require(by_id.get(dependency, {}).get("status") == "DONE",
                        f"{tid}: active/completed task has unfinished dependency {dependency}")
            for gate in task_gates:
                require(gates.get(gate, {}).get("status") == "SATISFIED",
                        f"{tid}: external gate not satisfied: {gate}")
        if task.get("status") == "DONE":
            evidence = evidence_file(task.get("evidence"), tid)
            require(bool(re.fullmatch(r"[0-9a-f]{40}", task.get("integration_commit") or "")),
                    f"{tid}: DONE requires a full integration commit SHA")
            if evidence:
                text = evidence.read_text(encoding="utf-8")
                for section in EVIDENCE_SECTIONS:
                    require(f"## {section}" in text, f"{tid}: missing evidence section {section}")

    require(requirements <= covered, f"uncovered requirements: {sorted(requirements - covered)}")
    for name, gate in gates.items():
        require(gate.get("status") in {"UNRESOLVED", "SATISFIED"}, f"{name}: invalid gate status")
        require(bool(gate.get("owner_role")), f"{name}: owner_role required")
        if gate.get("status") == "SATISFIED":
            evidence_file(gate.get("evidence"), name)

    # Dependency DAG, including tasks in other routes.
    visiting: set[str] = set()
    visited: set[str] = set()
    order: list[str] = []
    def visit(tid: str) -> None:
        if tid in visiting:
            errors.append(f"dependency cycle at {tid}")
            return
        if tid in visited or tid not in by_id:
            return
        visiting.add(tid)
        for dependency in by_id[tid].get("depends_on", []):
            visit(dependency)
        visiting.remove(tid)
        visited.add(tid)
        order.append(tid)
    for tid in ids:
        visit(tid)

    # One readable card per task, with matching hard dependencies.
    seen_cards: list[str] = []
    for route_id, route in routes.items():
        route_path = (root / route.get("file", "")).resolve()
        require(route_path.is_relative_to(root), f"{route_id}: route path escapes plan")
        if not route_path.is_file():
            errors.append(f"{route_id}: missing route file")
            continue
        text = route_path.read_text(encoding="utf-8")
        matches = list(re.finditer(rf"^## ({TASK_ID}) (.+)$", text, re.M))
        require(bool(matches), f"{route_id}: no task cards")
        for index, match in enumerate(matches):
            tid, title = match.group(1), match.group(2)
            seen_cards.append(tid)
            require(tid in by_id, f"unknown task card {tid}")
            task = by_id.get(tid, {})
            require(task.get("route") == route_id, f"{tid}: route mismatch")
            require(task.get("title") == title, f"{tid}: title mismatch")
            end = matches[index + 1].start() if index + 1 < len(matches) else len(text)
            card = text[match.end():end]
            for section in CARD_SECTIONS:
                require(f"**{section}**" in card, f"{tid}: missing card section {section}")
            line = re.search(r"^依赖：([^\n]+)$", card, re.M)
            require(line is not None, f"{tid}: dependency line required")
            card_dependencies = re.findall(TASK_ID, line.group(1)) if line else []
            require(card_dependencies == task.get("depends_on", []), f"{tid}: card dependencies drift")
            gate_line = re.search(r"^外部条件：([^\n]+)$", card, re.M)
            card_gates = re.findall(r"[A-Z][A-Z0-9_]+", gate_line.group(1)) if gate_line else []
            require(card_gates == task.get("external_gates", []), f"{tid}: card external gates drift")
            require("验收标识：" in card, f"{tid}: test identity required")
    require(Counter(seen_cards) == Counter(ids), "task/card coverage mismatch")

    # Check local Markdown links. External source URLs are not fetched here.
    for document in root.rglob("*.md"):
        text = document.read_text(encoding="utf-8")
        for raw in re.findall(r"(?<!!)\[[^\]]+\]\(([^)]+)\)", text):
            link = raw.strip().strip("<>")
            parsed = urlsplit(link)
            if parsed.scheme or link.startswith("#"):
                continue
            target = (document.parent / unquote(parsed.path)).resolve()
            require(target.is_relative_to(root), f"{document.name}: link escapes plan: {link}")
            require(target.exists(), f"{document.name}: broken local link: {link}")

    # Milestone profiles must include all transitive task prerequisites.
    profiles = plan.get("profiles", {})
    expanded: dict[str, set[str]] = {}
    expanding: set[str] = set()
    def expand(name: str) -> set[str]:
        if name in expanding:
            errors.append(f"profile cycle at {name}")
            return set()
        if name in expanded:
            return expanded[name]
        if name not in profiles:
            errors.append(f"unknown profile {name}")
            return set()
        expanding.add(name)
        profile = profiles[name]
        result = set(profile.get("tasks", []))
        for prerequisite in profile.get("requires_profiles", []):
            result |= expand(prerequisite)
        expanding.remove(name)
        expanded[name] = result
        return result
    for name, profile in profiles.items():
        members = expand(name)
        require(bool(members), f"{name}: empty profile")
        for tid in members:
            require(tid in by_id, f"{name}: unknown task {tid}")
            for dependency in by_id.get(tid, {}).get("depends_on", []):
                require(dependency in members, f"{name}: missing dependency {dependency} for {tid}")
        if profile.get("certification_evidence"):
            evidence_file(profile["certification_evidence"], name)
            require(all(by_id.get(tid, {}).get("status") == "DONE" for tid in members),
                    f"{name}: certification evidence recorded before tasks complete")

    ready = [tid for tid in order if by_id[tid].get("status") == "PLANNED"
             and all(by_id.get(d, {}).get("status") == "DONE" for d in by_id[tid].get("depends_on", []))
             and all(gates.get(g, {}).get("status") == "SATISFIED" for g in by_id[tid].get("external_gates", []))]
    summary = {"routes": len(routes), "tasks": len(tasks), "requirements": len(requirements),
               "task_cards": len(seen_cards), "ready_tasks": ready,
               "topological_order": order,
               "profiles": {name: {"required_tasks": len(members),
                                   "done_tasks": sum(by_id.get(tid, {}).get("status") == "DONE" for tid in members),
                                   "certification_evidence": profiles[name].get("certification_evidence")}
                            for name, members in expanded.items()}}
    return errors, summary


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    args = parser.parse_args()
    try:
        errors, summary = validate(args.root.resolve())
    except (OSError, ValueError, TypeError, KeyError, AttributeError) as exc:
        print(f"PLAN_CHECK=FAIL\nUnable to validate plan: {exc}", file=sys.stderr)
        return 1
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        print("PLAN_CHECK=FAIL")
        return 1
    print("PLAN_CHECK=PASS (document structure only; not product or release certification)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
