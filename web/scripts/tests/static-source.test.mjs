import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, dirname } from 'node:path'
import test from 'node:test'
import assert from 'node:assert/strict'
import { StaticSource, componentFile, readStrictJson } from '../lib/static-source.mjs'

function fixture(t, files) {
  const root = mkdtempSync(join(tmpdir(), 'ui-source-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  for (const [path, content] of Object.entries(files)) {
    const file = join(root, path)
    mkdirSync(dirname(file), { recursive: true })
    writeFileSync(file, content)
  }
  return { root, reader: new StaticSource(root) }
}
const component = '<template><main>Real content</main></template>'

test('reads aliases, satisfies, split arrays, nested routes and literal loaders without evaluating imports', (t) => {
  const { root, reader } = fixture(t, {
    'src/router/index.ts': `import {createRouter as configure, createWebHistory} from 'vue-router'; import {entries as renamed} from './other';
      export const router=configure({history:createWebHistory(),routes:[...renamed,{path:'/old',redirect:'/platform/tenants'}],scrollBehavior:()=>{throw Error('never execute')}})`,
    'src/router/other.ts': `import Tenant from '../Tenant.vue'; export const entries=[{path:'/platform',meta:{surface:'platform'},children:[{path:'tenants',component:Tenant,meta:{pageTemplate:'ListPage'}},{path:'other',component:()=>import('../Tenant.vue')}]}] satisfies unknown[];`,
    'src/Tenant.vue': component,
  })
  const routes = reader.router(join(root, 'src/router/index.ts'))
  assert.equal(routes.length, 4)
  assert.equal(routes[1].path, '/platform/tenants')
  assert.equal(routes[1].meta.surface, 'platform')
  assert.equal(componentFile(routes[1].component), join(root, 'src/Tenant.vue'))
  assert.equal(componentFile(routes[2].component), join(root, 'src/Tenant.vue'))
  assert.equal(routes[3].redirect, '/platform/tenants')
})

test('ignores a commented route rather than accepting it as executable data', (t) => {
  const { root, reader } = fixture(t, { 'src/router.ts': `import {createRouter} from 'vue-router'; export const router=createRouter({routes: [/* {path:'/platform/tenants'} */]})` })
  assert.deepEqual(reader.router(join(root, 'src/router.ts')), [])
})

test('rejects dynamic route construction without running user code', (t) => {
  const { root, reader } = fixture(t, { 'src/router.ts': `import {createRouter} from 'vue-router'; const execute=()=>{throw Error('bad')}; export const router=createRouter({routes:execute()})` })
  assert.throws(() => reader.router(join(root, 'src/router.ts')), /Unsupported static expression: CallExpression/)
})

test('rejects cycles instead of recursing indefinitely', (t) => {
  const { root, reader } = fixture(t, { 'a.ts': `import {b} from './b'; export const a=[...b]`, 'b.ts': `import {a} from './a'; export const b=[...a]` })
  assert.throws(() => reader.exported(join(root, 'a.ts'), 'a'), /Cyclic/)
})

test('resolves explicit re-exports and rejects missing exported names', (t) => {
  const { root, reader } = fixture(t, { 'a.ts': `export {inner as publicName} from './b'`, 'b.ts': `export const inner=['one']` })
  assert.deepEqual(reader.exported(join(root, 'a.ts'), 'publicName'), ['one'])
  assert.throws(() => reader.exported(join(root, 'a.ts'), 'absent'), /Missing static export/)
})

test('rejects duplicate literal properties and missing component files', (t) => {
  const { root, reader } = fixture(t, {
    'duplicate.ts': `export const routes=[{path:'/a',path:'/b'}]`,
    'missing.ts': `export const routes=[{path:'/a',component:()=>import('./Absent.vue')}]`,
  })
  assert.throws(() => reader.exported(join(root, 'duplicate.ts'), 'routes'), /Duplicate property/)
  assert.throws(() => reader.exported(join(root, 'missing.ts'), 'routes'), /Missing source/)
})

test('does not confuse an object forged by source code with a resolved component loader', (t) => {
  const { root, reader } = fixture(t, { 'a.ts': `export const fake={componentFile:'/arbitrary.vue'}` })
  assert.equal(componentFile(reader.exported(join(root, 'a.ts'), 'fake')), undefined)
})

test('strict JSON rejects duplicate nested keys and does not confuse equal values with duplicate keys', (t) => {
  const { root } = fixture(t, { 'bad.json': '{"routes":[{"path":"/a","path":"/b"}]}', 'good.json': '{"a":"same","b":"same"}' })
  assert.throws(() => readStrictJson(join(root, 'bad.json')), /duplicate JSON property/)
  assert.deepEqual(readStrictJson(join(root, 'good.json')), {a:'same',b:'same'})
})
