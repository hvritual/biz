from __future__ import annotations

import json
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def git_show(ref: str, path: str) -> str:
    return subprocess.check_output(['git', 'show', f'{ref}:{path}'], cwd=ROOT, text=True)


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.write_text(content, encoding='utf-8')


router_path = 'web/src/router/index.ts'
router = git_show('HEAD', router_path)
platform_routes = '''    {
      path: '/platform/commercial/features',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '商业功能', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'features' },
    },
    {
      path: '/platform/commercial/add-ons',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '增购项', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'add-ons' },
    },
    {
      path: '/platform/commercial/subscriptions',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '租户订阅', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'subscriptions' },
    },
    {
      path: '/platform/commercial/changes',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '套餐变更', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'changes' },
    },
    {
      path: '/platform/commercial/expiry',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '到期与宽限', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'expiry' },
    },
    {
      path: '/platform/commercial/authorization',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '授权诊断', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'authorization' },
    },
    {
      path: '/platform/commercial/quotas',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '额度管理', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'quotas' },
    },
    {
      path: '/platform/commercial/overrides',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '专项授权', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'overrides' },
    },
    {
      path: '/platform/commercial/usage-billing',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '用量计费', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'usage-billing' },
    },
    {
      path: '/platform/commercial/audit',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '商业审计', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'audit' },
    },
'''
anchor = "    {\n      path: '/enterprise/members',"
if '/platform/commercial/features' not in router:
    if anchor not in router:
        raise RuntimeError('canonical enterprise route insertion anchor missing')
    router = router.replace(anchor, platform_routes + anchor, 1)
for path in ['MembersView.vue', 'RolesView.vue', 'OrganizationView.vue', 'PlansView.vue', 'CompanyView.vue']:
    if path not in router:
        raise RuntimeError(f'canonical enterprise page lost during main sync: {path}')
write(router_path, router)

# Main navigation is a strict superset of the branch platform navigation while enterprise navigation is unchanged.
for path in ['web/src/router/navigation.ts', 'web/src/router/navigation.spec.ts']:
    write(path, git_show('origin/main', path))

# UI contracts are merged as a union: branch owns canonical enterprise contracts and convergence rules;
# main contributes the new platform lifecycle surfaces.
contracts_path = 'web/ui-contracts.json'
ours = json.loads(git_show('HEAD', contracts_path))
theirs = json.loads(git_show('origin/main', contracts_path))
merged = dict(ours)
merged['rules'] = {**theirs.get('rules', {}), **ours.get('rules', {})}
merged['navigation'] = ours.get('navigation', theirs.get('navigation', {}))
routes = []
seen = set()
for route in [*ours.get('routes', []), *theirs.get('routes', [])]:
    path = route.get('path')
    if path in seen:
        continue
    seen.add(path)
    routes.append(route)
merged['routes'] = routes
write(contracts_path, json.dumps(merged, ensure_ascii=False, indent=2) + '\n')

for required in [
    '/enterprise/members', '/enterprise/roles', '/enterprise/organization', '/enterprise/plan', '/enterprise/company',
    '/platform/commercial/features', '/platform/commercial/subscriptions', '/platform/commercial/authorization',
    '/platform/commercial/quotas', '/platform/commercial/usage-billing', '/platform/commercial/changes',
    '/platform/commercial/add-ons', '/platform/commercial/overrides', '/platform/commercial/expiry', '/platform/commercial/audit',
]:
    if required not in {route.get('path') for route in routes}:
        raise RuntimeError(f'missing merged UI contract: {required}')

subprocess.check_call(['git', 'add', router_path, 'web/src/router/navigation.ts', 'web/src/router/navigation.spec.ts', contracts_path], cwd=ROOT)
unmerged = subprocess.check_output(['git', 'diff', '--name-only', '--diff-filter=U'], cwd=ROOT, text=True).strip()
if unmerged:
    raise RuntimeError(f'unexpected unresolved main-sync conflicts:\n{unmerged}')
print('Main synchronization conflicts resolved: canonical enterprise routes preserved, platform lifecycle routes/contracts absorbed.')
