import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CommercialApiError, mutate, request } from '@/services/commercial/platformCommercial'
import { readSession, type TrustedSession } from '@/services/runtime/api'
import { isPersonalProfileSession } from './personalProfileRuntime'
import {
  changeNotificationPreference, confirmNotificationPreference, readNotificationPreferences,
  parseNotificationPreferences, preferenceWriteWasRejected, notificationPreferenceErrorKey,
  NotificationPreferenceError, PreferenceReadbackChanged,
} from './notificationPreferencesRuntime'

vi.mock('@/services/commercial/platformCommercial', async (original) => ({
  ...await original<typeof import('@/services/commercial/platformCommercial')>(), mutate: vi.fn(), request: vi.fn(),
}))
vi.mock('@/services/runtime/api', async (original) => ({
  ...await original<typeof import('@/services/runtime/api')>(), readSession: vi.fn(),
}))
const session: TrustedSession = { authenticated: true, actor_kind: 'user', user_id: 'u', active_tenant_id: 'a', context_version: 3 }
const initial = () => ({ tenant_id: 'a', user_id: 'u', policy: 'optional-notifications-v1',
  sms: { channel: 'sms', state: 'default', allowed: true, version: 0, updated_at: null },
  email: { channel: 'email', state: 'default', allowed: true, version: 0, updated_at: null } })
const denial = () => ({ channel: 'sms', state: 'deny', allowed: false, version: 1, updated_at: '2026-09-25T01:00:00Z' })
const receipt = () => ({ tenant_id: 'a', user_id: 'u', policy: 'optional-notifications-v1', receipt_id: 'a'.repeat(64), preference: denial() })
beforeEach(() => { vi.resetAllMocks(); vi.mocked(readSession).mockResolvedValue(session) })

describe('notification preference runtime', () => {
  it('accepts real user sessions and legacy tenant fixtures but rejects platform or incomplete identity', () => {
    expect(isPersonalProfileSession(session)).toBe(true)
    expect(isPersonalProfileSession({ ...session, actor_kind: 'tenant' })).toBe(true)
    for (const value of [null, { ...session, actor_kind: 'platform' }, { ...session, user_id: '' }, { ...session, authenticated: false }]) {
      expect(isPersonalProfileSession(value)).toBe(false)
    }
  })
  it('reads effective defaults from the server and retains explicit refusal separately', () => {
    const value = initial(); value.sms.allowed = false
    expect(parseNotificationPreferences(session, value).sms.allowed).toBe(false)
    const explicit = { ...value, sms: denial() }
    expect(parseNotificationPreferences(session, explicit).sms.state).toBe('deny')
  })
  it('rejects wrong owners, channels, corrupt states, missing values and unsafe versions', () => {
    for (const value of [null, {}, { ...initial(), tenant_id: 'b' }, { ...initial(), user_id: 'other' },
      { ...initial(), email: initial().sms }, { ...initial(), sms: { ...denial(), version: Number.MAX_SAFE_INTEGER + 1 } },
      { ...initial(), sms: { ...denial(), allowed: true } }, { ...initial(), sms: { ...denial(), state: 'important' } },
      { ...initial(), sms: { ...denial(), allowed: null } }, { ...initial(), sms: { ...denial(), updated_at: null } }]) {
      expect(() => parseNotificationPreferences(session, value)).toThrow('invalidResponse')
    }
  })
  it('does not turn a read failure into default allow', async () => {
    vi.mocked(request).mockRejectedValue(new Error('network'))
    await expect(readNotificationPreferences(session)).rejects.toThrow('network')
    expect(mutate).not.toHaveBeenCalled()
  })
  it('binds exact CAS payload and idempotency key without client-selected owner', async () => {
    vi.mocked(mutate).mockResolvedValue(receipt())
    const value = await changeNotificationPreference(session, parseNotificationPreferences(session, initial()), 'sms', false, 'same-key')
    expect(value.preference.allowed).toBe(false)
    expect(mutate).toHaveBeenCalledWith('/auth/personal/notification-preferences', 'POST',
      { channel: 'sms', allowed: false, expected_version: 0 },
      { idempotencyKey: 'same-key', sessionContext: expect.stringContaining('"actor_kind":"user"') })
  })
  it('rejects context drift before mutation and after a read', async () => {
    vi.mocked(readSession).mockResolvedValue({ ...session, active_tenant_id: 'b' })
    await expect(changeNotificationPreference(session, parseNotificationPreferences(session, initial()), 'sms', false, 'key')).rejects.toThrow('sessionChanged')
    expect(mutate).not.toHaveBeenCalled()
    vi.mocked(readSession).mockResolvedValueOnce(session).mockResolvedValueOnce({ ...session, context_version: 4 })
    vi.mocked(request).mockResolvedValue(initial())
    await expect(readNotificationPreferences(session)).rejects.toThrow('sessionChanged')
  })
  it('never accepts an unrelated or imprecise write receipt as success', async () => {
    const current = parseNotificationPreferences(session, initial())
    for (const value of [{ ...receipt(), tenant_id: 'b' }, { ...receipt(), policy: 'other-policy' },
      { ...receipt(), receipt_id: '' }, { ...receipt(), preference: { ...denial(), version: 2 } }]) {
      vi.mocked(mutate).mockResolvedValue(value)
      await expect(changeNotificationPreference(session, current, 'sms', false, 'key')).rejects.toThrow('invalidResponse')
    }
  })
  it('readback uses only reads and reports newer state without an overwrite', async () => {
    vi.mocked(mutate).mockResolvedValue(receipt())
    const accepted = await changeNotificationPreference(session, parseNotificationPreferences(session, initial()), 'sms', false, 'key')
    vi.mocked(mutate).mockClear()
    vi.mocked(request).mockResolvedValue({ ...initial(), sms: denial() })
    expect((await confirmNotificationPreference(session, accepted)).sms.allowed).toBe(false)
    vi.mocked(request).mockResolvedValue({ ...initial(), sms: { ...denial(), state: 'allow', allowed: true, version: 2 } })
    await expect(confirmNotificationPreference(session, accepted)).rejects.toBeInstanceOf(PreferenceReadbackChanged)
    expect(mutate).not.toHaveBeenCalled()
  })
  it('distinguishes definitive rejection from unknown outcome and sanitizes errors', () => {
    expect(preferenceWriteWasRejected(new CommercialApiError('PREFERENCE_VERSION_CONFLICT', 409, 'conflict'))).toBe(true)
    expect(preferenceWriteWasRejected(new CommercialApiError('unavailable', 503, 'http'))).toBe(false)
    expect(preferenceWriteWasRejected(new NotificationPreferenceError('invalidResponse'))).toBe(false)
    expect(notificationPreferenceErrorKey(new Error('secret@example.invalid'))).toBe('unavailable')
  })
})
