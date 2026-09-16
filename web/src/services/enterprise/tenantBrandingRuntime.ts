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
import { uiThemePresets, type UiThemePresetName } from '@/ui/base/theme'

export type EnterpriseTenantBranding = Readonly<{
  tenantId: string
  preset: UiThemePresetName | 'custom'
  primary: string
  version: string | number
  canManage: boolean
}>

export type EnterpriseTenantBrandingDraft = Readonly<{
  preset: UiThemePresetName | 'custom'
  primary: string
}>

export function tenantBrandingRequestId(suffix = '') {
  return commercialRequestId(`enterprise-tenant-branding-update${suffix ? `-${suffix}` : ''}`)
}

export function tenantBrandingRuntimeError(error: unknown) {
  if (error instanceof CommercialApiError) {
    if (error.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
    if (error.code === 'forbidden') return '当前账号没有维护企业品牌主题的权限。'
    if (error.code === 'conflict') return '企业品牌主题已被其他操作修改，请重新读取后再试。'
    return error.message
  }
  return error instanceof Error ? error.message : '企业品牌主题服务请求失败。'
}

function requireTenantSession(session: TrustedSession) {
  if (!session.authenticated) throw new Error('请先登录业务账号。')
  if (!session.active_tenant_id) throw new Error('请选择可访问的租户。')
}

function brandingSnapshot(value: Partial<EnterpriseTenantBranding>): EnterpriseTenantBranding {
  const preset = value.preset === 'custom' || (typeof value.preset === 'string' && value.preset in uiThemePresets)
    ? value.preset as EnterpriseTenantBranding['preset']
    : 'blue'
  return Object.freeze({
    tenantId: value.tenantId ?? '',
    preset,
    primary: value.primary ?? '',
    version: value.version ?? 0,
    canManage: Boolean(value.canManage),
  })
}

export async function readEnterpriseTenantBrandingSession() {
  return readSession()
}

export async function getEnterpriseTenantBranding(session: TrustedSession) {
  requireTenantSession(session)
  const value = await request<EnterpriseTenantBranding>('/v1/tenant/branding', {
    headers: { 'X-Biz-Session-Context': sessionContext(session) },
  })
  const snapshot = brandingSnapshot(value)
  if (snapshot.tenantId && snapshot.tenantId !== session.active_tenant_id) {
    throw new Error('企业品牌主题服务返回了不属于当前租户的数据。')
  }
  return snapshot
}

export async function updateEnterpriseTenantBranding(
  session: TrustedSession,
  current: EnterpriseTenantBranding,
  draft: EnterpriseTenantBrandingDraft,
  idempotencyKey: string,
) {
  requireTenantSession(session)
  await mutate<EnterpriseTenantBranding>(
    '/v1/tenant/branding',
    'PATCH',
    {
      preset: draft.preset,
      primary: draft.preset === 'custom' ? draft.primary.trim().toLowerCase() : '',
      version: current.version,
    },
    { idempotencyKey, sessionContext: sessionContext(session) },
  )
  return getEnterpriseTenantBranding(session)
}
