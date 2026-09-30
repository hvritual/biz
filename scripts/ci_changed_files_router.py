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

SKILL_PREFIXES = (".agents/skills/",)
SKILL_COMPANION_FILES = {
    "docs/design/B2B-PRODUCT-UX.md",
}
DESIGN_GOVERNANCE_PREFIXES = (
    "docs/design/",
)
DESIGN_GOVERNANCE_FILES = {
    "web/ui-contracts.json",
    "web/AGENTS.md",
    "scripts/check_ui_skill_source.py",
    "scripts/test_ui_skill_source.py",
    "scripts/check_b2b_ux_skill.py",
    "scripts/test_b2b_ux_skill.py",
}
DESIGN_WEB_FILES = {
    "web/ui-contracts.json",
    "web/AGENTS.md",
}

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
    "enterprise_180_",
    "enterprise_181_",
    "enterprise_182_",
    "enterprise_183_",
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

NATIVE_LOGIN_PATH_PREFIXES = (
    "cmd/biz-idp/",
    "internal/access/infrastructure/persistence/first_party_idp",
    "internal/access/infrastructure/persistence/login_identifier",
    "internal/access/infrastructure/persistence/verification",
    "internal/bizruntime/first_party_idp",
    "internal/bizruntime/first_party_native_login",
    "integration/enterprise_172_",
    "integration/ce12_browser_seed_test.go",
    "web/tests/ce12/",
    "scripts/accept-local-login.mjs",
)
NATIVE_LOGIN_FILES = {
    ".github/workflows/enterprise-172-native-login.yml",
    "internal/bizruntime/config.go",
    "go.mod",
    "go.sum",
}
DELIVERY_ISOLATION_FILES = {
    ".github/workflows/delivery-workspace-isolation.yml",
    ".yunka/source.env",
    "go.mod",
    "go.sum",
    "go.work",
    "go.work.sum",
    "Makefile",
    "scripts/consumer-resolution-check.sh",
    "scripts/verify-yunka-source.sh",
}

WEB_E2E_HARNESS_PREFIXES = ("web/e2e/",)
WEB_E2E_HARNESS_FILES = {
    "web/playwright.config.ts",
    ".github/workflows/web-e2e-harness.yml",
}

COFFEELINK_GATE_FILES = {
    ".github/workflows/coffeelink-web.yml",
    "scripts/coffeelink_performance.py",
    "scripts/test_coffeelink_performance.py",
    "web/e2e/theme-appearance.spec.ts",
    "web/e2e/i18n.spec.ts",
    "web/e2e/design-acceptance.spec.ts",
    "web/e2e/console.spec.ts",
}

CE13_GATE_FILES = {
    ".github/workflows/ce13-plan-catalog-qualification.yml",
    ".github/workflows/ce13-platform-web-session.yml",
    "scripts/ci_ce13_mysql.sh",
    "scripts/check_ci_ce13.py",
    "scripts/test_ci_ce13_runtime.py",
    "web/playwright.ce13-plan-catalog.config.ts",
    "web/playwright.ce13-platform.config.ts",
}
CE13_GATE_PREFIXES = (
    "web/tests/ce13-plan-catalog/",
    "web/tests/ce13-platform/",
)

ENTERPRISE_180_PATH_PREFIXES = (
    "integration/enterprise_180_",
    "internal/access/domain/data_policy",
    "internal/access/ports/data_policy",
    "internal/access/infrastructure/persistence/data_policy",
    "internal/access/application/tenant_data_policy",
    "internal/access/application/member_business_scope",
    "web/src/features/enterprise/components/policies/",
    "web/src/services/enterprise/dataPolicy",
    "web/e2e/enterprise-data-policy",
)
ENTERPRISE_180_FILES = {
    "contracts/proto/access/v1/tenant_role.proto",
    "contracts/proto/access/v1/tenant_member.proto",
    "contracts/proto/deviceops/v1/deviceops.proto",
    ".github/workflows/enterprise-role-qualification.yml",
    "internal/access/infrastructure/persistence/business_scope.go",
    "internal/access/infrastructure/persistence/member_business_scope.go",
    "internal/access/infrastructure/persistence/bootstrap_business_scope.go",
    "internal/access/infrastructure/persistence/migrations/0016_enterprise_data_policy.sql",
    "internal/bizruntime/data_policy_errors.go",
    "internal/deviceops/security/scope.go",
    "internal/deviceops/infrastructure/persistence/site_directory.go",
    "internal/deviceops/application/site_scope_directory.go",
    "internal/deviceops/ports/site_directory.go",
    "docs/enterprise-center/enterprise180-policy-contract.v1.json",
    "docs/enterprise-center/enterprise180-policy-negative-examples.md",
}

ROLE_GRANT_PATH_PREFIXES = (
    "internal/access/authorization/",
    "web/src/features/enterprise/components/roles/",
    "web/src/features/enterprise/components/security/",
    "web/src/features/enterprise/composables/usePersonalSecurity",
    "web/src/services/enterprise/personalSecurity",
)
ROLE_GRANT_FILES = {
    ".github/workflows/enterprise-179-role-grant-tree.yml",
    "internal/access/application/tenant_role_permission.go",
    "internal/access/domain/model.go",
    "internal/access/ports/role.go",
    "internal/access/infrastructure/persistence/role.go",
    "internal/bizruntime/access_skeleton.go",
    "internal/bizruntime/role_entitlements.go",
    "internal/bizruntime/role_entitlements_test.go",
    "internal/bizruntime/web_authorization.go",
    "integration/b12_role_runtime_mysql_test.go",
    "integration/enterprise_179_role_grant_tree_mysql_test.go",
    "web/src/services/enterprise/rolePermissionCatalog.ts",
    "web/src/services/enterprise/roleRuntime.ts",
    "web/src/stores/enterprise.ts",
    "web/src/types/enterprise.ts",
    "web/src/ui/base/UiInput.vue",
    "web/src/ui/base/controls.spec.ts",
    "web/e2e/enterprise-roles-real.spec.ts",
    "web/e2e/enterprise-personal-profile-real.spec.ts",
}


def clean(path: str) -> str:
    value = path.strip().replace("\\", "/")
    while value.startswith("./"):
        value = value[2:]
    return str(PurePosixPath(value))


def is_skill_path(path: str) -> bool:
    return any(path.startswith(prefix) for prefix in SKILL_PREFIXES)


def is_skill_companion_path(path: str) -> bool:
    return path in SKILL_COMPANION_FILES


def is_design_governance_path(path: str) -> bool:
    return path in DESIGN_GOVERNANCE_FILES or any(
        path.startswith(prefix) for prefix in DESIGN_GOVERNANCE_PREFIXES
    )


def is_docs_only_path(path: str) -> bool:
    if path in ENTERPRISE_180_FILES:
        return False
    if path.startswith("docs/"):
        return True
    if path in DOC_NAMES:
        return True
    if path.endswith(".md") and "/" not in path:
        return True
    return False


def classify_path(path: str) -> set[str]:
    result: set[str] = set()
    if path in {"scripts/ci_access_tests.json", "scripts/ci_access_qualification.py"}:
        result.add("access")
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

    has_skill_source = any(is_skill_path(path) for path in files)
    skill_only = bool(files) and has_skill_source and all(
        is_skill_path(path) or is_skill_companion_path(path) for path in files
    )
    design_governance = bool(files) and not skill_only and any(
        is_design_governance_path(path) for path in files
    ) and all(
        is_design_governance_path(path) or is_docs_only_path(path) for path in files
    )
    docs_only = bool(files) and not skill_only and not design_governance and all(
        is_docs_only_path(path) for path in files
    )
    product_change = bool(files) and not docs_only and not skill_only and not design_governance
    change_class = (
        "skill_only" if skill_only else
        "design_governance" if design_governance else
        "docs_only" if docs_only else
        "product_change"
    )
    design_web_check = design_governance and any(path in DESIGN_WEB_FILES for path in files)

    native_login = any(
        path in NATIVE_LOGIN_FILES or any(path.startswith(prefix) for prefix in NATIVE_LOGIN_PATH_PREFIXES)
        for path in files
    )
    delivery_isolation = any(path in DELIVERY_ISOLATION_FILES for path in files)
    ce_receipts = any(
        path == ".github/workflows/ce-round-receipts.yml"
        or path.startswith("docs/commercial-entitlements/")
        for path in files
    )

    web_e2e_harness = any(
        path in WEB_E2E_HARNESS_FILES or any(path.startswith(prefix) for prefix in WEB_E2E_HARNESS_PREFIXES)
        for path in files
    )
    coffeelink_governance = any(path in COFFEELINK_GATE_FILES for path in files)
    ce13_governance = any(
        path in CE13_GATE_FILES or any(path.startswith(prefix) for prefix in CE13_GATE_PREFIXES)
        for path in files
    )
    web_product = coffeelink_governance or any(
        path.startswith("web/")
        and not is_design_governance_path(path)
        and path not in WEB_E2E_HARNESS_FILES
        and not any(path.startswith(prefix) for prefix in WEB_E2E_HARNESS_PREFIXES)
        for path in files
    )

    enterprise180 = any(
        path in ENTERPRISE_180_FILES or any(path.startswith(prefix) for prefix in ENTERPRISE_180_PATH_PREFIXES)
        for path in files
    )

    role_grants = enterprise180 or any(
        path in ROLE_GRANT_FILES or any(path.startswith(prefix) for prefix in ROLE_GRANT_PATH_PREFIXES)
        for path in files
    )

    domains: set[str] = set()
    non_derived_source = False
    for path in files:
        if is_docs_only_path(path) or is_skill_path(path) or is_skill_companion_path(path) or is_design_governance_path(path):
            continue
        is_derived = any(path.startswith(prefix) for prefix in DERIVED_PREFIXES)
        if not is_derived:
            non_derived_source = True
        classified = classify_path(path)
        domains.update(classified)

        if not is_derived and not classified:
            domains.add("core")

    if product_change and files and not domains:
        domains.add("core")
    if product_change and not non_derived_source and domains == set() and files:
        domains.add("core")

    selected = {domain: domain in domains for domain in DOMAINS}
    matrix = [{"domain": domain} for domain in DOMAINS if selected[domain]]
    return {
        "files": files,
        "change_class": change_class,
        "docs_only": docs_only,
        "skill_only": skill_only,
        "design_governance": design_governance,
        "design_web_check": design_web_check,
        "product_change": product_change,
        "domains": selected,
        "domain_matrix": matrix,
        "domain_count": len(matrix),
        "merge_gate_required": product_change,
        "native_login": native_login,
        "delivery_isolation": delivery_isolation,
        "ce_receipts": ce_receipts,
        "web_e2e_harness": web_e2e_harness,
        "web_product": web_product,
        "coffeelink_governance": coffeelink_governance,
        "ce13_governance": ce13_governance,
        "role_grants": role_grants,
        "enterprise180": enterprise180,
    }


def emit_github_output(path: str, result: dict[str, object]) -> None:
    domains = result["domains"]
    assert isinstance(domains, dict)
    with open(path, "a", encoding="utf-8") as handle:
        handle.write(f"change_class={result['change_class']}\n")
        handle.write(f"docs_only={str(result['docs_only']).lower()}\n")
        handle.write(f"skill_only={str(result['skill_only']).lower()}\n")
        handle.write(f"design_governance={str(result['design_governance']).lower()}\n")
        handle.write(f"design_web_check={str(result['design_web_check']).lower()}\n")
        handle.write(f"product_change={str(result['product_change']).lower()}\n")
        handle.write(f"merge_gate_required={str(result['merge_gate_required']).lower()}\n")
        handle.write(f"domain_count={result['domain_count']}\n")
        handle.write(f"native_login={str(result['native_login']).lower()}\n")
        handle.write(f"delivery_isolation={str(result['delivery_isolation']).lower()}\n")
        handle.write(f"ce_receipts={str(result['ce_receipts']).lower()}\n")
        handle.write(f"web_e2e_harness={str(result['web_e2e_harness']).lower()}\n")
        handle.write(f"web_product={str(result['web_product']).lower()}\n")
        handle.write(f"coffeelink_governance={str(result['coffeelink_governance']).lower()}\n")
        handle.write(f"ce13_governance={str(result['ce13_governance']).lower()}\n")
        handle.write(f"role_grants={str(result['role_grants']).lower()}\n")
        handle.write(f"enterprise180={str(result['enterprise180']).lower()}\n")
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
