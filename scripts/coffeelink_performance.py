#!/usr/bin/env python3
"""CoffeeLink targeted performance receipt.

This intentionally separates performance acceptance from GitHub's hard execution timeout.
The receipt is candidate-bound and lane-specific so CI can fail with diagnostics preserved.
"""
from __future__ import annotations

import argparse
import json
import os
import pathlib
import time


PASS = "PASS"
DEGRADED = "PERFORMANCE_DEGRADED"
BUDGET_EXCEEDED = "PERFORMANCE_BUDGET_EXCEEDED"
TEST_FAILURE = "TEST_FAILURE"


def classify(elapsed_seconds: int, target_seconds: int, hard_seconds: int, job_status: str) -> str:
    if job_status != "success":
        return TEST_FAILURE
    if elapsed_seconds <= target_seconds:
        return PASS
    if elapsed_seconds <= hard_seconds:
        return DEGRADED
    return BUDGET_EXCEEDED


def read_started(path: pathlib.Path) -> int:
    value = int(path.read_text(encoding="utf-8").strip())
    if value <= 0:
        raise ValueError("started timestamp must be positive")
    return value


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--lane", required=True)
    parser.add_argument("--started-file", required=True)
    parser.add_argument("--target-seconds", required=True, type=int)
    parser.add_argument("--hard-seconds", required=True, type=int)
    parser.add_argument("--candidate-sha", required=True)
    parser.add_argument("--job-status", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--enforce", action="store_true")
    args = parser.parse_args()

    if args.target_seconds <= 0 or args.hard_seconds <= args.target_seconds:
        raise SystemExit("invalid performance budget: require 0 < target < hard")

    started = read_started(pathlib.Path(args.started_file))
    observed = int(time.time())
    elapsed = max(0, observed - started)
    state = classify(elapsed, args.target_seconds, args.hard_seconds, args.job_status)

    receipt = {
        "schema_version": 1,
        "candidate_sha": args.candidate_sha,
        "lane": args.lane,
        "state": state,
        "elapsed_seconds": elapsed,
        "target_seconds": args.target_seconds,
        "hard_seconds": args.hard_seconds,
        "job_status": args.job_status,
        "observed_at_epoch": observed,
    }
    output = pathlib.Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print("COFFEELINK_PERFORMANCE_RECEIPT=" + json.dumps(receipt, sort_keys=True))

    if args.enforce and state != PASS:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
