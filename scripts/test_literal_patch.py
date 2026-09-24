#!/usr/bin/env python3
import subprocess
import tempfile
from pathlib import Path
import unittest
from literal_patch import apply, blob_sha, replace_once


class PatchTests(unittest.TestCase):
    def test_js_replacement_regression_reproduces(self):
        js = '''const src="OLD\\nTAIL"; const text="^Test$'";
        if(src.replace("OLD",text)==="^Test$'\\nTAIL") process.exit(1);
        if(src.replace("OLD",()=>text)!=="^Test$'\\nTAIL") process.exit(2);'''
        subprocess.run(['node', '-e', js], check=True, timeout=10)

    def test_dollar_tokens_are_literal(self):
        for value in ["^Test$'", '$&', '$`', '$$', '$1']:
            self.assertEqual(replace_once('OLD\nTAIL', 'OLD', value), value + '\nTAIL')

    def test_ambiguous_context_rejected(self):
        with self.assertRaises(ValueError):
            replace_once('OLD OLD', 'OLD', 'NEW')

    def test_source_stale_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'file.txt'
            path.write_text('OLD')
            with self.assertRaisesRegex(ValueError, 'BASE_BLOB'):
                apply(path, '0' * 40, 'OLD', 'NEW')
            self.assertEqual(path.read_text(), 'OLD')

    def test_invalid_workflow_never_written(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'gate.yml'
            text = 'jobs:\n  test:\n    steps:\n      - run: echo ok\n'
            path.write_text(text)
            with self.assertRaisesRegex(ValueError, 'SHELL_SYNTAX'):
                apply(path, blob_sha(text.encode()), 'echo ok', "echo 'broken")
            self.assertEqual(path.read_text(), text)


if __name__ == '__main__':
    unittest.main()
