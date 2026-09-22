#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import pathlib
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "coffeelink_performance",
    ROOT / "scripts" / "coffeelink_performance.py",
)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader
SPEC.loader.exec_module(MODULE)


class CoffeeLinkPerformanceTests(unittest.TestCase):
    def test_pass_at_target_boundary(self):
        self.assertEqual(MODULE.classify(180, 180, 240, "success"), MODULE.PASS)

    def test_degraded_between_target_and_hard_budget(self):
        self.assertEqual(MODULE.classify(181, 180, 240, "success"), MODULE.DEGRADED)
        self.assertEqual(MODULE.classify(240, 180, 240, "success"), MODULE.DEGRADED)

    def test_budget_exceeded_above_hard_budget(self):
        self.assertEqual(MODULE.classify(241, 180, 240, "success"), MODULE.BUDGET_EXCEEDED)

    def test_test_failure_has_precedence(self):
        self.assertEqual(MODULE.classify(10, 180, 240, "failure"), MODULE.TEST_FAILURE)

    def test_started_timestamp_must_be_positive(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / "started"
            path.write_text("0\n", encoding="utf-8")
            with self.assertRaises(ValueError):
                MODULE.read_started(path)


if __name__ == "__main__":
    unittest.main()
