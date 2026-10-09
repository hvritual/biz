import { beforeEach, describe, expect, it, vi } from 'vitest'
import { readSession } from '@/services/runtime/api'
import { rememberInitialSubscription, recallInitialSubscription, forgetInitialSubscription } from './initialSubscriptionJournal'

vi.mock('@/services/runtime/api', () => ({ readSession: vi.fn() }))
const record = { tenantId: 'tenant-a', changeId: 'change-a', salesScope: 'office',
  input: { requestId: 'request-a', previewHash: 'b'.repeat(64), reason: 'approved original request' } }
beforeEach(() => {
  sessionStorage.clear()
  vi.mocked(readSession).mockResolvedValue({ authenticated: true, actor_kind: 'platform', platform_subject: 'admin-a' })
})
describe('tab-local first subscription request journal', () => {
  it('retains only the original request, never a success or entitlement', async () => {
    expect(await rememberInitialSubscription(record)).toBe(true)
    expect(await recallInitialSubscription('tenant-a', 'change-a')).toEqual(record)
    expect(await recallInitialSubscription('tenant-b', 'change-a')).toBeNull()
    expect(await recallInitialSubscription('tenant-a', 'other')).toBeNull()
    await forgetInitialSubscription('tenant-a', 'change-a')
    expect(await recallInitialSubscription('tenant-a', 'change-a')).toBeNull()
  })
  it('does not expose an original actor request in another account context', async () => {
    await rememberInitialSubscription(record)
    vi.mocked(readSession).mockResolvedValue({ authenticated: true, actor_kind: 'platform', platform_subject: 'admin-b' })
    expect(await recallInitialSubscription('tenant-a', 'change-a')).toBeNull()
  })
  it('does not load or write without a trusted platform identity', async () => {
    vi.mocked(readSession).mockResolvedValue({ authenticated: false })
    expect(await rememberInitialSubscription(record)).toBe(false)
    expect(await recallInitialSubscription('tenant-a', 'change-a')).toBeNull()
  })
  it('does not accept corrupt, oversized or cross-tenant local records', async () => {
    await rememberInitialSubscription(record)
    const key = sessionStorage.key(0)!
    sessionStorage.setItem(key, '{broken')
    expect(await recallInitialSubscription('tenant-a', 'change-a')).toBeNull()
    sessionStorage.setItem(key, JSON.stringify({ ...record, tenantId: 'tenant-b' }))
    expect(await recallInitialSubscription('tenant-a', 'change-a')).toBeNull()
    sessionStorage.setItem(key, ' '.repeat(12001))
    expect(await recallInitialSubscription('tenant-a', 'change-a')).toBeNull()
  })
})
