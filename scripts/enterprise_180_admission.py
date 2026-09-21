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
POLICY_CONTRACT = ROOT / "docs/enterprise-center/enterprise180-policy-contract.v1.json"
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


def validate_policy_contract(payload: dict[str, object]) -> list[dict[str, str]]:
    blockers: list[dict[str, str]] = []

    def require_contract(ok: bool, reason: str, description: str) -> None:
        if not ok:
            blockers.append({
                "id": "POLICY_CONTRACT",
                "status": "INVALID",
                "reason": reason,
                "description": description,
            })

    require_contract(payload.get("schema_version") == 1, "POLICY_CONTRACT_SCHEMA_INVALID", "policy contract schema_version 必须为 1")
    require_contract(payload.get("status") == "ACCEPTED", "POLICY_CONTRACT_NOT_ACCEPTED", "policy contract 必须为 ACCEPTED")
    require_contract(
        payload.get("accepted_decisions") == ["Q-009", "Q-011"],
        "POLICY_CONTRACT_DECISION_BINDING_INVALID",
        "policy contract 必须精确绑定 Q-009/Q-011",
    )

    q009 = payload.get("q009_business_scope_binding")
    q011 = payload.get("q011_data_policy_composition")
    require_contract(isinstance(q009, dict), "Q009_CONTRACT_MISSING", "缺少 Q-009 结构化合同")
    require_contract(isinstance(q011, dict), "Q011_CONTRACT_MISSING", "缺少 Q-011 结构化合同")
    if isinstance(q009, dict):
        require_contract(q009.get("assignment_mode") == "explicit", "Q009_ASSIGNMENT_MODE_INVALID", "Q-009 必须采用显式范围绑定")
        require_contract(q009.get("tenant_bound") is True, "Q009_TENANT_BOUND_REQUIRED", "Q-009 必须 tenant-bound")
        require_contract(q009.get("assignable_only") is True, "Q009_ASSIGNABLE_ONLY_REQUIRED", "Q-009 只允许当前可分配对象")
        require_contract(q009.get("authoritative_readback") is True, "Q009_READBACK_REQUIRED", "Q-009 写后必须权威回读")
        require_contract(q009.get("organization_relation_grants_access") is False, "Q009_IMPLICIT_ORG_GRANT_FORBIDDEN", "组织关系不得隐式授予数据权限")
        require_contract(q009.get("derived_data_scope_authoritative") is False, "Q009_DERIVED_SCOPE_AUTHORITY_FORBIDDEN", "derived_data_scope 不得成为授权权威")

    if isinstance(q011, dict):
        require_contract(q011.get("role_policy_cardinality") == "zero_or_one", "Q011_CARDINALITY_INVALID", "一期 Role 最多引用一个 Data Policy")
        require_contract(q011.get("multiple_policies_per_role") is False, "Q011_MULTI_POLICY_FORBIDDEN", "一期不允许 Role 多策略组合")
        require_contract(q011.get("effective_scope_operator") == "intersection", "Q011_SCOPE_OPERATOR_INVALID", "有效范围必须使用约束性交集")
        require_contract(
            q011.get("effective_scope_dimensions") == [
                "applicable_role_policy_scope",
                "member_explicit_scope",
                "current_tenant_assignable_scope",
            ],
            "Q011_SCOPE_DIMENSIONS_INVALID",
            "有效范围必须精确由 role policy、member explicit scope、tenant assignable scope 三维交集",
        )
        require_contract(q011.get("missing_required_policy_behavior") == "deny", "Q011_FAIL_CLOSED_REQUIRED", "缺失必需策略必须 fail-closed")
        require_contract(
            q011.get("policy_contraction_behavior") == "next_sensitive_request_must_re_evaluate_and_deny_if_out_of_scope",
            "Q011_CONTRACTION_INVALID",
            "策略收缩必须在下一敏感请求按当前事实重新计算并拒绝越界",
        )
    return blockers


def evaluate(
    text: str,
    required: bool,
    policy_contract: dict[str, object] | None = None,
) -> tuple[str, list[dict[str, str]]]:
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

    if not blockers and policy_contract is not None:
        blockers.extend(validate_policy_contract(policy_contract))
    return ("BLOCKED" if blockers else "ADMITTED"), blockers


def receipt(required: bool) -> dict[str, object]:
    decisions_text = DECISIONS.read_text(encoding="utf-8")
    policy_contract = json.loads(POLICY_CONTRACT.read_text(encoding="utf-8"))
    state, blockers = evaluate(decisions_text, required, policy_contract)
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
            "policy_contract_path": str(POLICY_CONTRACT.relative_to(ROOT)),
            "policy_contract_sha256": sha256(POLICY_CONTRACT),
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
