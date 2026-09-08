import type { Member, MemberAction, Role } from '@/types/enterprise'
export function memberActionError(
  action: MemberAction,
  member: Member,
  members: Member[],
  roles: Role[],
  targetRoles?: string[],
): string | null {
  const owners = members.filter((m) => m.status === 'active' && m.roleIds.includes('owner'))
  const lastOwner = owners.length === 1 && owners[0]?.id === member.id
  if (
    lastOwner &&
    (['suspend', 'remove'].includes(action) || (action === 'role' && !targetRoles?.includes('owner')))
  )
    return '不能停用、移除或降权最后一位企业所有者。请先完成所有者交接。'
  if (action === 'activate' && member.status !== 'suspended' && member.status !== 'invited')
    return '仅待激活或已禁用成员可以启用。已移除成员需要重新邀请。'
  if (
    action === 'activate' &&
    (!member.roleIds.length || member.roleIds.some((id) => !roles.some((r) => r.id === id && r.enabled)))
  )
    return '启用前请重新分配有效角色。'
  if (action === 'remove' && member.status === 'removed') return '该成员关系已经移除，不能重复执行。'
  if (action === 'suspend' && member.status !== 'active') return '仅启用状态的成员可以禁用。'
  if (member.status === 'removed' && ['edit', 'role', 'reset', 'suspend'].includes(action))
    return '已移除成员保留只读历史记录，不能直接恢复权限。'
  if (
    action === 'role' &&
    (!targetRoles?.length || targetRoles.some((id) => !roles.some((r) => r.id === id && r.enabled)))
  )
    return '请选择至少一个已启用的有效角色。'
  return null
}
export function applyStatusAction(action: 'activate' | 'suspend' | 'remove', member: Member): Member {
  return {
    ...member,
    status: action === 'activate' ? 'active' : action === 'suspend' ? 'suspended' : 'removed',
    online: false,
    version: member.version + 1,
  }
}
