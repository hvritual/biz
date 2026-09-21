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

let liveRolePermissionCatalog: RolePermissionDefinition[] = []


function permissionPresentation(permission: string, groups: string[]) {
  const groupKey =
    permission.startsWith('tenant.member.') ? 'member'
      : permission.startsWith('tenant.organization.') ? 'organization'
        : permission.startsWith('tenant.role.') ? 'role'
          : permission.startsWith('tenant.delegation.') ? 'delegation'
            : permission.startsWith('tenant.audit.') ? 'audit'
              : permission.startsWith('tenant.profile.') || permission.startsWith('tenant.branding.') ? 'enterpriseInfo'
                : permission.startsWith('device.') || permission.startsWith('site.') ? 'deviceOperations'
                  : permission.startsWith('tenant.entitlement.') || permission.startsWith('commercial.') ? 'enterpriseEntitlements'
                    : 'uncategorized'

  return {
    group: backendTermLabel('permissionGroup', groupKey),
    label: backendTermLabel('permission', permission),
    description: groups.length
      ? groups.map((group) => backendTermLabel('permissionGroup', group)).join(' · ')
      : backendTermLabel('permission', permission),
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
        description: actions.length ? actions.map((action) => backendTermLabel('permissionAction', action)).join(' · ') : presentation.description,
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
  return backendTermLabel('dataScope', scope)
}

export function rolePermissionLabel(permission: string) {
  return liveRolePermissionCatalog.find((item) => item.permission === permission)?.label ?? backendTermLabel('permission', permission)
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
