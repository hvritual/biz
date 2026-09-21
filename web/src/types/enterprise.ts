export type MemberStatus = 'active' | 'invited' | 'suspended' | 'removed'
export type DataScope = 'all' | 'department' | 'department_tree' | 'self' | 'custom'
export type MemberActivationMode = '' | 'activation_link' | 'sms_initial_password'
export interface Member {
  id: string
  username?: string
  activationMode?: MemberActivationMode
  name: string
  email: string
  phone: string
  employeeId: string
  departmentId: string
  position: string
  roleIds: string[]
  scope: DataScope
  status: MemberStatus
  online: boolean
  joinedAt: string
  lastLogin: string | null
  version: number
  runtimeVersion?: string | number
  mfa: boolean
  note: string
}
export interface Role {
  id: string
  name: string
  description: string
  roleCode?: string
  builtin: boolean
  enabled: boolean
  scope: DataScope
  permissions: string[]
  memberCount?: number
  updatedAt: string
  runtimeVersion?: string | number
}
export interface Department {
  id: string
  name: string
  parentId: string | null
  leaderId: string
  code: string
  description: string
  enabled: boolean
  email?: string
  phone?: string
  sort?: number
  runtimeVersion?: string | number
}
export interface Company {
  name: string
  shortName: string
  email: string
  phone: string
  contact: string
  industry: string
  size: string
  timezone: string
  description: string
  address: string
  logoAssetRef?: string
  runtimeVersion?: string | number
}
export interface AuditRecord {
  id: string
  time: string
  actor: string
  module: string
  action: string
  target: string
  result: 'success' | 'failure'
  risk: 'low' | 'medium' | 'high'
  requestId: string
  before: string
  after: string
  reason: string
}
export interface TenantSnapshot {
  members: Member[]
  roles: Role[]
  departments: Department[]
  company: Company
  logs: AuditRecord[]
  settings: Record<string, string | boolean | number>
}
export type MemberAction = 'create' | 'invite' | 'edit' | 'role' | 'reset' | 'suspend' | 'activate' | 'remove'
export const statusLabels: Record<MemberStatus, string> = {
  active: '启用',
  invited: '待激活',
  suspended: '已禁用',
  removed: '已移除',
}
export const scopeLabels: Record<DataScope, string> = {
  all: '全部数据',
  department: '所属部门',
  department_tree: '部门及下级',
  self: '仅本人',
  custom: '指定数据',
}
