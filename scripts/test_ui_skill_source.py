"""Synthetic content proves hash checking, not upstream typography approval."""
import hashlib
import json
from pathlib import Path
import tempfile
import unittest

from check_ui_skill_source import FILES, validate


class SkillSourceTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.manifest = {'schema_version': 1, 'repository': 'jakubkrehel/skills',
                         'commit': '267330e1adfc66a718fb65fa6918c1f06d0a689e',
                         'license': 'MIT', 'files': {}}
        for name in FILES:
            self.put(name, b'Synthetic reference\n')
        (self.root / 'PROJECT-INTEGRATION.md').write_text('Project fixture')
        self.save()

    def put(self, name, content):
        (self.root / name).write_bytes(content)
        self.manifest['files'][name] = hashlib.sha1(b'blob ' + str(len(content)).encode() + b'\0' + content).hexdigest()

    def save(self):
        (self.root / 'SOURCE.json').write_text(json.dumps(self.manifest))

    def test_clean_snapshot_is_not_visual_approval(self):
        result = validate(self.root)
        self.assertEqual('PASS', result['result'])
        self.assertEqual('NOT_PERFORMED', result['visual_review'])
        self.assertEqual(8, len(result['file_sha256']))

    def test_modified_upstream_file_fails(self):
        (self.root / 'SKILL.md').write_text('tampered')
        self.assertIn('UPSTREAM_BLOB_MISMATCH: SKILL.md', validate(self.root)['errors'])

    def test_missing_reference_fails(self):
        (self.root / 'choosing-fonts.md').unlink()
        self.assertEqual('FAIL', validate(self.root)['result'])

    def test_unpinned_source_fails(self):
        self.manifest['commit'] = 'main'
        self.save()
        self.assertEqual('FAIL', validate(self.root)['result'])

    def test_font_or_extra_payload_rejected(self):
        (self.root / 'font.woff2').write_bytes(b'not a font')
        self.assertEqual('FAIL', validate(self.root)['result'])

    def test_symlink_file_rejected(self):
        (self.root / 'SKILL.md').unlink()
        (self.root / 'SKILL.md').symlink_to(self.root / 'LICENSE')
        self.assertEqual('FAIL', validate(self.root)['result'])

    def test_broken_local_link_fails_even_with_valid_hash(self):
        self.put('SKILL.md', b'[missing](missing.md)\n')
        self.save()
        self.assertIn('BROKEN_SKILL_REFERENCE: SKILL.md -> missing.md', validate(self.root)['errors'])

    def test_integration_rule_required(self):
        (self.root / 'PROJECT-INTEGRATION.md').unlink()
        self.assertEqual('FAIL', validate(self.root)['result'])


if __name__ == '__main__':
    unittest.main()
