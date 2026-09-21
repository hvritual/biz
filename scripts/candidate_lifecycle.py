#!/usr/bin/env python3
"""Executable Candidate lifecycle contract for Biz delivery control."""
from __future__ import annotations

import argparse
import json
import os
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
CONTRACT = ROOT / "scripts" / "candidate_lifecycle.json"

REQUIRED_STATES = (
    "WORKING",
    "FAST_VERIFIED",
    "DOMAIN_QUALIFIED",
    "CANDIDATE_FROZEN",
    "MERGE_QUALIFYING",
    "MERGE_READY",
    "MERGED",
    "MAIN_VERIFIED",
)
REQUIRED_INVARIANTS = (
    "candidate_frozen_requires_current_main_ancestor",
    "head_change_invalidates_frozen_candidate",
    "full_merge_gate_requires_non_draft_pr",
    "qualification_is_read_only",
    "full_merge_gate_runs_once_per_candidate_sha",
    "main_verified_requires_exact_remote_main_tip",
)


def git(*args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=check,
    )


def rev_parse(ref: str) -> str:
    return git("rev-parse", ref).stdout.strip()


def validate_contract(path: pathlib.Path = CONTRACT) -> list[str]:
    errors: list[str] = []
    data = json.loads(path.read_text(encoding="utf-8"))
    states = data.get("states", [])
    transitions = data.get("transitions", {})
    invariants = data.get("invariants", {})

    if tuple(states) != REQUIRED_STATES:
        errors.append("candidate lifecycle states do not match the canonical ordered state machine")

    for state in REQUIRED_STATES:
        if state not in transitions:
            errors.append(f"missing transition declaration for {state}")
    for source, targets in transitions.items():
        if source not in REQUIRED_STATES:
            errors.append(f"unknown transition source: {source}")
        for target in targets:
            if target not in REQUIRED_STATES:
                errors.append(f"unknown transition target: {source}->{target}")

    for name in REQUIRED_INVARIANTS:
        if invariants.get(name) is not True:
            errors.append(f"required lifecycle invariant is not enabled: {name}")

    if "WORKING" not in transitions.get("FAST_VERIFIED", []):
        errors.append("FAST_VERIFIED must fall back to WORKING after candidate mutation/failure")
    if "WORKING" not in transitions.get("CANDIDATE_FROZEN", []):
        errors.append("CANDIDATE_FROZEN must be invalidated by a new HEAD")
    if transitions.get("MERGED") != ["MAIN_VERIFIED"]:
        errors.append("MERGED must transition only to MAIN_VERIFIED")
    if transitions.get("MAIN_VERIFIED") != []:
        errors.append("MAIN_VERIFIED must be terminal")
    return errors


def write_output(values: dict[str, str]) -> None:
    path = os.getenv("GITHUB_OUTPUT")
    if not path:
        return
    with open(path, "a", encoding="utf-8") as handle:
        for key, value in values.items():
            handle.write(f"{key}={value}\n")


def assert_fresh(main_ref: str, candidate_ref: str) -> int:
    main_sha = rev_parse(main_ref)
    candidate_sha = rev_parse(candidate_ref)
    result = git("merge-base", "--is-ancestor", main_sha, candidate_sha, check=False)
    if result.returncode != 0:
        behind = git("rev-list", "--count", f"{candidate_sha}..{main_sha}").stdout.strip()
        merge_base = git("merge-base", main_sha, candidate_sha).stdout.strip()
        print(
            "CANDIDATE_STALE_BASE "
            f"candidate={candidate_sha} current_main={main_sha} behind={behind} merge_base={merge_base}",
            file=sys.stderr,
        )
        write_output({
            "state": "WORKING",
            "candidate_sha": candidate_sha,
            "main_sha": main_sha,
            "fresh": "false",
            "behind": behind,
        })
        return 1

    print(f"CANDIDATE_LIFECYCLE=CANDIDATE_FROZEN candidate={candidate_sha} main={main_sha}")
    write_output({
        "state": "CANDIDATE_FROZEN",
        "candidate_sha": candidate_sha,
        "main_sha": main_sha,
        "fresh": "true",
        "behind": "0",
    })
    return 0


def verify_main(main_ref: str, remote_main_ref: str) -> int:
    checked_sha = rev_parse(main_ref)
    remote_sha = rev_parse(remote_main_ref)
    if checked_sha != remote_sha:
        print(
            f"MAIN_RECEIPT_STALE checked={checked_sha} remote_main={remote_sha}",
            file=sys.stderr,
        )
        write_output({
            "state": "MERGED",
            "main_sha": checked_sha,
            "remote_main_sha": remote_sha,
            "verified": "false",
        })
        return 1

    print(f"CANDIDATE_LIFECYCLE=MAIN_VERIFIED main={checked_sha}")
    write_output({
        "state": "MAIN_VERIFIED",
        "main_sha": checked_sha,
        "remote_main_sha": remote_sha,
        "verified": "true",
    })
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("validate-contract")

    fresh = sub.add_parser("assert-fresh")
    fresh.add_argument("--main-ref", required=True)
    fresh.add_argument("--candidate-ref", required=True)

    receipt = sub.add_parser("verify-main")
    receipt.add_argument("--main-ref", required=True)
    receipt.add_argument("--remote-main-ref", required=True)

    args = parser.parse_args()
    if args.command == "validate-contract":
        errors = validate_contract()
        if errors:
            for error in errors:
                print(f"CANDIDATE-LIFECYCLE: {error}", file=sys.stderr)
            return 1
        print("CANDIDATE-LIFECYCLE: CONTRACT PASS")
        return 0
    if args.command == "assert-fresh":
        return assert_fresh(args.main_ref, args.candidate_ref)
    if args.command == "verify-main":
        return verify_main(args.main_ref, args.remote_main_ref)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
