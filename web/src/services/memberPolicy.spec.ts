import { describe, it, expect } from 'vitest'
import { createSeed } from './demo/seed'
import { memberActionError, applyStatusAction, prepareMemberPermissionCopy } from './memberPolicy'
const data = createSeed('shanghai'),
  owner = data.members[0]!,
  member = data.members[1]!
describe('membership policy', () => {
  it.each(['suspend', 'remove'] as const)('blocks last owner %s', (action) =>
    expect(memberActionError(action, owner, data.members, data.roles)).toContain('最后一位'),
  )
  it('blocks last owner demotion', () =>
    expect(memberActionError('role', owner, data.members, data.roles, ['role-2'])).toContain('最后一位'))
  it('permits owner transition after another active owner exists', () => {
    const members = [...data.members, { ...member, id: 'new-owner', roleIds: ['owner'] }]
    expect(memberActionError('suspend', owner, members, data.roles)).toBeNull()
  })
  it('does not count an invited owner as active replacement', () => {
    const members = [
      ...data.members,
      { ...member, id: 'invited-owner', roleIds: ['owner'], status: 'invited' as const },
    ]
    expect(memberActionError('remove', owner, members, data.roles)).not.toBeNull()
  })
  it('allows normal member suspension and closes presence', () => {
    expect(memberActionError('suspend', member, data.members, data.roles)).toBeNull()
    expect(applyStatusAction('suspend', member)).toMatchObject({
      status: 'suspended',
      online: false,
      version: 2,
    })
    expect(member.status).toBe('active')
  })
  it('removed membership cannot reactivate', () =>
    expect(
      memberActionError('activate', { ...member, status: 'removed' }, data.members, data.roles),
    ).toContain('回收站恢复'))
  it('rejects duplicate removal', () =>
    expect(
      memberActionError('remove', { ...member, status: 'removed' }, data.members, data.roles),
    ).not.toBeNull())
  it('requires valid enabled target roles', () => {
    expect(memberActionError('role', member, data.members, data.roles, [])).not.toBeNull()
    expect(memberActionError('role', member, data.members, data.roles, ['missing'])).not.toBeNull()
  })
  it('requires valid roles before activation', () =>
    expect(
      memberActionError(
        'activate',
        { ...member, status: 'suspended', roleIds: ['missing'] },
        data.members,
        data.roles,
      ),
    ).not.toBeNull())
  it('copies only roles after validating every selected target', () => {
    const source = data.members[1]!, target = data.members[2]!
    const copied = prepareMemberPermissionCopy(data.members, data.roles, source.id, [{ id: target.id, version: target.version }])
    const next = copied.find((member) => member.id === target.id)!
    expect(next.roleIds).toEqual(source.roleIds)
    expect(next.scope).toBe(target.scope)
    expect(next.email).toBe(target.email)
    expect(next.version).toBe(target.version + 1)
    expect(data.members.find((member) => member.id === target.id)).toEqual(target)
  })
  it('blocks unsafe or stale member permission copies before changing any target', () => {
    const source = data.members[1]!, ownerTarget = data.members[0]!, target = data.members[2]!
    expect(() => prepareMemberPermissionCopy(data.members, data.roles, source.id, [{ id: ownerTarget.id, version: ownerTarget.version }])).toThrow('最后一位')
    expect(() => prepareMemberPermissionCopy(data.members, data.roles, source.id, [{ id: target.id, version: 0 }])).toThrow('状态已变化')
    expect(() => prepareMemberPermissionCopy(data.members, data.roles, source.id, [{ id: source.id, version: source.version }])).toThrow('来源成员')
    expect(data.members.find((member) => member.id === target.id)).toEqual(target)
  })
})
