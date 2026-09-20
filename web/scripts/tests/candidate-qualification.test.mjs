import test from 'node:test'
import assert from 'node:assert/strict'
import {
  extractFailureSignature,
  groupFailureSignatures,
  latestRequiredRuns,
  qualificationSnapshot,
} from '../../../.github/scripts/candidate-qualification.mjs'

test('candidate qualification selects the latest run for each required workflow', () => {
  const selected = latestRequiredRuns([
    { id: 1, name: 'A', status: 'completed', conclusion: 'failure' },
    { id: 2, name: 'A', status: 'completed', conclusion: 'success' },
    { id: 3, name: 'B', status: 'in_progress', conclusion: null },
  ], ['A', 'B'])
  assert.equal(selected.get('A').id, 2)
  assert.equal(selected.get('B').id, 3)
})

test('candidate snapshot distinguishes missing, active, failed and successful workflows', () => {
  const selected = new Map([
    ['A', { status: 'completed', conclusion: 'success' }],
    ['B', { status: 'completed', conclusion: 'failure' }],
    ['C', { status: 'in_progress', conclusion: null }],
  ])
  assert.deepEqual(qualificationSnapshot(['A', 'B', 'C', 'D'], selected), {
    missing: ['D'],
    active: ['C'],
    failed: ['B'],
    success: ['A'],
  })
})

test('same failing test line is grouped across workflows', () => {
  const log = "Error: expect(locator).toContainText(expected) failed\n    at /home/runner/work/biz/biz/web/e2e/enterprise-members-real.spec.ts:590:42"
  const signature = extractFailureSignature(log, 'fallback')
  const groups = groupFailureSignatures([
    { signature, workflow: 'CoffeeLink Vue console', job: 'web' },
    { signature, workflow: 'Enterprise 177 member lifecycle qualification', job: 'web' },
  ])
  assert.equal(groups.length, 1)
  assert.deepEqual(groups[0].workflows, [
    'CoffeeLink Vue console',
    'Enterprise 177 member lifecycle qualification',
  ])
})

test('failure signature does not depend on absolute runner path', () => {
  const a = extractFailureSignature('Error: expect failed\n at /home/runner/work/biz/biz/web/e2e/a.spec.ts:42:7')
  const b = extractFailureSignature('Error: expect failed\n at web/e2e/a.spec.ts:42:7')
  assert.equal(a, b)
})
