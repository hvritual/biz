#!/usr/bin/env python3
"""Classify Biz changes into bounded CI qualification domains.

The router is intentionally fail-closed: unknown source/configuration paths select
"core" rather than silently skipping qualification. Generated outputs are treated
as derived unless they are the only changed source.
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import PurePosixPath

DOMAINS = ("access", "commercial", "deviceops", "web", "core")

DOC_NAMES = {"README.md", "AGENTS.md", "LICENSE", "CHANGELOG.md"}
DERIVED_PREFIXES = (
    "contracts/generated/",
    "contracts/gen/",
    "contracts/commercial/generated/",
)

ACCESS_PREFIXES = (
    "internal/access/",
    "internal/bizruntime/",
    "contracts/proto/access/",
    "cmd/access-action-catalog/",
)
COMMERCIAL_PREFIXES = (
    "internal/commercial/",
    "contracts/commercial/",
    "cmd/commercial-catalog/",
)
DEVICEOPS_PREFIXES = (
    "internal/deviceops/",
)
WEB_PREFIXES = ("web/",)
CORE_PREFIXES = (
    ".github/",
    ".yunka/",
    "internal/architecture/",
    "internal/assembly/",
    "modules/",
    "cmd/biz/",
    "scripts/ci_",
)
CORE_FILES = {
    "Makefile",
    "go.mod",
    "go.sum",
    "go.work",
    "go.work.sum",
    "docker-compose.yml",
}

ACCESS_INTEGRATION_PREFIXES = (
    "b12_",
    "ec_ri_02_",
    "ec_ri_03_",
    "ec_ri_04_",
    "ec_ri_05_",
    "ec_ri_07_",
    "enterprise_168_",
    "enterprise_169_",
    "enterprise_170_",
    "enterprise_171_",
    "enterprise_172_",
    "enterprise_173_",
    "enterprise_174_",
    "enterprise_175_",
    "enterprise_176_",
    "enterprise_177_",
    "enterprise_178_",
    "ce12_",
    "tenant_branding_",
)
COMMERCIAL_INTEGRATION_PREFIXES = (
    "ce02_",
    "ce04_",
    "ce05_",
    "ce06_",
    "ce07_",
    "ce08_",
    "ce09_",
    "ce10_",
    "ce13_",
    "ce16_",
    "ec_ri_06_",
)
CORE_INTEGRATION_PREFIXES = ("c9_", "ag02_", "shared_database_")
DEVICEOPS_INTEGRATION_PREFIXES = ("deviceops_",)


def clean(path: str) -> str:
    return str(PurePosixPath(path.strip())).lstrip("./")


def is_docs_only_path(path: str) -> bool:
    if path.startswith("docs/"):
        return True
    if path in DOC_NAMES:
        return True
    if path.endswith(".md") and "/" not in path:
        return True
    return False


def classify_path(path: str) -> set[str]:
    result: set[str] = set()
    if any(path.startswith(prefix) for prefix in WEB_PREFIXES):
        result.add("web")
    if any(path.startswith(prefix) for prefix in ACCESS_PREFIXES):
        result.add("access")
    if any(path.startswith(prefix) for prefix in COMMERCIAL_PREFIXES):
        result.add("commercial")
    if any(path.startswith(prefix) for prefix in DEVICEOPS_PREFIXES):
        result.add("deviceops")

    if path.startswith("integration/"):
        name = path.removeprefix("integration/")
        if name.startswith(ACCESS_INTEGRATION_PREFIXES):
            result.add("access")
        if name.startswith(COMMERCIAL_INTEGRATION_PREFIXES):
            result.add("commercial")
        if name.startswith(DEVICEOPS_INTEGRATION_PREFIXES):
            result.add("deviceops")
        if name.startswith(CORE_INTEGRATION_PREFIXES):
            result.add("core")

    if path in CORE_FILES or any(path.startswith(prefix) for prefix in CORE_PREFIXES):
        result.add("core")

    return result


def route(paths: list[str]) -> dict[str, object]:
    files = [clean(p) for p in paths if p.strip()]
    docs_only = bool(files) and all(is_docs_only_path(p) for p in files)

    domains: set[str] = set()
    non_derived_source = False
    for path in files:
        if is_docs_only_path(path):
            continue
        is_derived = any(path.startswith(prefix) for prefix in DERIVED_PREFIXES)
        if not is_derived:
            non_derived_source = True
        domains.update(classify_path(path))

        if not is_derived and not classify_path(path):
            # Unknown non-document source/config paths must not bypass qualification.
            domains.add("core")

    if files and not domains and not docs_only:
        domains.add("core")
    if not non_derived_source and domains == set() and files and not docs_only:
        domains.add("core")

    selected = {domain: domain in domains for domain in DOMAINS}
    matrix = [{"domain": domain} for domain in DOMAINS if selected[domain]]
    return {
        "files": files,
        "docs_only": docs_only,
        "domains": selected,
        "domain_matrix": matrix,
        "domain_count": len(matrix),
        "merge_gate_required": not docs_only,
    }


def emit_github_output(path: str, result: dict[str, object]) -> None:
    domains = result["domains"]
    assert isinstance(domains, dict)
    with open(path, "a", encoding="utf-8") as handle:
        handle.write(f"docs_only={str(result['docs_only']).lower()}\n")
        handle.write(f"merge_gate_required={str(result['merge_gate_required']).lower()}\n")
        handle.write(f"domain_count={result['domain_count']}\n")
        handle.write("domain_matrix=" + json.dumps(result["domain_matrix"], separators=(",", ":")) + "\n")
        for domain in DOMAINS:
            handle.write(f"{domain}={str(bool(domains[domain])).lower()}\n")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("files", nargs="*")
    parser.add_argument("--github-output", default=os.getenv("GITHUB_OUTPUT"))
    args = parser.parse_args()

    paths = args.files or [line.rstrip("\n") for line in sys.stdin]
    result = route(paths)
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    if args.github_output:
        emit_github_output(args.github_output, result)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
