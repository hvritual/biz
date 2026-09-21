#!/usr/bin/env python3
import unittest

from ci_changed_files_router import route


class RouteTests(unittest.TestCase):
    def test_docs_only_skips_heavy_domains(self):
        result = route(["docs/enterprise-center/README.md", "README.md"])
        self.assertTrue(result["docs_only"])
        self.assertEqual(result["domain_count"], 0)
        self.assertFalse(result["merge_gate_required"])

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


if __name__ == "__main__":
    unittest.main()
