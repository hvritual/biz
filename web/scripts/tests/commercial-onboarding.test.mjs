import assert from 'node:assert/strict'
import { test } from 'node:test'
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { accessSelectorFindings, checkCommercialOnboardingUI } from '../lib/commercial-onboarding.mjs'

const root = resolve(import.meta.dirname, '../..')
function fixture(t) {
  const repo = mkdtempSync(join(tmpdir(), 'commercial-onboarding-'))
  t.after(() => rmSync(repo, { recursive: true, force: true }))
  const write = (path, value) => {
    const target = join(repo, path)
    mkdirSync(dirname(target), { recursive: true })
    writeFileSync(target, typeof value === 'string' ? value : JSON.stringify(value))
  }
  const op = { operationId: 'example.list', security: {tenantRequired: true, authentication: ['web-session']}, bindings: {rpc: '/example.v1.App/List'} }
  write('contracts/generated/operation-plans.json', {schemaVersion: 2, operations: [op]})
  write('contracts/commercial/generated/catalog.json', {schema_version: 1, capabilities: [{capability_code: 'example.use', module_code: 'example'}], operations: [{operation_id: 'example.list', classification: 'tenant_business', module_code: 'example', capability_codes: ['example.use']}]})
  write('contracts/commercial/onboarding.v1.json', {schema_version: 1, modules: [{module_code: 'example', ui_routes: ['/example'], acceptance: []}]})
  write('web/src/router/index.ts', "import {createRouter} from 'vue-router'; export const router=createRouter({routes:[{path:'/example',component:()=>import('../Page.vue'),meta:{surface:'tenant',authorizationActions:['example.list']}}]})")
  write('web/src/router/navigation.ts', "export const navigation=[{path:'/example',authorizationModule:'example',authorizationActions:['example.list']}]")
  write('web/src/services/runtime/authorization.ts', 'export const allowed = (snapshot, action) => snapshot.button_codes.includes(action)')
  write('web/src/Page.vue', '<template><main>Fixture only</main></template>')
  return {repo, root:join(repo,'web'), write, mutate(path, change) { const file=join(repo,path); write(path,change(readFileSync(file,'utf8'))) }}
}

test('actual route/action references match canonical onboarding without claiming runtime access', () => {
  const report = checkCommercialOnboardingUI(root)
  assert.deepEqual(report.findings, [])
  assert.equal(report.result, 'PASS')
  assert.equal(report.sales_admission, 'NOT_PERFORMED')
  assert.equal(report.runtime_verification, 'NOT_PERFORMED')
  assert.ok(report.consumers.some((entry) => entry.route === '/enterprise/members' && entry.commercial_modules.includes('access-management')))
  assert.ok(report.consumers.some((entry) => entry.route === '/workspace/:resource(devices)' && entry.commercial_modules.includes('device-operations')))
})

test('source fixture passes but is never a runtime grant', (t) => {
  const f=fixture(t); const report=checkCommercialOnboardingUI(f.root)
  assert.deepEqual(report.findings,[])
  assert.equal(report.sales_admission,'NOT_PERFORMED')
})

test('a newly mapped business route requires a reference entry', (t) => {
  const f=fixture(t)
  f.mutate('contracts/commercial/onboarding.v1.json', text => text.replace('"/example"','"/missing"'))
  const failures=checkCommercialOnboardingUI(f.root).findings.join('\n')
  assert.match(failures,/missing onboarding consumer/)
  assert.match(failures,/does not exist/)
})

test('an unknown action cannot be authorized by the module label', (t) => {
  const f=fixture(t)
  f.mutate('web/src/router/navigation.ts',text=>text.replace('example.list','example.unknown'))
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/unknown authorization action/)
})

test('module/action ownership mismatch is blocked', (t) => {
  const f=fixture(t)
  f.mutate('web/src/router/navigation.ts',text=>text.replace("authorizationModule:'example'","authorizationModule:'other'"))
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/conflicts with.*owner/)
})

test('internal or platform operations cannot be exposed as tenant UI actions', (t) => {
  const f=fixture(t)
  f.mutate('contracts/generated/operation-plans.json',text=>text.replace('"tenantRequired":true','"tenantRequired":false'))
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/not a public tenant action/)
})

test('tenant-required recovery operations remain valid tenant web actions without becoming module owners', (t) => {
  const f=fixture(t)
  f.mutate('contracts/commercial/generated/catalog.json', text => text.replace('"classification":"tenant_business"','"classification":"recovery"'))
  f.mutate('contracts/commercial/onboarding.v1.json', text => text.replace('"ui_routes":["/example"]','"ui_routes":[]'))
  assert.deepEqual(checkCommercialOnboardingUI(f.root).findings,[])
})

test('a public platform web action is valid only on a platform surface', (t) => {
  const f=fixture(t)
  f.mutate('contracts/generated/operation-plans.json', text => text.replace('"tenantRequired":true','"tenantRequired":false'))
  f.mutate('contracts/commercial/generated/catalog.json', text => text.replace('"classification":"tenant_business"','"classification":"platform_management"'))
  f.mutate('contracts/commercial/onboarding.v1.json', text => text.replace('"ui_routes":["/example"]','"ui_routes":[]'))
  f.write('web/src/router/index.ts', "import {createRouter} from 'vue-router'; export const router=createRouter({routes:[{path:'/platform/example',component:()=>import('../Page.vue'),meta:{surface:'platform',authorizationActions:['example.list']}}]})")
  f.write('web/src/router/navigation.ts', "export const navigation=[{id:'platform-commercial',authorizationActions:['example.list']},{path:'/platform/example',authorizationActions:['example.list']}]")
  assert.deepEqual(checkCommercialOnboardingUI(f.root).findings,[])
})

test('a platform action is rejected from a tenant surface', (t) => {
  const f=fixture(t)
  f.mutate('contracts/generated/operation-plans.json', text => text.replace('"tenantRequired":true','"tenantRequired":false'))
  f.mutate('contracts/commercial/generated/catalog.json', text => text.replace('"classification":"tenant_business"','"classification":"platform_management"'))
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/not a public tenant action/)
})

test('a tenant business action is rejected from a platform surface', (t) => {
  const f=fixture(t)
  f.write('web/src/router/index.ts', "import {createRouter} from 'vue-router'; export const router=createRouter({routes:[{path:'/platform/example',component:()=>import('../Page.vue'),meta:{surface:'platform',authorizationActions:['example.list']}}]})")
  f.write('web/src/router/navigation.ts', "export const navigation=[{path:'/platform/example',authorizationActions:['example.list']}]")
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/not a public platform web action/)
})

test('api-key-only or unbound platform operations cannot be exposed to platform UI', (t) => {
  const f=fixture(t)
  f.mutate('contracts/generated/operation-plans.json', text => text
    .replace('"tenantRequired":true','"tenantRequired":false')
    .replace('"authentication":["web-session"]','"authentication":["api-key"]'))
  f.mutate('contracts/commercial/generated/catalog.json', text => text.replace('"classification":"tenant_business"','"classification":"platform_management"'))
  f.mutate('contracts/commercial/onboarding.v1.json', text => text.replace('"ui_routes":["/example"]','"ui_routes":[]'))
  f.write('web/src/router/index.ts', "import {createRouter} from 'vue-router'; export const router=createRouter({routes:[{path:'/platform/example',component:()=>import('../Page.vue'),meta:{surface:'platform',authorizationActions:['example.list']}}]})")
  f.write('web/src/router/navigation.ts', "export const navigation=[{path:'/platform/example',authorizationActions:['example.list']}]")
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/not a public platform web action/)
})

test('a commented authorization declaration cannot satisfy an indexed route', (t) => {
  const f=fixture(t)
  f.mutate('web/src/router/index.ts',text=>text.replace("authorizationActions:['example.list']","/* authorizationActions:['example.list'] */"))
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/lacks its commercial authorization action/)
})

test('dynamic access metadata fails closed without importing product code', (t) => {
  const f=fixture(t)
  f.mutate('web/src/router/index.ts',text=>text.replace("['example.list']",'makeActions()'))
  assert.throws(()=>checkCommercialOnboardingUI(f.root),/Unsupported static expression/)
})

test('duplicate index keys are rejected rather than last-wins', (t) => {
  const f=fixture(t)
  f.mutate('contracts/commercial/onboarding.v1.json',text=>text.replace('"schema_version":1','"schema_version":1,"schema_version":1'))
  assert.throws(()=>checkCommercialOnboardingUI(f.root),/duplicate JSON property/)
})

test('plan names/codes and tenant types cannot control access projections', () => {
  for (const code of [
    "if (tenant.planName === '专业版') showMarketing()",
    "const allowed = user['tenant_type'] === 'dealer'",
    "const {plan_code: code} = subscription; allow(code)",
    "const allowed = planVersion > 2",
  ]) assert.ok(accessSelectorFindings(code).length > 0,code)
  assert.deepEqual(accessSelectorFindings("// if (tenant.planName === '专业版')\nconst description = 'tenantType'"),[])
})

test('canonical authorization service is included in the selector check', (t) => {
  const f=fixture(t)
  f.write('web/src/services/runtime/authorization.ts',"export const allowed = tenant => tenant.planCode === 'pro'")
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/plan\/tenant-type selector planCode/)
})

test('unknown authorization mode and empty action arrays are blocked', (t) => {
  const f=fixture(t)
  f.mutate('web/src/router/navigation.ts',text=>text.replace("authorizationActions:['example.list']","authorizationActions:['example.list'],authorizationMode:'sometimes'"))
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/unknown authorizationMode/)
  f.mutate('web/src/router/index.ts',text=>text.replace("['example.list']",'[]'))
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/nonempty unique/)
})

test('a layout and its default child are one concrete consumer, not duplicate routes', (t) => {
  const f=fixture(t)
  f.write('web/src/router/index.ts', "import {createRouter} from 'vue-router'; export const router=createRouter({routes:[{path:'/example',component:()=>import('../Page.vue'),children:[{path:'',component:()=>import('../Page.vue'),meta:{surface:'tenant',authorizationActions:['example.list']}}]}]})")
  assert.deepEqual(checkCommercialOnboardingUI(f.root).findings,[])
})

test('two concrete leaf routes with the same path remain blocked', (t) => {
  const f=fixture(t)
  f.write('web/src/router/index.ts', "import {createRouter} from 'vue-router'; export const router=createRouter({routes:[{path:'/example',component:()=>import('../Page.vue'),meta:{authorizationActions:['example.list']}},{path:'/example',component:()=>import('../Page.vue'),meta:{authorizationActions:['example.list']}}]})")
  assert.match(checkCommercialOnboardingUI(f.root).findings.join('\n'),/Duplicate concrete route/)
})
