#!/usr/bin/env python3
from __future__ import annotations

import json
import pathlib
import subprocess
import tempfile
import unittest

import candidate_lifecycle


class CandidateLifecycleTests(unittest.TestCase):
    def test_contract_is_valid(self):
        self.assertEqual(candidate_lifecycle.validate_contract(), [])

    def test_contract_has_stale_candidate_fallback(self):
        data = json.loads(candidate_lifecycle.CONTRACT.read_text(encoding="utf-8"))
        for state in ("FAST_VERIFIED", "DOMAIN_QUALIFIED", "CANDIDATE_FROZEN", "MERGE_QUALIFYING", "MERGE_READY"):
            self.assertIn("WORKING", data["transitions"][state])

    def test_freshness_requires_main_to_be_candidate_ancestor(self):
        with tempfile.TemporaryDirectory() as tmp:
            repo = pathlib.Path(tmp)
            subprocess.run(["git", "init", "-q", str(repo)], check=True)
            subprocess.run(["git", "-C", str(repo), "config", "user.email", "test@example.invalid"], check=True)
            subprocess.run(["git", "-C", str(repo), "config", "user.name", "candidate-test"], check=True)
            (repo / "a.txt").write_text("base\n", encoding="utf-8")
            subprocess.run(["git", "-C", str(repo), "add", "a.txt"], check=True)
            subprocess.run(["git", "-C", str(repo), "commit", "-qm", "base"], check=True)
            base = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()

            subprocess.run(["git", "-C", str(repo), "checkout", "-qb", "candidate"], check=True)
            (repo / "a.txt").write_text("candidate\n", encoding="utf-8")
            subprocess.run(["git", "-C", str(repo), "commit", "-qam", "candidate"], check=True)
            candidate = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()

            fresh = subprocess.run(
                ["git", "-C", str(repo), "merge-base", "--is-ancestor", base, candidate],
                check=False,
            )
            self.assertEqual(fresh.returncode, 0)

            subprocess.run(["git", "-C", str(repo), "checkout", "-q", "--detach", base], check=True)
            (repo / "main.txt").write_text("new main\n", encoding="utf-8")
            subprocess.run(["git", "-C", str(repo), "add", "main.txt"], check=True)
            subprocess.run(["git", "-C", str(repo), "commit", "-qm", "main moved"], check=True)
            moved_main = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()

            stale = subprocess.run(
                ["git", "-C", str(repo), "merge-base", "--is-ancestor", moved_main, candidate],
                check=False,
            )
            self.assertNotEqual(stale.returncode, 0)


if __name__ == "__main__":
    unittest.main()
