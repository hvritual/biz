#!/usr/bin/env python3
"""Read-only Delivery Execution Control Plane; GitHub facts, not prose, advance gates."""
from __future__ import annotations

import argparse
import hashlib
import io
import json
import os
import pathlib
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile
from datetime import datetime, timezone

ROOT = pathlib.Path(__file__).resolve().parents[1]
CONTRACT = ROOT / "scripts/delivery_execution_contract.json"
RECEIPT = "delivery-execution.json"
BAD = {"failure", "cancelled", "timed_out", "action_required", "startup_failure", "stale"}


class Blocked(Exception):
    def __init__(self, code: str, evidence=None, repair: bool = False):
        super().__init__(code)
        self.code, self.evidence, self.repair = code, evidence, repair


def require(ok, code, evidence=None, repair=False):
    if not ok:
        raise Blocked(code, evidence, repair)


def digest(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT, text=True, timeout=20).strip()


def expected_jobs(topology):
    full = topology["full_merge_gate"]
    files = full["expected_workflows"]
    require(len(files) == full["expected_units"] and len(set(files)) == len(files), "FULL_TOPOLOGY_INVALID")
    return [f"full-{n:02d}-{path.removesuffix('.yml')}" for n, path in enumerate(files, 1)]


def full_results(needs, expected):
    actual = {name: value.get("result") for name, value in needs.items() if name.startswith("full-")}
    require(set(actual) == set(expected), "FULL_RESULT_SET_MISMATCH", {"expected": expected, "actual": actual})
    bad = {name: result for name, result in actual.items() if result != "success"}
    require(not bad, "FULL_GATE_FAILED", bad, repair=True)
    for name in ("route", "freeze-candidate", "wait-qualification"):
        require(needs.get(name, {}).get("result") == "success", "CONTROL_PREREQUISITE_FAILED", name)
    return actual


def run_binds_pr(run, pr):
    """Resolve live PR links, or GitHub's immutable reusable-workflow PR refs.

    GitHub may remove pull_requests after merge. Never substitute a branch name,
    actor, display title, success status, or a caller-supplied PR number as proof.
    A non-empty conflicting live link is authoritative and cannot use the fallback.
    """
    pulls = run.get("pull_requests") or []
    if pulls:
        return any(pull.get("number") == pr for pull in pulls)
    references = run.get("referenced_workflows") or []
    expected = f"refs/pull/{pr}/merge"
    return bool(references) and all(
        isinstance(reference, dict) and reference.get("ref") == expected and
        re.fullmatch(r"[0-9a-f]{40}", reference.get("sha", "")) is not None and
        reference.get("path", "").endswith("@" + reference["sha"])
        for reference in references)


def matching_runs(runs, workflow, candidate, pr):
    return sorted((run for run in runs if
        run.get("path", "").split("@")[0] == workflow and
        run.get("head_sha") == candidate and run.get("event") == "pull_request" and
        run_binds_pr(run, pr)),
        key=lambda run: (int(run["id"]), int(run.get("run_attempt", 1))), reverse=True)


def latest_success(runs, workflow, candidate, pr):
    selected = matching_runs(runs, workflow, candidate, pr)
    require(selected, "MATCHING_RUN_MISSING", workflow)
    run = selected[0]  # Never filter successes before selecting the latest attempt.
    require(run.get("status") == "completed" and run.get("conclusion") == "success", "LATEST_RUN_NOT_SUCCESS", run_ref(run))
    return run


def run_ref(run):
    return {key: run.get(key) for key in ("id", "run_attempt", "path", "head_sha", "status", "conclusion")}


def failed_jobs(jobs):
    return [job for job in jobs if job.get("conclusion") in BAD or
            any(step.get("conclusion") in BAD for step in job.get("steps", []))]


def root_causes(jobs):
    """Conservative failure signatures; retain exact job/step IDs, never infer a code root cause."""
    roots = {}
    for job in failed_jobs(jobs):
        steps = [step for step in job.get("steps", []) if step.get("conclusion") in BAD] or [{}]
        for step in steps:
            error = re.sub(r"\s+", " ", str(step.get("name") or job.get("name", "unknown"))).strip()
            key = digest(json.dumps(["CI_STEP_FAILURE", job.get("name"), step.get("number"), error]).encode())
            roots[key] = {"signature": key, "rule": "CI_STEP_FAILURE", "file": None, "line": None,
                          "normalized_error": error, "job_id": job.get("id"), "step": step.get("number")}
    return list(roots.values())


def progress_key(jobs):
    return json.dumps([(j.get("id"), j.get("status"), j.get("conclusion"),
        [(s.get("number"), s.get("status"), s.get("conclusion")) for s in j.get("steps", [])])
        for j in jobs], sort_keys=True)


class SafeRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        require(urllib.parse.urlparse(newurl).scheme == "https", "UNSAFE_REDIRECT")
        result = super().redirect_request(req, fp, code, msg, headers, newurl)
        if result and urllib.parse.urlparse(req.full_url).netloc != urllib.parse.urlparse(newurl).netloc:
            result.remove_header("Authorization")
        return result


class API:
    def __init__(self, repository, token, limits):
        require(re.fullmatch(r"[\w.-]+/[\w.-]+", repository or "") and token, "API_ENVIRONMENT_MISSING")
        self.prefix, self.token, self.limits = "/repos/" + repository, token, limits
        self.opener = urllib.request.build_opener(SafeRedirect())

    def raw(self, path, cap=8 * 1024 * 1024):
        require(path.startswith(self.prefix + "/"), "API_SCOPE_VIOLATION")
        for attempt in range(self.limits["transport_attempts"]):
            request = urllib.request.Request("https://api.github.com" + path, headers={
                "Authorization": "Bearer " + self.token, "Accept": "application/vnd.github+json",
                "X-GitHub-Api-Version": "2022-11-28"})
            try:
                with self.opener.open(request, timeout=self.limits["request_seconds"]) as response:
                    body = response.read(cap + 1)
                require(len(body) <= cap, "API_RESPONSE_TOO_LARGE")
                return body
            except urllib.error.HTTPError as error:
                if error.code not in {429, 500, 502, 503, 504}:
                    raise Blocked("API_HTTP_ERROR", {"status": error.code, "path": path}) from error
            except (urllib.error.URLError, TimeoutError):
                pass
            if attempt + 1 < self.limits["transport_attempts"]:
                time.sleep(2 ** attempt)
        raise Blocked("API_TRANSPORT_BUDGET_EXHAUSTED", path)

    def get(self, path):
        return json.loads(self.raw(self.prefix + path))

    def pages(self, path, key=None):
        items = []
        for page in range(1, self.limits["max_pages"] + 1):
            payload = self.get(path + ("&" if "?" in path else "?") + f"per_page=100&page={page}")
            batch = payload[key] if key else payload
            require(isinstance(batch, list), "API_COLLECTION_INVALID")
            items.extend(batch)
            if len(batch) < 100:
                return items
        raise Blocked("API_PAGE_BUDGET_EXHAUSTED", path)

    def runs(self, candidate):
        return self.pages("/actions/runs?event=pull_request&head_sha=" + candidate, "workflow_runs")

    def jobs(self, run):
        return self.pages(f"/actions/runs/{run['id']}/attempts/{run['run_attempt']}/jobs", "jobs")


def bound_refs(api, pr, candidate, allow_draft=False):
    pull = api.get(f"/pulls/{pr}")
    require(pull.get("state") == "open" and pull.get("base", {}).get("ref") == "main", "PR_NOT_OPEN_TO_MAIN")
    require(allow_draft or not pull.get("draft"), "PR_IS_DRAFT")
    require(pull.get("head", {}).get("sha") == candidate, "CANDIDATE_HEAD_CHANGED")
    main_sha = api.get("/git/ref/heads/main")["object"]["sha"]
    require(git("rev-parse", "HEAD") == candidate, "CHECKOUT_SHA_MISMATCH")
    fresh = subprocess.run(["git", "merge-base", "--is-ancestor", main_sha, candidate], cwd=ROOT, timeout=20).returncode == 0
    require(fresh, "CANDIDATE_STALE_BASE", main_sha)
    return {"candidate_sha": candidate, "candidate_tree": git("rev-parse", "HEAD^{tree}"),
            "frozen_main_sha": main_sha, "pr_number": pr,
            "issue_numbers": sorted(set(int(n) for n in re.findall(r"(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#(\d+)", pull.get("body") or "")))}


def route_for_pr(api, pr):
    from ci_changed_files_router import route
    files = []
    for item in api.pages(f"/pulls/{pr}/files"):
        filename = item.get("filename")
        previous = item.get("previous_filename")
        if filename:
            files.append(filename)
        if previous:
            files.append(previous)
    files = sorted(set(files))
    require(files, "PR_CHANGED_FILES_MISSING")
    return route(files)


def verify_lightweight_jobs(jobs):
    light = [job for job in jobs if job.get("name") == "lightweight-ready"]
    require(len(light) == 1 and light[0].get("status") == "completed" and
            light[0].get("conclusion") == "success", "LIGHTWEIGHT_JOB_NOT_SUCCESS")
    full = [job for job in jobs if job.get("name", "").startswith("full-")]
    require(all(job.get("status") == "completed" and job.get("conclusion") == "skipped"
                for job in full), "LIGHTWEIGHT_FULL_JOB_EXECUTED")
    return light[0]


def verify_lightweight_receipt(receipt, contract_hash, candidate, tree, pr,
                               merge_run, qualification_run, routing):
    required = {
        "schema_version": 1,
        "state": "LIGHTWEIGHT_MERGE_READY",
        "contract_sha256": contract_hash,
        "candidate_sha": candidate,
        "candidate_tree": tree,
        "pr_number": pr,
        "merge_run_id": str(merge_run["id"]),
        "merge_run_attempt": int(merge_run["run_attempt"]),
        "change_class": routing["change_class"],
        "merge_gate_required": False,
        "full_results": {},
        "root_cause_signatures": [],
    }
    for field, value in required.items():
        require(type(receipt.get(field)) is type(value) and receipt.get(field) == value,
                "LIGHTWEIGHT_RECEIPT_BINDING_MISMATCH", field)
    require(receipt.get("qualification_run") == run_ref(qualification_run),
            "QUALIFICATION_PROOF_SUPERSEDED")


def lightweight_ready(api, contract, pr, candidate, run_id, attempt):
    bound = bound_refs(api, pr, candidate)
    routing = route_for_pr(api, pr)
    require(not routing["merge_gate_required"] and
            routing["change_class"] in {"docs_only", "skill_only", "design_governance"},
            "LIGHTWEIGHT_CLASS_REQUIRED", routing["change_class"])
    runs = api.runs(candidate)
    assert_single_full(runs, contract, candidate, pr, run_id, attempt)
    qualification = latest_success(runs, contract["qualification_workflow"], candidate, pr)
    current = api.get(f"/actions/runs/{run_id}")
    require(current.get("head_sha") == candidate and current.get("run_attempt") == attempt and
            current.get("path", "").split("@")[0] == contract["merge_workflow"] and
            run_binds_pr(current, pr), "LIGHTWEIGHT_RUN_BINDING_MISMATCH")
    return {
        **bound,
        "state": "LIGHTWEIGHT_MERGE_READY",
        "change_class": routing["change_class"],
        "merge_gate_required": False,
        "qualification_run": run_ref(qualification),
        "full_results": {},
        "root_cause_signatures": [],
        "verification_scope": "lightweight",
    }


def assert_single_full(runs, contract, candidate, pr, run_id, attempt):
    full = matching_runs(runs, contract["merge_workflow"], candidate, pr)
    require(len(full) == contract["limits"]["full_runs_per_candidate"] and str(full[0]["id"]) == str(run_id), "FULL_RUN_BUDGET_EXCEEDED")
    require(int(attempt) == contract["limits"]["full_attempts_per_candidate"] == int(full[0]["run_attempt"]), "FULL_ATTEMPT_BUDGET_EXCEEDED")


def wait_qualification(api, contract, pr, candidate, run_id, attempt, clock=time.monotonic, sleep=time.sleep):
    limits, started = contract["limits"], clock()
    previous, progressed, bound = None, started, None
    while True:
        elapsed = clock() - started
        require(elapsed < limits["qualification_seconds"], "QUALIFICATION_DEADLINE_EXCEEDED")
        current = bound_refs(api, pr, candidate)
        require(bound is None or bound == current, "FROZEN_BINDING_CHANGED")
        bound = current
        runs = api.runs(candidate)
        assert_single_full(runs, contract, candidate, pr, run_id, attempt)
        selected = matching_runs(runs, contract["qualification_workflow"], candidate, pr)
        if not selected:
            require(elapsed < limits["discovery_seconds"], "QUALIFICATION_RUN_MISSING")
        else:
            run = selected[0]
            jobs = api.jobs(run)
            failures = root_causes(jobs)
            require(not failures, "QUALIFICATION_JOB_FAILED", failures, repair=True)
            key = (run["id"], run["run_attempt"], progress_key(jobs))
            if key != previous:
                previous, progressed = key, clock()
            if run.get("status") == "completed":
                require(run.get("conclusion") == "success", "QUALIFICATION_NOT_SUCCESS", run_ref(run), repair=True)
                return {**bound, "state": "DOMAIN_QUALIFIED", "qualification_run": run_ref(run)}
            require(clock() - progressed < limits["no_progress_seconds"], "QUALIFICATION_PROGRESS_LEASE_EXPIRED", run_ref(run))
        print(f"DELIVERY_WAIT candidate={candidate} elapsed_seconds={int(elapsed)}", flush=True)
        sleep(limits["poll_seconds"])


def verify_receipt(receipt, contract_hash, candidate, tree, pr, merge_run, qualification_run, expected):
    required = {"schema_version": 1, "state": "MERGE_READY", "contract_sha256": contract_hash,
                "candidate_sha": candidate, "candidate_tree": tree, "pr_number": pr,
                "merge_run_id": str(merge_run["id"]), "merge_run_attempt": int(merge_run["run_attempt"]),
                "active_full_jobs": 0, "root_cause_signatures": []}
    for field, value in required.items():
        require(receipt.get(field) == value, "RECEIPT_BINDING_MISMATCH", field)
    require(receipt.get("qualification_run") == run_ref(qualification_run), "QUALIFICATION_PROOF_SUPERSEDED")
    require(set(receipt.get("full_results", {})) == set(expected) and
            all(result == "success" for result in receipt["full_results"].values()), "RECEIPT_FULL_SET_INVALID")


def read_receipt_zip(data, limit):
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        require(archive.namelist() == [RECEIPT], "RECEIPT_ARCHIVE_INVALID")
        require(archive.getinfo(RECEIPT).file_size <= limit, "RECEIPT_TOO_LARGE")
        return json.loads(archive.read(RECEIPT))


def verify_main(api, contract, contract_hash, repository, main_sha, expected):
    require(api.get("/git/ref/heads/main")["object"]["sha"] == main_sha, "MAIN_TIP_CHANGED")
    pulls = api.pages(f"/commits/{main_sha}/pulls")
    merged = [p for p in pulls if p.get("merged_at") and p.get("merge_commit_sha") == main_sha and p.get("base", {}).get("ref") == "main"]
    require(len(merged) == 1, "MAIN_MERGED_PR_BINDING_MISSING")
    pull = merged[0]
    candidate, pr = pull["head"]["sha"], pull["number"]
    tree = api.get(f"/git/commits/{candidate}")["tree"]["sha"]
    require(api.get(f"/git/commits/{main_sha}")["tree"]["sha"] == tree, "MAIN_CANDIDATE_TREE_MISMATCH")
    routing = route_for_pr(api, pr)
    runs = api.runs(candidate)
    merge_run = latest_success(runs, contract["merge_workflow"], candidate, pr)
    assert_single_full(runs, contract, candidate, pr, merge_run["id"], merge_run["run_attempt"])
    qualification = latest_success(runs, contract["qualification_workflow"], candidate, pr)
    name = f"delivery-execution-{merge_run['id']}-{merge_run['run_attempt']}"
    artifacts = [a for a in api.pages(f"/actions/runs/{merge_run['id']}/artifacts", "artifacts") if a.get("name") == name]
    require(len(artifacts) == 1 and not artifacts[0].get("expired"), "MERGE_RECEIPT_MISSING_OR_EXPIRED")
    artifact, limit = artifacts[0], contract["limits"]["max_receipt_bytes"]
    require(artifact["size_in_bytes"] <= limit, "RECEIPT_TOO_LARGE")
    data = api.raw(api.prefix + f"/actions/artifacts/{artifact['id']}/zip", cap=limit)
    receipt = read_receipt_zip(data, limit)
    require(receipt.get("repository") == repository, "RECEIPT_REPOSITORY_MISMATCH")

    if routing["merge_gate_required"]:
        verify_receipt(receipt, contract_hash, candidate, tree, pr, merge_run, qualification, expected)
        from ci_dependency_recovery import verify_run
        recovery = verify_run(api, ROOT, repository, candidate, tree, pr, merge_run, receipt["frozen_main_sha"])
        require(receipt.get("dependency_preparation") == recovery, "RECOVERY_PROOF_EXECUTION_DRIFT")
        verification_scope = "full"
    else:
        verify_lightweight_receipt(receipt, contract_hash, candidate, tree, pr,
                                   merge_run, qualification, routing)
        verify_lightweight_jobs(api.jobs(merge_run))
        verification_scope = "lightweight"

    require(api.get("/git/ref/heads/main")["object"]["sha"] == main_sha, "MAIN_TIP_CHANGED")
    return {**receipt, "state": "MAIN_VERIFIED", "main_sha": main_sha,
            "change_class": routing["change_class"], "verification_scope": verification_scope,
            "merge_receipt_artifact_id": artifact["id"]}


def validate_contract(contract):
    require(contract.get("schema_version") == 1, "CONTRACT_SCHEMA_INVALID")
    require(contract.get("lifecycle_authority") == "scripts/candidate_lifecycle.json" and
            contract.get("topology_authority") == "scripts/ci_topology_contract.json", "CONTRACT_AUTHORITY_DRIFT")
    for key in ("discovery_seconds", "qualification_seconds", "no_progress_seconds", "poll_seconds", "request_seconds", "transport_attempts", "max_pages", "max_receipt_bytes"):
        require(type(contract["limits"].get(key)) is int and contract["limits"][key] > 0, "CONTRACT_LIMIT_INVALID", key)
    require(contract["limits"]["full_runs_per_candidate"] == contract["limits"]["full_attempts_per_candidate"] == 1, "FULL_BUDGET_DRIFT")
    hooks = {"pr-qualification.yml": ["delivery_execution.py validate-contract", "test_delivery_execution.py"],
             "pr-merge-gate.yml": ["delivery_execution.py wait-qualification", "delivery_execution.py lightweight-ready", "delivery_execution.py merge-ready"],
             "main-receipt.yml": ["delivery_execution.py verify-main"]}
    for name, commands in hooks.items():
        text = (ROOT / ".github/workflows" / name).read_text()
        require(all(command in text for command in commands), "DELIVERY_HOOK_MISSING", name)
    base = os.getenv("BASE_SHA")
    if base:
        before = subprocess.run(["git", "show", f"{base}:scripts/delivery_execution_contract.json"], cwd=ROOT, capture_output=True)
        if before.returncode == 0 and not os.getenv("GITHUB_HEAD_REF", "").startswith("chore/ci-"):
            for path in ["scripts/delivery_execution_contract.json", "scripts/delivery_execution.py", "scripts/test_delivery_execution.py"] + [".github/workflows/" + name for name in hooks]:
                original = subprocess.run(["git", "show", f"{base}:{path}"], cwd=ROOT, capture_output=True)
                require(original.returncode == 0 and original.stdout == (ROOT / path).read_bytes(), "DELIVERY_CONTROL_CHANGE_REQUIRES_GOVERNANCE", path)


def require_proof_receipt(path, bound, run_id, attempt, repository):
    require(path is not None and pathlib.Path(path).is_file(), "PROOF_RECEIPT_MISSING")
    from ci_dependency_recovery import strict_json
    proof = strict_json(pathlib.Path(path).read_bytes())
    require(proof.get("state") == "VERIFIED", "PROOF_AUDIT_BLOCKED", proof.get("reason"))
    fields = {"schema_version": 1, "stage": "audit-run", "repository": repository,
              "candidate_sha": bound["candidate_sha"], "candidate_tree": bound["candidate_tree"],
              "frozen_main_sha": bound["frozen_main_sha"], "pr_number": bound["pr_number"],
              "run_id": str(run_id), "run_attempt": attempt, "canonical_units": 35,
              "contract_sha256": digest((ROOT / "scripts/ci_proof_contract.json").read_bytes()),
              "topology_sha256": digest((ROOT / "scripts/ci_topology_contract.json").read_bytes())}
    for key, value in fields.items():
        require(type(proof.get(key)) is type(value) and proof[key] == value, "PROOF_RECEIPT_BINDING_MISMATCH", key)
    require(proof.get("hard_violations") == [] and proof.get("dependency_preparation", {}).get("artifact_state") in {"READY", "RECOVERED"},
            "PROOF_RECOVERY_OR_PERFORMANCE_INVALID")
    return proof["dependency_preparation"]


def emit(report, output):
    report["observed_at"] = datetime.now(timezone.utc).isoformat()
    pathlib.Path(output).parent.mkdir(parents=True, exist_ok=True)
    pathlib.Path(output).write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
    print(json.dumps(report, sort_keys=True))
    if os.getenv("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as handle:
            handle.write("\n## Delivery Execution Control Plane\n\n```json\n" + json.dumps(report, indent=2) + "\n```\n")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["validate-contract", "wait-qualification", "lightweight-ready", "merge-ready", "verify-main"])
    parser.add_argument("--output", default=str(pathlib.Path(os.getenv("RUNNER_TEMP", ".")) / RECEIPT))
    parser.add_argument("--proof-receipt", type=pathlib.Path)
    args = parser.parse_args()
    report = {"schema_version": 1, "state": "BLOCKED", "repository": os.getenv("GITHUB_REPOSITORY"),
              "candidate_sha": os.getenv("CANDIDATE_SHA"), "owner": os.getenv("GITHUB_ACTOR", "repository-maintainer"),
              "stage": args.command, "root_cause_signatures": []}
    try:
        contract = json.loads(CONTRACT.read_text())
        report["contract_sha256"] = digest(CONTRACT.read_bytes())
        topology = json.loads((ROOT / contract["topology_authority"]).read_text())
        expected = expected_jobs(topology)
        if args.command == "validate-contract":
            validate_contract(contract)
            print("DELIVERY_EXECUTION_CONTRACT=PASS")
            return 0
        api = API(report["repository"], os.getenv("GH_TOKEN") or os.getenv("GITHUB_TOKEN"), contract["limits"])
        if args.command == "verify-main":
            report.update(verify_main(api, contract, report["contract_sha256"], report["repository"], os.environ["MAIN_SHA"], expected))
        else:
            pr, candidate = int(os.environ["PR_NUMBER"]), os.environ["CANDIDATE_SHA"]
            run_id, attempt = os.environ["GITHUB_RUN_ID"], int(os.environ["GITHUB_RUN_ATTEMPT"])
            report.update(merge_run_id=run_id, merge_run_attempt=attempt)
            if args.command == "wait-qualification":
                report.update(wait_qualification(api, contract, pr, candidate, run_id, attempt))
                if os.getenv("GITHUB_OUTPUT"):
                    with open(os.environ["GITHUB_OUTPUT"], "a") as handle:
                        handle.write(f"qualification_run_id={report['qualification_run']['id']}\nqualification_attempt={report['qualification_run']['run_attempt']}\nfrozen_main_sha={report['frozen_main_sha']}\n")
            elif args.command == "lightweight-ready":
                report.update(lightweight_ready(api, contract, pr, candidate, run_id, attempt))
            else:
                report.update(bound_refs(api, pr, candidate))
                report["dependency_preparation"] = require_proof_receipt(args.proof_receipt, report, run_id, attempt, report["repository"])
                needs = json.loads(os.environ["FULL_RESULTS"])
                jobs = api.jobs({"id": run_id, "run_attempt": attempt})
                report["root_cause_signatures"] = root_causes(jobs)
                report["full_results"] = full_results(needs, expected)
                report["active_full_jobs"] = sum(j.get("status") != "completed" for j in jobs if j.get("name", "").startswith("full-"))
                require(report["active_full_jobs"] == 0, "FULL_JOBS_STILL_ACTIVE")
                require(not report["root_cause_signatures"], "UNRESOLVED_FAILURES", repair=True)
                runs = api.runs(candidate)
                assert_single_full(runs, contract, candidate, pr, run_id, attempt)
                qualification = latest_success(runs, contract["qualification_workflow"], candidate, pr)
                outputs = needs["wait-qualification"].get("outputs", {})
                require(str(qualification["id"]) == outputs.get("qualification_run_id") and str(qualification["run_attempt"]) == outputs.get("qualification_attempt"), "QUALIFICATION_PROOF_SUPERSEDED")
                require(report["frozen_main_sha"] == outputs.get("frozen_main_sha"), "FROZEN_MAIN_CHANGED")
                report.update(state="MERGE_READY", qualification_run=run_ref(qualification))
        report["next_action"] = "merge_exact_candidate" if report["state"] in {"MERGE_READY", "LIGHTWEIGHT_MERGE_READY"} else "continue_canonical_lifecycle"
        emit(report, args.output)
        return 0
    except Blocked as error:
        report.update(state="REPAIR_REQUIRED" if error.repair else "BLOCKED", reason=error.code, evidence=error.evidence,
                      next_action="collect_terminal_evidence_then_fix_once_and_create_new_candidate" if error.repair else "resolve_recorded_blocker_then_requalify_without_bypassing_gate")
    except Exception as error:
        report.update(reason="CONTROL_PLANE_ERROR", evidence=type(error).__name__, next_action="inspect_control_plane_error_and_requalify")
        print(str(error), file=sys.stderr)
    emit(report, args.output)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
