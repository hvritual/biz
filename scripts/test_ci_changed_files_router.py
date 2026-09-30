#!/usr/bin/env python3
import unittest

from ci_changed_files_router import route


class RouteTests(unittest.TestCase):
    def test_docs_only_skips_heavy_domains(self):
        result = route(["docs/enterprise-center/README.md", "README.md"])
        self.assertTrue(result["docs_only"])
        self.assertEqual(result["domain_count"], 0)
        self.assertFalse(result["merge_gate_required"])


    def test_empty_change_set_is_fail_closed_product_change(self):
        result = route([])
        self.assertEqual(result["change_class"], "product_change")
        self.assertTrue(result["product_change"])
        self.assertTrue(result["merge_gate_required"])

    def test_skill_only_routes_to_lightweight_skill_lane(self):
        result = route([
            ".agents/skills/b2b-product-ux/SKILL.md",
            ".agents/skills/b2b-product-ux/UX-CONTRACT.md",
            "docs/design/B2B-PRODUCT-UX.md",
        ])
        self.assertEqual(result["change_class"], "skill_only")
        self.assertTrue(result["skill_only"])
        self.assertFalse(result["design_governance"])
        self.assertFalse(result["product_change"])
        self.assertEqual(result["domain_count"], 0)
        self.assertFalse(result["merge_gate_required"])
        self.assertFalse(result["web_product"])

    def test_renamed_runtime_source_into_skill_stays_product_change(self):
        result = route([
            "internal/access/application/runtime.go",
            ".agents/skills/b2b-product-ux/runtime.go",
        ])
        self.assertEqual(result["change_class"], "product_change")
        self.assertTrue(result["domains"]["access"])
        self.assertTrue(result["merge_gate_required"])

    def test_skill_mixed_with_product_source_escalates_fail_closed(self):
        result = route([
            ".agents/skills/b2b-product-ux/SKILL.md",
            "web/src/features/enterprise/pages/MembersView.vue",
        ])
        self.assertEqual(result["change_class"], "product_change")
        self.assertFalse(result["skill_only"])
        self.assertTrue(result["product_change"])
        self.assertTrue(result["domains"]["web"])
        self.assertTrue(result["merge_gate_required"])

    def test_design_docs_use_design_governance_without_product_domains(self):
        result = route(["docs/design/PAGE-PATTERNS.md"])
        self.assertEqual(result["change_class"], "design_governance")
        self.assertTrue(result["design_governance"])
        self.assertFalse(result["docs_only"])
        self.assertEqual(result["domain_count"], 0)
        self.assertFalse(result["merge_gate_required"])
        self.assertFalse(result["design_web_check"])

    def test_design_machine_contract_requests_fast_web_without_product_gate(self):
        result = route(["web/ui-contracts.json"])
        self.assertEqual(result["change_class"], "design_governance")
        self.assertTrue(result["design_web_check"])
        self.assertFalse(result["web_product"])
        self.assertFalse(result["domains"]["web"])
        self.assertFalse(result["merge_gate_required"])

    def test_design_governance_mixed_with_runtime_escalates_to_product(self):
        result = route([
            "web/ui-contracts.json",
            "internal/access/application/tenant_member_lifecycle.go",
        ])
        self.assertEqual(result["change_class"], "product_change")
        self.assertTrue(result["domains"]["access"])
        self.assertTrue(result["merge_gate_required"])

    def test_root_makefile_never_becomes_lightweight_design_change(self):
        result = route([
            "Makefile",
            "scripts/check_b2b_ux_skill.py",
        ])
        self.assertEqual(result["change_class"], "product_change")
        self.assertTrue(result["domains"]["core"])
        self.assertTrue(result["merge_gate_required"])

    def test_access_and_web_do_not_select_deviceops_or_core(self):
        result = route([
            "internal/access/application/tenant_member_lifecycle.go",
            "contracts/proto/access/v1/tenant_member.proto",
            "web/src/features/enterprise/pages/MembersView.vue",
            "contracts/generated/openapi.json",
        ])
        self.assertTrue(result["domains"]["access"])
        self.assertTrue(result["domains"]["web"])
        self.assertFalse(result["domains"]["deviceops"])
        self.assertFalse(result["domains"]["core"])

    def test_enterprise_178_integration_selects_access(self):
        result = route(["integration/enterprise_178_role_lifecycle_mysql_test.go"])
        self.assertTrue(result["domains"]["access"])
        self.assertFalse(result["domains"]["commercial"])

    def test_commercial_contract_selects_commercial(self):
        result = route(["contracts/commercial/operation-capabilities.v1.json"])
        self.assertTrue(result["domains"]["commercial"])

    def test_deviceops_isolated(self):
        result = route(["internal/deviceops/application/device.go", "integration/deviceops_mysql_test.go"])
        self.assertTrue(result["domains"]["deviceops"])
        self.assertFalse(result["domains"]["access"])
        self.assertFalse(result["domains"]["commercial"])

    def test_toolchain_change_fails_closed_to_core(self):
        result = route(["go.mod"])
        self.assertTrue(result["domains"]["core"])
        self.assertTrue(result["merge_gate_required"])

    def test_unknown_source_fails_closed_to_core(self):
        result = route(["internal/future-domain/service.go"])
        self.assertTrue(result["domains"]["core"])

    def test_native_login_is_explicit_subroute(self):
        result = route(["internal/bizruntime/first_party_native_login.go"])
        self.assertTrue(result["domains"]["access"])
        self.assertTrue(result["native_login"])
        self.assertFalse(result["delivery_isolation"])

    def test_workspace_lock_routes_delivery_isolation(self):
        result = route([".yunka/source.env"])
        self.assertTrue(result["domains"]["core"])
        self.assertTrue(result["delivery_isolation"])

    def test_dot_github_path_preserves_core_identity(self):
        result = route([".github/workflows/pr-qualification.yml"])
        self.assertTrue(result["domains"]["core"])

    def test_commercial_receipt_docs_stay_lightweight(self):
        result = route(["docs/commercial-entitlements/tasks.json"])
        self.assertTrue(result["docs_only"])
        self.assertTrue(result["ce_receipts"])
        self.assertEqual(result["domain_count"], 0)

    def test_e2e_only_change_uses_harness_not_product_web(self):
        result = route(["web/e2e/enterprise-roles-real.spec.ts", "web/e2e/ui.helpers.ts"])
        self.assertTrue(result["domains"]["web"])
        self.assertTrue(result["web_e2e_harness"])
        self.assertFalse(result["web_product"])

    def test_product_web_change_uses_product_gate(self):
        result = route(["web/src/features/enterprise/pages/RolesView.vue"])
        self.assertTrue(result["domains"]["web"])
        self.assertFalse(result["web_e2e_harness"])
        self.assertTrue(result["web_product"])

    def test_coffeelink_gate_change_runs_targeted_product_performance(self):
        result = route([".github/workflows/coffeelink-web.yml"])
        self.assertTrue(result["domains"]["core"])
        self.assertTrue(result["web_product"])
        self.assertTrue(result["coffeelink_governance"])

    def test_coffeelink_core_stability_spec_runs_product_and_e2e_gates(self):
        result = route(["web/e2e/console.spec.ts"])
        self.assertTrue(result["domains"]["web"])
        self.assertTrue(result["web_e2e_harness"])
        self.assertTrue(result["web_product"])
        self.assertTrue(result["coffeelink_governance"])

    def test_ce13_gate_change_runs_targeted_ce13_batch(self):
        result = route([".github/workflows/ce13-plan-catalog-qualification.yml"])
        self.assertTrue(result["domains"]["core"])
        self.assertTrue(result["ce13_governance"])

    def test_ce13_browser_test_change_runs_targeted_ce13_batch(self):
        result = route(["web/tests/ce13-platform/session.spec.ts"])
        self.assertTrue(result["domains"]["web"])
        self.assertTrue(result["ce13_governance"])

    def test_role_grant_change_uses_role_grant_gate(self):
        result = route([
            "internal/bizruntime/role_entitlements.go",
            "web/src/features/enterprise/components/roles/RoleEditor.vue",
        ])
        self.assertTrue(result["domains"]["access"])
        self.assertTrue(result["domains"]["web"])
        self.assertTrue(result["role_grants"])

    def test_enterprise_180_integration_selects_access_role_and_admission(self):
        result = route(["integration/enterprise_180_data_policy_mysql_test.go"])
        self.assertTrue(result["domains"]["access"])
        self.assertTrue(result["role_grants"])
        self.assertTrue(result["enterprise180"])

    def test_policy_source_contract_and_reusable_lane_cannot_skip_admission(self):
        for path in [
            "contracts/proto/access/v1/tenant_role.proto",
            "contracts/proto/access/v1/tenant_member.proto",
            "contracts/proto/deviceops/v1/deviceops.proto",
            ".github/workflows/enterprise-role-qualification.yml",
            "internal/access/application/tenant_data_policy.go",
            "internal/access/infrastructure/persistence/business_scope.go",
            "web/src/features/enterprise/components/policies/RoleDataPolicyDialog.vue",
            "docs/enterprise-center/enterprise180-policy-contract.v1.json",
        ]:
            with self.subTest(path=path):
                result = route([path])
                self.assertTrue(result["enterprise180"])
                self.assertTrue(result["role_grants"])
                self.assertFalse(result["docs_only"])

    def test_admission_infrastructure_does_not_impersonate_business_candidate(self):
        result = route([
            "scripts/enterprise_180_admission.py",
            "docs/delivery/ENTERPRISE-180-ADMISSION.md",
        ])
        self.assertFalse(result["enterprise180"])

    def test_unrelated_member_change_does_not_use_role_grant_gate(self):
        result = route(["internal/access/application/tenant_member_lifecycle.go"])
        self.assertTrue(result["domains"]["access"])
        self.assertFalse(result["role_grants"])
        self.assertFalse(result["enterprise180"])


    def test_integration_uses_access_lane(self):
        result = route(["integration/enterprise_183_notification_preferences_mysql_test.go"])
        self.assertTrue(result["domains"]["access"])
        self.assertFalse(result["domains"]["core"])
        self.assertFalse(result["enterprise180"])

    def test_preference_boundary_and_bff_use_access(self):
        result = route([
            "internal/access/infrastructure/persistence/notification_preference_session.go",
            "internal/bizruntime/web_notification_preferences.go",
        ])
        self.assertTrue(result["domains"]["access"])
        self.assertFalse(result["role_grants"])
        self.assertFalse(result["enterprise180"])


if __name__ == "__main__":
    unittest.main()
