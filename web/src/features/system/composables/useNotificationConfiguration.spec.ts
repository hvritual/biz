import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { TrustedSession } from '@/services/runtime/api'
import { CommercialApiError } from '@/services/commercial/platformCommercial'
import {
  ConfigurationReadbackChanged, confirmConfigurationReceipt, getMessageConfiguration, listMessageChannels,
  listMessageConfigurations, listMessageTypes, submitConfigurationCommand, type MessageConfiguration, type MessageConfigurationReceipt, type MessagePage,
} from '@/services/enterprise/notificationConfigurationRuntime'
import { useNotificationConfiguration } from './useNotificationConfiguration'
vi.mock('@/services/enterprise/notificationConfigurationRuntime', async original => ({
  ...await original<typeof import('@/services/enterprise/notificationConfigurationRuntime')>(), listMessageChannels: vi.fn(),
  listMessageConfigurations: vi.fn(), listMessageTypes: vi.fn(), getMessageConfiguration: vi.fn(), submitConfigurationCommand: vi.fn(), confirmConfigurationReceipt: vi.fn(),
}))
const session: TrustedSession = { authenticated: true, actor_kind: 'user', user_id: 'operator', active_tenant_id: 'tenant-a', context_version: 3 }
const config = (): MessageConfiguration => ({ id: 'config-a', tenantId: 'tenant-a', groupId: 'site-a', groupName: 'A', level: 'urgent', channels: ['in_app'], primaryUserId: 'operator', secondaryUserId: '', additionalUserIds: [], notes: 'original', version: '1', createdAt: '2026-09-25T00:00:00Z', updatedAt: '2026-09-25T00:00:00Z', deleted: false })
const rows = (): MessagePage<MessageConfiguration> => ({ items: [config()], total: 1, page: 1, pageSize: 10 })
const receipt = (): MessageConfigurationReceipt => ({ receiptId: 'receipt-a', tenantId: 'tenant-a', configurations: [config()] })
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(listMessageChannels).mockResolvedValue([{ code: 'in_app', name: 'Inbox', configurable: true, unavailableReason: '' }])
  vi.mocked(listMessageConfigurations).mockResolvedValue(rows())
  vi.mocked(listMessageTypes).mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 10, groups: [{ level: 'urgent', total: 0 }, { level: 'important', total: 0 }, { level: 'general', total: 0 }] })
  vi.mocked(getMessageConfiguration).mockResolvedValue(config()); vi.mocked(submitConfigurationCommand).mockResolvedValue(receipt()); vi.mocked(confirmConfigurationReceipt).mockResolvedValue()
})
const make = (current = () => session as TrustedSession | null, allowed = () => true) => useNotificationConfiguration(current, allowed)
async function prepare() { const flow = make(); await flow.load(); await flow.startEdit(config()); return flow }
describe('enterprise message confirmation lifecycle', () => {
  it('loads server facts and cancels a draft without mutation', async () => {
    const flow = await prepare(); flow.draft.notes = 'unsaved'; flow.close()
    expect(flow.rows.value?.items[0]?.notes).toBe('original'); expect(submitConfigurationCommand).not.toHaveBeenCalled()
  })
  it('refetches a selected row before editing and honors independent action permissions', async () => {
    const flow = make(() => session, operation => operation.endsWith('.delete'))
    await flow.load(); flow.startCreate(); await flow.startEdit(config())
    expect(flow.editorOpen.value).toBe(false); expect(getMessageConfiguration).not.toHaveBeenCalled()
    await flow.startEdit(config(), 'delete'); expect(flow.deleteOpen.value).toBe(true)
  })
  it('withholds success until exact authoritative readback finishes', async () => {
    let finish!: () => void
    vi.mocked(confirmConfigurationReceipt).mockImplementation(() => new Promise<void>(resolve => { finish = resolve }))
    const flow = await prepare(), task = flow.confirm('update')
    await vi.waitFor(() => expect(confirmConfigurationReceipt).toHaveBeenCalled())
    expect(flow.outcome.value).toBe(''); expect(flow.busy.value).toBe(true)
    finish(); await task; expect(flow.outcome.value).toBe('saved'); expect(flow.editorOpen.value).toBe(false)
  })
  it('retains original command/key when response is lost, even if dialog closes', async () => {
    vi.mocked(submitConfigurationCommand).mockRejectedValueOnce(new Error('response lost')).mockResolvedValue(receipt())
    const flow = await prepare(); await flow.confirm('update')
    const command = flow.pending.value; expect(flow.recovery.value).toBe('write'); expect(flow.outcome.value).toBe('')
    flow.close(); flow.startCreate(); expect(flow.editorOpen.value).toBe(false)
    flow.draft.notes = 'must not change the captured request'; await flow.recover()
    expect(vi.mocked(submitConfigurationCommand).mock.calls[1]?.[0]).toBe(command)
    expect(command?.values.notes).toBe('original'); expect(flow.outcome.value).toBe('saved')
  })
  it('retries GET only when write is acknowledged and readback fails', async () => {
    vi.mocked(confirmConfigurationReceipt).mockRejectedValueOnce(new Error('read lost')).mockResolvedValue()
    const flow = await prepare(); await flow.confirm('update'); expect(flow.recovery.value).toBe('read')
    expect(flow.outcome.value).toBe(''); flow.close(); await flow.recover()
    expect(submitConfigurationCommand).toHaveBeenCalledTimes(1); expect(confirmConfigurationReceipt).toHaveBeenCalledTimes(2)
    expect(flow.outcome.value).toBe('saved')
  })
  it('requires reload after definite CAS failure and never overwrites newer state', async () => {
    vi.mocked(submitConfigurationCommand).mockRejectedValue(new CommercialApiError('NOTIFICATION_VERSION_CONFLICT', 409, 'conflict'))
    const flow = await prepare(); await flow.confirm('update')
    expect(flow.recovery.value).toBe(''); expect(flow.pending.value).toBeNull(); expect(flow.mustReload.value).toBe(true)
    await flow.confirm('update'); expect(submitConfigurationCommand).toHaveBeenCalledTimes(1)
    await flow.load(); expect(flow.mustReload.value).toBe(false)
  })
  it('does not turn a generic in-progress idempotency conflict into permission to start a new write', async () => {
    vi.mocked(submitConfigurationCommand).mockRejectedValue(new CommercialApiError('idempotency conflict', 409, 'conflict'))
    const flow = await prepare(); await flow.confirm('update')
    expect(flow.recovery.value).toBe('write'); expect(flow.canStart.value).toBe(false)
  })
  it('reports a later edit distinctly from failed or successful confirmation', async () => {
    vi.mocked(confirmConfigurationReceipt).mockRejectedValue(new ConfigurationReadbackChanged())
    const flow = await prepare(); await flow.confirm('update')
    expect(flow.outcome.value).toBe('changed'); expect(flow.pending.value).toBeNull(); expect(flow.recovery.value).toBe('')
  })
  it('drops late writes when tenant changes and does not carry a retry into the other enterprise', async () => {
    let resolve!: (value: MessageConfigurationReceipt) => void
    vi.mocked(submitConfigurationCommand).mockImplementation(() => new Promise(done => { resolve = done }))
    let current = session
    const flow = make(() => current); await flow.load(); await flow.startEdit(config()); const task = flow.confirm('update')
    await vi.waitFor(() => expect(submitConfigurationCommand).toHaveBeenCalled())
    current = { ...session, active_tenant_id: 'tenant-b', context_version: 4 }; flow.invalidate('sessionChanged'); resolve(receipt()); await task
    expect(flow.rows.value).toBeNull(); expect(flow.pending.value).toBeNull(); expect(flow.outcome.value).toBe(''); expect(confirmConfigurationReceipt).not.toHaveBeenCalled()
  })
  it('does not paint stale filtered results after a newer query completes', async () => {
    const flow = make(); await flow.load()
    let resolve!: (value: MessagePage<MessageConfiguration>) => void
    vi.mocked(listMessageConfigurations).mockImplementationOnce(() => new Promise(done => { resolve = done })).mockResolvedValueOnce({ ...rows(), items: [], total: 0 })
    const older = flow.loadList(); flow.query.level = 'general'; await flow.loadList(); resolve(rows()); await older
    expect(flow.rows.value?.total).toBe(0)
  })
  it('keeps a failed list distinguishable from a successful empty result', async () => {
    const flow = make(); await flow.load(); vi.mocked(listMessageConfigurations).mockRejectedValue(new Error('offline')); await flow.loadList()
    expect(flow.rows.value).toBeNull(); expect(flow.listError.value).toBe('unavailable')
  })
})
