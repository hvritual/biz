"""Read-only commercial Make recipes; probes are not product qualification."""
from pathlib import Path
import subprocess
import tempfile
import unittest


class ReadOnlyMakeEntrypointsTest(unittest.TestCase):
    """Exercise the actual Make recipes with import probes, without framework/DB IO."""

    def run_recipes(self, remove_bytecode_guard=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            makefile = (Path(__file__).resolve().parents[1] / 'Makefile').read_text()
            if remove_bytecode_guard:
                makefile = makefile.replace('python3 -B ', 'python3 ')
            (root / 'Makefile').write_text(makefile)
            for folder in ('scripts', 'docs/commercial-entitlements/tools'):
                target = root / folder
                target.mkdir(parents=True)
                (target / 'readonly_probe.py').write_text('VALUE = 1\n')
            for file in ('docs/commercial-entitlements/tools/check_plan.py',
                         'scripts/check_commercial_delivery.py',
                         'scripts/check_ui_skill_source.py'):
                (root / file).write_text('import readonly_probe\nassert readonly_probe.VALUE == 1\n')
            for file in ('test_commercial_delivery.py', 'test_ui_skill_source.py'):
                (root / 'scripts' / file).write_text(
                    'import unittest, readonly_probe\n'
                    'class Probe(unittest.TestCase):\n'
                    '    def test_import(self):\n'
                    '        self.assertEqual(1, readonly_probe.VALUE)\n')
            # A parent's -B/PYTHONDONTWRITEBYTECODE must not mask a broken Make recipe.
            import os
            env = os.environ.copy()
            env.pop('PYTHONDONTWRITEBYTECODE', None)
            env.pop('PYTHONPYCACHEPREFIX', None)
            result = subprocess.run(
                ['make', '--no-print-directory', 'commercial-delivery-check', 'ui-skill-check'],
                cwd=root, env=env, text=True, capture_output=True, timeout=20)
            self.assertEqual(0, result.returncode, result.stdout + result.stderr)
            return sorted(str(file.relative_to(root)) for file in root.rglob('*.pyc'))

    def test_check_recipes_do_not_leave_bytecode_in_source(self):
        self.assertEqual([], self.run_recipes())

    def test_probe_detects_bytecode_when_make_guard_is_removed(self):
        self.assertTrue(self.run_recipes(remove_bytecode_guard=True))


if __name__ == '__main__':
    unittest.main()
