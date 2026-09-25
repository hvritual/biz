import type { Notification_V1_MessageConfigurationDTO as ConfigurationDTO } from '../../../../contracts/generated/client'
import { CommercialApiError, commercialRequestId, mutate, request } from '@/services/commercial/platformCommercial'
import { readSession, sessionContext, type TrustedSession } from '@/services/runtime/api'
import { isPersonalProfileSession, samePersonalProfileSession } from './personalProfileRuntime'

export const messageLevels = ['urgent', 'important', 'general'] as const
export type MessageLevel = typeof messageLevels[number]
export type MessageType = Readonly<{ code: string; name: string; level: MessageLevel }>
export type MessageChannel = Readonly<{ code: string; name: string; configurable: boolean; unavailableReason: string }>
export type MessageDirectoryEntry = Readonly<{ id: string; name: string }>
export type MessageConfiguration = Readonly<Omit<Required<ConfigurationDTO>, 'level' | 'channels' | 'additionalUserIds'> & {
  level: MessageLevel; channels: readonly string[]; additionalUserIds: readonly string[]
}>
export type ConfigurationDraft = {
  groupId: string; levels: string[]; channels: string[]; primaryUserId: string; secondaryUserId: string
  additionalUserIds: string[]; notes: string
}
export type MessagePage<T> = Readonly<{ items: readonly T[]; total: number; page: number; pageSize: number }>
export type ConfigurationQuery = { groupId?: string; level?: string; recipientId?: string; page: number; pageSize: number }
export type TypeQuery = { code?: string; name?: string; level?: string; page: number; pageSize: number }
export type MessageTypesPage = MessagePage<MessageType> & { groups: readonly { level: MessageLevel; total: number }[] }
export type FrozenConfigurationDraft = Readonly<Omit<ConfigurationDraft, 'levels' | 'channels' | 'additionalUserIds'> & { levels: readonly string[]; channels: readonly string[]; additionalUserIds: readonly string[] }>
export type MessageConfigurationReceipt = Readonly<{ receiptId: string; tenantId: string; configurations: readonly MessageConfiguration[] }>
export type ConfigurationCommand = Readonly<{
  kind: 'create' | 'update' | 'delete'; key: string; session: TrustedSession; target: MessageConfiguration | null
  path: string; method: 'POST' | 'PATCH'; body: Readonly<Record<string, unknown>>; values: FrozenConfigurationDraft
}>
export class NotificationConfigurationError extends Error {
  constructor(readonly key: string, readonly beforeWrite = false) { super(key) }
}
export class ConfigurationReadbackChanged extends NotificationConfigurationError {
  constructor() { super('changedAfterSave') }
}
const endpoint = '/v1/tenant/notification'
const maxVersion = 18446744073709551615n
function requireValue(value: unknown, key = 'invalidResponse', beforeWrite = false): asserts value {
  if (!value) throw new NotificationConfigurationError(key, beforeWrite)
}
function object(value: unknown): Record<string, unknown> {
  requireValue(value !== null && typeof value === 'object' && !Array.isArray(value))
  return value as Record<string, unknown>
}
function text(value: unknown, max = 512, empty = false): string {
  requireValue(typeof value === 'string' && (empty || value.length > 0) && [...value].length <= max)
  return value
}
function id(value: unknown, empty = false): string {
  const result = text(value, 64, empty)
  requireValue((empty && result === '') || /^[\x21-\x7e]{1,64}$/.test(result))
  return result
}
function level(value: unknown): MessageLevel {
  requireValue(messageLevels.some(item => item === value)); return value as MessageLevel
}
function version(value: unknown): string {
  requireValue(typeof value === 'string' && /^[1-9]\d*$/.test(value) && value.length <= 20 && BigInt(value) <= maxVersion)
  return value
}
function count(value: unknown): number {
  requireValue(typeof value === 'number' || typeof value === 'string')
  requireValue(/^(0|[1-9]\d*)$/.test(String(value)))
  const result = Number(value); requireValue(Number.isSafeInteger(result) && result >= 0); return result
}
function array(value: unknown): unknown[] {
  if (value === undefined) return [] // proto3 repeated fields may be omitted; an absent response object is never accepted.
  requireValue(Array.isArray(value)); return value
}
function ids(value: unknown, max: number): readonly string[] {
  const result = array(value).map(value => id(value))
  requireValue(result.length <= max && new Set(result).size === result.length)
  return Object.freeze(result)
}
function owner(session: TrustedSession, row: Record<string, unknown>) {
  requireValue(row.tenantId === session.active_tenant_id)
}
function date(value: unknown): string {
  const result = text(value); requireValue(Number.isFinite(Date.parse(result))); return result
}
export function parseMessageConfiguration(session: TrustedSession, value: unknown): MessageConfiguration {
  const row = object(value); owner(session, row)
  const primary = id(row.primaryUserId), secondary = id(row.secondaryUserId ?? '', true)
  const additional = ids(row.additionalUserIds, 100), channels = ids(row.channels, 64)
  requireValue(channels.length > 0 && (!secondary || secondary !== primary) && !additional.includes(primary) && (!secondary || !additional.includes(secondary)))
  requireValue(row.deleted === undefined || typeof row.deleted === 'boolean')
  return Object.freeze({ id: id(row.id), tenantId: id(row.tenantId), groupId: id(row.groupId), groupName: text(row.groupName ?? '', 256, true),
    level: level(row.level), channels, primaryUserId: primary, secondaryUserId: secondary, additionalUserIds: additional,
    notes: text(row.notes ?? '', 500, true), version: version(row.version), createdAt: date(row.createdAt), updatedAt: date(row.updatedAt), deleted: row.deleted === true })
}
export async function requireNotificationSession(session: TrustedSession) {
  requireValue(isPersonalProfileSession(session) && Number.isSafeInteger(session.context_version) && (session.context_version ?? 0) > 0, 'signIn', true)
  requireValue(samePersonalProfileSession(session, await readSession()), 'sessionChanged', true)
}
async function read<T>(session: TrustedSession, path: string, parse: (value: unknown) => T): Promise<T> {
  await requireNotificationSession(session)
  const result = parse(await request<unknown>(endpoint + path, { headers: { 'X-Biz-Session-Context': sessionContext(session) } }))
  await requireNotificationSession(session)
  return result
}
function query(values: Record<string, string | number | undefined>) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(values)) if (value !== undefined && String(value) !== '') params.set(key, String(value))
  return params.size ? `?${params}` : ''
}
function page<T>(session: TrustedSession, value: unknown, expected: { page: number; pageSize: number }, parser: (item: unknown) => T): MessagePage<T> {
  const row = object(value); owner(session, row)
  const total = count(row.total ?? '0'), items = array(row.items).map(parser)
  requireValue(row.page === expected.page && row.pageSize === expected.pageSize && items.length <= expected.pageSize && items.length <= total)
  return Object.freeze({ items: Object.freeze(items), total, page: expected.page, pageSize: expected.pageSize })
}
export function listMessageTypes(session: TrustedSession, input: TypeQuery): Promise<MessageTypesPage> {
  return read(session, `/types${query({ code: input.code, name: input.name, level: input.level, page: input.page, page_size: input.pageSize })}`, value => {
    const result = page(session, value, input, value => { const row = object(value); return Object.freeze({ code: id(row.code), name: text(row.name, 256), level: level(row.level) }) })
    const groups = array(object(value).groups).map(value => { const row = object(value); return Object.freeze({ level: level(row.level), total: count(row.total ?? '0') }) })
    requireValue(groups.length === 3 && new Set(groups.map(row => row.level)).size === 3 && groups.reduce((sum, row) => sum + row.total, 0) === result.total)
    return Object.freeze({ ...result, groups: Object.freeze(groups) })
  })
}
export function listMessageChannels(session: TrustedSession): Promise<readonly MessageChannel[]> {
  return read(session, '/channels', value => {
    const row = object(value); owner(session, row)
    const result = array(row.items).map(value => {
      const channel = object(value)
      requireValue(channel.configurable === undefined || typeof channel.configurable === 'boolean')
      const configurable = channel.configurable === true, reason = text(channel.unavailableReason ?? '', 256, configurable)
      return Object.freeze({ code: id(channel.code), name: text(channel.name, 256), configurable, unavailableReason: reason })
    })
    requireValue(new Set(result.map(row => row.code)).size === result.length)
    return Object.freeze(result)
  })
}
export function listMessageDirectory(session: TrustedSession, kind: 'groups' | 'recipients', search: string, currentPage = 1, pageSize = 20): Promise<MessagePage<MessageDirectoryEntry>> {
  requireValue(kind === 'groups' || kind === 'recipients', 'invalidInput', true)
  return read(session, `/${kind}${query({ query: search, page: currentPage, page_size: pageSize })}`, value =>
    page(session, value, { page: currentPage, pageSize }, value => { const row = object(value); return Object.freeze({ id: id(row.id), name: text(row.name, 256) }) }))
}
export function listMessageConfigurations(session: TrustedSession, input: ConfigurationQuery): Promise<MessagePage<MessageConfiguration>> {
  return read(session, `/configurations${query({ group_id: input.groupId, level: input.level, recipient_id: input.recipientId, page: input.page, page_size: input.pageSize })}`, value =>
    page(session, value, input, value => { const result = parseMessageConfiguration(session, value); requireValue(!result.deleted); return result }))
}
export function getMessageConfiguration(session: TrustedSession, configurationId: string): Promise<MessageConfiguration> {
  return read(session, `/configurations/${encodeURIComponent(id(configurationId))}`, value => {
    const result = parseMessageConfiguration(session, value); requireValue(result.id === configurationId && !result.deleted); return result
  })
}
function sameSet(a: readonly string[], b: readonly string[]) { return a.length === b.length && [...a].sort().every((value, index) => value === [...b].sort()[index]) }
function sameValues(a: MessageConfiguration, b: FrozenConfigurationDraft) {
  return a.groupId === b.groupId && a.primaryUserId === b.primaryUserId && a.secondaryUserId === b.secondaryUserId && a.notes === b.notes && sameSet(a.channels, b.channels) && sameSet(a.additionalUserIds, b.additionalUserIds)
}
export function prepareConfigurationCommand(kind: ConfigurationCommand['kind'], session: TrustedSession, draft: ConfigurationDraft, target: MessageConfiguration | null, channels: readonly MessageChannel[]): ConfigurationCommand {
  requireValue(isPersonalProfileSession(session), 'signIn', true)
  requireValue(['create', 'update', 'delete'].includes(kind), 'invalidInput', true)
  if (kind !== 'create') requireValue(target && target.tenantId === session.active_tenant_id && !target.deleted && BigInt(version(target.version)) < maxVersion, 'invalidInput', true)
  if (kind !== 'delete') {
    const all = [draft.primaryUserId, ...(draft.secondaryUserId ? [draft.secondaryUserId] : []), ...draft.additionalUserIds]
    requireValue(all.every(value => /^[\x21-\x7e]{1,64}$/.test(value)) && new Set(all).size === all.length && draft.additionalUserIds.length <= 100, 'invalidInput', true)
    requireValue(draft.channels.length > 0 && new Set(draft.channels).size === draft.channels.length && draft.channels.every(code => channels.some(channel => channel.code === code && channel.configurable)), 'channelUnavailable', true)
    requireValue([...draft.notes].length <= 500 && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/.test(draft.notes), 'invalidInput', true)
    requireValue(draft.levels.length > 0 && draft.levels.length <= 3 && new Set(draft.levels).size === draft.levels.length && draft.levels.every(value => messageLevels.some(level => level === value)), 'invalidInput', true)
    requireValue(/^[\x21-\x7e]{1,64}$/.test(draft.groupId), 'invalidInput', true)
    if (kind === 'update') requireValue(target && draft.groupId === target.groupId && draft.levels.length === 1 && draft.levels[0] === target.level, 'invalidInput', true)
  }
  const values = Object.freeze({ ...draft, levels: Object.freeze([...draft.levels]), channels: Object.freeze([...draft.channels]), additionalUserIds: Object.freeze([...draft.additionalUserIds]) })
  const editable = { channels: values.channels, primaryUserId: values.primaryUserId, secondaryUserId: values.secondaryUserId, additionalUserIds: values.additionalUserIds, notes: values.notes }
  const body = kind === 'create' ? { groupId: values.groupId, levels: values.levels, ...editable }
    : kind === 'update' ? { expectedVersion: target!.version, ...editable } : { expectedVersion: target!.version }
  return Object.freeze({ kind, key: commercialRequestId('notification-config'), session: Object.freeze({ ...session }), target, values,
    method: kind === 'update' ? 'PATCH' : 'POST', path: `/configurations${kind === 'create' ? '' : `/${encodeURIComponent(target!.id)}${kind === 'delete' ? '/delete' : ''}`}`, body: Object.freeze(body) })
}
export async function submitConfigurationCommand(command: ConfigurationCommand): Promise<MessageConfigurationReceipt> {
  await requireNotificationSession(command.session)
  const row = object(await mutate<unknown>(endpoint + command.path, command.method, command.body, { idempotencyKey: command.key, sessionContext: sessionContext(command.session) }))
  owner(command.session, row)
  const configurations = array(row.configurations).map(value => parseMessageConfiguration(command.session, value))
  const expectedLevels = command.kind === 'create' ? command.values.levels : [command.target!.level]
  requireValue(configurations.length === expectedLevels.length && sameSet(configurations.map(value => value.level), expectedLevels) && new Set(configurations.map(value => value.id)).size === configurations.length)
  for (const item of configurations) {
    requireValue(item.deleted === (command.kind === 'delete'))
    requireValue(item.version === (command.kind === 'create' ? '1' : String(BigInt(command.target!.version) + 1n)))
    if (command.target) requireValue(item.id === command.target.id && item.groupId === command.target.groupId)
    if (command.kind !== 'delete') requireValue(sameValues(item, command.values))
  }
  return Object.freeze({ receiptId: text(row.receiptId), tenantId: id(row.tenantId), configurations: Object.freeze(configurations) })
}
export async function confirmConfigurationReceipt(session: TrustedSession, receipt: MessageConfigurationReceipt): Promise<void> {
  for (const expected of receipt.configurations) {
    try {
      const current = await getMessageConfiguration(session, expected.id)
      // Identity + exact version + complete editable fields, not clock precision,
      // bind the authoritative readback (MySQL may round sub-millisecond times).
      if (expected.deleted || current.level !== expected.level || current.version !== expected.version || !sameValues(current, { ...expected, levels: [expected.level], channels: [...expected.channels], additionalUserIds: [...expected.additionalUserIds] })) throw new ConfigurationReadbackChanged()
    } catch (error) {
      if (error instanceof CommercialApiError && error.status === 404 && error.message === 'NOTIFICATION_NOT_FOUND') {
        await requireNotificationSession(session)
        if (expected.deleted) continue
        throw new ConfigurationReadbackChanged()
      }
      throw error
    }
  }
}
export function notificationConfigurationErrorKey(error: unknown): string {
  if (error instanceof NotificationConfigurationError) return error.key
  if (error instanceof CommercialApiError) {
    if (error.status === 401) return 'signIn'
    if (error.status === 403) return 'forbidden'
    if (error.message === 'SESSION_CONTEXT_CHANGED') return 'sessionChanged'
    if (error.message === 'NOTIFICATION_DUPLICATE') return 'duplicate'
    if (error.status === 404) return 'notFound'
    if (error.status === 412) return 'channelUnavailable'
    if (error.status === 409) return 'conflict'
    if (error.status === 400 || error.status === 422) return 'invalidInput'
  }
  return 'unavailable'
}
export function configurationWriteWasRejected(error: unknown): boolean {
  return (error instanceof NotificationConfigurationError && error.beforeWrite) || (error instanceof CommercialApiError && error.code !== 'invalid-response' && ([400, 401, 403, 404, 412, 422].includes(error.status) || (error.status === 409 && ['NOTIFICATION_DUPLICATE', 'NOTIFICATION_VERSION_CONFLICT', 'NOTIFICATION_REPLAY_CONFLICT'].includes(error.message))))
}
