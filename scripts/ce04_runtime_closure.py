"""Compare runtime evidence with generated contracts without fixed app counts."""
from __future__ import annotations
import argparse
import json
from pathlib import Path
from typing import Any

REQUIRED = {
    'application:access/tenant_lifecycle',
    'application:access/tenant_member_lifecycle',
    'application:access/tenant_role_permission',
    'application:deviceops/device_management',
    'application:deviceops/device_transfer',
    'application:deviceops/site_management',
    'application:commercial/module_catalog',
    'application:commercial/entitlement_management',
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValueError('CE04-RUNTIME-CLOSURE: ' + message)


def validate(manifest: dict[str, Any], config: dict[str, Any],
             status: dict[str, Any], graph: dict[str, Any],
             diagnostics: dict[str, Any]) -> dict[str, int]:
    services = [s for s in manifest['services'] if s.get('application')]
    expected = sorted('application:' + s['domain'] + '/' + s['application']['name']
                      for s in services)
    require(len(expected) == len(set(expected)) and REQUIRED <= set(expected),
            'missing baseline applications or duplicate declaration')
    configured = config['processes']
    require(len(configured) == 1 and configured[0]['name'] == 'biz',
            'expected the declared single biz process')
    require(sorted(configured[0]['graphNodes']) == expected,
            'dev profile differs from the generated application inventory')
    require(status['state'] == 'running' and len(status['processes']) == 1,
            'runtime process is not running')
    process = status['processes'][0]
    require(process['name'] == 'biz' and process['state'] == 'ready'
            and process['ready'] is True and process['diagnostics']['ready'] is True,
            'process or diagnostics is not ready')
    require(sorted(process['graphNodes']) == expected,
            'observed process nodes differ from the generated inventory')
    observed_nodes = sorted(n['id'] for n in graph['nodes'] if n['kind'] == 'application')
    # Yunka also emits the process-level application:biz runtime root.
    root_node = 'application:' + config['runtime']['application']
    require(root_node == 'application:biz', 'wrong runtime application root')
    require(observed_nodes == sorted(expected + [root_node]),
            'application graph has missing/extra/duplicate nodes')
    runs = [e for e in graph['edges'] if e['kind'] == 'runs']
    require(all(e['from'] == 'process:biz' for e in runs), 'unexpected process owns an application')
    require(sorted(e['to'] for e in runs) == expected,
            'runs edges have missing/extra/duplicate applications')
    expected_routes = sorted({b['path'] for s in services
                              for m in (s.get('methods') or [])
                              for b in (m.get('http') or [])})
    core = diagnostics['core']
    require(diagnostics['schemaVersion'] == 1 and core['schemaVersion'] == 1,
            'unknown diagnostics schema')
    require(core['state'] == 'ready' and core['health']['ready'] is True,
            'core is not ready')
    require(core['runtime']['rpcServerCount'] == 1, 'RPC server is not ready')
    require(sorted(core['routes']) == expected_routes
            and core['runtime']['routeCount'] == len(expected_routes),
            'advertised HTTP routes differ from the generated bindings')
    return {'applications': len(expected), 'routes': len(expected_routes)}


def main() -> None:
    parser = argparse.ArgumentParser()
    for name in ('manifest', 'config', 'status', 'graph', 'diagnostics'):
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    data = {name: json.loads(path.read_text()) for name, path in vars(args).items()}
    result = validate(**data)
    print('CE04_RUNTIME_CLOSURE=PASS ' + json.dumps(result, sort_keys=True))


if __name__ == '__main__':
    main()
