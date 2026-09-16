import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { validatePatternBindings } from '../lib/pattern-contracts.mjs'
import { validateUiContract } from '../lib/ui-contract-schema.mjs'

const current = () => JSON.parse(readFileSync(new URL('../../ui-contracts.json', import.meta.url), 'utf8'))
test('all current pages declare actual mandatory regions, with no empty contracts', () => {
  const contract = current()
  validateUiContract(contract)
  validatePatternBindings(contract)
  assert.equal(contract.routes.filter((page) => !page.required_regions.length).length, 0)
  assert.equal(contract.patterns.MetricsPage.status, 'reserved')
  assert.equal(contract.patterns.MetricsPage.examples.length, 0)
})
test('unpaginated role list is valid without meaningless pagination', () => {
  const contract = current()
  const roles = contract.routes.find((page) => page.path === '/enterprise/roles')
  assert.ok(!roles.required_regions.includes('pagination'))
  validatePatternBindings(contract)
})
test('removing a required query from both page and markup cannot disable its pattern obligation', () => {
  const contract = current()
  contract.routes.find((page) => page.path === '/enterprise/members').required_regions = ['page-heading', 'data']
  assert.throws(() => validatePatternBindings(contract), /missing mandatory pattern region query/)
})
test('a new form inherits scope and workspace obligations', () => {
  const contract = current()
  contract.routes.push({path:'/enterprise/example',component:'@/features/enterprise/pages/Example.vue',surface:'tenant',template:'FormPage',required_regions:['page-heading','form-workspace']})
  assert.throws(() => validatePatternBindings(contract), /missing mandatory pattern region scope/)
  contract.routes.at(-1).required_regions.push('scope')
  validatePatternBindings(contract)
})
test('a precise reviewed page exception does not waive other pages or regions', () => {
  const contract = current()
  const page = contract.routes.find((page) => page.path === '/enterprise/roles')
  page.required_regions = ['page-heading','data']
  page.exceptions = [{rule:'pattern.ListPage.query',reason:'Isolated read-only report fixture has no selectable dimensions.',evidence:'Fixture only; not a production exemption.',review_condition:'Remove when filters become applicable.'}]
  validateUiContract(contract)
  validatePatternBindings(contract)
  page.exceptions[0].rule = 'pattern.ListPage.*'
  assert.throws(() => validatePatternBindings(contract), /missing mandatory pattern region query/)
})
test('reserved patterns and misleading example references are rejected', () => {
  const contract = current()
  contract.routes[0].template = 'MetricsPage'
  assert.throws(() => validatePatternBindings(contract), /example must use this pattern|reserved/)
})
test('duplicate regions and contradictory required/optional regions fail', () => {
  const contract = current()
  contract.routes[0].required_regions.push('page-heading')
  assert.throws(() => validateUiContract(contract), /duplicate value/)
  const other = current()
  other.patterns.ListPage.optional_regions.push('query')
  assert.throws(() => validatePatternBindings(other), /both required and optional/)
})
