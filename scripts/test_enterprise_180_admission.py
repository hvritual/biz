#!/usr/bin/env python3
import json
import pathlib
import tempfile
import unittest
from unittest.mock import patch

import enterprise_180_admission as gate


def table(q009="PENDING_HUMAN", q011="PENDING_HUMAN"):
    return f"""| Q | Status | Decision | Block |
| --- | --- | --- | --- |
| Q-009 范围绑定 | {q009} | scope decision | #180 |
| Q-011 数据策略 | {q011} | policy combination | #180 |
"""


class Enterprise180AdmissionTests(unittest.TestCase):
    def test_current_repository_truth_is_blocked(self):
        state, blockers = gate.evaluate(gate.DECISIONS.read_text(encoding="utf-8"), True)
        self.assertEqual(state, "BLOCKED")
        self.assertEqual(
            [(item["id"], item["status"]) for item in blockers],
            [("Q-009", "PENDING_HUMAN"), ("Q-011", "PENDING_HUMAN")],
        )

    def test_both_accepted_is_admitted(self):
        state, blockers = gate.evaluate(table("ACCEPTED", "ACCEPTED"), True)
        self.assertEqual(state, "ADMITTED")
        self.assertEqual(blockers, [])

    def test_single_pending_decision_blocks_only_itself(self):
        state, blockers = gate.evaluate(table("ACCEPTED", "PENDING_HUMAN"), True)
        self.assertEqual(state, "BLOCKED")
        self.assertEqual(len(blockers), 1)
        self.assertEqual(blockers[0]["id"], "Q-011")

    def test_missing_decision_fails_closed(self):
        state, blockers = gate.evaluate("| Q-009 范围绑定 | ACCEPTED | x | y |\n", True)
        self.assertEqual(state, "BLOCKED")
        self.assertEqual(blockers[0]["id"], "Q-011")
        self.assertEqual(blockers[0]["status"], "MISSING")

    def test_duplicate_decision_fails_closed(self):
        text = table("ACCEPTED", "ACCEPTED") + "| Q-011 数据策略 | ACCEPTED | duplicate | #180 |\n"
        state, blockers = gate.evaluate(text, True)
        self.assertEqual(state, "BLOCKED")
        self.assertEqual(blockers[0]["id"], "Q-011")
        self.assertEqual(blockers[0]["status"], "AMBIGUOUS")

    def test_not_required_does_not_block_unrelated_role_work(self):
        state, blockers = gate.evaluate(table(), False)
        self.assertEqual(state, "NOT_APPLICABLE")
        self.assertEqual(blockers, [])

    def test_receipt_explicitly_disclaims_policy_semantics(self):
        report = gate.receipt(True)
        self.assertFalse(report["semantics_implemented"])
        self.assertEqual(report["issue_number"], 180)
        self.assertEqual(report["state"], "BLOCKED")
        self.assertEqual([b["id"] for b in report["blockers"]], ["Q-009", "Q-011"])

    def test_cli_writes_machine_readable_blocked_receipt(self):
        with tempfile.TemporaryDirectory() as temp:
            output = pathlib.Path(temp) / "receipt.json"
            with patch("sys.argv", ["enterprise_180_admission.py", "check", "--required", "true", "--output", str(output)]):
                self.assertEqual(gate.main(), 2)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["state"], "BLOCKED")
            self.assertFalse(report["semantics_implemented"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
