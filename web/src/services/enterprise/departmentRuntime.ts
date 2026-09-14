import {
  CommercialApiError,
  commercialRequestId,
  mutate,
  request,
} from '@/services/commercial/platformCommercial'
import {
  readSession,
  selectSessionTenant,
  sessionContext,
  type TrustedSession,
} from '@/services/runtime/api'

export type DepartmentMutation = 'create' | 'update' | 'enable' | 'disable'

export type EnterpriseDepartment = Readonly<{
  departmentId: string
  name: string
  parentId: string
  leaderUserId: string
  email: string
  phone: string
  status: string
  sort: number
  version: string | number
}>

export type EnterpriseDepartmentDraft = {
  name: string
  parentId: string
  leaderUserId: string
  email: string
  phone: string
  sort: number
  enabled: boolean
}

export function departmentRequestId(action: DepartmentMutation, suffix = '') {
  return commercialRequestId(`enterprise-department-${action}${suffix ? `-${suffix}` : ''}`)
}

export function departmentStatusLabel(status: string) {
  return (
    {
      TENANT_DEPARTMENT_STATUS_ACTIVE: '已启用',
      TENANT_DEPARTMENT_STATUS_DISABLED: '已停用',
    }[status] ?? status
  )
}

export function departmentRuntimeError(error: unknown) {
  if (error instanceof CommercialApiError) {
    if (error.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
    if (error.code === 'forbidden') return '当前账号没有管理企业组织架构的权限。'
    if (error.code === 'conflict') return '部门版本、层级、负责人或成员归属规则发生冲突，请刷新后重试。'
    return error.message
  }
  return error instanceof Error ? error.message : '组织架构服务请求失败。'
}

function requireTenantSession(session: TrustedSession) {
  if (!session.authenticated) throw new Error('请先登录业务账号。')
  if (!session.active_tenant_id) throw new Error('请选择可访问的租户。')
}

function headers(session: TrustedSession) {
  return { 'X-Biz-Session-Context': sessionContext(session) }
}

function departmentSnapshot(value: Partial<EnterpriseDepartment>): EnterpriseDepartment {
  return Object.freeze({
    departmentId: value.departmentId ?? '',
    name: value.name ?? '',
    parentId: value.parentId ?? '',
    leaderUserId: value.leaderUserId ?? '',
    email: value.email ?? '',
    phone: value.phone ?? '',
    status: value.status ?? '',
    sort: Number(value.sort ?? 0),
    version: value.version ?? 0,
  })
}

export async function readEnterpriseDepartmentSession() {
  return readSession()
}

export async function switchEnterpriseDepartmentTenant(tenantId: string) {
  return selectSessionTenant(tenantId)
}

export async function listEnterpriseDepartments(session: TrustedSession) {
  requireTenantSession(session)
  const result = await request<{ departments?: EnterpriseDepartment[] }>('/v1/tenant/departments', {
    headers: headers(session),
  })
  return Array.isArray(result.departments) ? result.departments.map(departmentSnapshot) : []
}

export async function getEnterpriseDepartment(session: TrustedSession, departmentId: string) {
  requireTenantSession(session)
  const value = await request<EnterpriseDepartment>(
    `/v1/tenant/departments/${encodeURIComponent(departmentId)}`,
    { headers: headers(session) },
  )
  return departmentSnapshot(value)
}

async function departmentMutate(
  session: TrustedSession,
  path: string,
  method: 'POST' | 'PATCH',
  body: unknown,
  idempotencyKey: string,
) {
  requireTenantSession(session)
  const value = await mutate<EnterpriseDepartment>(path, method, body, {
    idempotencyKey,
    sessionContext: sessionContext(session),
  })
  return departmentSnapshot(value)
}

export function createEnterpriseDepartment(
  session: TrustedSession,
  draft: EnterpriseDepartmentDraft,
  idempotencyKey: string,
) {
  return departmentMutate(session, '/v1/tenant/departments', 'POST', {
    name: draft.name.trim(),
    parentId: draft.parentId.trim(),
    leaderUserId: draft.leaderUserId.trim(),
    email: draft.email.trim(),
    phone: draft.phone.trim(),
    sort: Number(draft.sort) || 0,
  }, idempotencyKey)
}

export function updateEnterpriseDepartment(
  session: TrustedSession,
  department: EnterpriseDepartment,
  draft: EnterpriseDepartmentDraft,
  idempotencyKey: string,
) {
  return departmentMutate(
    session,
    `/v1/tenant/departments/${encodeURIComponent(department.departmentId)}`,
    'PATCH',
    {
      departmentId: department.departmentId,
      name: draft.name.trim(),
      parentId: draft.parentId.trim(),
      leaderUserId: draft.leaderUserId.trim(),
      email: draft.email.trim(),
      phone: draft.phone.trim(),
      sort: Number(draft.sort) || 0,
      version: department.version,
    },
    idempotencyKey,
  )
}

export function enableEnterpriseDepartment(
  session: TrustedSession,
  department: EnterpriseDepartment,
  idempotencyKey: string,
) {
  return departmentMutate(
    session,
    `/v1/tenant/departments/${encodeURIComponent(department.departmentId)}/enable`,
    'POST',
    { departmentId: department.departmentId, version: department.version },
    idempotencyKey,
  )
}

export function disableEnterpriseDepartment(
  session: TrustedSession,
  department: EnterpriseDepartment,
  idempotencyKey: string,
) {
  return departmentMutate(
    session,
    `/v1/tenant/departments/${encodeURIComponent(department.departmentId)}/disable`,
    'POST',
    { departmentId: department.departmentId, version: department.version },
    idempotencyKey,
  )
}
