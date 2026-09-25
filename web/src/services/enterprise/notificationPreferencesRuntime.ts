import { CommercialApiError, mutate, request } from '@/services/commercial/platformCommercial'
import { readSession, sessionContext, type TrustedSession } from '@/services/runtime/api'
import { isPersonalProfileSession, samePersonalProfileSession } from './personalProfileRuntime'

export type PreferenceChannel = 'sms' | 'email'
export type NotificationPreference = Readonly<{
  channel: PreferenceChannel
  state: 'default' | 'allow' | 'deny'
  allowed: boolean
  version: number
  updated_at: string | null
}>
export type NotificationPreferences = Readonly<{
  tenant_id: string
  user_id: string
  policy: string
  sms: NotificationPreference
  email: NotificationPreference
}>
export type PreferenceReceipt = Readonly<{
  tenant_id: string
  user_id: string
  policy: string
  receipt_id: string
  preference: NotificationPreference
}>
export class NotificationPreferenceError extends Error {
  constructor(readonly key: string, readonly beforeWrite = false) { super(key) }
}
export class PreferenceReadbackChanged extends NotificationPreferenceError {
  constructor(readonly current: NotificationPreferences) { super('changedAfterSave') }
}
const endpoint = '/auth/personal/notification-preferences'
function requireValue(value: unknown, key = 'invalidResponse', beforeWrite = false): asserts value {
  if (!value) throw new NotificationPreferenceError(key, beforeWrite)
}
function object(value: unknown): Record<string, unknown> {
  requireValue(value !== null && typeof value === 'object' && !Array.isArray(value))
  return value as Record<string, unknown>
}
function owner(session: TrustedSession, value: Record<string, unknown>) {
  requireValue(value.tenant_id === session.active_tenant_id && value.user_id === session.user_id)
  requireValue(typeof value.policy === 'string' && value.policy.length > 0 && value.policy.length <= 64)
  return { tenant_id: value.tenant_id as string, user_id: value.user_id as string, policy: value.policy }
}
function preference(value: unknown, channel: PreferenceChannel): NotificationPreference {
  const row = object(value)
  requireValue(row.channel === channel && typeof row.allowed === 'boolean')
  requireValue(typeof row.version === 'number' && Number.isSafeInteger(row.version) && row.version >= 0)
  requireValue(row.state === 'default' || row.state === 'allow' || row.state === 'deny')
  if (row.state === 'default') requireValue(row.version === 0 && row.updated_at === null)
  else {
    requireValue(row.version > 0 && typeof row.updated_at === 'string' && Number.isFinite(Date.parse(row.updated_at)))
    requireValue(row.allowed === (row.state === 'allow'))
  }
  // Effective default comes from the server; the browser does not invent it.
  return Object.freeze({ channel, state: row.state, allowed: row.allowed, version: row.version,
    updated_at: row.updated_at as string | null })
}
export function parseNotificationPreferences(session: TrustedSession, value: unknown): NotificationPreferences {
  const row = object(value)
  return Object.freeze({ ...owner(session, row), sms: preference(row.sms, 'sms'), email: preference(row.email, 'email') })
}
async function requireCurrentSession(expected: TrustedSession) {
  requireValue(isPersonalProfileSession(expected) && Number.isSafeInteger(expected.context_version) &&
    (expected.context_version ?? 0) > 0, 'signIn', true)
  const actual = await readSession()
  requireValue(samePersonalProfileSession(expected, actual), 'sessionChanged', true)
}
export async function readNotificationPreferences(session: TrustedSession): Promise<NotificationPreferences> {
  await requireCurrentSession(session)
  const value = await request<unknown>(endpoint, { headers: { 'X-Biz-Session-Context': sessionContext(session) } })
  const result = parseNotificationPreferences(session, value)
  await requireCurrentSession(session)
  return result
}
export async function changeNotificationPreference(session: TrustedSession, current: NotificationPreferences,
  channel: PreferenceChannel, allowed: boolean, key: string): Promise<PreferenceReceipt> {
  await requireCurrentSession(session)
  requireValue(current.tenant_id === session.active_tenant_id && current.user_id === session.user_id, 'sessionChanged', true)
  requireValue((channel === 'sms' || channel === 'email') && typeof allowed === 'boolean' && /^[\x21-\x7e]{1,256}$/.test(key), 'invalidInput', true)
  const version = current[channel].version
  requireValue(Number.isSafeInteger(version) && version >= 0 && version < Number.MAX_SAFE_INTEGER, 'invalidInput', true)
  const result = object(await mutate<unknown>(endpoint, 'POST', { channel, allowed, expected_version: version },
    { idempotencyKey: key, sessionContext: sessionContext(session) }))
  const identity = owner(session, result), chosen = preference(result.preference, channel)
  requireValue(identity.policy === current.policy && chosen.version === version + 1 && chosen.allowed === allowed &&
    chosen.state === (allowed ? 'allow' : 'deny') && typeof result.receipt_id === 'string' && /^[0-9a-f]{64}$/.test(result.receipt_id))
  return Object.freeze({ ...identity, receipt_id: result.receipt_id, preference: chosen })
}
export async function confirmNotificationPreference(session: TrustedSession, receipt: PreferenceReceipt) {
  const current = await readNotificationPreferences(session)
  const actual = current[receipt.preference.channel], expected = receipt.preference
  if (current.policy !== receipt.policy || actual.version !== expected.version || actual.state !== expected.state ||
    actual.allowed !== expected.allowed || actual.updated_at !== expected.updated_at) {
    throw new PreferenceReadbackChanged(current)
  }
  return current
}
export function notificationPreferenceErrorKey(error: unknown): string {
  if (error instanceof NotificationPreferenceError) return error.key
  if (error instanceof CommercialApiError) {
    if (error.status === 401) return 'signIn'
    if (error.status === 403) return 'forbidden'
    if (error.message === 'SESSION_CONTEXT_CHANGED') return 'sessionChanged'
    if (error.status === 409) return 'conflict'
    if (error.status === 400 || error.status === 422) return 'invalidInput'
  }
  return 'unavailable'
}
export function preferenceWriteWasRejected(error: unknown): boolean {
  return (error instanceof NotificationPreferenceError && error.beforeWrite) ||
    (error instanceof CommercialApiError && error.code !== 'invalid-response' && [400, 401, 403, 409, 422].includes(error.status))
}
