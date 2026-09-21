#!/usr/bin/env python3
"""Enterprise #180 delivery admission.

This gate is deliberately semantic-free. It only checks whether the Human-owned
policy decisions required by #180 are ACCEPTED. It must not infer or implement
Data Policy behavior.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import sys
from datetime import datetime, timezone

ROOT = pathlib.Path(__file__).resolve().parents[1]
DECISIONS = ROOT / "docs/enterprise-center/decisions.md"
CONTRACTS = ROOT / "docs/enterprise-center/contracts.md"
RECEIPT = "enterprise180-admission.json"

REQUIRED = {
    "Q-009": {
        "reason": "BUSINESS_SCOPE_CONTRACT_NOT_ACCEPTED",
        "description": "成员范围绑定合同尚未由 Human 接受",
    },
    "Q-011": {
        "reason": "DATA_POLICY_COMPOSITION_NOT_ACCEPTED",
        "description": "多 Data Policy 组合/冲突语义尚未由 Human 接受",
    },
}


def sha256(path: pathlib.Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def parse_decisions(text: str) -> dict[str, list[str]]:
    found: dict[str, list[str]] = {}
    for raw in text.splitlines():
        if not raw.startswith("|"):
            continue
        cells = [cell.strip() for cell in raw.strip().strip("|").split("|")]
        if len(cells) < 2:
            continue
        match = re.match(r"^(Q-\d{3})(?:\s|$)", cells[0])
        if not match:
            continue
        found.setdefault(match.group(1), []).append(cells[1])
    return found


def evaluate(text: str, required: bool) -> tuple[str, list[dict[str, str]]]:
    if not required:
        return "NOT_APPLICABLE", []

    decisions = parse_decisions(text)
    blockers: list[dict[str, str]] = []
    for decision_id, rule in REQUIRED.items():
        statuses = decisions.get(decision_id, [])
        if not statuses:
            blockers.append({
                "id": decision_id,
                "status": "MISSING",
                "reason": "DECISION_RECORD_MISSING",
                "description": f"{decision_id} 决策记录缺失",
            })
            continue
        if len(statuses) != 1:
            blockers.append({
                "id": decision_id,
                "status": "AMBIGUOUS",
                "reason": "DECISION_RECORD_AMBIGUOUS",
                "description": f"{decision_id} 存在重复或冲突记录",
            })
            continue
        if statuses[0] != "ACCEPTED":
            blockers.append({
                "id": decision_id,
                "status": statuses[0],
                "reason": rule["reason"],
                "description": rule["description"],
            })

    return ("BLOCKED" if blockers else "ADMITTED"), blockers


def receipt(required: bool) -> dict[str, object]:
    decisions_text = DECISIONS.read_text(encoding="utf-8")
    state, blockers = evaluate(decisions_text, required)
    return {
        "schema_version": 1,
        "gate": "enterprise180_delivery_admission",
        "issue_number": 180,
        "state": state,
        "candidate_sha": os.getenv("CANDIDATE_SHA") or os.getenv("GITHUB_SHA"),
        "pr_number": int(os.getenv("PR_NUMBER", "0") or 0) or None,
        "required": required,
        "semantics_implemented": False,
        "authority": {
            "decisions_path": str(DECISIONS.relative_to(ROOT)),
            "decisions_sha256": sha256(DECISIONS),
            "contracts_path": str(CONTRACTS.relative_to(ROOT)),
            "contracts_sha256": sha256(CONTRACTS),
        },
        "blockers": blockers,
        "next_action": (
            "Human 接受并固化 Q-009 与 Q-011 的精确合同后，重新运行同一候选资格；"
            "本 gate 不得自行补写 Data Policy 语义。"
            if state == "BLOCKED"
            else "continue_delivery_control_plane"
        ),
        "observed_at": datetime.now(timezone.utc).isoformat(),
    }


def write_report(report: dict[str, object], output: pathlib.Path) -> None:
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, sort_keys=True))
    summary = os.getenv("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a", encoding="utf-8") as handle:
            handle.write("\n## Enterprise #180 Delivery Admission\n\n")
            handle.write(f"- state: **{report['state']}**\n")
            handle.write(f"- candidate: \`{report.get('candidate_sha')}\`\n")
            if report["blockers"]:
                for blocker in report["blockers"]:
                    handle.write(
                        f"- blocker: \`{blocker['id']}\` / \`{blocker['status']}\` / "
                        f"\`{blocker['reason']}\`\n"
                    )
            handle.write("- Data Policy semantics implemented by this gate: **false**\n")


def bool_arg(value: str) -> bool:
    lowered = value.strip().lower()
    if lowered in {"1", "true", "yes"}:
        return True
    if lowered in {"0", "false", "no"}:
        return False
    raise argparse.ArgumentTypeError("expected true/false")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["check"])
    parser.add_argument("--required", type=bool_arg, default=True)
    parser.add_argument(
        "--output",
        default=str(pathlib.Path(os.getenv("RUNNER_TEMP", ".")) / RECEIPT),
    )
    args = parser.parse_args()

    try:
        report = receipt(args.required)
    except FileNotFoundError as error:
        report = {
            "schema_version": 1,
            "gate": "enterprise180_delivery_admission",
            "issue_number": 180,
            "state": "BLOCKED",
            "candidate_sha": os.getenv("CANDIDATE_SHA") or os.getenv("GITHUB_SHA"),
            "required": args.required,
            "semantics_implemented": False,
            "blockers": [{
                "id": "ADMISSION_AUTHORITY",
                "status": "MISSING",
                "reason": "ADMISSION_AUTHORITY_MISSING",
                "description": str(error),
            }],
            "next_action": "restore canonical decision authority before qualification",
            "observed_at": datetime.now(timezone.utc).isoformat(),
        }

    write_report(report, pathlib.Path(args.output))
    return 0 if report["state"] in {"ADMITTED", "NOT_APPLICABLE"} else 2


if __name__ == "__main__":
    raise SystemExit(main())
