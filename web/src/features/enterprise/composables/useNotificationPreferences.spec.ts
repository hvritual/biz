import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { TrustedSession } from '@/services/runtime/api'
import { CommercialApiError } from '@/services/commercial/platformCommercial'
import {
  changeNotificationPreference, confirmNotificationPreference, readNotificationPreferences,
  PreferenceReadbackChanged, type NotificationPreferences, type PreferenceReceipt,
} from '@/services/enterprise/notificationPreferencesRuntime'
import { useNotificationPreferences } from './useNotificationPreferences'

vi.mock('@/services/enterprise/notificationPreferencesRuntime', async (original) => ({
  ...await original<typeof import('@/services/enterprise/notificationPreferencesRuntime')>(),
  changeNotificationPreference: vi.fn(), confirmNotificationPreference: vi.fn(), readNotificationPreferences: vi.fn(),
}))
const session: TrustedSession = { authenticated: true, actor_kind: 'user', user_id: 'u', active_tenant_id: 'a', context_version: 3 }
const initial = (): NotificationPreferences => ({ tenant_id: 'a', user_id: 'u', policy: 'optional-notifications-v1',
  sms: { channel: 'sms', state: 'default', allowed: true, version: 0, updated_at: null },
  email: { channel: 'email', state: 'default', allowed: true, version: 0, updated_at: null } })
const saved = (): NotificationPreferences => ({ ...initial(), sms: { channel: 'sms', state: 'deny', allowed: false, version: 1, updated_at: '2026-09-25T00:00:00Z' } })
const receipt = (): PreferenceReceipt => ({ tenant_id: 'a', user_id: 'u', policy: 'optional-notifications-v1', receipt_id: 'a'.repeat(64), preference: saved().sms })
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(readNotificationPreferences).mockResolvedValue(initial())
  vi.mocked(changeNotificationPreference).mockResolvedValue(receipt())
  vi.mocked(confirmNotificationPreference).mockResolvedValue(saved())
})

describe('notification preference confirmation and recovery', () => {
  it('requires confirmation and restores the switch on cancellation without a write', async () => {
    const flow = useNotificationPreferences(() => session)
    await flow.load(); flow.begin('sms', false)
    expect(flow.draft.sms).toBe(false)
    expect(flow.snapshot.value?.sms.allowed).toBe(true)
    flow.close()
    expect(flow.draft.sms).toBe(true)
    expect(changeNotificationPreference).not.toHaveBeenCalled()
  })
  it('publishes success only after a matching server readback', async () => {
    const flow = useNotificationPreferences(() => session)
    let resolve!: (value: NotificationPreferences) => void
    vi.mocked(confirmNotificationPreference).mockImplementation(() => new Promise((done) => { resolve = done }))
    await flow.load(); flow.begin('sms', false)
    const saving = flow.confirm()
    await vi.waitFor(() => expect(confirmNotificationPreference).toHaveBeenCalled())
    expect(flow.success.value).toBe(false)
    expect(flow.snapshot.value?.sms.allowed).toBe(true)
    resolve(saved()); await saving
    expect(flow.success.value).toBe(true)
    expect(flow.draft.sms).toBe(false)
    expect(flow.snapshot.value?.email.allowed).toBe(true)
  })
  it('restores the last confirmed value on rejection and requires a fresh read', async () => {
    const flow = useNotificationPreferences(() => session)
    vi.mocked(changeNotificationPreference).mockRejectedValue(new CommercialApiError('PREFERENCE_VERSION_CONFLICT', 409, 'conflict'))
    await flow.load(); flow.begin('sms', false); await flow.confirm()
    expect(flow.draft.sms).toBe(true)
    expect(flow.error.value).toBe('conflict')
    expect(flow.canEdit.value).toBe(false)
    expect(flow.recovery.value).toBe('')
    await flow.load()
    expect(flow.canEdit.value).toBe(true)
  })
  it('recovers acknowledged writes using GET only, even after the dialog closes', async () => {
    const flow = useNotificationPreferences(() => session)
    vi.mocked(confirmNotificationPreference).mockRejectedValueOnce(new Error('read unavailable')).mockResolvedValue(saved())
    await flow.load(); flow.begin('sms', false); await flow.confirm()
    expect(flow.recovery.value).toBe('read')
    expect(flow.success.value).toBe(false)
    flow.close(); await flow.recover()
    expect(changeNotificationPreference).toHaveBeenCalledTimes(1)
    expect(confirmNotificationPreference).toHaveBeenCalledTimes(2)
    expect(flow.success.value).toBe(true)
  })
  it('uncertain retries use exactly the original payload and key and block new edits', async () => {
    const flow = useNotificationPreferences(() => session)
    vi.mocked(changeNotificationPreference).mockRejectedValueOnce(new Error('connection lost')).mockResolvedValue(receipt())
    await flow.load(); flow.begin('sms', false); await flow.confirm()
    expect(flow.recovery.value).toBe('write')
    const original = vi.mocked(changeNotificationPreference).mock.calls[0]
    flow.close(); flow.begin('email', false)
    expect(flow.dialogOpen.value).toBe(false)
    await flow.recover()
    expect(vi.mocked(changeNotificationPreference).mock.calls[1]).toEqual(original)
    expect(flow.success.value).toBe(true)
  })
  it('ignores a late receipt after switching tenant and invalidating the context', async () => {
    let current = session
    const flow = useNotificationPreferences(() => current)
    let resolve!: (value: PreferenceReceipt) => void
    vi.mocked(changeNotificationPreference).mockImplementation(() => new Promise((done) => { resolve = done }))
    await flow.load(); flow.begin('sms', false)
    const saving = flow.confirm()
    current = { ...session, active_tenant_id: 'b', context_version: 4 }; flow.invalidate('sessionChanged')
    resolve(receipt()); await saving
    expect(flow.snapshot.value).toBeNull()
    expect(flow.success.value).toBe(false)
    expect(confirmNotificationPreference).not.toHaveBeenCalled()
  })
  it('shows a concurrently newer value without replaying or claiming success', async () => {
    const flow = useNotificationPreferences(() => session)
    const newer: NotificationPreferences = { ...saved(), sms: { ...saved().sms, allowed: true, state: 'allow', version: 2 } }
    vi.mocked(confirmNotificationPreference).mockRejectedValue(new PreferenceReadbackChanged(newer))
    await flow.load(); flow.begin('sms', false); await flow.confirm()
    expect(flow.snapshot.value).toEqual(newer)
    expect(flow.success.value).toBe(false)
    expect(flow.error.value).toBe('changedAfterSave')
    expect(changeNotificationPreference).toHaveBeenCalledTimes(1)
  })
  it('does not create local defaults after read failure', async () => {
    const flow = useNotificationPreferences(() => session)
    vi.mocked(readNotificationPreferences).mockRejectedValue(new Error('network'))
    await flow.load()
    expect(flow.snapshot.value).toBeNull()
    expect(flow.canEdit.value).toBe(false)
    flow.begin('sms', false); await flow.confirm()
    expect(changeNotificationPreference).not.toHaveBeenCalled()
  })
})
