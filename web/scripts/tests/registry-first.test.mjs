import assert from 'node:assert/strict'
import { cpSync, mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'
import test from 'node:test'
import { buildDesignIndex, searchDesignIndex } from '../lib/design-index.mjs'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const read = (path) => readFileSync(join(webRoot, path), 'utf8')

function fixture(t, checker) {
  const root = mkdtempSync(join(webRoot, '.registry-first-fixture-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  cpSync(join(webRoot, 'src'), join(root, 'src'), { recursive: true })
  mkdirSync(join(root, 'scripts'), { recursive: true })
  cpSync(join(webRoot, 'scripts', checker), join(root, 'scripts', checker))
  return root
}
function mutate(root, path, transform) {
  const file = join(root, path)
  writeFileSync(file, transform(readFileSync(file, 'utf8')))
}
function run(root, checker) {
  return spawnSync(process.execPath, [join(root, 'scripts', checker)], { cwd: root, encoding: 'utf8' })
}

test('Registry search finds canonical pilot pages and shared pagination', () => {
  const index = buildDesignIndex(webRoot)
  const pagination = searchDesignIndex(index, { query: 'pagination', kind: 'component', layer: 'common', limit: 20 })
  assert.ok(pagination.some((entry) => entry.id === 'ui/common/AppPagination'))
  assert.ok(searchDesignIndex(index, { query: '/platform/tenants', kind: 'page', limit: 20 }).some((entry) => entry.name === 'PlatformTenantsView'))
  assert.ok(searchDesignIndex(index, { query: 'members', kind: 'page', limit: 20 }).some((entry) => entry.name === 'MembersView'))
})

test('tenant pilot consumes selected Registry capability while members remains canonical without duplicate regions', () => {
  const tenants = read('src/features/platform/pages/PlatformTenantsView.vue')
  const members = read('src/features/enterprise/pages/MembersView.vue')
  const contract = JSON.parse(read('ui-contracts.json'))
  assert.match(tenants, /import AppPagination from ['"]@\/ui\/common\/AppPagination\.vue['"]/)
  assert.match(tenants, /<AppPagination[^>]+data-ui-region="pagination"/)
  assert.doesNotMatch(tenants, /<div class="pagination"/)
  assert.equal((members.match(/data-ui-region="page-heading"/g) ?? []).length, 1)
  assert.match(members, /<AppPagination/)
  for (const [path, component] of [
    ['/platform/tenants', '@/features/platform/pages/PlatformTenantsView.vue'],
    ['/enterprise/members', '@/features/enterprise/pages/MembersView.vue'],
  ]) {
    const page = contract.routes.find((entry) => entry.path === path)
    assert.ok(page, `${path} contract missing`)
    assert.equal(page.component, component)
    assert.equal(page.template, 'ListPage')
    assert.doesNotMatch(page.component, /RuntimeConsole/)
  }
})

test('isolated wrong common-layer dependency is rejected by real architecture gate', (t) => {
  const root = fixture(t, 'check-architecture.mjs')
  mutate(root, 'src/ui/common/AppPagination.vue', (source) => source.replace('<script setup lang="ts">', '<script setup lang="ts">\nimport BrokenLayer from \'@/features/platform/pages/PlatformTenantsView.vue\'\nvoid BrokenLayer'))
  const result = run(root, 'check-architecture.mjs')
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /common UI cannot depend on business/)
})

test('isolated raw pilot brand color is rejected by real architecture gate', (t) => {
  const root = fixture(t, 'check-architecture.mjs')
  mutate(root, 'src/features/platform/pages/PlatformTenantsView.vue', (source) => source.replace('</style>', '.registry-first-bad-color{color:#ff00aa}</style>'))
  const result = run(root, 'check-architecture.mjs')
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /scoped styles must consume design tokens/)
})

test('isolated hard-coded pilot copy is rejected by real i18n gate', (t) => {
  const root = fixture(t, 'check-i18n.mjs')
  mutate(root, 'src/features/platform/pages/PlatformTenantsView.vue', (source) => source.replace('data-testid="platform-tenant-management">', 'data-testid="platform-tenant-management"><span>硬编码租户文案</span>'))
  const result = run(root, 'check-i18n.mjs')
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /hard-coded visible Han text/)
})
