import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, dirname } from 'node:path'
import test from 'node:test'
import assert from 'node:assert/strict'
import { StaticSource } from '../lib/static-source.mjs'
import { readVue, templateMarkers, verifyPageSource } from '../lib/vue-source.mjs'

function fixture(t, files) {
  const root = mkdtempSync(join(tmpdir(), 'ui-template-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  for (const [path, content] of Object.entries(files)) {
    mkdirSync(dirname(join(root, path)), { recursive: true })
    writeFileSync(join(root, path), content)
  }
  return { root, reader: new StaticSource(root) }
}
const page = {path:'/platform/tenants',component:'@/Page.vue',template:'ListPage',surface:'platform',required_regions:['query','data']}

test('real nodes, named slots and component regions are accepted with either quote style', (t) => {
  const {root,reader} = fixture(t, {
    'src/Page.vue': `<script setup>import Grid from './Grid.vue'</script><template><main data-ui-template='ListPage'><form data-ui-region='query'><label>Filter</label></form><Grid/></main></template>`,
    'src/Grid.vue': `<template><section data-ui-region="data"><slot>Rows</slot></section></template>`,
  })
  assert.deepEqual(verifyPageSource(reader, page, join(root,'src/Page.vue')), [])
})

test('comments, empty native nodes and statically hidden regions cannot satisfy the contract', (t) => {
  const {root,reader} = fixture(t, {'src/Page.vue': `<template><main data-ui-template="ListPage"><!-- data-ui-region="query" --><div data-ui-region="query"/><div hidden><p data-ui-region="data">hidden</p></div><template v-if="false"><p data-ui-region="data">never</p></template></main></template>`})
  const errors = verifyPageSource(reader,page,join(root,'src/Page.vue'))
  assert.equal(errors.length,2)
  assert.ok(errors.some((error)=>error.includes('query')))
  assert.ok(errors.some((error)=>error.includes('data')))
})

test('unused imports do not satisfy missing regions', (t) => {
  const {root,reader} = fixture(t, {
    'src/Page.vue': `<script setup>import Unused from './Unused.vue'</script><template><main data-ui-template="ListPage">No regions</main></template>`,
    'src/Unused.vue': `<template><section data-ui-region="query">Q</section><section data-ui-region="data">D</section></template>`,
  })
  assert.equal(verifyPageSource(reader,page,join(root,'src/Page.vue')).length,2)
})

test('runtime console behind an alias and a component barrel is blocked on business surfaces', (t) => {
  const {root,reader} = fixture(t, {
    'src/Page.vue': `<script setup>import {Console as Widget} from './components'</script><template><main data-ui-template="ListPage"><Widget/></main></template>`,
    'src/components.ts': `export {default as Console} from './RuntimeConsoleView.vue'`,
    'src/RuntimeConsoleView.vue': `<template><div data-ui-region="query">Q</div><div data-ui-region="data">D</div></template>`,
  })
  assert.ok(verifyPageSource(reader,page,join(root,'src/Page.vue')).some((error)=>error.includes('RuntimeConsole')))
  assert.deepEqual(verifyPageSource(reader,{...page,surface:'runtime'},join(root,'src/Page.vue')),[])
})

test('runtime console behind a dynamic component binding is blocked', (t) => {
  const {root,reader} = fixture(t, {
    'src/Page.vue': `<script setup>import Console from './RuntimeConsoleView.vue'</script><template><main data-ui-template="ListPage"><component :is="Console"/></main></template>`,
    'src/RuntimeConsoleView.vue': `<template><div data-ui-region="query">Q</div><div data-ui-region="data">D</div></template>`,
  })
  assert.ok(verifyPageSource(reader,page,join(root,'src/Page.vue')).some((error)=>error.includes('RuntimeConsole')))
})

test('a marker in a script or style is not a template marker', (t) => {
  const {root} = fixture(t, {'src/Page.vue': `<script setup>const fake='data-ui-template="ListPage"'</script><template><p>Real</p></template>`})
  const markers = templateMarkers(readVue(join(root,'src/Page.vue')).descriptor)
  assert.equal(markers.templates.size,0)
})
