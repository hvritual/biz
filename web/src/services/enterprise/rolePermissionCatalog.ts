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
      return {
        permission: item.permission.trim(),
        group: groups[0] ?? 'unclassified',
        label: item.permission.trim(),
        description: actions.length ? actions.join(' · ') : item.permission.trim(),
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
