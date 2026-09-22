#!/usr/bin/env python3
"""Generic CI performance receipt classifier."""
from __future__ import annotations

import argparse
import json
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

def main() -> int:
    p=argparse.ArgumentParser()
    p.add_argument("--surface", required=True)
    p.add_argument("--lane", required=True)
    p.add_argument("--started-file", required=True)
    p.add_argument("--target-seconds", type=int, required=True)
    p.add_argument("--hard-seconds", type=int, required=True)
    p.add_argument("--candidate-sha", required=True)
    p.add_argument("--job-status", required=True)
    p.add_argument("--output", required=True)
    p.add_argument("--enforce", action="store_true")
    a=p.parse_args()
    if not (0 < a.target_seconds < a.hard_seconds):
        raise SystemExit("invalid performance budget: require 0 < target < hard")
    started=int(pathlib.Path(a.started_file).read_text().strip())
    if started <= 0:
        raise SystemExit("invalid start timestamp")
    observed=int(time.time())
    elapsed=max(0, observed-started)
    state=classify(elapsed,a.target_seconds,a.hard_seconds,a.job_status)
    receipt={
        "schema_version":1,
        "surface":a.surface,
        "lane":a.lane,
        "candidate_sha":a.candidate_sha,
        "state":state,
        "elapsed_seconds":elapsed,
        "target_seconds":a.target_seconds,
        "hard_seconds":a.hard_seconds,
        "job_status":a.job_status,
        "observed_at_epoch":observed,
    }
    out=pathlib.Path(a.output)
    out.parent.mkdir(parents=True,exist_ok=True)
    out.write_text(json.dumps(receipt,indent=2,sort_keys=True)+"\n")
    print("CI_PERFORMANCE_RECEIPT="+json.dumps(receipt,sort_keys=True))
    return 1 if a.enforce and state != PASS else 0

if __name__ == "__main__":
    raise SystemExit(main())
