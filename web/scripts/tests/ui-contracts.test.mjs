import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { tmpdir } from 'node:os'
import test from 'node:test'
import assert from 'node:assert/strict'
import { validateUiContract } from '../lib/ui-contract-schema.mjs'
import { checkUiModel } from '../lib/check-ui-model.mjs'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const contract = () => JSON.parse(readFileSync(join(webRoot, 'ui-contracts.json'), 'utf8'))
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'ui-contract-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  cpSync(join(webRoot, 'src'), join(root, 'src'), { recursive: true })
  cpSync(join(webRoot, 'ui-contracts.json'), join(root, 'ui-contracts.json'))
  return root
}
function edit(root, path, transform) {
  const file = join(root, path)
  writeFileSync(file, transform(readFileSync(file, 'utf8')))
}

test('current contract and resolved current source are valid and non-empty', () => {
  const result = checkUiModel(webRoot)
  assert.deepEqual(result.failures, [])
  assert.ok(result.routes.length >= result.contract.routes.length)
  assert.ok(result.contract.routes.length >= 21)
})

for (const [name, mutate, error] of [
  ['unknown root field', (c) => { c.wrong = true }, /unknown field/],
  ['unknown page field', (c) => { c.routes[0].wrong = true }, /unknown field/],
  ['unsupported version', (c) => { c.schema_version = 99 }, /unsupported version/],
  ['duplicate route', (c) => { c.routes.push(c.routes[0]) }, /Duplicate page contract/],
  ['unknown template', (c) => { c.routes[0].template = 'Anything' }, /Unknown page template/],
  ['unknown surface', (c) => { c.routes[0].surface = 'admin' }, /Unknown surface/],
  ['missing regions array', (c) => { delete c.routes[0].required_regions }, /expected array/],
  ['duplicate region', (c) => { c.routes[0].required_regions = ['data','data'] }, /duplicate value/],
  ['disabled protection', (c) => { c.rules.platform_surface_forbids_runtime_console = false }, /cannot be disabled/],
  ['missing viewport', (c) => { c.visual_viewports.pop() }, /four CoffeeLink/],
  ['unbounded exception', (c) => { c.routes[0].exceptions = [{rule:'all'}] }, /expected non-empty string/],
]) {
  test(`schema rejects ${name}`, () => {
    const value = contract()
    mutate(value)
    assert.throws(() => validateUiContract(value), error)
  })
}

test('product surfaces reject implementation terminology', (t) => {
  const root = fixture(t)
  edit(root, 'src/features/app-shell/components/AppHeader.vue', (source) =>
    source.replace('</header>', '<span>实时数据</span></header>'),
  )
  assert.ok(checkUiModel(root).failures.some((error) => error.includes('product UI exposes engineering language')))
})

test('EnterpriseSourceBanner cannot be reintroduced', (t) => {
  const root = fixture(t)
  writeFileSync(
    join(root, 'src/features/enterprise/components/EnterpriseSourceBanner.vue'),
    '<template><div>business data source</div></template>',
  )
  assert.ok(checkUiModel(root).failures.some((error) => error.includes('EnterpriseSourceBanner must not exist')))
})

test('removing a page declaration cannot hide an existing business route', (t) => {
  const root = fixture(t)
  edit(root,'ui-contracts.json',(source)=>{
    const c=JSON.parse(source)
    c.routes=c.routes.filter((page)=>page.path!=='/platform/tenants')
    for (const pattern of Object.values(c.patterns ?? {})) pattern.examples = pattern.examples.filter((path)=>path!=='/platform/tenants')
    return JSON.stringify(c)
  })
  assert.ok(checkUiModel(root).failures.some((error)=>error.includes('/platform/tenants') && error.includes('missing')))
})

test('changing a route surface and canonical component is detected', (t) => {
  const root = fixture(t)
  edit(root,'src/router/index.ts',(source)=>source.replace("@/features/platform/pages/PlatformTenantsView.vue", "@/features/runtime/pages/RuntimeConsoleView.vue"))
  const failures = checkUiModel(root).failures
  assert.ok(failures.some((error)=>error.includes('canonical component')))
  assert.ok(failures.some((error)=>error.includes('non-runtime route')))
})

test('navigation labels can change without changing business identity or group checks', (t) => {
  const root = fixture(t)
  edit(root,'src/router/navigation.ts',(source)=>source.replace("label: '客户经营'", "label: 'Customer operations'"))
  edit(root,'src/router/customerNavigation.ts',(source)=>source.replaceAll("label: '饮品配置'", "label: 'Drink configuration'"))
  assert.deepEqual(checkUiModel(root).failures, [])
})

test('a wrong navigation ID or rental target is rejected independently of display labels', (t) => {
  const root = fixture(t)
  edit(root,'src/router/navigation.ts',(source)=>source.replace("id: 'customer-operations'", "id: 'unexpected-domain'"))
  edit(root,'src/router/customerNavigation.ts',(source)=>source.replaceAll("path: '/rental/delivery'", "path: '/customers/work'"))
  const failures=checkUiModel(root).failures
  assert.ok(failures.some((error)=>error.includes('Primary navigation')))
  assert.ok(failures.some((error)=>error.includes('Rental navigation delivery')))
})
