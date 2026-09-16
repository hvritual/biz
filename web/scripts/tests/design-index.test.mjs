import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync, unlinkSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import test from 'node:test'
import assert from 'node:assert/strict'
import { buildDesignIndex, writeDesignIndex, readFreshIndex, verifyIndex, searchDesignIndex, indexPath } from '../lib/design-index.mjs'
import { componentApi } from '../lib/component-api.mjs'
import { readVue } from '../lib/vue-source.mjs'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'design-index-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const put = (path, value) => { mkdirSync(dirname(join(root, path)), { recursive: true }); writeFileSync(join(root, path), value) }
  const contract = JSON.parse(readFileSync(join(webRoot, 'ui-contracts.json'), 'utf8'))
  contract.routes = [{path:'/platform/tenants',component:'@/features/platform/pages/Tenants.vue',surface:'platform',template:'ListPage',required_regions:['page-heading']}]
  delete contract.patterns
  put('ui-contracts.json', JSON.stringify(contract))
  put('package.json', '{"name":"fixture"}')
  put('package-lock.json', '{"lockfileVersion":3,"packages":{}}')
  put('src/features/component-scopes.json', JSON.stringify({'platform/components/Badge.vue':{scenario:'租户状态',scope:'平台租户列表',aliases:['tenant status'],useWhen:['展示租户状态'],avoidWhen:['设备连接状态'],examples:['src/features/platform/pages/Tenants.vue']}}))
  put('src/ui/base/Button.vue', `<script setup lang="ts">withDefaults(defineProps<{label:string;size?:'sm'|'lg'}>(),{size:'sm'}); defineEmits<{submit:[id:string]}>();</script><template><button><slot/></button></template>`)
  put('src/features/platform/components/Badge.vue', `<script setup lang="ts">defineProps<{status:string}>()</script><template><span>{{status}}</span></template>`)
  put('src/features/platform/pages/Tenants.vue', `<script setup lang="ts">import Badge from '../components/Badge.vue'</script><template><main data-ui-template="ListPage"><h1 data-ui-region="page-heading">租户列表</h1><Badge status="active"/></main></template>`)
  return { root, put }
}

test('same source generates identical index and actual APIs, slots, consumers and scoped search', (t) => {
  const {root} = fixture(t)
  const a = buildDesignIndex(root), b = buildDesignIndex(root)
  assert.deepEqual(a,b)
  const button = a.entries.find((entry)=>entry.name==='Button')
  assert.equal(button.api.props.find((prop)=>prop.name==='size').defaultExpression,"'sm'")
  assert.equal(button.api.props.find((prop)=>prop.name==='label').required,true)
  assert.equal(button.api.emits[0].name,'submit')
  assert.equal(button.api.slots[0].name,'default')
  const badge = a.entries.find((entry)=>entry.name==='Badge')
  assert.deepEqual(badge.consumers,['src/features/platform/pages/Tenants.vue'])
  assert.equal(searchDesignIndex(a,{query:'租户状态',kind:'component'})[0].name,'Badge')
  assert.equal(searchDesignIndex(a,{query:'tenant status',domain:'platform'})[0].name,'Badge')
  assert.deepEqual(searchDesignIndex(a,{query:'completely-unavailable-widget-xyz'}),[])
})

test('freshness rejects stale source and generated API updates without a second handwritten API', (t) => {
  const {root,put}=fixture(t)
  writeDesignIndex(root)
  assert.ok(readFreshIndex(root).entries.length)
  put('src/features/platform/components/Badge.vue',`<script setup lang="ts">defineProps<{status:string;lastSeen?:string}>()</script><template><span>{{status}}</span></template>`)
  assert.throws(()=>readFreshIndex(root),/stale/)
  assert.throws(()=>verifyIndex(root),/stale/)
  const regenerated=writeDesignIndex(root)
  assert.ok(regenerated.entries.find((entry)=>entry.name==='Badge').api.props.some((prop)=>prop.name==='lastSeen'))
  assert.deepEqual(verifyIndex(root),regenerated.summary)
})

test('missing cache is rebuilt, tampered data is rejected by deterministic verification', (t) => {
  const {root}=fixture(t)
  assert.throws(()=>readFreshIndex(root),/missing/)
  assert.ok(verifyIndex(root).components>0)
  const index=readFreshIndex(root)
  index.entries[0].name='Forged'
  writeFileSync(indexPath(root),JSON.stringify(index))
  assert.throws(()=>verifyIndex(root),/differs/)
})

test('stale scope, missing example and manual API overrides fail', (t) => {
  const {root,put}=fixture(t)
  const scopePath='src/features/component-scopes.json'
  const original=readFileSync(join(root,scopePath),'utf8')
  let scope=JSON.parse(original)
  scope['platform/components/Badge.vue'].props={fake:'string'}
  put(scopePath,JSON.stringify(scope))
  assert.throws(()=>buildDesignIndex(root),/metadata cannot define props/)
  scope=JSON.parse(original);scope['platform/components/Badge.vue'].examples=['src/Missing.vue'];put(scopePath,JSON.stringify(scope))
  assert.throws(()=>buildDesignIndex(root),/missing\/unsafe example/)
  put(scopePath,original);unlinkSync(join(root,'src/features/platform/components/Badge.vue'))
  assert.throws(()=>buildDesignIndex(root),/Stale component scope/)
})

test('imported type shape remains unknown while local interface/model/typed slots remain declared', (t) => {
  const {root,put}=fixture(t)
  put('src/Test.vue',`<script setup lang="ts">import type {External} from './types'; interface Local {label:string} defineProps<External>(); defineSlots<{footer:(props:{busy:boolean})=>unknown}>(); defineModel<number>('page');</script><template><div><slot name="footer"/></div></template>`)
  const api=componentApi(readVue(join(root,'src/Test.vue')).descriptor,join(root,'src/Test.vue'))
  assert.equal(api.unknown[0].kind,'props')
  assert.equal(api.props[0].name,'page')
  assert.equal(api.emits[0].name,'update:page')
  assert.equal(api.slots[0].name,'footer')
})

test('query input is bounded and filters are exact', (t) => {
  const {root}=fixture(t), index=buildDesignIndex(root)
  assert.throws(()=>searchDesignIndex(index,{query:'x'.repeat(257)}),/exceeds/)
  assert.throws(()=>searchDesignIndex(index,{limit:0}),/Limit/)
  assert.deepEqual(searchDesignIndex(index,{domain:'unknown'}),[])
  assert.equal(searchDesignIndex(index,{kind:'pattern',limit:10}).length,4)
})

test('actual repository generates a non-empty deterministic index',()=>{
  const index=buildDesignIndex(webRoot)
  assert.ok(index.summary.components>10)
  assert.ok(index.summary.pages>5)
  assert.equal(index.summary.patterns,4)
  assert.ok(index.entries.some((entry)=>entry.name==='PlatformTenantsView'))
  assert.ok(index.entries.some((entry)=>entry.name==='MemberDetailDrawer'))
  assert.deepEqual(index,buildDesignIndex(webRoot))
})
