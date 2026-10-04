#!/usr/bin/env python3
"""Validate the #191 fixed-candidate acceptance contract without inventing runtime PASS."""
from __future__ import annotations

import argparse
from collections import Counter
import json
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CONTRACT = ROOT / "docs/enterprise-center/enterprise191-acceptance-contract.v1.json"

REQUIRED_METRICS = {
    "legal_login",
    "password_reset",
    "menu_and_api_denial",
    "member_write",
    "data_leakage",
    "permission_contraction",
    "in_app_timeliness",
    "mark_all_read",
    "gateway_auth",
}


def require(ok: bool, reason: str) -> None:
    if not ok:
        raise ValueError(reason)


def load_json(path: Path) -> dict:
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, "DUPLICATE_KEY:" + key)
            result[key] = value
        return result

    return json.loads(path.read_text(encoding="utf-8"), object_pairs_hook=pairs)


def parse_test_ref(ref: str) -> tuple[str, str]:
    require(isinstance(ref, str) and ref.strip(), "EMPTY_TEST_REFERENCE")
    path, sep, target = ref.partition("::")
    require(path.startswith(("integration/", "web/", "internal/", "scripts/", "docs/")), "INVALID_TEST_REFERENCE:" + ref)
    return path, target if sep else ""


def validate_test_reference(ref: str, root: Path) -> None:
    relative, target = parse_test_ref(ref)
    path = root / relative
    require(path.is_file() and not path.is_symlink(), "TEST_SOURCE_MISSING:" + relative)
    resolved = path.resolve()
    require(resolved.is_relative_to(root.resolve()), "TEST_SOURCE_ESCAPES_ROOT:" + relative)
    if not target:
        return
    text = path.read_text(encoding="utf-8")
    if path.suffix == ".go" and target.startswith("Test"):
        require(
            re.search(r"^func\s+" + re.escape(target) + r"\s*\(\s*\w+\s+\*testing\.T\s*\)", text, re.M),
            "GO_TEST_DECLARATION_MISSING:" + ref,
        )
    else:
        require(target in text, "TEST_EVIDENCE_TARGET_MISSING:" + ref)


def expected_ids(prefix: str, start: int, end: int, width: int = 3) -> list[str]:
    return [f"{prefix}-{number:0{width}d}" for number in range(start, end + 1)]


def validate(contract: dict, root: Path) -> dict:
    require(contract.get("schema_version") == 1, "SCHEMA_VERSION")
    require(contract.get("issue") == "#191", "ISSUE_ID")
    require(contract.get("authoritative_mapping") == "docs/enterprise-center/requirements-map.md", "MAPPING_AUTHORITY")

    stories = contract.get("user_stories")
    require(isinstance(stories, list), "USER_STORIES_SCHEMA")
    story_ids = [story.get("id") for story in stories]
    require(story_ids == expected_ids("US", 1, 51), "USER_STORY_SET")
    require(len(set(story_ids)) == 51, "USER_STORY_DUPLICATE")

    for story in stories:
        require(isinstance(story.get("owner_issues"), list) and story["owner_issues"], "STORY_OWNER:" + story["id"])
        require(isinstance(story.get("tests"), list) and story["tests"], "STORY_TESTS:" + story["id"])
        require(story.get("status") not in (None, "", "PASS"), "STORY_STATIC_STATUS_INVALID:" + story["id"])
        for ref in story["tests"]:
            validate_test_reference(ref, root)

    partition = contract.get("story_partition", {})
    require(partition.get("ui", {}).get("count") == 45, "UI_STORY_DENOMINATOR")
    require(partition.get("api_integration", {}).get("count") == 6, "API_STORY_DENOMINATOR")
    require(story_ids[:45] == expected_ids("US", 1, 45), "UI_STORY_RANGE")
    require(story_ids[45:] == expected_ids("US", 46, 51), "API_STORY_RANGE")

    requirements = contract.get("functional_requirements")
    require(isinstance(requirements, list), "FR_SCHEMA")
    fr_ids = [item.get("id") for item in requirements]
    require(fr_ids == [f"FR-{number}" for number in range(1, 184)], "FR_SET")
    require(len(set(fr_ids)) == 183, "FR_DUPLICATE")
    superseded = 0
    for item in requirements:
        require(item.get("source") == "docs/enterprise-center/requirements-map.md", "FR_SOURCE:" + item["id"])
        require(isinstance(item.get("owner_issues"), list) and item["owner_issues"], "FR_OWNER:" + item["id"])
        require(isinstance(item.get("user_story_evidence"), list) and item["user_story_evidence"], "FR_STORY_EVIDENCE:" + item["id"])
        for story_id in item["user_story_evidence"]:
            require(story_id in story_ids, "FR_UNKNOWN_STORY:" + item["id"] + ":" + story_id)
        require(isinstance(item.get("tests"), list) and item["tests"], "FR_TESTS:" + item["id"])
        for ref in item["tests"]:
            validate_test_reference(ref, root)
        status = item.get("status")
        require(status in {"COVERED_BY_REQUIRED_EVIDENCE", "SUPERSEDED_WITH_REPLACEMENT_ACCEPTANCE"}, "FR_STATUS:" + item["id"])
        superseded += int(status == "SUPERSEDED_WITH_REPLACEMENT_ACCEPTANCE")
    require(superseded == 2, "SUPERSEDED_FR_COUNT")

    metrics = contract.get("success_metrics")
    require(isinstance(metrics, list), "SUCCESS_METRICS_SCHEMA")
    metric_ids = [metric.get("id") for metric in metrics]
    require(set(metric_ids) == REQUIRED_METRICS and len(metric_ids) == len(REQUIRED_METRICS), "SUCCESS_METRIC_SET")
    for metric in metrics:
        require(metric.get("target") and metric.get("sample_model"), "SUCCESS_METRIC_DEFINITION:" + str(metric.get("id")))
        require(metric.get("evidence_state") == "REQUIRED", "SUCCESS_METRIC_STATE:" + metric["id"])
        validate_test_reference(metric.get("test", ""), root)

    counterexamples = contract.get("mandatory_counterexamples")
    require(isinstance(counterexamples, list) and counterexamples, "COUNTEREXAMPLE_SCHEMA")
    states = Counter(item.get("state") for item in counterexamples)
    migration = [item for item in counterexamples if item.get("id") == "migration_restore_no_permission_resurrection"]
    require(len(migration) == 1, "MIGRATION_COUNTEREXAMPLE_MISSING")
    require(migration[0].get("state") == "NOT_EVALUATED_CURRENT_DIRECTIVE", "MIGRATION_COUNTEREXAMPLE_MUST_NOT_PASS")

    semantics = contract.get("result_semantics", {})
    require("exact candidate SHA/tree" in semantics.get("runtime_pass", ""), "RUNTIME_PASS_BINDING")
    require("BLOCKED/FAIL" in semantics.get("skipped_or_missing", ""), "SKIP_FAIL_CLOSED")

    return {
        "state": "CONTRACT_VALID",
        "issue": "#191",
        "user_stories": len(stories),
        "ui_stories": 45,
        "api_integration_stories": 6,
        "functional_requirements": len(requirements),
        "success_metrics": len(metrics),
        "mandatory_counterexamples": len(counterexamples),
        "counterexample_states": dict(states),
        "runtime_pass_claimed": False,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--contract", type=Path, default=DEFAULT_CONTRACT)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    try:
        contract = load_json(args.contract)
        result = validate(contract, ROOT)
    except Exception as error:
        result = {"state": "BLOCKED", "reason": str(error), "runtime_pass_claimed": False}
    payload = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(payload, encoding="utf-8")
    sys.stdout.write(payload)
    return 0 if result["state"] == "CONTRACT_VALID" else 1


if __name__ == "__main__":
    raise SystemExit(main())
