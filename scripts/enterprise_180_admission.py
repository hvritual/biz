#!/usr/bin/env python3
"""Validate the approved #180 admission contract, never authorize business data."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import re
import subprocess
from datetime import datetime, timezone

ROOT = pathlib.Path(__file__).resolve().parents[1]
DECISIONS = ROOT / "docs/enterprise-center/decisions.md"
CONTRACTS = ROOT / "docs/enterprise-center/contracts.md"
POLICY_CONTRACT = ROOT / "docs/enterprise-center/enterprise180-policy-contract.v1.json"
RECEIPT = "enterprise180-admission.json"
# Accepted v1 semantic digest, not a second copy of its policy rules. A future
# contract version needs explicit review, a new identity and new qualification.
ACCEPTED_POLICY_DIGEST = "71409857293a1a55fb1345ebdffc321b49e80b8dccce6c73ffd9d4fcc6a5fcd9"
REQUIRED = {
    "Q-009": "BUSINESS_SCOPE_CONTRACT_NOT_ACCEPTED",
    "Q-011": "DATA_POLICY_COMPOSITION_NOT_ACCEPTED",
}
MAX_AUTHORITY_BYTES = 1024 * 1024


def blocker(identity, reason, status="INVALID"):
    return {"id": identity, "status": status, "reason": reason}


def semantic_digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"),
                                     ensure_ascii=False, allow_nan=False).encode()).hexdigest()


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON key: " + key)
        result[key] = value
    return result


def read_authority(path):
    with path.open("rb") as handle:
        raw = handle.read(MAX_AUTHORITY_BYTES + 1)
    if len(raw) > MAX_AUTHORITY_BYTES:
        raise ValueError("authority exceeds size limit")
    return raw


def parse_decisions(text):
    found = {}
    for line in text.splitlines():
        cells = [value.strip() for value in line.strip().strip("|").split("|")]
        if not line.lstrip().startswith("|") or len(cells) < 2:
            continue
        match = re.match(r"^(Q-\d{3})(?:\s|$)", cells[0])
        if match:
            found.setdefault(match.group(1), []).append(cells[1])
    return found


def validate_policy_contract(payload):
    if payload is None:
        return [blocker("POLICY_CONTRACT", "POLICY_CONTRACT_MISSING", "MISSING")]
    if not isinstance(payload, dict):
        return [blocker("POLICY_CONTRACT", "POLICY_CONTRACT_SCHEMA_INVALID")]
    try:
        matched = semantic_digest(payload) == ACCEPTED_POLICY_DIGEST
    except (TypeError, ValueError):
        matched = False
    return [] if matched else [blocker("POLICY_CONTRACT", "POLICY_CONTRACT_APPROVED_VERSION_MISMATCH")]


def evaluate(text, required, policy_contract=None):
    if not required:
        return "NOT_APPLICABLE", []
    decisions, blockers = parse_decisions(text), []
    for identity, reason in REQUIRED.items():
        values = decisions.get(identity, [])
        if not values:
            blockers.append(blocker(identity, "DECISION_RECORD_MISSING", "MISSING"))
        elif len(values) != 1:
            blockers.append(blocker(identity, "DECISION_RECORD_AMBIGUOUS", "AMBIGUOUS"))
        elif values[0] != "ACCEPTED":
            blockers.append(blocker(identity, reason, values[0]))
    blockers.extend(validate_policy_contract(policy_contract))
    return ("BLOCKED" if blockers else "ADMITTED"), blockers


def receipt(required):
    report = {
        "schema_version": 1, "gate": "enterprise180_delivery_admission",
        "issue_number": 180, "scope": "admission_only", "state": "BLOCKED",
        "required": required, "semantics_implemented": False,
        "repository": os.getenv("GITHUB_REPOSITORY"),
        "candidate_sha": os.getenv("CANDIDATE_SHA") or os.getenv("GITHUB_SHA"),
        "pr_number": os.getenv("PR_NUMBER") or None,
        "run_id": os.getenv("GITHUB_RUN_ID"), "run_attempt": os.getenv("GITHUB_RUN_ATTEMPT"),
        "evidence_source": "github_actions" if os.getenv("GITHUB_ACTIONS") == "true" else "local",
        "authority": {}, "blockers": [],
        "observed_at": datetime.now(timezone.utc).isoformat(),
    }
    if not required:
        report.update(state="NOT_APPLICABLE", next_action="continue_delivery_control_plane")
        return report
    try:
        if report["pr_number"] is not None:
            report["pr_number"] = int(report["pr_number"])
        if report["evidence_source"] == "github_actions":
            head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT,
                                           text=True, timeout=10).strip()
            if head != report["candidate_sha"] or not all(report[key] for key in ("repository", "run_id", "run_attempt")):
                raise ValueError("candidate or workflow identity missing/mismatched")
            report["candidate_tree"] = subprocess.check_output(
                ["git", "rev-parse", "HEAD^{tree}"], cwd=ROOT, text=True, timeout=10).strip()
        texts = {}
        for label, path in (("decisions", DECISIONS), ("contracts", CONTRACTS), ("policy_contract", POLICY_CONTRACT)):
            raw = read_authority(path)
            texts[label] = raw.decode("utf-8")
            report["authority"][label + "_path"] = str(path.relative_to(ROOT))
            report["authority"][label + "_sha256"] = hashlib.sha256(raw).hexdigest()
        policy = json.loads(texts["policy_contract"], object_pairs_hook=unique_object,
                            parse_constant=lambda value: (_ for _ in ()).throw(ValueError(value)))
        state, blockers = evaluate(texts["decisions"], True, policy)
        report.update(state=state, blockers=blockers)
        report["authority"]["policy_semantic_sha256"] = semantic_digest(policy)
    except (OSError, UnicodeError, ValueError, subprocess.SubprocessError) as error:
        report["blockers"] = [blocker("ADMISSION_AUTHORITY", "ADMISSION_AUTHORITY_INVALID", type(error).__name__)]
    report["next_action"] = (
        "continue_delivery_control_plane" if report["state"] == "ADMITTED"
        else "resolve_recorded_authority_blockers_and_qualify_a_new_candidate"
    )
    return report


def write_report(report, output):
    output.parent.mkdir(parents=True, exist_ok=True)
    content = json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    temporary = output.with_suffix(output.suffix + ".tmp")
    temporary.write_text(content, encoding="utf-8")
    temporary.replace(output)
    print(content, end="")
    if os.getenv("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as handle:
            handle.write("\n## Enterprise180 Delivery Admission\n\n```json\n" + content + "```\n")


def bool_arg(value):
    if value.lower() in ("true", "1", "yes"):
        return True
    if value.lower() in ("false", "0", "no"):
        return False
    raise argparse.ArgumentTypeError("expected true or false")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["check"])
    parser.add_argument("--required", type=bool_arg, default=True)
    parser.add_argument("--output", default=str(pathlib.Path(os.getenv("RUNNER_TEMP", ".")) / RECEIPT))
    args = parser.parse_args()
    report = receipt(args.required)
    write_report(report, pathlib.Path(args.output))
    return 0 if report["state"] in ("ADMITTED", "NOT_APPLICABLE") else 2


if __name__ == "__main__":
    raise SystemExit(main())
