import type { PermissionGrant } from '@/services/runtime/api'
import { backendTermLabel } from '@/i18n/backend-terms'

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

type CachedRolePermissionDefinition = {
  permission: string
  groupKey: string
  groups: string[]
  actions: string[]
}

let liveRolePermissionCatalog: CachedRolePermissionDefinition[] = []


function permissionGroupKey(permission: string) {
  return permission.startsWith('tenant.member.') ? 'member'
    : permission.startsWith('tenant.organization.') ? 'organization'
      : permission.startsWith('tenant.role.') ? 'role'
        : permission.startsWith('tenant.delegation.') ? 'delegation'
          : permission.startsWith('tenant.audit.') ? 'audit'
            : permission.startsWith('tenant.profile.') || permission.startsWith('tenant.branding.') ? 'enterpriseInfo'
              : permission.startsWith('device.') || permission.startsWith('site.') ? 'deviceOperations'
                : permission.startsWith('tenant.entitlement.') || permission.startsWith('commercial.') ? 'enterpriseEntitlements'
                  : 'uncategorized'
}

function permissionPresentation(item: CachedRolePermissionDefinition): RolePermissionDefinition {
  const label = backendTermLabel('permission', item.permission)
  const description = item.actions.length
    ? item.actions.map((action) => backendTermLabel('permissionAction', action)).join(' · ')
    : item.groups.length
      ? item.groups.map((group) => backendTermLabel('permissionGroup', group)).join(' · ')
      : label
  return {
    permission: item.permission,
    group: backendTermLabel('permissionGroup', item.groupKey),
    label,
    description,
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
    .map((item) => ({
      permission: item.permission.trim(),
      groupKey: permissionGroupKey(item.permission.trim()),
      groups: [...new Set(item.groups.map((value) => value.trim()).filter(Boolean))].sort(),
      actions: [...new Set(item.actions.map((value) => value.trim()).filter(Boolean))].sort(),
    }))
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
  return backendTermLabel('dataScope', scope)
}

export function rolePermissionLabel(permission: string) {
  return backendTermLabel('permission', permission)
}

export function rolePermissionGroups() {
  const groups = new Map<string, RolePermissionDefinition[]>()
  for (const cached of liveRolePermissionCatalog) {
    const item = permissionPresentation(cached)
    const values = groups.get(item.group) ?? []
    values.push(item)
    groups.set(item.group, values)
  }
  return [...groups.entries()].map(([name, permissions]) => ({ name, permissions }))
}

export function cloneGrants(grants: PermissionGrant[] = []): PermissionGrant[] {
  return grants.map((grant) => ({ ...grant }))
}
