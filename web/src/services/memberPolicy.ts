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
  if (action === 'activate' && member.status !== 'suspended')
    return member.status === 'invited'
      ? '待激活成员必须完成激活链接或首次改密流程，不能由管理员直接启用。'
      : '仅已禁用成员可以重新启用。已移除成员请从成员回收站恢复。'
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

/** Validate the whole selection against a working copy before publishing any mutation. */
export function prepareMemberStatusBatch(
  members: Member[],
  roles: Role[],
  targets: { id: string; version: number }[],
  action: 'activate' | 'suspend',
): Member[] {
  if (!targets.length) throw new Error('请先选择成员。')
  if (new Set(targets.map((t) => t.id)).size !== targets.length) throw new Error('不能重复选择同一成员。')
  let updated = members.map((m) => ({ ...m }))
  for (const target of targets) {
    const current = updated.find((m) => m.id === target.id)
    if (!current || current.version !== target.version) throw new Error('成员状态已变化，请重新选择后重试。')
    const error = memberActionError(action, current, updated, roles)
    if (error) throw new Error(`${current.name}：${error}`)
    updated = updated.map((m) => (m.id === target.id ? applyStatusAction(action, m) : m))
  }
  return updated
}

/**
 * Validates a permission-copy operation against one consistent member snapshot.
 * Only role bindings are copied: personal profile, authentication state and data
 * scope remain owned by each target member.
 */
export function prepareMemberPermissionCopy(
  members: Member[],
  roles: Role[],
  sourceId: string,
  targets: { id: string; version: number }[],
): Member[] {
  const source = members.find((member) => member.id === sourceId)
  if (!source) throw new Error('请选择存在的权限来源成员。')
  if (source.status === 'removed') throw new Error('已移除成员不能作为权限来源。')
  if (!source.roleIds.length) throw new Error('权限来源成员没有可复制的角色。')
  if (!targets.length) throw new Error('请至少选择一名目标成员。')
  if (new Set(targets.map((target) => target.id)).size !== targets.length) {
    throw new Error('不能重复选择同一目标成员。')
  }
  if (targets.some((target) => target.id === sourceId)) {
    throw new Error('权限来源成员不能同时作为复制目标。')
  }

  const roleIds = [...new Set(source.roleIds)]
  if (roleIds.some((id) => !roles.some((role) => role.id === id && role.enabled))) {
    throw new Error('权限来源包含已停用或不存在的角色，不能复制。')
  }

  const updated = members.map((member) => ({ ...member, roleIds: [...member.roleIds] }))
  for (const target of targets) {
    const current = updated.find((member) => member.id === target.id)
    if (!current || current.version !== target.version) {
      throw new Error('目标成员状态已变化，请重新选择后再复制。')
    }
    const policy = memberActionError('role', current, updated, roles, roleIds)
    if (policy) throw new Error(`${current.name}：${policy}`)
    current.roleIds = [...roleIds]
    current.version += 1
  }
  return updated
}
