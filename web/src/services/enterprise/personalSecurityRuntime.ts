import { CommercialApiError, mutate } from '@/services/commercial/platformCommercial'
import { readSession, sessionContext, type TrustedSession } from '@/services/runtime/api'
import { getMyPersonalProfile, samePersonalProfileSession, type TenantPersonalProfile } from './personalProfileRuntime'

export type SecurityChannel = 'email' | 'sms'
export type SecurityMode = 'contact' | 'deletion'
export type SecurityChallenge = Readonly<{
  challenge_id: string
  flow_id: string
  masked_destination: string
  expires_at: string
  delivery_state: string
  version: number
  resend_after_seconds: number
}>
export type ContactChangeReceipt = Readonly<{
  tenant_id: string
  user_id: string
  email: string
  phone: string
  version: number
  notification_event_id: string
  notification_state: string
}>
export type TenantDeletionReceipt = Readonly<{
  tenant_id: string
  user_id: string
  version: number
  deleted_at: string
  notification_event_id: string
  notification_state: string
  reauthentication_required: boolean
}>

export class PersonalSecurityError extends Error {
  constructor(readonly key: string) { super(key) }
}

const knownErrors: Record<string, string> = {
  CURRENT_PASSWORD_INVALID: 'password', CONTACT_UNCHANGED: 'unchanged', CONTACT_UNAVAILABLE: 'unbound',
  VERIFICATION_INVALID: 'verification', VERIFICATION_RATE_LIMITED: 'rateLimited',
  VERIFICATION_EXPIRED: 'expired', VERIFICATION_CONSUMED: 'consumed', VERIFICATION_CONFLICT: 'conflict',
  CONTACT_CONFLICT: 'contactConflict', MEMBERSHIP_CONFLICT: 'conflict', LAST_OWNER_PROTECTED: 'lastOwner',
  MEMBERSHIP_NOT_FOUND: 'unavailable', SESSION_CONTEXT_CHANGED: 'sessionChanged',
  DELETION_CONFIRMATION_REQUIRED: 'confirmation', SELF_SECURITY_UNAVAILABLE: 'unavailable',
}
export function personalSecurityErrorKey(error: unknown): string {
  if (error instanceof PersonalSecurityError) return error.key
  if (error instanceof CommercialApiError) {
    if (error.status === 401) return 'signIn'
    if (error.status === 403) return 'forbidden'
    return knownErrors[error.message] ?? 'unavailable'
  }
  // Never display untrusted response text, credentials, contacts or transport details.
  return 'unavailable'
}
function requireValue(valid: unknown, key = 'invalidResponse'): asserts valid {
  if (!valid) throw new PersonalSecurityError(key)
}
export function safeSecurityVersion(value: unknown): number {
  requireValue(typeof value === 'number' && Number.isSafeInteger(value) && value > 0)
  return value
}
function masked(value: unknown, allowEmpty = true): string {
  requireValue(typeof value === 'string' && ((allowEmpty && value === '') || value.includes('***')))
  return value
}
function nonempty(value: unknown): string {
  requireValue(typeof value === 'string' && value.trim() && value.length <= 512)
  return value
}
export function parseSecurityChallenge(value: SecurityChallenge): SecurityChallenge {
  const expiry = nonempty(value.expires_at)
  requireValue(Number.isFinite(Date.parse(expiry)))
  requireValue(Number.isSafeInteger(value.resend_after_seconds) && value.resend_after_seconds >= 60)
  return Object.freeze({
    challenge_id: nonempty(value.challenge_id), flow_id: nonempty(value.flow_id),
    masked_destination: masked(value.masked_destination, false), expires_at: expiry,
    delivery_state: nonempty(value.delivery_state), version: safeSecurityVersion(value.version),
    resend_after_seconds: value.resend_after_seconds,
  })
}
export async function requirePersonalSecuritySession(expected: TrustedSession) {
  requireValue(expected.authenticated && expected.actor_kind === 'tenant' && expected.user_id && expected.active_tenant_id, 'signIn')
  const actual = await readSession()
  requireValue(samePersonalProfileSession(expected, actual), 'sessionChanged')
}
function assertReceiptOwner(session: TrustedSession, receipt: { user_id: string; tenant_id: string; version: number }) {
  requireValue(receipt.user_id === session.user_id && receipt.tenant_id === session.active_tenant_id)
  safeSecurityVersion(receipt.version)
}
export async function requestSecurityCode(session: TrustedSession, mode: SecurityMode, channel: SecurityChannel,
  password: string, destination: string, key: string) {
  await requirePersonalSecuritySession(session)
  if (mode === 'contact') requireValue(password && destination.trim() && !destination.includes('*'), 'contactInput')
  const result = await mutate<SecurityChallenge>(`/auth/personal/${mode === 'contact' ? 'contact-change' : 'tenant-deletion'}/request`,
    'POST', mode === 'contact' ? { channel, current_password: password, destination: destination.trim() } : { channel },
    { idempotencyKey: key, sessionContext: sessionContext(session) })
  await requirePersonalSecuritySession(session)
  return parseSecurityChallenge(result)
}
export async function completeContactChange(session: TrustedSession, channel: SecurityChannel, challenge: SecurityChallenge,
  destination: string, code: string, key: string): Promise<ContactChangeReceipt> {
  await requirePersonalSecuritySession(session)
  const result = await mutate<ContactChangeReceipt>('/auth/personal/contact-change/complete', 'POST', {
    channel, destination, challenge_id: challenge.challenge_id, flow_id: challenge.flow_id,
    otp_code: code, version: safeSecurityVersion(challenge.version),
  }, { idempotencyKey: key, sessionContext: sessionContext(session) })
  assertReceiptOwner(session, result)
  requireValue(result.version === challenge.version + 1)
  return Object.freeze({ ...result, email: masked(result.email), phone: masked(result.phone),
    notification_event_id: nonempty(result.notification_event_id), notification_state: nonempty(result.notification_state) })
}
export async function confirmContactReadback(session: TrustedSession, receipt: ContactChangeReceipt): Promise<TenantPersonalProfile> {
  const profile = await getMyPersonalProfile(session)
  await requirePersonalSecuritySession(session)
  requireValue(String(profile.version) === String(receipt.version) && profile.email === receipt.email && profile.phone === receipt.phone, 'confirmRead')
  return profile
}
export async function completeTenantDeletion(session: TrustedSession, channel: SecurityChannel, challenge: SecurityChallenge,
  code: string, confirmed: boolean, key: string): Promise<TenantDeletionReceipt> {
  requireValue(confirmed, 'confirmation')
  await requirePersonalSecuritySession(session)
  const result = await mutate<TenantDeletionReceipt>('/auth/personal/tenant-deletion/complete', 'POST', {
    channel, challenge_id: challenge.challenge_id, flow_id: challenge.flow_id, otp_code: code,
    version: safeSecurityVersion(challenge.version), confirm_tenant_id: session.active_tenant_id, confirm_irreversible: true,
  }, { idempotencyKey: key, sessionContext: sessionContext(session) })
  assertReceiptOwner(session, result)
  requireValue(result.reauthentication_required === true && result.version === challenge.version + 1 && Number.isFinite(Date.parse(result.deleted_at)))
  return result
}
