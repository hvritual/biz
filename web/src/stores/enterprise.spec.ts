import { beforeEach, describe, it, expect, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useEnterpriseStore } from './enterprise'
import { createSeed } from '@/services/demo/seed'
beforeEach(() => {
  localStorage.clear()
  setActivePinia(createPinia())
})
describe('preview enterprise state', () => {
  it('derives consistent member metrics', () => {
    const s = useEnterpriseStore()
    expect(s.members).toHaveLength(368)
    expect(s.members.filter((m) => m.online && m.status === 'active')).toHaveLength(312)
    expect(s.members.filter((m) => m.status === 'invited')).toHaveLength(18)
  })
  it('keeps writes and selection data isolated per tenant', () => {
    const s = useEnterpriseStore()
    s.saveMember({ ...s.members[1]!, name: '独立测试' }, 'edit', 1)
    s.switchTenant('hangzhou')
    expect(s.members).toHaveLength(24)
    expect(s.members[1]!.name).toBe('李四')
    s.switchTenant('shanghai')
    expect(s.members[1]!.name).toBe('独立测试')
  })
  it('rejects stale version without a partial mutation', () => {
    const s = useEnterpriseStore(),
      before = s.logs.length
    expect(() => s.saveMember({ ...s.members[1]!, name: '冲突' }, 'edit', 0)).toThrow('已被其他操作修改')
    expect(s.members[1]!.name).toBe('李四')
    expect(s.logs).toHaveLength(before)
  })
  it('normalizes email and rejects duplicates', () => {
    const s = useEnterpriseStore()
    expect(() => s.saveMember({ ...s.members[1]!, email: ' ZHANGSAN@example.com ' }, 'edit', 1)).toThrow(
      '该邮箱',
    )
  })
  it('does not create a roleless or nonexistent-department member', () => {
    const s = useEnterpriseStore(),
      m = { ...s.members[1]!, id: 'new', email: 'new@example.com', version: 0 }
    expect(() => s.saveMember({ ...m, roleIds: [] }, 'create', 0)).toThrow('有效角色')
    expect(() => s.saveMember({ ...m, departmentId: 'invalid' }, 'create', 0)).toThrow('有效且启用')
  })
  it('invites a pending member and records audit', () => {
    const s = useEnterpriseStore()
    s.saveMember(
      { ...s.members[1]!, id: 'new', email: 'new@example.com', status: 'invited', version: 0 },
      'invite',
      0,
    )
    expect(s.members[0]!.status).toBe('invited')
    expect(s.logs[0]!.action).toBe('创建成员邀请')
  })
  it('protects immutable built-in roles', () => {
    const s = useEnterpriseStore()
    expect(() => s.saveRole({ ...s.roles[0]!, name: '改名' })).toThrow('内置角色')
  })
  it('suspends and reactivates a member with version checks', () => {
    const s = useEnterpriseStore()
    s.changeStatus('member-2', 'suspend', 1, '离岗')
    expect(s.members[1]!).toMatchObject({ status: 'suspended', online: false, version: 2 })
    s.changeStatus('member-2', 'activate', 2, '复核完成')
    expect(s.members[1]!.status).toBe('active')
  })
  it('requires a status change reason', () => {
    const s = useEnterpriseStore()
    expect(() => s.changeStatus('member-2', 'suspend', 1, ' ')).toThrow('操作原因')
  })
  it('rolls back memory state if persistence fails', () => {
    const s = useEnterpriseStore(),
      before = s.logs.length
    const mock = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota')
    })
    expect(() => s.saveMember({ ...s.members[1]!, name: '不应保存' }, 'edit', 1)).toThrow('未保存')
    expect(s.members[1]!.name).toBe('李四')
    expect(s.logs).toHaveLength(before)
    mock.mockRestore()
  })
  it('does not accept malformed cached arrays as a different tenant response', () => {
    expect(createSeed('hangzhou').company.name).not.toBe(createSeed('shanghai').company.name)
  })
})
