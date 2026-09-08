import { describe, it, expect } from 'vitest'
import { createSeed } from './demo/seed'
import { memberActionError, applyStatusAction } from './memberPolicy'
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
    ).toContain('重新邀请'))
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
})
