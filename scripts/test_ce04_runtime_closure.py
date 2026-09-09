"""The runtime gate must reject omissions, not merely accept a larger count."""
import copy
import json
import unittest
from pathlib import Path
from ce04_runtime_closure import validate

ROOT = Path(__file__).resolve().parents[1]


def fixture():
    manifest = json.loads((ROOT/'contracts/generated/manifest.json').read_text())
    config = json.loads((ROOT/'.yunka/dev.json').read_text())
    nodes = sorted('application:' + s['domain'] + '/' + s['application']['name']
                   for s in manifest['services'] if s.get('application'))
    routes = sorted({b['path'] for s in manifest['services'] for m in (s.get('methods') or [])
                     for b in (m.get('http') or [])})
    process = {'name': 'biz', 'state': 'ready', 'ready': True,
               'diagnostics': {'ready': True}, 'graphNodes': nodes}
    status = {'state': 'running', 'processes': [process]}
    graph = {'nodes': [{'kind': 'application', 'id': n} for n in nodes + ['application:biz']],
             'edges': [{'kind': 'runs', 'from': 'process:biz', 'to': n} for n in nodes]}
    diagnostics = {'schemaVersion': 1, 'core': {'schemaVersion': 1, 'state': 'ready',
        'health': {'ready': True}, 'runtime': {'rpcServerCount': 1, 'routeCount': len(routes)},
        'routes': routes}}
    return {'manifest': manifest, 'config': config, 'status': status,
            'graph': graph, 'diagnostics': diagnostics}


class RuntimeClosureTests(unittest.TestCase):
    def test_complete_inventory(self):
        data = fixture()
        self.assertEqual(validate(**data)['applications'], len(data['manifest']['services']))

    def reject(self, change):
        data = copy.deepcopy(fixture())
        change(data)
        with self.assertRaises(ValueError):
            validate(**data)

    def test_profile_omission(self):
        self.reject(lambda d: d['config']['processes'][0]['graphNodes'].pop())

    def test_runtime_omission(self):
        self.reject(lambda d: d['status']['processes'][0]['graphNodes'].pop())

    def test_duplicate_graph_node(self):
        self.reject(lambda d: d['graph']['nodes'].append(d['graph']['nodes'][0]))

    def test_missing_runtime_root(self):
        self.reject(lambda d: d['graph']['nodes'].pop())

    def test_missing_runs_edge(self):
        self.reject(lambda d: d['graph']['edges'].pop())

    def test_duplicate_runs_edge(self):
        self.reject(lambda d: d['graph']['edges'].append(d['graph']['edges'][0]))

    def test_missing_generated_route(self):
        self.reject(lambda d: d['diagnostics']['core']['routes'].pop())

    def test_ready_text_without_ready_state(self):
        self.reject(lambda d: d['status']['processes'][0].update(ready=False))


if __name__ == '__main__':
    unittest.main()
