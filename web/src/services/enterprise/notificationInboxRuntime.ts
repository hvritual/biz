import { CommercialApiError, commercialRequestId, mutate, request } from '@/services/commercial/platformCommercial'
import { readSession, sessionContext, type TrustedSession } from '@/services/runtime/api'
import { isPersonalProfileSession, samePersonalProfileSession } from './personalProfileRuntime'

export type NotificationLevel = 'general' | 'important' | 'urgent'
export type InAppNotification = Readonly<{
  message_id: string
  type_code: string
  level: NotificationLevel
  reference_kind: string
  reference_id: string
  created_at: string
}>
export type NotificationInboxSnapshot = Readonly<{
  tenant_id: string
  user_id: string
  unread_count: number
  messages: readonly InAppNotification[]
  as_of: string
}>
export type MarkAllReadReceipt = Readonly<{
  receipt_id: string
  tenant_id: string
  user_id: string
  marked_count: number
  read_at: string
}>

export class NotificationInboxError extends Error {
  constructor(readonly key: string, readonly beforeWrite = false) {
    super(key)
  }
}

const endpoint = '/auth/personal/in-app-notifications'
const idPattern = /^[0-9a-f]{64}$/
const typePattern = /^[a-z0-9][a-z0-9_.-]{0,63}$/
const referenceKindPattern = /^[a-z0-9][a-z0-9_.-]{0,63}$/

function requireValue(value: unknown, key = 'invalidResponse', beforeWrite = false): asserts value {
  if (!value) throw new NotificationInboxError(key, beforeWrite)
}

function object(value: unknown): Record<string, unknown> {
  requireValue(value !== null && typeof value === 'object' && !Array.isArray(value))
  return value as Record<string, unknown>
}

function validDate(value: unknown): value is string {
  return typeof value === 'string' && value.length <= 64 && Number.isFinite(Date.parse(value))
}

function safeReference(value: unknown, max = 160): value is string {
  if (typeof value !== 'string' || value.length > max) return false
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index)
    if (code < 0x20 || code === 0x7f) return false
  }
  return true
}

function requireOwner(session: TrustedSession, value: Record<string, unknown>) {
  requireValue(value.tenant_id === session.active_tenant_id && value.user_id === session.user_id)
  return { tenant_id: value.tenant_id as string, user_id: value.user_id as string }
}

function parseMessage(value: unknown): InAppNotification {
  const row = object(value)
  requireValue(typeof row.message_id === 'string' && idPattern.test(row.message_id))
  requireValue(typeof row.type_code === 'string' && typePattern.test(row.type_code))
  requireValue(row.level === 'general' || row.level === 'important' || row.level === 'urgent')
  requireValue(typeof row.reference_kind === 'string' && referenceKindPattern.test(row.reference_kind))
  requireValue(safeReference(row.reference_id) && validDate(row.created_at))
  return Object.freeze({
    message_id: row.message_id,
    type_code: row.type_code,
    level: row.level,
    reference_kind: row.reference_kind,
    reference_id: row.reference_id,
    created_at: row.created_at,
  })
}

export function parseNotificationInbox(session: TrustedSession, value: unknown): NotificationInboxSnapshot {
  const row = object(value)
  const identity = requireOwner(session, row)
  requireValue(Number.isSafeInteger(row.unread_count) && Number(row.unread_count) >= 0)
  requireValue(Array.isArray(row.messages) && row.messages.length <= 50 && validDate(row.as_of))
  const messages = Object.freeze(row.messages.map(parseMessage))
  requireValue(messages.length <= Number(row.unread_count))
  return Object.freeze({
    ...identity,
    unread_count: Number(row.unread_count),
    messages,
    as_of: row.as_of as string,
  })
}

function parseReceipt(session: TrustedSession, value: unknown): MarkAllReadReceipt {
  const row = object(value)
  const identity = requireOwner(session, row)
  requireValue(typeof row.receipt_id === 'string' && idPattern.test(row.receipt_id))
  requireValue(Number.isSafeInteger(row.marked_count) && Number(row.marked_count) >= 0)
  requireValue(validDate(row.read_at))
  return Object.freeze({
    ...identity,
    receipt_id: row.receipt_id,
    marked_count: Number(row.marked_count),
    read_at: row.read_at,
  })
}

async function requireCurrentSession(expected: TrustedSession, beforeWrite = true) {
  requireValue(
    isPersonalProfileSession(expected) && Number.isSafeInteger(expected.context_version) && (expected.context_version ?? 0) > 0,
    'signIn',
    beforeWrite,
  )
  const actual = await readSession()
  requireValue(samePersonalProfileSession(expected, actual), 'sessionChanged', beforeWrite)
}

export async function readNotificationInbox(session: TrustedSession): Promise<NotificationInboxSnapshot> {
  await requireCurrentSession(session)
  const value = await request<unknown>(endpoint, {
    headers: { 'X-Biz-Session-Context': sessionContext(session) },
  })
  const snapshot = parseNotificationInbox(session, value)
  await requireCurrentSession(session)
  return snapshot
}

export async function markAllNotificationsRead(session: TrustedSession, idempotencyKey: string): Promise<MarkAllReadReceipt> {
  await requireCurrentSession(session)
  requireValue(/^[\x21-\x7e]{1,256}$/.test(idempotencyKey), 'invalidInput', true)
  const value = await mutate<unknown>(`${endpoint}/read-all`, 'POST', {}, {
    idempotencyKey,
    sessionContext: sessionContext(session),
  })
  return parseReceipt(session, value)
}

export function notificationInboxRequestId() {
  return commercialRequestId('notification-read-all')
}

export function notificationInboxErrorKey(error: unknown): string {
  if (error instanceof NotificationInboxError) return error.key
  if (error instanceof CommercialApiError) {
    if (error.status === 401) return 'signIn'
    if (error.status === 403) return 'forbidden'
    if (error.message === 'SESSION_CONTEXT_CHANGED') return 'sessionChanged'
    if (error.status === 400 || error.status === 422) return 'invalidInput'
  }
  if (error instanceof DOMException && error.name === 'AbortError') return 'sessionChanged'
  return 'unavailable'
}
