import {
  createRuntimeApi,
  type Device,
  type Tenant,
  type TenantMember,
  type TenantRole,
  type PermissionGrant,
  type TrustedSession,
} from './api'
export type Resource = 'tenants' | 'members' | 'roles' | 'devices'
export type RuntimeRow = Tenant | TenantMember | TenantRole | Device
export interface Field {
  key: string
  label: string
  required?: boolean
}
export const labels: Record<Resource, string> = {
  tenants: '租户管理',
  members: '业务成员',
  roles: '业务角色',
  devices: '业务设备',
}
export const actions: Record<Resource, Record<string, string>> = {
  tenants: { create: '创建租户', rename: '修改名称', activate: '启用', suspend: '停用', close: '关闭租户' },
  members: { create: '邀请成员', activate: '启用', suspend: '停用', remove: '移除' },
  roles: {
    create: '创建角色',
    rename: '修改名称',
    enable: '启用',
    disable: '停用',
    permissions: '权限范围',
    assign: '分配成员',
    revoke: '撤销成员',
  },
  devices: { create: '创建设备', rename: '修改设备', transfer: '迁移设备', delete: '删除设备' },
}
export function rowId(row: RuntimeRow) {
  return 'userId' in row ? row.userId : row.id
}
export function rowName(row: RuntimeRow) {
  return 'name' in row ? row.name : row.email
}
export function rowStatus(row: RuntimeRow) {
  return 'status' in row ? row.status : row.siteId
}
export function fields(resource: Resource, action: string): Field[] {
  const field = (key: string, label: string): Field => ({ key, label, required: true })
  if (action === 'create') {
    if (resource === 'tenants')
      return [
        field('name', '租户名称'),
        field('ownerUserId', '所有者用户编号'),
        field('ownerEmail', '所有者邮箱'),
        field('salesScope', '适用范围'),
      ]
    if (resource === 'members') return [field('email', '成员邮箱')]
    if (resource === 'devices')
      return [field('name', '设备名称'), field('serial', '序列号'), field('siteId', '点位编号')]
    return [field('name', '角色名称')]
  }
  if (action === 'rename')
    return resource === 'devices'
      ? [field('name', '设备名称'), field('siteId', '点位编号')]
      : [field('name', '名称')]
  if (action === 'transfer') return [field('targetSiteId', '目标点位编号')]
  if (action === 'assign' || action === 'revoke') return [field('userId', '成员用户编号')]
  return []
}
export async function loadRows(resource: Resource, session?: TrustedSession): Promise<RuntimeRow[]> {
  const { platformApi, tenantApi } = createRuntimeApi('read', session)
  if (resource === 'tenants') return platformApi.listTenants()
  if (resource === 'members') return tenantApi.listMembers()
  if (resource === 'roles') return tenantApi.listRoles()
  return tenantApi.listDevices()
}
export async function executeCommand(
  resource: Resource,
  action: string,
  row: RuntimeRow | null,
  input: Record<string, string>,
  permissions: PermissionGrant[],
  key: string,
  session?: TrustedSession,
) {
  const { platformApi: p, tenantApi: t } = createRuntimeApi(key, session)
  const value = (name: string) => input[name]?.trim() ?? ''
  for (const f of fields(resource, action))
    if (f.required && !value(f.key)) throw new Error(`请填写${f.label}`)
  if (action !== 'create' && !row) throw new Error('请重新选择记录')
  if (resource === 'tenants') {
    const tenant = row as Tenant
    if (action === 'create')
      return p.createTenant({
        name: value('name'),
        ownerEmail: value('ownerEmail'),
        ownerUserId: value('ownerUserId'),
        salesScope: value('salesScope'),
        requestId: key,
      })
    if (action === 'rename') return p.updateTenant(tenant, value('name'))
    if (action === 'activate') return p.activateTenant(tenant)
    if (action === 'suspend') return p.suspendTenant(tenant)
    if (action === 'close') return p.closeTenant(tenant)
  }
  if (resource === 'members') {
    const member = row as TenantMember
    if (action === 'create') return t.inviteMember(value('email'))
    if (action === 'activate') return t.activateMember(member)
    if (action === 'suspend') return t.suspendMember(member)
    if (action === 'remove') return t.removeMember(member)
  }
  if (resource === 'roles') {
    const role = row as TenantRole
    if (action === 'create') return t.createRole(value('name'))
    if (action === 'rename') return t.updateRole(role, value('name'))
    if (action === 'enable') return t.enableRole(role)
    if (action === 'disable') return t.disableRole(role)
    if (action === 'permissions') return t.setRolePermissions(role, permissions)
    if (action === 'assign') return t.assignRoleMember(role.id, value('userId'))
    if (action === 'revoke') return t.revokeRoleMember(role.id, value('userId'))
  }
  if (resource === 'devices') {
    const device = row as Device
    if (action === 'create')
      return t.createDevice({ name: value('name'), serial: value('serial'), siteId: value('siteId') })
    if (action === 'rename') return t.updateDevice(device, { name: value('name'), siteId: value('siteId') })
    if (action === 'transfer') return t.transferDevice(device, value('targetSiteId'))
    if (action === 'delete') return t.deleteDevice(device)
  }
  throw new Error('不支持的操作')
}
