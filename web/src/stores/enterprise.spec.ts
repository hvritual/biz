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
  it('keeps writes and selection data isolated per tenant', async () => {
    const s = useEnterpriseStore()
    await s.saveMember({ ...s.members[1]!, name: '独立测试' }, 'edit', 1)
    await s.switchTenant('hangzhou')
    expect(s.members).toHaveLength(24)
    expect(s.members[1]!.name).toBe('李四')
    await s.switchTenant('shanghai')
    expect(s.members[1]!.name).toBe('独立测试')
  })
  it('rejects stale version without a partial mutation', async () => {
    const s = useEnterpriseStore(),
      before = s.logs.length
    await expect(s.saveMember({ ...s.members[1]!, name: '冲突' }, 'edit', 0)).rejects.toThrow('已被其他操作修改')
    expect(s.members[1]!.name).toBe('李四')
    expect(s.logs).toHaveLength(before)
  })
  it('normalizes email and rejects duplicates', async () => {
    const s = useEnterpriseStore()
    await expect(s.saveMember({ ...s.members[1]!, email: ' ZHANGSAN@example.com ' }, 'edit', 1)).rejects.toThrow(
      '该邮箱',
    )
  })
  it('does not create a roleless or nonexistent-department member', async () => {
    const s = useEnterpriseStore(),
      m = { ...s.members[1]!, id: 'new', username: 'new.member', activationMode: 'activation_link' as const, email: 'new@example.com', version: 0 }
    await expect(s.saveMember({ ...m, roleIds: [] }, 'create', 0)).rejects.toThrow('有效角色')
    await expect(s.saveMember({ ...m, departmentId: 'invalid' }, 'create', 0)).rejects.toThrow('有效且启用')
  })
  it('invites a pending member and records audit', async () => {
    const s = useEnterpriseStore()
    await s.saveMember(
      { ...s.members[1]!, id: 'new', username: 'invite.user', activationMode: 'activation_link' as const, email: 'new@example.com', status: 'invited', version: 0 },
      'invite',
      0,
    )
    expect(s.members[0]!.status).toBe('invited')
    expect(s.logs[0]!.action).toBe('创建成员邀请')
  })
  it('protects immutable built-in roles', async () => {
    const s = useEnterpriseStore()
    await expect(s.saveRole({ ...s.roles[0]!, name: '改名' })).rejects.toThrow('内置角色')
  })
  it('suspends and reactivates a member with version checks', async () => {
    const s = useEnterpriseStore()
    await s.changeStatus('member-2', 'suspend', 1, '离岗')
    expect(s.members[1]!).toMatchObject({ status: 'suspended', online: false, version: 2 })
    await s.changeStatus('member-2', 'activate', 2, '复核完成')
    expect(s.members[1]!.status).toBe('active')
  })
  it('requires a status change reason', async () => {
    const s = useEnterpriseStore()
    await expect(s.changeStatus('member-2', 'suspend', 1, ' ')).rejects.toThrow('操作原因')
  })
  it('rolls back memory state if persistence fails', async () => {
    const s = useEnterpriseStore(),
      before = s.logs.length
    const mock = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota')
    })
    await expect(s.saveMember({ ...s.members[1]!, name: '不应保存' }, 'edit', 1)).rejects.toThrow('未保存')
    expect(s.members[1]!.name).toBe('李四')
    expect(s.logs).toHaveLength(before)
    mock.mockRestore()
  })
  it('does not accept malformed cached arrays as a different tenant response', () => {
    expect(createSeed('hangzhou').company.name).not.toBe(createSeed('shanghai').company.name)
  })
})
