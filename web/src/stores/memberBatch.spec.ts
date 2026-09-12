import { beforeEach, describe, it, expect, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useEnterpriseStore } from './enterprise'
import { prepareMemberStatusBatch } from '@/services/memberPolicy'
import { createSeed } from '@/services/demo/seed'
beforeEach(() => {
  localStorage.clear()
  setActivePinia(createPinia())
})
describe('member selection operations', () => {
  it('suspends selected members and records one audit event per member', () => {
    const store = useEnterpriseStore()
    store.changeStatuses(
      [
        { id: 'member-2', version: 1 },
        { id: 'member-3', version: 1 },
      ],
      'suspend',
      '暂时离岗',
    )
    expect(store.members.slice(1, 3).every((m) => m.status === 'suspended' && m.version === 2)).toBe(true)
    expect(store.logs.slice(0, 2).map((log) => log.target)).toEqual(['李四', '王五'])
    store.changeStatuses(
      [
        { id: 'member-2', version: 2 },
        { id: 'member-3', version: 2 },
      ],
      'activate',
      '重新核对身份',
    )
    expect(store.members.slice(1, 3).every((m) => m.status === 'active')).toBe(true)
  })
  it('fails the whole batch when a later record has a stale version', () => {
    const store = useEnterpriseStore(),
      before = JSON.stringify(store.members),
      logs = store.logs.length
    expect(() =>
      store.changeStatuses(
        [
          { id: 'member-2', version: 1 },
          { id: 'member-3', version: 0 },
        ],
        'suspend',
        '测试',
      ),
    ).toThrow('状态已变化')
    expect(JSON.stringify(store.members)).toBe(before)
    expect(store.logs).toHaveLength(logs)
  })
  it('refuses an owner in a batch without changing an earlier selected member', () => {
    const store = useEnterpriseStore()
    expect(() =>
      store.changeStatuses(
        [
          { id: 'member-2', version: 1 },
          { id: 'member-1', version: 1 },
        ],
        'suspend',
        '测试',
      ),
    ).toThrow('最后一位企业所有者')
    expect(store.members[1]?.status).toBe('active')
  })
  it('prevents collectively disabling all owners, not only one-at-a-time checks', () => {
    const seed = createSeed('shanghai')
    seed.members[1]!.roleIds = ['owner']
    expect(() =>
      prepareMemberStatusBatch(
        seed.members,
        seed.roles,
        [
          { id: 'member-1', version: 1 },
          { id: 'member-2', version: 1 },
        ],
        'suspend',
      ),
    ).toThrow('最后一位企业所有者')
    expect(seed.members[0]?.status).toBe('active')
  })
  it('requires a reason, nonempty unique selection and valid statuses', () => {
    const store = useEnterpriseStore()
    expect(() => store.changeStatuses([{ id: 'member-2', version: 1 }], 'suspend', '')).toThrow('原因')
    expect(() => store.changeStatuses([], 'suspend', '测试')).toThrow('选择')
    expect(() =>
      store.changeStatuses(
        [
          { id: 'member-2', version: 1 },
          { id: 'member-2', version: 1 },
        ],
        'suspend',
        '测试',
      ),
    ).toThrow('重复')
    expect(() => store.changeStatuses([{ id: 'member-2', version: 1 }], 'activate', '测试')).toThrow(
      '可以启用',
    )
  })
  it('rolls back the whole batch and its audit events on storage failure', () => {
    const store = useEnterpriseStore(),
      before = store.logs.length
    const mock = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage')
    })
    expect(() => store.changeStatuses([{ id: 'member-2', version: 1 }], 'suspend', '测试')).toThrow('未保存')
    expect(store.members[1]?.status).toBe('active')
    expect(store.logs).toHaveLength(before)
    mock.mockRestore()
  })
})
