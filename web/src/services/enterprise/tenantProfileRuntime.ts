import { backendErrorFallback } from '@/i18n/backend-terms'
import {
  CommercialApiError,
  commercialRequestId,
  mutate,
  request,
} from '@/services/commercial/platformCommercial'
import {
  readSession,
  sessionContext,
  type TrustedSession,
} from '@/services/runtime/api'

export type EnterpriseTenantProfile = Readonly<{
  tenantId: string
  name: string
  shortName: string
  industry: string
  companySize: string
  timezone: string
  contactName: string
  phone: string
  email: string
  address: string
  description: string
  logoAssetRef: string
  version: string | number
}>

export type EnterpriseTenantProfileDraft = Omit<EnterpriseTenantProfile, 'tenantId' | 'version'>

export function tenantProfileRequestId(suffix = '') {
  return commercialRequestId(`enterprise-tenant-profile-update${suffix ? `-${suffix}` : ''}`)
}

export function tenantProfileRuntimeError(error: unknown) {
  if (error instanceof CommercialApiError) {
    if (error.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
    if (error.code === 'forbidden') return '当前账号没有维护企业资料的权限。'
    if (error.code === 'conflict') return '企业资料已被其他操作修改，请刷新后重试。'
    return backendErrorFallback('profile')
  }
  return error instanceof Error ? error.message : backendErrorFallback('profile')
}

function requireTenantSession(session: TrustedSession) {
  if (!session.authenticated) throw new Error('请先登录业务账号。')
  if (!session.active_tenant_id) throw new Error('请选择可访问的租户。')
}

function profileSnapshot(value: Partial<EnterpriseTenantProfile>): EnterpriseTenantProfile {
  return Object.freeze({
    tenantId: value.tenantId ?? '',
    name: value.name ?? '',
    shortName: value.shortName ?? '',
    industry: value.industry ?? '',
    companySize: value.companySize ?? '',
    timezone: value.timezone ?? 'Asia/Shanghai',
    contactName: value.contactName ?? '',
    phone: value.phone ?? '',
    email: value.email ?? '',
    address: value.address ?? '',
    description: value.description ?? '',
    logoAssetRef: value.logoAssetRef ?? '',
    version: value.version ?? 0,
  })
}

export async function readEnterpriseTenantProfileSession() {
  return readSession()
}

export async function getEnterpriseTenantProfile(session: TrustedSession) {
  requireTenantSession(session)
  const value = await request<EnterpriseTenantProfile>('/v1/tenant/profile', {
    headers: { 'X-Biz-Session-Context': sessionContext(session) },
  })
  return profileSnapshot(value)
}

export async function updateEnterpriseTenantProfile(
  session: TrustedSession,
  current: EnterpriseTenantProfile,
  draft: EnterpriseTenantProfileDraft,
  idempotencyKey: string,
) {
  requireTenantSession(session)
  const value = await mutate<EnterpriseTenantProfile>(
    '/v1/tenant/profile',
    'PATCH',
    {
      name: draft.name.trim(),
      shortName: draft.shortName.trim(),
      industry: draft.industry.trim(),
      companySize: draft.companySize.trim(),
      timezone: draft.timezone.trim(),
      contactName: draft.contactName.trim(),
      phone: draft.phone.trim(),
      email: draft.email.trim(),
      address: draft.address.trim(),
      description: draft.description.trim(),
      logoAssetRef: draft.logoAssetRef.trim(),
      version: current.version,
    },
    { idempotencyKey, sessionContext: sessionContext(session) },
  )
  return profileSnapshot(value)
}
