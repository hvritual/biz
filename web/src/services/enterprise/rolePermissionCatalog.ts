import type { PermissionGrant } from '@/services/runtime/api'

export type RoleGrantScope = 'none' | 'self' | 'sites' | 'all'

export type RolePermissionDefinition = {
  permission: string
  group: string
  label: string
  description: string
}

// API mode must only expose permission keys that are declared by the current
// server contracts. This list is intentionally separate from demo/seed data.
export const rolePermissionCatalog: RolePermissionDefinition[] = [
  { permission: 'tenant.member.read', group: '企业成员', label: '查看成员', description: '读取当前租户成员与成员档案。' },
  { permission: 'tenant.member.manage', group: '企业成员', label: '管理成员', description: '邀请、启停、移除及维护成员档案。' },
  { permission: 'tenant.organization.read', group: '组织架构', label: '查看组织', description: '读取当前租户部门层级、负责人和成员归属。' },
  { permission: 'tenant.organization.manage', group: '组织架构', label: '管理组织', description: '创建、修改、启停部门并维护组织关系。' },
  { permission: 'tenant.role.read', group: '角色权限', label: '查看角色', description: '读取当前租户角色与权限授权。' },
  { permission: 'tenant.role.manage', group: '角色权限', label: '管理角色', description: '创建、修改、启停角色并维护授权关系。' },
  { permission: 'tenant.delegation.read', group: '租户授权', label: '查看授权', description: '读取租户间资源授权记录。' },
  { permission: 'tenant.delegation.manage', group: '租户授权', label: '管理授权', description: '创建和撤销租户间资源授权。' },
  { permission: 'device.read', group: '设备运营', label: '查看设备', description: '读取租户设备资产。' },
  { permission: 'device.create', group: '设备运营', label: '创建设备', description: '创建设备资产。' },
  { permission: 'device.update', group: '设备运营', label: '更新设备', description: '修改或转移设备资产。' },
  { permission: 'device.delete', group: '设备运营', label: '删除设备', description: '删除设备资产。' },
  { permission: 'site.read', group: '设备运营', label: '查看点位', description: '读取设备操作依赖的点位信息。' },
  { permission: 'tenant.entitlement.read', group: '企业权益', label: '查看企业权益', description: '读取当前租户套餐权益决策。' },
  { permission: 'commercial.catalog.read', group: '企业权益', label: '读取商业目录', description: '读取权益解析依赖的商业目录。' },
]

export const roleGrantScopeOptions: Array<{ value: RoleGrantScope; label: string }> = [
  { value: 'none', label: '无数据' },
  { value: 'self', label: '仅本人' },
  { value: 'sites', label: '授权点位' },
  { value: 'all', label: '全部数据' },
]

export function roleGrantScopeLabel(scope: string) {
  return roleGrantScopeOptions.find((item) => item.value === scope)?.label ?? scope
}

export function rolePermissionLabel(permission: string) {
  return rolePermissionCatalog.find((item) => item.permission === permission)?.label ?? permission
}

export function rolePermissionGroups() {
  const groups = new Map<string, RolePermissionDefinition[]>()
  for (const item of rolePermissionCatalog) {
    const values = groups.get(item.group) ?? []
    values.push(item)
    groups.set(item.group, values)
  }
  return [...groups.entries()].map(([name, permissions]) => ({ name, permissions }))
}

export function cloneGrants(grants: PermissionGrant[] = []): PermissionGrant[] {
  return grants.map((grant) => ({ ...grant }))
}
