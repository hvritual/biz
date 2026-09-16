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

export type EnterpriseAuditRisk = 'low' | 'medium' | 'high'
export type EnterpriseAuditResult = 'pending' | 'success' | 'failure' | 'panic'

export type EnterpriseAuditRecord = Readonly<{
  auditId: string
  occurredAt: string
  actorSubject: string
  actorUserId: string
  authMethod: string
  authChannel: string
  sessionRef: string
  requestId: string
  idempotencyRef: string
  operationId: string
  module: string
  target: string
  result: EnterpriseAuditResult
  risk: EnterpriseAuditRisk
  receiptRef: string
  reason: string
  requestDigest: string
}>

export type EnterpriseAuditFilter = {
  query?: string
  operationId?: string
  result?: EnterpriseAuditResult | ''
  risk?: EnterpriseAuditRisk | ''
  page?: number
  pageSize?: number
}

export type EnterpriseAuditPage = Readonly<{
  records: EnterpriseAuditRecord[]
  total: number
  page: number
  pageSize: number
}>

export type EnterpriseAuditExport = Readonly<{
  exportId: string
  generatedAt: string
  records: EnterpriseAuditRecord[]
}>

function requireTenantSession(session: TrustedSession) {
  if (!session.authenticated) throw new Error('请先登录业务账号。')
  if (!session.active_tenant_id) throw new Error('请选择可访问的租户。')
}

function readHeaders(session: TrustedSession) {
  return { 'X-Biz-Session-Context': sessionContext(session) }
}

function auditSnapshot(value: Partial<EnterpriseAuditRecord>): EnterpriseAuditRecord {
  const result = value.result === 'pending' || value.result === 'success' || value.result === 'failure' || value.result === 'panic'
    ? value.result
    : 'failure'
  const risk = value.risk === 'low' || value.risk === 'medium' || value.risk === 'high'
    ? value.risk
    : 'low'
  return Object.freeze({
    auditId: value.auditId ?? '',
    occurredAt: value.occurredAt ?? '',
    actorSubject: value.actorSubject ?? '',
    actorUserId: value.actorUserId ?? '',
    authMethod: value.authMethod ?? '',
    authChannel: value.authChannel ?? '',
    sessionRef: value.sessionRef ?? '',
    requestId: value.requestId ?? '',
    idempotencyRef: value.idempotencyRef ?? '',
    operationId: value.operationId ?? '',
    module: value.module ?? '',
    target: value.target ?? '',
    result,
    risk,
    receiptRef: value.receiptRef ?? '',
    reason: value.reason ?? '',
    requestDigest: value.requestDigest ?? '',
  })
}

function auditQuery(filter: EnterpriseAuditFilter) {
  const query = new URLSearchParams()
  if (filter.query?.trim()) query.set('query', filter.query.trim())
  if (filter.operationId?.trim()) query.set('operation_id', filter.operationId.trim())
  if (filter.result) query.set('result', filter.result)
  if (filter.risk) query.set('risk', filter.risk)
  query.set('page', String(Math.max(1, Math.trunc(filter.page ?? 1))))
  query.set('page_size', String(Math.min(100, Math.max(1, Math.trunc(filter.pageSize ?? 20)))))
  return query.toString()
}

export function auditRequestId() {
  return commercialRequestId('enterprise-audit-export')
}

export function auditRuntimeError(error: unknown) {
  if (error instanceof CommercialApiError) {
    if (error.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
    if (error.code === 'forbidden') return '当前账号没有读取或导出企业审计日志的权限。'
    if (error.code === 'conflict') return '审计导出请求发生幂等冲突，请刷新后重新发起。'
    return error.message
  }
  return error instanceof Error ? error.message : '审计日志服务请求失败。'
}

export function sameAuditSession(a: TrustedSession, b: TrustedSession) {
  return (
    a.authenticated === b.authenticated &&
    (a.actor_kind ?? '') === (b.actor_kind ?? '') &&
    (a.user_id ?? '') === (b.user_id ?? '') &&
    (a.active_tenant_id ?? '') === (b.active_tenant_id ?? '')
  )
}

export async function readEnterpriseAuditSession() {
  return readSession()
}

export async function switchEnterpriseAuditTenant(tenantId: string) {
  return selectSessionTenant(tenantId)
}

export async function listEnterpriseAuditRecords(
  session: TrustedSession,
  filter: EnterpriseAuditFilter = {},
): Promise<EnterpriseAuditPage> {
  requireTenantSession(session)
  const response = await request<{
    records?: EnterpriseAuditRecord[]
    total?: number | string
    page?: number | string
    pageSize?: number | string
  }>(`/v1/tenant/audit-logs?${auditQuery(filter)}`, { headers: readHeaders(session) })
  return Object.freeze({
    records: Array.isArray(response.records) ? response.records.map(auditSnapshot) : [],
    total: Number(response.total ?? 0),
    page: Number(response.page ?? filter.page ?? 1),
    pageSize: Number(response.pageSize ?? filter.pageSize ?? 20),
  })
}

export async function getEnterpriseAuditRecord(session: TrustedSession, auditId: string) {
  requireTenantSession(session)
  const value = await request<EnterpriseAuditRecord>(
    `/v1/tenant/audit-logs/${encodeURIComponent(auditId)}`,
    { headers: readHeaders(session) },
  )
  return auditSnapshot(value)
}

export async function exportEnterpriseAuditRecords(
  session: TrustedSession,
  filter: Omit<EnterpriseAuditFilter, 'page' | 'pageSize'>,
  idempotencyKey: string,
  maxRows = 5000,
): Promise<EnterpriseAuditExport> {
  requireTenantSession(session)
  const response = await mutate<{
    exportId?: string
    generatedAt?: string
    records?: EnterpriseAuditRecord[]
  }>(
    '/v1/tenant/audit-logs/exports',
    'POST',
    {
      query: filter.query?.trim() ?? '',
      operationId: filter.operationId?.trim() ?? '',
      result: filter.result ?? '',
      risk: filter.risk ?? '',
      maxRows: Math.min(5000, Math.max(1, Math.trunc(maxRows))),
    },
    { idempotencyKey, sessionContext: sessionContext(session) },
  )
  return Object.freeze({
    exportId: response.exportId ?? '',
    generatedAt: response.generatedAt ?? '',
    records: Array.isArray(response.records) ? response.records.map(auditSnapshot) : [],
  })
}
