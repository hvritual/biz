import { loadSnapshot, saveSnapshot } from '@/services/demo/repository'
import {
  listEnterpriseMembers,
  readEnterpriseMemberSession,
} from '@/services/enterprise/memberRuntime'
import {
  listEnterpriseRoles,
} from '@/services/enterprise/roleRuntime'
import {
  listEnterpriseDepartments,
} from '@/services/enterprise/departmentRuntime'
import {
  getEnterpriseTenantProfile,
} from '@/services/enterprise/tenantProfileRuntime'
import { selectSessionTenant, type TrustedSession } from '@/services/runtime/api'
import type {
  Company,
  DataScope,
  Department,
  Member,
  MemberStatus,
  Role,
  TenantSnapshot,
} from '@/types/enterprise'
import type { EnterpriseTenantMember } from '@/services/enterprise/memberRuntime'
import type { EnterpriseTenantRole } from '@/services/enterprise/roleRuntime'
import type { EnterpriseDepartment } from '@/services/enterprise/departmentRuntime'
import type { EnterpriseTenantProfile } from '@/services/enterprise/tenantProfileRuntime'

export type EnterpriseSourceKind = 'demo' | 'api'

export type EnterpriseSourceState = {
  tenantId: string
  snapshot: TenantSnapshot
  session: TrustedSession | null
}

export type EnterpriseDataSource = {
  kind: EnterpriseSourceKind
  initial(): EnterpriseSourceState
  load(tenantId?: string): Promise<EnterpriseSourceState>
  switchTenant(tenantId: string): Promise<EnterpriseSourceState>
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
  return {
    members: [],
    roles: [],
    departments: [],
    company: emptyCompany(),
    logs: [],
    settings: {},
  }
}

function numericVersion(value: string | number | undefined) {
  const version = Number(value ?? 0)
  return Number.isFinite(version) ? version : 0
}

function memberStatus(status: string): MemberStatus {
  switch (status) {
    case 'TENANT_MEMBER_STATUS_ACTIVE':
      return 'active'
    case 'TENANT_MEMBER_STATUS_INVITED':
      return 'invited'
    case 'TENANT_MEMBER_STATUS_SUSPENDED':
      return 'suspended'
    case 'TENANT_MEMBER_STATUS_REMOVED':
      return 'removed'
    default:
      return 'invited'
  }
}

function viewScope(scope: string): DataScope {
  switch (scope) {
    case 'all':
    case 'DATA_SCOPE_ALL':
      return 'all'
    case 'self':
    case 'DATA_SCOPE_SELF':
      return 'self'
    case 'sites':
    case 'DATA_SCOPE_SITES':
      return 'custom'
    default:
      return 'custom'
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
    name: value.name || value.email,
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
  return {
    id: value.id,
    name: value.name,
    description: '',
    builtin: value.protectedOwner,
    enabled: value.status === 'TENANT_ROLE_STATUS_ACTIVE',
    scope: strongestRoleScope(value),
    permissions: value.permissions.map((grant) => grant.permission),
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

async function loadApiState(session?: TrustedSession): Promise<EnterpriseSourceState> {
  const current = session ?? (await readEnterpriseMemberSession())
  const tenantId = current.active_tenant_id ?? ''
  if (!current.authenticated || !tenantId) {
    return { tenantId, snapshot: emptyEnterpriseSnapshot(), session: current }
  }

  const [memberRows, roleRows, departmentRows, profile] = await Promise.all([
    listEnterpriseMembers(current),
    listEnterpriseRoles(current),
    listEnterpriseDepartments(current),
    getEnterpriseTenantProfile(current),
  ])

  if (profile.tenantId && profile.tenantId !== tenantId) {
    throw new Error('企业资料服务返回了不属于当前租户的数据。')
  }

  return {
    tenantId,
    session: current,
    snapshot: {
      members: memberRows.map(projectMember),
      roles: roleRows.map(projectRole),
      departments: departmentRows.map(projectDepartment),
      company: projectCompany(profile),
      logs: [],
      settings: {},
    },
  }
}

function createDemoDataSource(): EnterpriseDataSource {
  return {
    kind: 'demo',
    initial() {
      return { tenantId: 'shanghai', snapshot: loadSnapshot('shanghai'), session: null }
    },
    async load(tenantId = 'shanghai') {
      return { tenantId, snapshot: loadSnapshot(tenantId), session: null }
    },
    async switchTenant(tenantId: string) {
      return { tenantId, snapshot: loadSnapshot(tenantId), session: null }
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
      return { tenantId: '', snapshot: emptyEnterpriseSnapshot(), session: null }
    },
    load() {
      return loadApiState()
    },
    async switchTenant(tenantId: string) {
      const session = await selectSessionTenant(tenantId)
      return loadApiState(session)
    },
    persist() {
      throw new Error('API 数据源禁止写入本地 demo snapshot。')
    },
  }
}

export function createEnterpriseDataSource(): EnterpriseDataSource {
  return (import.meta.env.VITE_DATA_MODE ?? 'demo') === 'api'
    ? createApiDataSource()
    : createDemoDataSource()
}
