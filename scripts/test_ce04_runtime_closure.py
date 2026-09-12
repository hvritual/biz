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

    def test_extra_profile_application(self):
        self.reject(lambda d: d['config']['processes'][0]['graphNodes'].append('application:fake/unlisted'))

    def test_duplicate_profile_application(self):
        self.reject(lambda d: d['config']['processes'][0]['graphNodes'].append(d['config']['processes'][0]['graphNodes'][0]))

    def test_extra_runtime_application(self):
        self.reject(lambda d: d['status']['processes'][0]['graphNodes'].append('application:fake/unlisted'))

    def test_duplicate_observed_application(self):
        self.reject(lambda d: d['status']['processes'][0]['graphNodes'].append(d['status']['processes'][0]['graphNodes'][0]))

    def test_extra_graph_application(self):
        self.reject(lambda d: d['graph']['nodes'].append({'kind': 'application', 'id': 'application:fake/unlisted'}))

    def test_unexpected_runs_owner(self):
        self.reject(lambda d: d['graph']['edges'][0].update({'from': 'process:other'}))

    def test_extra_runs_target(self):
        self.reject(lambda d: d['graph']['edges'].append({'kind': 'runs', 'from': 'process:biz', 'to': 'application:fake/unlisted'}))

    def test_duplicate_contract_declaration(self):
        self.reject(lambda d: d['manifest']['services'].append(d['manifest']['services'][0]))

    def test_missing_baseline_contract(self):
        self.reject(lambda d: d['manifest']['services'].pop())

    def test_extra_http_route(self):
        self.reject(lambda d: d['diagnostics']['core']['routes'].append('/unregistered'))

    def test_duplicate_http_route(self):
        self.reject(lambda d: d['diagnostics']['core']['routes'].append(d['diagnostics']['core']['routes'][0]))

    def test_route_count_mismatch(self):
        self.reject(lambda d: d['diagnostics']['core']['runtime'].update(routeCount=0))

    def test_diagnostics_not_ready(self):
        self.reject(lambda d: d['status']['processes'][0]['diagnostics'].update(ready=False))

    def test_core_health_not_ready(self):
        self.reject(lambda d: d['diagnostics']['core']['health'].update(ready=False))

    def test_missing_rpc_server(self):
        self.reject(lambda d: d['diagnostics']['core']['runtime'].update(rpcServerCount=0))

    def test_future_declared_application_is_not_rejected_by_fixed_count(self):
        data = fixture()
        node = 'application:example/new_application'
        data['manifest']['services'].append({'domain': 'example',
            'application': {'name': 'new_application'},
            'methods': [{'http': [{'path': '/v1/example'}]}]})
        data['config']['processes'][0]['graphNodes'].append(node)
        data['status']['processes'][0]['graphNodes'].append(node)
        data['graph']['nodes'].append({'kind': 'application', 'id': node})
        data['graph']['edges'].append({'kind': 'runs', 'from': 'process:biz', 'to': node})
        data['diagnostics']['core']['routes'].append('/v1/example')
        data['diagnostics']['core']['runtime']['routeCount'] += 1
        result = validate(**data)
        self.assertEqual(result['applications'], len(data['manifest']['services']))
        self.assertEqual(result['routes'], len(data['diagnostics']['core']['routes']))


if __name__ == '__main__':
    unittest.main()
