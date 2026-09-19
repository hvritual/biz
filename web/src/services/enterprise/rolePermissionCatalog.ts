import type { PermissionGrant } from '@/services/runtime/api'

export type RoleGrantScope = 'none' | 'self' | 'sites' | 'all'

export type ServerPermissionDefinition = {
  permission: string
  groups: string[]
  actions: string[]
}

export type RolePermissionDefinition = {
  permission: string
  group: string
  label: string
  description: string
}

let liveRolePermissionCatalog: RolePermissionDefinition[] = []


function permissionPresentation(permission: string, groups: string[]) {
  const group =
    permission.startsWith('tenant.member.') ? '企业成员'
      : permission.startsWith('tenant.organization.') ? '组织架构'
        : permission.startsWith('tenant.role.') ? '角色权限'
          : permission.startsWith('tenant.delegation.') ? '租户授权'
            : permission.startsWith('tenant.audit.') ? '操作日志'
              : permission.startsWith('tenant.profile.') || permission.startsWith('tenant.branding.') ? '企业信息'
                : permission.startsWith('device.') || permission.startsWith('site.') ? '设备运营'
                  : permission.startsWith('tenant.entitlement.') || permission.startsWith('commercial.') ? '企业权益'
                    : (groups[0] ?? '未分类')

  const labels: Record<string, string> = {
    'tenant.member.read': '查看成员',
    'tenant.member.manage': '管理成员',
    'tenant.organization.read': '查看组织',
    'tenant.organization.manage': '管理组织',
    'tenant.role.read': '查看角色',
    'tenant.role.manage': '管理角色',
    'tenant.delegation.read': '查看授权',
    'tenant.delegation.manage': '管理授权',
    'tenant.audit.read': '查看日志',
    'tenant.audit.export': '导出日志',
    'tenant.profile.read': '查看企业信息',
    'tenant.profile.manage': '管理企业信息',
    'tenant.branding.read': '查看品牌',
    'tenant.branding.manage': '管理品牌',
    'device.read': '查看设备',
    'device.create': '创建设备',
    'device.update': '更新设备',
    'device.delete': '删除设备',
    'site.read': '查看点位',
    'tenant.entitlement.read': '查看企业权益',
    'commercial.catalog.read': '读取商业目录',
  }

  return {
    group,
    label: labels[permission] ?? permission,
  }
}

export function replaceRolePermissionCatalog(definitions: ServerPermissionDefinition[]) {
  const seen = new Set<string>()
  liveRolePermissionCatalog = definitions
    .filter((item) => {
      const permission = item.permission.trim()
      if (!permission || seen.has(permission)) return false
      seen.add(permission)
      return true
    })
    .map((item) => {
      const groups = [...new Set(item.groups.map((value) => value.trim()).filter(Boolean))].sort()
      const actions = [...new Set(item.actions.map((value) => value.trim()).filter(Boolean))].sort()
      const permission = item.permission.trim()
      const presentation = permissionPresentation(permission, groups)
      return {
        permission,
        group: presentation.group,
        label: presentation.label,
        // Presentation metadata never grants authority: action codes and permission
        // membership still come exclusively from the server Action Catalog.
        description: actions.length ? actions.join(' · ') : permission,
      }
    })
    .sort((left, right) => left.permission.localeCompare(right.permission))
}

export function clearRolePermissionCatalog() {
  liveRolePermissionCatalog = []
}

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
  return liveRolePermissionCatalog.find((item) => item.permission === permission)?.label ?? permission
}

export function rolePermissionGroups() {
  const groups = new Map<string, RolePermissionDefinition[]>()
  for (const item of liveRolePermissionCatalog) {
    const values = groups.get(item.group) ?? []
    values.push(item)
    groups.set(item.group, values)
  }
  return [...groups.entries()].map(([name, permissions]) => ({ name, permissions }))
}

export function cloneGrants(grants: PermissionGrant[] = []): PermissionGrant[] {
  return grants.map((grant) => ({ ...grant }))
}
