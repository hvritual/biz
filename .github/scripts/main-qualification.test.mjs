import test from 'node:test'
import assert from 'node:assert/strict'
import { selectAssociatedMergedPr, selectSuccessfulMergeGate } from './main-qualification.mjs'

test('selectAssociatedMergedPr requires exact main merge commit and main base', () => {
  const pulls = [
    { number: 1, merged_at: '2026-01-01T00:00:00Z', merge_commit_sha: 'other', base: { ref: 'main' } },
    { number: 2, merged_at: '2026-01-01T00:00:00Z', merge_commit_sha: 'main-sha', base: { ref: 'dev' } },
    { number: 3, merged_at: '2026-01-01T00:00:00Z', merge_commit_sha: 'main-sha', base: { ref: 'main' } },
  ]
  assert.equal(selectAssociatedMergedPr(pulls, 'main-sha')?.number, 3)
})

test('selectAssociatedMergedPr rejects direct push without merged PR proof', () => {
  assert.equal(selectAssociatedMergedPr([], 'main-sha'), null)
})

test('selectSuccessfulMergeGate requires exact candidate and completed success', () => {
  const runs = [
    { id: 1, name: 'PR Merge Gate', head_sha: 'candidate', event: 'pull_request', status: 'completed', conclusion: 'failure' },
    { id: 2, name: 'PR Merge Gate', head_sha: 'other', event: 'pull_request', status: 'completed', conclusion: 'success' },
    { id: 3, name: 'PR Merge Gate', head_sha: 'candidate', event: 'pull_request', status: 'completed', conclusion: 'success' },
  ]
  assert.equal(selectSuccessfulMergeGate(runs, 'candidate')?.id, 3)
})

test('selectSuccessfulMergeGate rejects missing proof', () => {
  assert.equal(selectSuccessfulMergeGate([], 'candidate'), null)
})
