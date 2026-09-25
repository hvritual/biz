"""Export canonical generated Git objects; never create commits or update refs."""
from __future__ import annotations
import hashlib
import json
import os
from pathlib import Path
import subprocess
import urllib.request

ALLOWED = {
    'contracts/commercial/generated/catalog.json',
    'contracts/commercial/generated/catalog.ts',
    'contracts/generated/assembly-plan.json',
    'contracts/generated/client.ts',
    'contracts/generated/manifest.json',
    'contracts/generated/openapi.json',
    'contracts/generated/operation-plans.json',
    'internal/access/authorization/catalog_gen.go',
    'internal/assembly/zz_yunka_assembly_gen.go',
    'contracts/gen/notification/v1/configuration.pb.go',
    'contracts/gen/notification/v1/configuration_grpc.pb.go',
    'internal/notification/application/zz_yunka_message_configuration_application_port_gen.go',
    'internal/notification/policy/zz_yunka_message_configuration_operation_plan_gen.go',
    'internal/notification/policy/zz_yunka_message_configuration_operation_policy_gen.go',
    'internal/notification/transport/rest/zz_yunka_message_configuration_operation_executor_gen.go',
    'internal/notification/transport/rpc/zz_yunka_message_configuration_operation_executor_gen.go',
}

def git(*args: str) -> str:
    return subprocess.check_output(['git', *args], text=True).strip()

def snapshot() -> list[dict[str, object]]:
    changed = set(git('diff', '--name-only', 'HEAD').splitlines())
    changed.update(git('ls-files', '--others', '--exclude-standard').splitlines())
    if not changed or not changed <= ALLOWED:
        raise RuntimeError('Unexpected or empty generated change set: ' + repr(sorted(changed - ALLOWED)))
    result = []
    for name in sorted(changed):
        path = Path(name)
        if path.is_symlink() or not path.is_file():
            raise RuntimeError('Non-regular generated output: ' + name)
        data = path.read_bytes()
        data.decode('utf-8', errors='strict')
        result.append({'path': name, 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest(), 'blob_sha': hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest()})
    return result

def api(path: str, body: dict[str, object]) -> dict[str, object]:
    # Fixed repository and object-only endpoint allowlist. No commit, ref,
    # workflow, secret, permission, branch-protection or administration writes.
    if path not in {'git/blobs', 'git/trees'}:
        raise RuntimeError('Non-object endpoint denied')
    token = os.environ['GH_TOKEN']
    request = urllib.request.Request(
        'https://api.github.com/repos/hvritual/biz/' + path,
        data=json.dumps(body, ensure_ascii=False).encode('utf-8'),
        headers={'Authorization': 'Bearer ' + token, 'Accept': 'application/vnd.github+json', 'X-GitHub-Api-Version': '2022-11-28', 'Content-Type': 'application/json'},
        method='POST',
    )
    with urllib.request.urlopen(request, timeout=45) as response:
        return json.load(response)

if __name__ == '__main__':
    output = Path(os.environ['OUTPUT_DIR'])
    output.mkdir(parents=True, exist_ok=True)
    candidate = os.environ['CANDIDATE_SHA']
    if git('rev-parse', 'HEAD') != candidate:
        raise RuntimeError('Candidate changed')
    actual = snapshot()
    import sys
    if sys.argv[1] == 'snapshot':
        (output / 'first-snapshot.json').write_text(json.dumps(actual, indent=2) + '\n')
    elif sys.argv[1] == 'export':
        expected = json.loads((output / 'first-snapshot.json').read_text())
        if actual != expected:
            raise RuntimeError('Second generation is not byte-stable')
        if os.environ.get('GITHUB_REPOSITORY') != 'hvritual/biz':
            raise RuntimeError('Unexpected repository')
        elements = []
        for item in actual:
            data = Path(item['path']).read_text()
            blob = api('git/blobs', {'content': data, 'encoding': 'utf-8'})
            if blob.get('sha') != item['blob_sha']:
                raise RuntimeError('Remote blob mismatch: ' + item['path'])
            elements.append({'path': item['path'], 'mode': '100644', 'type': 'blob', 'sha': blob['sha']})
        base_tree = git('rev-parse', 'HEAD^{tree}')
        tree = api('git/trees', {'base_tree': base_tree, 'tree': elements})
        result = {'candidate_sha': candidate, 'base_tree': base_tree, 'generated_tree': tree['sha'], 'files': actual, 'run_id': os.environ['GITHUB_RUN_ID'], 'run_attempt': os.environ['GITHUB_RUN_ATTEMPT'], 'ref_updated': False, 'commit_created': False}
        (output / 'generated-objects.json').write_text(json.dumps(result, indent=2) + '\n')
        print('GENERATED_OBJECT_TREE=' + str(tree['sha']))
    else:
        raise RuntimeError('Unknown mode')
