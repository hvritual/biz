import { loadSnapshot, saveSnapshot } from '@/services/demo/repository'
import { listEnterpriseMembers, readEnterpriseMemberSession } from '@/services/enterprise/memberRuntime'
import { listEnterpriseRoles } from '@/services/enterprise/roleRuntime'
import { listEnterpriseDepartments } from '@/services/enterprise/departmentRuntime'
import { getEnterpriseTenantProfile } from '@/services/enterprise/tenantProfileRuntime'
import { selectSessionTenant, type TrustedSession } from '@/services/runtime/api'
import type { Company, DataScope, Department, Member, MemberStatus, Role, TenantSnapshot } from '@/types/enterprise'
import type { EnterpriseTenantMember } from '@/services/enterprise/memberRuntime'
import type { EnterpriseTenantRole } from '@/services/enterprise/roleRuntime'
import type { EnterpriseDepartment } from '@/services/enterprise/departmentRuntime'
import type { EnterpriseTenantProfile } from '@/services/enterprise/tenantProfileRuntime'

export type EnterpriseSourceKind = 'demo' | 'api'
export type EnterpriseDomain = 'members' | 'roles' | 'departments' | 'company'

export type EnterpriseSourceState = {
  tenantId: string
  snapshot: TenantSnapshot
  session: TrustedSession | null
  loadedDomains: EnterpriseDomain[]
}

export type EnterpriseDataSource = {
  kind: EnterpriseSourceKind
  initial(): EnterpriseSourceState
  load(tenantId?: string, domains?: EnterpriseDomain[]): Promise<EnterpriseSourceState>
  switchTenant(tenantId: string, domains?: EnterpriseDomain[]): Promise<EnterpriseSourceState>
  persist(tenantId: string, snapshot: TenantSnapshot): void
}

const emptyCompany = (): Company => ({
  name: '',
  shortName: '',
  email: '',
  phone: '',
  contact: '',
  industry: '',
  size: '',
  timezone: 'Asia/Shanghai',
  description: '',
  address: '',
})

export function emptyEnterpriseSnapshot(): TenantSnapshot {
  return { members: [], roles: [], departments: [], company: emptyCompany(), logs: [], settings: {} }
}

function numericVersion(value: string | number | undefined) {
  const version = Number(value ?? 0)
  return Number.isFinite(version) ? version : 0
}

function memberStatus(status: string): MemberStatus {
  switch (status) {
    case 'TENANT_MEMBER_STATUS_ACTIVE': return 'active'
    case 'TENANT_MEMBER_STATUS_INVITED': return 'invited'
    case 'TENANT_MEMBER_STATUS_SUSPENDED': return 'suspended'
    case 'TENANT_MEMBER_STATUS_REMOVED': return 'removed'
    default: return 'invited'
  }
}

function viewScope(scope: string): DataScope {
  switch (scope) {
    case 'all':
    case 'DATA_SCOPE_ALL': return 'all'
    case 'self':
    case 'DATA_SCOPE_SELF': return 'self'
    case 'sites':
    case 'DATA_SCOPE_SITES': return 'custom'
    default: return 'custom'
  }
}

function strongestRoleScope(role: EnterpriseTenantRole): DataScope {
  const scopes = role.permissions.map((grant) => viewScope(grant.scope))
  if (scopes.includes('all')) return 'all'
  if (scopes.includes('custom')) return 'custom'
  if (scopes.includes('self')) return 'self'
  return 'custom'
}

export function projectMember(value: EnterpriseTenantMember): Member {
  return {
    id: value.userId,
    username: value.username,
    activationMode: '',
    name: value.name || value.username || value.email,
    email: value.email,
    phone: value.phone,
    employeeId: value.employeeId,
    departmentId: value.departmentId,
    position: value.position,
    roleIds: value.roles.map((role) => role.roleId),
    scope: viewScope(value.derivedDataScope),
    status: memberStatus(value.status),
    online: false,
    joinedAt: '',
    lastLogin: null,
    version: numericVersion(value.version),
    runtimeVersion: value.version,
    mfa: false,
    note: '',
  }
}

export function projectRole(value: EnterpriseTenantRole): Role {
  const displayName =
    value.roleCode === 'tenant_owner'
      ? '企业所有者'
      : value.roleCode === 'tenant_admin'
        ? '企业管理员'
        : value.name
  return {
    id: value.id,
    name: displayName,
    description: value.description,
    roleCode: value.roleCode,
    builtin: value.protectedSystem,
    enabled: value.status === 'TENANT_ROLE_STATUS_ACTIVE',
    scope: strongestRoleScope(value),
    permissions: value.permissions.map((grant) => grant.permission),
    memberCount: value.memberCount,
    dataPolicy: value.dataPolicy ? {
      policyId: value.dataPolicy.policyId,
      policyName: value.dataPolicy.policyName,
      policyVersion: value.dataPolicy.policyVersion,
      acceptedVersion: value.dataPolicy.acceptedVersion,
      effective: value.dataPolicy.effective,
      invalidReason: value.dataPolicy.invalidReason,
    } : undefined,
    updatedAt: '',
    runtimeVersion: value.version,
  }
}

export function projectDepartment(value: EnterpriseDepartment): Department {
  return {
    id: value.departmentId,
    name: value.name,
    parentId: value.parentId || null,
    leaderId: value.leaderUserId,
    code: '',
    description: '',
    enabled: value.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE',
    email: value.email,
    phone: value.phone,
    sort: value.sort,
    runtimeVersion: value.version,
  }
}

export function projectCompany(value: EnterpriseTenantProfile): Company {
  return {
    name: value.name,
    shortName: value.shortName,
    email: value.email,
    phone: value.phone,
    contact: value.contactName,
    industry: value.industry,
    size: value.companySize,
    timezone: value.timezone,
    description: value.description,
    address: value.address,
    logoAssetRef: value.logoAssetRef,
    runtimeVersion: value.version,
  }
}

async function loadApiState(session?: TrustedSession, domains: EnterpriseDomain[] = []): Promise<EnterpriseSourceState> {
  const current = session ?? (await readEnterpriseMemberSession())
  const tenantId = current.active_tenant_id ?? ''
  const snapshot = emptyEnterpriseSnapshot()
  if (current.active_tenant_timezone) snapshot.company.timezone = current.active_tenant_timezone
  if (!current.authenticated || !tenantId) return { tenantId, snapshot, session: current, loadedDomains: [] }

  await Promise.all(domains.map(async (domain) => {
    if (domain === 'members') snapshot.members = (await listEnterpriseMembers(current)).map(projectMember)
    else if (domain === 'roles') snapshot.roles = (await listEnterpriseRoles(current)).map(projectRole)
    else if (domain === 'departments') snapshot.departments = (await listEnterpriseDepartments(current)).map(projectDepartment)
    else if (domain === 'company') {
      const profile = await getEnterpriseTenantProfile(current)
      if (profile.tenantId && profile.tenantId !== tenantId) throw new Error('企业资料服务返回了不属于当前租户的数据。')
      snapshot.company = projectCompany(profile)
    }
  }))

  return { tenantId, session: current, snapshot, loadedDomains: [...domains] }
}

function createDemoDataSource(): EnterpriseDataSource {
  return {
    kind: 'demo',
    initial() {
      return { tenantId: 'shanghai', snapshot: loadSnapshot('shanghai'), session: null, loadedDomains: ['members', 'roles', 'departments', 'company'] }
    },
    async load(tenantId = 'shanghai') {
      return { tenantId, snapshot: loadSnapshot(tenantId), session: null, loadedDomains: ['members', 'roles', 'departments', 'company'] }
    },
    async switchTenant(tenantId: string) {
      return { tenantId, snapshot: loadSnapshot(tenantId), session: null, loadedDomains: ['members', 'roles', 'departments', 'company'] }
    },
    persist(tenantId: string, snapshot: TenantSnapshot) {
      saveSnapshot(tenantId, snapshot)
    },
  }
}

function createApiDataSource(): EnterpriseDataSource {
  return {
    kind: 'api',
    initial() {
      return { tenantId: '', snapshot: emptyEnterpriseSnapshot(), session: null, loadedDomains: [] }
    },
    load(_tenantId?: string, domains: EnterpriseDomain[] = []) {
      return loadApiState(undefined, domains)
    },
    async switchTenant(tenantId: string, domains: EnterpriseDomain[] = []) {
      const session = await selectSessionTenant(tenantId)
      return loadApiState(session, domains)
    },
    persist() {
      throw new Error('API 数据源禁止写入本地 demo snapshot。')
    },
  }
}

export function createEnterpriseDataSource(): EnterpriseDataSource {
  return (import.meta.env.VITE_DATA_MODE ?? 'demo') === 'api' ? createApiDataSource() : createDemoDataSource()
}
