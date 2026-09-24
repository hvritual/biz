#!/usr/bin/env python3
"""Exact-once literal source replacement with expected blob and pre-write YAML/bash validation."""
import argparse
import hashlib
from pathlib import Path
import os
import tempfile


def blob_sha(data):
    return hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest()


def replace_once(source, old, new):
    if not old or source.count(old) != 1:
        raise ValueError('PATCH_CONTEXT_MUST_MATCH_EXACTLY_ONCE')
    # Python literal replacement does not interpret JS $&, $`, $', $$ tokens.
    return source.replace(old, new, 1)


def apply(path, expected_blob, old, new):
    if path.is_symlink():
        raise ValueError('PATCH_SYMLINK_FORBIDDEN')
    before = path.read_bytes()
    if blob_sha(before) != expected_blob:
        raise ValueError('PATCH_BASE_BLOB_CHANGED')
    after = replace_once(before.decode('utf-8'), old, new)
    if path.suffix in {'.yml', '.yaml'}:
        from check_ci_source_safety import validate_workflow
        validate_workflow(after, path.name)
    if path.read_bytes() != before:
        raise ValueError('PATCH_SOURCE_CHANGED_DURING_VALIDATION')
    fd, tmp = tempfile.mkstemp(dir=path.parent, prefix='.literal-patch-')
    try:
        with os.fdopen(fd, 'wb') as output:
            output.write(after.encode('utf-8'))
            output.flush()
            os.fsync(output.fileno())
        os.chmod(tmp, path.stat().st_mode)
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)
    return blob_sha(after.encode('utf-8'))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--path', type=Path, required=True)
    parser.add_argument('--expected-blob', required=True)
    parser.add_argument('--old-file', type=Path, required=True)
    parser.add_argument('--new-file', type=Path, required=True)
    args = parser.parse_args()
    print(apply(args.path, args.expected_blob, args.old_file.read_text(), args.new_file.read_text()))


if __name__ == '__main__':
    main()
