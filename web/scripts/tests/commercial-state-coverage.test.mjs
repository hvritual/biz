import assert from 'node:assert/strict'
import { test } from 'node:test'
import { resolve } from 'node:path'
import { readFileSync, mkdtempSync, mkdirSync, copyFileSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname } from 'node:path'
import { StaticSource } from '../lib/static-source.mjs'
import { checkCommercialStateCoverage, coverageFailures, goConstants, protoEnum, serverStateGroups, literalReturns } from '../lib/commercial-state-coverage.mjs'

const root = resolve(import.meta.dirname, '../..')
function inputs() {
  const source = new StaticSource(root)
  return {
    groups: serverStateGroups(resolve(root, '..')),
    catalog: source.exported(resolve(root, 'src/i18n/backend-terms.ts'), 'backendTermCatalog'),
    messages: source.exported(resolve(root, 'src/i18n/backend-term-messages.ts'), 'backendTermMessages'),
  }
}

test('all canonical commercial wire values have Chinese and English projections', () => {
  const result = checkCommercialStateCoverage(root)
  assert.deepEqual(result.failures, [])
  assert.equal(Object.keys(result.groups).length, 13)
  assert.ok(result.groups.subscriptionState.includes('GRACE'))
  assert.ok(result.groups.changeClassification.includes('SAME_TIER'))
  assert.ok(result.groups.changeClassification.includes('RENEW'))
})

test('an added backend subscription state fails until its real mapping and both translations exist', () => {
  const { groups, catalog, messages } = inputs()
  const path = resolve(root, '../internal/commercial/domain/subscription/model.go')
  const source = readFileSync(path, 'utf8').replace('StateEnded', 'StatePaused = "PAUSED"\n StateEnded')
  groups.subscriptionState = goConstants(source).filter((entry) => entry.name.startsWith('State')).map((entry) => entry.value)
  assert.ok(coverageFailures(groups, catalog, messages).some((entry) => entry.includes('PAUSED')))
  catalog.subscriptionState.PAUSED = 'paused'
  messages['zh-CN'].subscriptionState.paused = '已暂停'
  assert.ok(coverageFailures(groups, catalog, messages).some((entry) => entry.includes('en-US:subscriptionState.PAUSED')))
  messages['en-US'].subscriptionState.paused = 'Paused'
  assert.deepEqual(coverageFailures(groups, catalog, messages), [])
})

test('removing a real wire mapping is not rescued by a friendly fallback', () => {
  const { groups, catalog, messages } = inputs()
  delete catalog.subscriptionState.GRACE
  assert.ok(coverageFailures(groups, catalog, messages).some((entry) => entry.includes('subscriptionState.GRACE')))
})

test('an empty translation and a removed group fail closed', () => {
  const { groups, catalog, messages } = inputs()
  messages['en-US'].sourceState.revoked = ''
  delete catalog.planState
  const failures = coverageFailures(groups, catalog, messages)
  assert.ok(failures.some((entry) => entry.includes('sourceState.revoked')))
  assert.ok(failures.some((entry) => entry.includes('planState: missing term group')))
})

test('Proto zero/unspecified and newly added enum values also require translations', () => {
  const source = 'enum Example { EXAMPLE_UNSPECIFIED = 0; EXAMPLE_NEW = 1; }'
  const values = protoEnum(source, 'Example')
  assert.deepEqual(values, ['EXAMPLE_UNSPECIFIED', 'EXAMPLE_NEW'])
  assert.equal(coverageFailures({ technicalStatus: values }, { technicalStatus: {} }, {}).length, 2)
})

test('comments cannot manufacture server-state coverage', () => {
  const source = `// const (\n// StateFake = "FAKE"\n// )\nconst (\nStateActive = "ACTIVE" // real\n/* StateBogus = "BOGUS" */\n)\n`
  assert.deepEqual(goConstants(source), [{ name: 'StateActive', type: '', value: 'ACTIVE' }])
  assert.deepEqual(protoEnum('/* enum Example { BAD = 1; } */ enum Example { GOOD = 0; }', 'Example'), ['GOOD'])
})

test('unsupported source expressions are rejected, not silently skipped', () => {
  assert.throws(() => goConstants('const (\nStateFuture = other.Value\n)'), /Unsupported/)
  assert.throws(() => goConstants('const (\nStateFuture = "FUT" + "URE"\n)'), /Unsupported/)
  assert.throws(() => protoEnum('enum Example { option allow_alias = true; GOOD = 0; }', 'Example'), /Unsupported/)
  assert.throws(() => goConstants('/* missing'), /Unterminated/)
})


test('single declarations are covered while quoted source is ignored', () => {
  assert.deepEqual(goConstants('const StatePaused = "PAUSED"'), [{ name: 'StatePaused', type: '', value: 'PAUSED' }])
  assert.deepEqual(goConstants('var example = `\nconst StateFake = "FAKE"\n`\nconst StateActive = "ACTIVE"'), [{ name: 'StateActive', type: '', value: 'ACTIVE' }])
  assert.throws(() => goConstants('const StatePaused = other.Value'), /Unsupported/)
})

test('a newly added package file cannot hide an unmapped state', () => {
  const fixture = mkdtempSync(resolve(tmpdir(), 'commercial-state-'))
  const files = [
    ...['subscription', 'plan', 'entitlement', 'subscriptionchange'].map((name) => `internal/commercial/domain/${name}/model.go`),
    'internal/commercial/modulecatalog/model.go',
    'contracts/proto/commercial/v1/entitlement.proto', 'contracts/proto/commercial/v1/module.proto',
  ]
  try {
    for (const file of files) {
      mkdirSync(dirname(resolve(fixture, file)), { recursive: true })
      copyFileSync(resolve(root, '..', file), resolve(fixture, file))
    }
    writeFileSync(resolve(fixture, 'internal/commercial/domain/subscription/new_state.go'), 'package subscription\nconst StatePaused = "PAUSED"\n')
    const { catalog, messages } = inputs()
    assert.ok(coverageFailures(serverStateGroups(fixture), catalog, messages).some((entry) => entry.includes('subscriptionState.PAUSED')))
  } finally {
    rmSync(fixture, { recursive: true, force: true })
  }
})


test('nonliteral state returns fail closed without counting nested helper returns', () => {
  const source = 'func State() string {\nhelper := func() string { return "HELPER" }\n_ = helper\nreturn "ACTIVE"\n}'
  assert.deepEqual(literalReturns(source, 'State'), ['ACTIVE'])
  assert.throws(() => literalReturns(source.replace('return "ACTIVE"', 'return nextState'), 'State'), /Unsupported state return/)
})
