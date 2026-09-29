#!/usr/bin/env python3
"""Verify the vendored typography documents; not a visual approval gate."""
import argparse
import hashlib
import json
from pathlib import Path
import re

from check_commercial_delivery import read_json

FILES = {'SKILL.md', 'choosing-fonts.md', 'css-cheat-sheet.md',
         'details-and-accessibility.md', 'spacing-and-sizing.md',
         'variable-fonts-and-opentype.md', 'wrapping-and-punctuation.md', 'LICENSE'}


def validate(directory: Path) -> dict:
    errors, hashes = [], {}
    try:
        # Do not follow a substituted skill directory or an ancestor symlink.
        if any(path.is_symlink() for path in (directory, *directory.parents)):
            raise ValueError('SYMLINK_SKILL_DIRECTORY')
        directory = directory.resolve()
        manifest_file = directory / 'SOURCE.json'
        if manifest_file.is_symlink():
            raise ValueError('SYMLINK_MANIFEST')
        manifest = read_json(manifest_file)
        if (manifest.get('schema_version') != 1 or manifest.get('repository') != 'jakubkrehel/skills' or
                manifest.get('commit') != '267330e1adfc66a718fb65fa6918c1f06d0a689e' or
                manifest.get('license') != 'MIT' or set(manifest.get('files', {})) != FILES):
            raise ValueError('SOURCE_LOCK_INVALID')
        allowed = FILES | {'SOURCE.json', 'PROJECT-INTEGRATION.md'}
        for path in directory.iterdir():
            if path.name not in allowed or not path.is_file() or path.is_symlink():
                raise ValueError('UNEXPECTED_SKILL_FILE: ' + path.name)
        if not (directory / 'PROJECT-INTEGRATION.md').is_file():
            raise ValueError('PROJECT_INTEGRATION_MISSING')
        for name, expected in manifest['files'].items():
            path = directory / name
            if path.is_symlink():
                raise ValueError('SYMLINK_SKILL_FILE: ' + name)
            content = path.read_bytes()
            actual = hashlib.sha1(b'blob ' + str(len(content)).encode('ascii') + b'\0' + content).hexdigest()
            hashes[name] = hashlib.sha256(content).hexdigest()
            if not isinstance(expected, str) or not re.fullmatch(r'[0-9a-f]{40}', expected) or actual != expected:
                errors.append('UPSTREAM_BLOB_MISMATCH: ' + name)
            if name.endswith('.md'):
                for reference in re.findall(r'\[[^\]]+\]\(([^)]+)\)', content.decode('utf-8')):
                    target = reference.split('#', 1)[0]
                    if not target or '://' in target:
                        continue
                    if target not in FILES or not (directory / target).is_file():
                        errors.append('BROKEN_SKILL_REFERENCE: ' + name + ' -> ' + reference)
        return {'result': 'FAIL' if errors else 'PASS', 'errors': errors, 'file_sha256': hashes,
                'upstream_commit': manifest['commit'], 'visual_review': 'NOT_PERFORMED'}
    except (OSError, ValueError, TypeError, AttributeError) as error:
        return {'result': 'FAIL', 'errors': [str(error)], 'visual_review': 'NOT_PERFORMED'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    args = parser.parse_args()
    result = validate(args.root / '.agents/skills/better-typography')
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result['result'] == 'PASS' else 1


if __name__ == '__main__':
    raise SystemExit(main())
