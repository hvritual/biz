import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { createHash } from 'node:crypto'
import assert from 'node:assert/strict'
import test from 'node:test'
import { checkCommercialStates } from '../lib/commercial-state-contract.mjs'

function fixture(t, mutate = () => {}) {
  const root = mkdtempSync(join(tmpdir(), 'commercial-state-'))
  t.after(() => rmSync(root, { force: true, recursive: true }))
  const backend = 'package subscription\nconst StateActive = "ACTIVE"\n'
  const data = {
    vocabulary: { subscriptionState: ['ACTIVE'] },
    sources: { 'state.go': createHash('sha256').update(backend).digest('hex') },
    terms: { subscriptionState: { ACTIVE: 'active' } },
    messages: {
      'zh-CN': { subscriptionState: { active: '有效' }, fallback: { subscriptionState: '状态未知' } },
      'en-US': { subscriptionState: { active: 'Active' }, fallback: { subscriptionState: 'Unknown' } },
    },
    backend,
  }
  mutate(data)
  const files = {
    'state.go': data.backend,
    'web/src/services/commercial/state-vocabulary.generated.ts': `export const commercialStateVocabulary=${JSON.stringify(data.vocabulary)} as const; export const commercialStateSources=${JSON.stringify(data.sources)} as const;`,
    'web/src/i18n/commercial-state-terms.ts': `export const commercialStateTerms=${JSON.stringify(data.terms)} as const;`,
    'web/src/i18n/backend-term-messages.ts': `export const backendTermMessages=${JSON.stringify(data.messages)} as const;`,
  }
  for (const [path, content] of Object.entries(files)) { const p=join(root,path); mkdirSync(dirname(p),{recursive:true}); writeFileSync(p,content) }
  return join(root,'web')
}

test('complete source-bound bilingual vocabulary passes', t => assert.deepEqual(checkCommercialStates(fixture(t)).failures, []))
test('new backend state invalidates stale generated projection', t => {
 const result=checkCommercialStates(fixture(t,d=>{d.backend+='const StateGrace = "GRACE"\n'}))
 assert.match(result.failures.join('\n'),/source changed/)
})
test('regenerated new state still needs an actual translation', t => {
 const result=checkCommercialStates(fixture(t,d=>{d.vocabulary.subscriptionState.push('GRACE')}))
 assert.match(result.failures.join('\n'),/GRACE: missing translation/)
})
test('missing English copy cannot fall back silently to Chinese', t => {
 const result=checkCommercialStates(fixture(t,d=>{delete d.messages['en-US'].subscriptionState.active}))
 assert.match(result.failures.join('\n'),/en-US.*missing user copy/)
})
test('unknown must not impersonate active state', t => {
 const result=checkCommercialStates(fixture(t,d=>{d.messages['zh-CN'].fallback.subscriptionState='有效'}))
 assert.match(result.failures.join('\n'),/impersonates/)
})
test('unsupported former state name is not a compatibility alias', t => {
 const result=checkCommercialStates(fixture(t,d=>{d.terms.subscriptionState.SUSPENDED='active'}))
 assert.match(result.failures.join('\n'),/unsupported wire alias/)
})
test('a declared kind cannot disappear from translation bindings', t => {
 const result=checkCommercialStates(fixture(t,d=>{delete d.terms.subscriptionState}))
 assert.match(result.failures.join('\n'),/missing translation binding/)
})
test('retired backend state is rejected as a stale frontend alias', t => {
 const result=checkCommercialStates(fixture(t,d=>{d.vocabulary.subscriptionState=['GRACE']}))
 assert.match(result.failures.join('\n'),/ACTIVE: unsupported wire alias/)
})
test('empty source inventory fails instead of creating vacuous green', t => {
 const result=checkCommercialStates(fixture(t,d=>{d.sources={}}))
 assert.match(result.failures.join('\n'),/source inventory is empty/)
})
