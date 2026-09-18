import { describe, expect, it } from 'vitest'
import { parseSessionContextSignalForTest } from './sessionCoordinator'

describe('enterprise 171 session context signal', () => {
  it('accepts only non-authoritative wake-up signals with a monotonic-compatible version value', () => {
    expect(parseSessionContextSignalForTest({
      type: 'session-context-changed',
      contextVersion: 7,
      nonce: 'n-1',
    })).toEqual({
      type: 'session-context-changed',
      contextVersion: 7,
      nonce: 'n-1',
    })
    expect(parseSessionContextSignalForTest({
      type: 'session-context-changed',
      contextVersion: -1,
      nonce: 'n-2',
    })).toBeNull()
    expect(parseSessionContextSignalForTest({
      type: 'tenant-authority',
      contextVersion: 8,
      nonce: 'n-3',
      tenantId: 'must-not-be-authority',
    })).toBeNull()
  })
})
