#!/usr/bin/env python3
"""Adversarial admission cases; fixtures are not runtime authorization evidence."""
import copy
import json
import os
import pathlib
import tempfile
import unittest
from unittest.mock import patch

import enterprise_180_admission as gate


def table(q009="ACCEPTED", q011="ACCEPTED"):
    return f"| Q-009 范围绑定 | {q009} | scope | #180 |\n| Q-011 数据策略 | {q011} | policy | #180 |\n"


class AdmissionTests(unittest.TestCase):
    def setUp(self):
        self.policy = json.loads(gate.POLICY_CONTRACT.read_text())

    def test_approved_version_is_admitted(self):
        self.assertEqual(gate.evaluate(table(), True, self.policy), ("ADMITTED", []))

    def test_contract_is_mandatory_even_when_markdown_says_accepted(self):
        self.assertEqual(gate.evaluate(table(), True)[0], "BLOCKED")

    def test_pending_decisions_remain_explicit_blockers(self):
        state, blockers = gate.evaluate(table("PENDING_HUMAN", "PENDING_HUMAN"), True, self.policy)
        self.assertEqual(state, "BLOCKED")
        self.assertEqual([item["id"] for item in blockers], ["Q-009", "Q-011"])

    def test_single_pending_blocks(self):
        self.assertEqual(gate.evaluate(table("ACCEPTED", "PENDING_HUMAN"), True, self.policy)[0], "BLOCKED")

    def test_missing_decision_blocks(self):
        self.assertEqual(gate.evaluate("", True, self.policy)[0], "BLOCKED")

    def test_duplicate_decision_blocks_even_when_both_accepted(self):
        self.assertEqual(gate.evaluate(table() + table(), True, self.policy)[0], "BLOCKED")

    def test_non_applicable_needs_no_policy(self):
        self.assertEqual(gate.evaluate("", False), ("NOT_APPLICABLE", []))

    def test_all_top_level_fields_are_bound(self):
        for field in self.policy:
            altered = copy.deepcopy(self.policy)
            altered.pop(field)
            with self.subTest(field=field):
                self.assertTrue(gate.validate_policy_contract(altered))

    def test_all_nested_policy_values_are_bound(self):
        for group in ("q009_business_scope_binding", "q011_data_policy_composition"):
            for field in self.policy[group]:
                altered = copy.deepcopy(self.policy)
                altered[group][field] = "unapproved"
                with self.subTest(group=group, field=field):
                    self.assertTrue(gate.validate_policy_contract(altered))

    def test_new_unknown_rule_cannot_be_smuggled_in(self):
        self.policy["allow_all"] = True
        self.assertTrue(gate.validate_policy_contract(self.policy))

    def test_wrong_types_fail_closed(self):
        for value in (None, [], 1, "ACCEPTED", True):
            self.assertTrue(gate.validate_policy_contract(value))

    def test_bool_is_not_integer_schema_version(self):
        self.policy["schema_version"] = True
        self.assertTrue(gate.validate_policy_contract(self.policy))

    def test_negative_examples_cannot_disappear(self):
        self.policy["required_negative_examples"].pop()
        self.assertTrue(gate.validate_policy_contract(self.policy))

    def test_formatting_and_key_order_do_not_change_semantics(self):
        value = json.loads(json.dumps(self.policy, sort_keys=True, indent=4))
        self.assertEqual(gate.validate_policy_contract(value), [])

    def test_duplicate_json_key_is_rejected(self):
        with self.assertRaises(ValueError):
            json.loads('{"status":"PENDING","status":"ACCEPTED"}', object_pairs_hook=gate.unique_object)

    def test_missing_authority_writes_blocked_report(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(gate, "DECISIONS", pathlib.Path(temp) / "missing"):
            self.assertEqual(gate.receipt(True)["state"], "BLOCKED")

    def test_not_applicable_never_reads_missing_authority(self):
        with patch.object(gate, "DECISIONS", pathlib.Path("/does-not-exist")):
            self.assertEqual(gate.receipt(False)["state"], "NOT_APPLICABLE")

    def test_receipt_cli_and_authority_hashes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            paths = [root / name for name in ("decisions.md", "contracts.md", "policy.json")]
            paths[0].write_text(table())
            paths[1].write_text("Approved contract projection")
            paths[2].write_text(json.dumps(self.policy))
            output = root / "evidence" / "receipt.json"
            with patch.object(gate, "ROOT", root), patch.object(gate, "DECISIONS", paths[0]), \
                 patch.object(gate, "CONTRACTS", paths[1]), patch.object(gate, "POLICY_CONTRACT", paths[2]), \
                 patch.dict(os.environ, {"GITHUB_ACTIONS": "false", "PR_NUMBER": ""}), \
                 patch("sys.argv", ["admission", "check", "--output", str(output)]):
                self.assertEqual(gate.main(), 0)
                result = json.loads(output.read_text())
                self.assertEqual(result["state"], "ADMITTED")
                self.assertEqual(result["authority"]["policy_semantic_sha256"], gate.ACCEPTED_POLICY_DIGEST)
                self.assertFalse(result["semantics_implemented"])
                paths[2].write_text("{bad-json}")
                self.assertEqual(gate.main(), 2)
                self.assertEqual(json.loads(output.read_text())["state"], "BLOCKED")

    def test_authority_size_is_bounded(self):
        with tempfile.NamedTemporaryFile() as handle, patch.object(gate, "MAX_AUTHORITY_BYTES", 5):
            handle.write(b"123456")
            handle.flush()
            with self.assertRaises(ValueError):
                gate.read_authority(pathlib.Path(handle.name))

    def test_ci_candidate_identity_must_match_checkout(self):
        with patch.dict(os.environ, {"GITHUB_ACTIONS": "true", "CANDIDATE_SHA": "a" * 40, "PR_NUMBER": "180"}), \
             patch.object(gate.subprocess, "check_output", return_value="b" * 40):
            self.assertEqual(gate.receipt(True)["state"], "BLOCKED")


if __name__ == "__main__":
    unittest.main(verbosity=2)
