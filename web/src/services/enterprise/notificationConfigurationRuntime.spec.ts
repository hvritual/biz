import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CommercialApiError, mutate, request } from '@/services/commercial/platformCommercial'
import { readSession, type TrustedSession } from '@/services/runtime/api'
import {
  ConfigurationReadbackChanged, NotificationConfigurationError, configurationWriteWasRejected,
  confirmConfigurationReceipt, listMessageChannels, listMessageConfigurations, listMessageDirectory, listMessageTypes,
  notificationConfigurationErrorKey, parseMessageConfiguration, prepareConfigurationCommand, submitConfigurationCommand,
  type ConfigurationDraft, type MessageConfigurationReceipt,
} from './notificationConfigurationRuntime'
vi.mock('@/services/commercial/platformCommercial', async original => ({
  ...await original<typeof import('@/services/commercial/platformCommercial')>(), request: vi.fn(), mutate: vi.fn(),
}))
vi.mock('@/services/runtime/api', async original => ({ ...await original<typeof import('@/services/runtime/api')>(), readSession: vi.fn() }))
const session: TrustedSession = { authenticated: true, actor_kind: 'user', user_id: 'operator', active_tenant_id: 'tenant-a', context_version: 3 }
const draft = (): ConfigurationDraft => ({ groupId: 'site-a', levels: ['urgent'], channels: ['in_app'], primaryUserId: 'operator', secondaryUserId: '', additionalUserIds: ['reader'], notes: 'Confirmed rule' })
const channels = [{ code: 'in_app', name: 'Inbox', configurable: true, unavailableReason: '' }, { code: 'sms', name: 'SMS', configurable: false, unavailableReason: 'not ready' }]
const wire = () => ({ id: 'config-a', tenantId: 'tenant-a', groupId: 'site-a', groupName: 'Site A', level: 'urgent', channels: ['in_app'], primaryUserId: 'operator', secondaryUserId: '', additionalUserIds: ['reader'], notes: 'Confirmed rule', version: '1', createdAt: '2026-09-25T01:00:00Z', updatedAt: '2026-09-25T01:00:00Z', deleted: false })
const receipt = (): MessageConfigurationReceipt => ({ tenantId: 'tenant-a', receiptId: 'receipt-a', configurations: [parseMessageConfiguration(session, wire())] })
beforeEach(() => { vi.resetAllMocks(); vi.mocked(readSession).mockResolvedValue(session) })
describe('enterprise notification configuration runtime contract', () => {
  it('keeps versions exact beyond JavaScript safe integers and copies nested collections', () => {
    const input = { ...wire(), version: '9007199254740993' }, value = parseMessageConfiguration(session, input)
    input.channels.push('sms'); input.additionalUserIds.push('other')
    expect(value.version).toBe('9007199254740993'); expect(value.channels).toEqual(['in_app']); expect(Object.isFrozen(value.additionalUserIds)).toBe(true)
  })
  it('rejects cross-tenant, corrupt and ambiguous configuration responses', () => {
    for (const value of [null, {}, { ...wire(), tenantId: 'tenant-b' }, { ...wire(), version: 1 }, { ...wire(), version: '18446744073709551616' },
      { ...wire(), channels: [] }, { ...wire(), channels: null }, { ...wire(), additionalUserIds: ['operator'] }, { ...wire(), secondaryUserId: 'operator' }, { ...wire(), deleted: 'false' }]) {
      expect(() => parseMessageConfiguration(session, value)).toThrow('invalidResponse')
    }
  })
  it('pins read requests and rejects stale sessions before any request', async () => {
    vi.mocked(readSession).mockResolvedValue({ ...session, active_tenant_id: 'tenant-b' })
    await expect(listMessageChannels(session)).rejects.toThrow('sessionChanged'); expect(request).not.toHaveBeenCalled()
    vi.mocked(readSession).mockResolvedValue(session)
    vi.mocked(request).mockResolvedValue({ tenantId: 'tenant-a', items: channels })
    await listMessageChannels(session)
    expect(request).toHaveBeenCalledWith('/v1/tenant/notification/channels', { headers: { 'X-Biz-Session-Context': expect.stringContaining('tenant-a') } })
  })
  it('checks tenant identity, page echo and full filtered counts, not just visible items', async () => {
    vi.mocked(request).mockResolvedValue({ tenantId: 'tenant-a', items: [wire()], total: '23', page: 2, pageSize: 10 })
    expect((await listMessageConfigurations(session, { page: 2, pageSize: 10 })).total).toBe(23)
    vi.mocked(request).mockResolvedValue({ tenantId: 'tenant-a', total: '1', page: 1, pageSize: 10 })
    await expect(listMessageConfigurations(session, { page: 2, pageSize: 10 })).rejects.toThrow('invalidResponse')
    vi.mocked(request).mockRejectedValue(new Error('down')); await expect(listMessageConfigurations(session, { page: 1, pageSize: 10 })).rejects.toThrow('down')
  })
  it('does not silently truncate directory pages or treat wildcards as another protocol', async () => {
    vi.mocked(request).mockResolvedValue({ tenantId: 'tenant-a', items: [{ id: 'site-103', name: 'Site 103' }], total: '103', page: 6, pageSize: 20 })
    const result = await listMessageDirectory(session, 'groups', '100%_', 6, 20)
    expect(result.total).toBe(103); expect(request).toHaveBeenCalledWith(expect.stringContaining('page=6&page_size=20'), expect.anything())
    expect(request).toHaveBeenCalledWith(expect.stringContaining('query=100%25_'), expect.anything())
  })
  it('requires three consistent level counts and sanitized channel availability', async () => {
    vi.mocked(request).mockResolvedValue({ tenantId: 'tenant-a', total: '0', page: 1, pageSize: 10, groups: [{ level: 'urgent' }, { level: 'important' }, { level: 'general' }] })
    expect((await listMessageTypes(session, { page: 1, pageSize: 10 })).groups).toHaveLength(3)
    vi.mocked(request).mockResolvedValue({ tenantId: 'tenant-a', items: [{ code: 'sms', name: 'SMS', configurable: 'false' }] })
    await expect(listMessageChannels(session)).rejects.toThrow('invalidResponse')
  })
  it('freezes the complete original command and transmits neither tenant nor actor in the body', async () => {
    const input = draft(), command = prepareConfigurationCommand('create', session, input, null, channels)
    input.notes = 'changed after click'; input.levels.push('general')
    vi.mocked(mutate).mockResolvedValue(receipt())
    await submitConfigurationCommand(command); await submitConfigurationCommand(command)
    expect(vi.mocked(mutate).mock.calls[0]).toEqual(vi.mocked(mutate).mock.calls[1])
    expect(command.values.notes).toBe('Confirmed rule'); expect(command.values.levels).toEqual(['urgent'])
    expect(command.body).not.toHaveProperty('tenantId'); expect(command.body).not.toHaveProperty('userId')
    expect(Object.isFrozen(command.body)).toBe(true); expect(Object.isFrozen(command.values.channels)).toBe(true)
  })
  it('validates complete contacts and rejects unavailable channels before a write', () => {
    for (const input of [{ ...draft(), primaryUserId: '' }, { ...draft(), secondaryUserId: 'operator' }, { ...draft(), additionalUserIds: ['reader', 'reader'] }, { ...draft(), levels: ['urgent', 'urgent'] }, { ...draft(), channels: ['sms'] }]) {
      expect(() => prepareConfigurationCommand('create', session, input, null, channels)).toThrow()
    }
    expect(mutate).not.toHaveBeenCalled()
  })
  it('binds receipts to expected operation, identities, levels, values and exact next versions', async () => {
    const command = prepareConfigurationCommand('create', session, draft(), null, channels)
    for (const value of [{ ...receipt(), tenantId: 'tenant-b' }, { ...receipt(), configurations: [] }, { ...receipt(), configurations: [{ ...wire(), notes: 'wrong' }] }, { ...receipt(), configurations: [{ ...wire(), version: '2' }] }]) {
      vi.mocked(mutate).mockResolvedValue(value); await expect(submitConfigurationCommand(command)).rejects.toThrow('invalidResponse')
    }
  })
  it('confirms saved state using reads only and treats later changes as a distinct result', async () => {
    vi.mocked(request).mockResolvedValue(wire()); await confirmConfigurationReceipt(session, receipt())
    expect(mutate).not.toHaveBeenCalled()
    vi.mocked(request).mockResolvedValue({ ...wire(), version: '2' }); await expect(confirmConfigurationReceipt(session, receipt())).rejects.toBeInstanceOf(ConfigurationReadbackChanged)
  })
  it('accepts deletion only with a matching deletion receipt and exact application not-found readback', async () => {
    const command = prepareConfigurationCommand('delete', session, draft(), parseMessageConfiguration(session, wire()), channels)
    vi.mocked(mutate).mockResolvedValue({ ...receipt(), configurations: [{ ...wire(), deleted: true, version: '2' }] })
    const accepted = await submitConfigurationCommand(command)
    vi.mocked(request).mockRejectedValue(new CommercialApiError('NOTIFICATION_NOT_FOUND', 404, 'http'))
    await confirmConfigurationReceipt(session, accepted)
    vi.mocked(request).mockRejectedValue(new CommercialApiError('unknown proxy response', 404, 'http'))
    await expect(confirmConfigurationReceipt(session, accepted)).rejects.toThrow('unknown proxy response')
  })
  it('keeps generic idempotency conflicts uncertain and never renders raw error content', () => {
    expect(configurationWriteWasRejected(new CommercialApiError('NOTIFICATION_VERSION_CONFLICT', 409, 'conflict'))).toBe(true)
    expect(configurationWriteWasRejected(new CommercialApiError('idempotency conflict', 409, 'conflict'))).toBe(false)
    expect(configurationWriteWasRejected(new CommercialApiError('bad JSON', 200, 'invalid-response'))).toBe(false)
    expect(configurationWriteWasRejected(new NotificationConfigurationError('invalidInput', true))).toBe(true)
    expect(notificationConfigurationErrorKey(new Error('credential@example.invalid'))).toBe('unavailable')
  })
})
