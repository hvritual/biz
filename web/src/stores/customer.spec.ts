import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { useCustomerStore } from './customer'
import { useEnterpriseStore } from './enterprise'
import { customerStorageKey } from '@/services/customer/repository'
beforeEach(() => {
  localStorage.clear()
  setActivePinia(createPinia())
})
describe('customer preview persistence and tenant safeguards', () => {
  it('persists and reloads customer work', () => {
    const s = useCustomerStore()
    s.run({ action: 'comment', id: 'CS-102', values: { comment: '保存测试' }, version: 1, key: 'one' })
    setActivePinia(createPinia())
    expect(useCustomerStore().snapshot.activities.some((a) => a.detail === '保存测试')).toBe(true)
  })
  it('switching tenant clears prior work changes and drafts', async () => {
    const s = useCustomerStore(),
      e = useEnterpriseStore()
    s.saveDraft('test', { name: '上海草稿' })
    e.switchTenant('hangzhou')
    await nextTick()
    expect(s.snapshot.tenant).toBe('hangzhou')
    expect(s.snapshot.drafts.test).toBeUndefined()
    e.switchTenant('shanghai')
    await nextTick()
    expect(s.snapshot.drafts.test?.name).toBe('上海草稿')
  })
  it('storage failure never publishes mutation', () => {
    const s = useCustomerStore(),
      before = JSON.stringify(s.snapshot),
      spy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new Error('quota')
      })
    expect(() =>
      s.run({ action: 'comment', id: 'CS-102', values: { comment: '不应保存' }, key: 'f' }),
    ).toThrow('存储失败')
    expect(JSON.stringify(s.snapshot)).toBe(before)
    spy.mockRestore()
  })
  it('does not silently overwrite corrupted preview data', () => {
    localStorage.setItem(customerStorageKey('shanghai'), '{bad')
    const s = useCustomerStore()
    expect(s.loadError).toContain('无法读取')
    expect(() => s.run({ action: 'read-notice', id: 'NT-01', values: {}, key: 'x' })).toThrow('无法读取')
    expect(localStorage.getItem(customerStorageKey('shanghai'))).toBe('{bad')
  })
  it('refreshes persisted versions before another tab writes', () => {
    const first = useCustomerStore()
    first.run({
      action: 'reschedule',
      id: 'CS-102',
      version: 1,
      key: 'first',
      values: { reason: '更新', nextAt: '2026-09-14T10:00', nextAction: '最新行动' },
    })
    setActivePinia(createPinia())
    const second = useCustomerStore()
    expect(() =>
      second.run({
        action: 'reschedule',
        id: 'CS-102',
        version: 1,
        key: 'stale',
        values: { reason: '旧草稿', nextAt: '2026-09-15T10:00', nextAction: '不能覆盖' },
      }),
    ).toThrow('草稿已保留')
    expect(second.snapshot.work[0]?.nextAction).toBe('最新行动')
  })
})
