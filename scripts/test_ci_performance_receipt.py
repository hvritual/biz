#!/usr/bin/env python3
from __future__ import annotations
import importlib.util
import pathlib
import unittest

ROOT=pathlib.Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location("ci_performance_receipt",ROOT/"scripts"/"ci_performance_receipt.py")
m=importlib.util.module_from_spec(spec); assert spec.loader; spec.loader.exec_module(m)

class ReceiptTests(unittest.TestCase):
    def test_pass(self): self.assertEqual(m.classify(180,180,240,"success"),m.PASS)
    def test_degraded(self): self.assertEqual(m.classify(181,180,240,"success"),m.DEGRADED)
    def test_hard_exceeded(self): self.assertEqual(m.classify(241,180,240,"success"),m.BUDGET_EXCEEDED)
    def test_failure_precedence(self): self.assertEqual(m.classify(1,180,240,"failure"),m.TEST_FAILURE)

if __name__=="__main__": unittest.main()
