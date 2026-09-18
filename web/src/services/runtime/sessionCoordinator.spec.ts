import { describe, expect, it } from 'vitest'
import {
  acceptSessionContextSignalForTest,
  parseSessionContextSignalForTest,
  resetSessionContextSignalsForTest,
} from './sessionCoordinator'

describe('enterprise 171 session context signal', () => {
  it('deduplicates the same wake-up signal across BroadcastChannel and storage fallback', () => {
    resetSessionContextSignalsForTest()
    const signal = {
      type: 'session-context-changed',
      contextVersion: 9,
      nonce: 'same-signal-through-two-transports',
    }
    expect(acceptSessionContextSignalForTest(signal)).toBe(true)
    expect(acceptSessionContextSignalForTest(signal)).toBe(false)
    expect(acceptSessionContextSignalForTest({ ...signal, nonce: 'next-signal' })).toBe(true)
  })

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
