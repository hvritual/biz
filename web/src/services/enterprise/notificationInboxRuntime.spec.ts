import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mutate, request } from '@/services/commercial/platformCommercial'
import { readSession, type TrustedSession } from '@/services/runtime/api'
import {
  markAllNotificationsRead,
  notificationInboxErrorKey,
  parseNotificationInbox,
  readNotificationInbox,
} from './notificationInboxRuntime'

vi.mock('@/services/commercial/platformCommercial', async (original) => ({
  ...await original<typeof import('@/services/commercial/platformCommercial')>(),
  mutate: vi.fn(),
  request: vi.fn(),
}))
vi.mock('@/services/runtime/api', async (original) => ({
  ...await original<typeof import('@/services/runtime/api')>(),
  readSession: vi.fn(),
}))

const session: TrustedSession = {
  authenticated: true,
  actor_kind: 'user',
  user_id: 'user-a',
  active_tenant_id: 'tenant-a',
  context_version: 7,
}
const message = (overrides: Record<string, unknown> = {}) => ({
  message_id: 'a'.repeat(64),
  type_code: 'device.fault',
  level: 'urgent',
  reference_kind: 'device',
  reference_id: 'machine-a',
  created_at: '2026-09-27T01:00:00Z',
  ...overrides,
})
const snapshot = (overrides: Record<string, unknown> = {}) => ({
  tenant_id: 'tenant-a',
  user_id: 'user-a',
  unread_count: 1,
  messages: [message()],
  as_of: '2026-09-27T01:01:00Z',
  ...overrides,
})

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(readSession).mockResolvedValue(session)
})

describe('notification inbox runtime', () => {
  it('accepts only the current tenant/user and stable unread shape', () => {
    const value = parseNotificationInbox(session, snapshot())
    expect(value.unread_count).toBe(1)
    expect(value.messages[0]?.message_id).toBe('a'.repeat(64))
    expect(Object.isFrozen(value.messages)).toBe(true)

    for (const invalid of [
      { ...snapshot(), tenant_id: 'tenant-b' },
      { ...snapshot(), user_id: 'other' },
      { ...snapshot(), unread_count: -1 },
      { ...snapshot(), unread_count: 0 },
      { ...snapshot(), messages: [message({ message_id: 'not-a-digest' })] },
      { ...snapshot(), messages: [message({ type_code: '<script>' })] },
      { ...snapshot(), messages: [message({ level: 'critical' })] },
      { ...snapshot(), messages: [message({ reference_kind: '../route' })] },
      { ...snapshot(), messages: [message({ reference_id: 'bad\u0000value' })] },
      { ...snapshot(), as_of: 'not-a-date' },
    ]) {
      expect(() => parseNotificationInbox(session, invalid)).toThrow('invalidResponse')
    }
  })

  it('treats arbitrary reference text as text data and never as navigation or markup', () => {
    const hostile = '<img src=x onerror="globalThis.__inbox_xss=1">'
    const value = parseNotificationInbox(session, snapshot({
      messages: [message({ reference_id: hostile })],
    }))
    expect(value.messages[0]?.reference_id).toBe(hostile)
    expect(value.messages[0]).not.toHaveProperty('href')
    expect(value.messages[0]).not.toHaveProperty('html')
  })

  it('pins the GET to trusted session context and rejects context drift after response', async () => {
    vi.mocked(request).mockResolvedValue(snapshot())
    expect((await readNotificationInbox(session)).unread_count).toBe(1)
    expect(request).toHaveBeenCalledWith('/auth/personal/in-app-notifications', {
      headers: { 'X-Biz-Session-Context': expect.stringContaining('"active_tenant_id":"tenant-a"') },
    })

    vi.mocked(readSession)
      .mockResolvedValueOnce(session)
      .mockResolvedValueOnce({ ...session, active_tenant_id: 'tenant-b', context_version: 8 })
    await expect(readNotificationInbox(session)).rejects.toThrow('sessionChanged')
  })

  it('marks all with only CSRF/session context/idempotency metadata and no client-selected owner', async () => {
    vi.mocked(mutate).mockResolvedValue({
      receipt_id: 'b'.repeat(64),
      tenant_id: 'tenant-a',
      user_id: 'user-a',
      marked_count: 3,
      read_at: '2026-09-27T01:02:00Z',
    })
    const receipt = await markAllNotificationsRead(session, 'notification-read-all-key')
    expect(receipt.marked_count).toBe(3)
    expect(mutate).toHaveBeenCalledWith(
      '/auth/personal/in-app-notifications/read-all',
      'POST',
      {},
      {
        idempotencyKey: 'notification-read-all-key',
        sessionContext: expect.stringContaining('"user_id":"user-a"'),
      },
    )
    expect(JSON.stringify(vi.mocked(mutate).mock.calls[0]?.[2])).not.toContain('tenant')
    expect(JSON.stringify(vi.mocked(mutate).mock.calls[0]?.[2])).not.toContain('user')
  })

  it('rejects a receipt for a different owner instead of reporting false success', async () => {
    vi.mocked(mutate).mockResolvedValue({
      receipt_id: 'b'.repeat(64),
      tenant_id: 'tenant-b',
      user_id: 'user-a',
      marked_count: 1,
      read_at: '2026-09-27T01:02:00Z',
    })
    await expect(markAllNotificationsRead(session, 'notification-read-all-key')).rejects.toThrow('invalidResponse')
  })

  it('sanitizes unknown errors to stable UI keys', () => {
    expect(notificationInboxErrorKey(new Error('secret@example.invalid'))).toBe('unavailable')
  })
})
