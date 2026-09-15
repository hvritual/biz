import {
  CommercialApiError,
  commercialRequestId,
  mutate,
  request,
  type PlanVersionDTO,
  type SubscriptionChangePreviewDTO,
  type SubscriptionChangeReceiptDTO,
} from '@/services/commercial/platformCommercial'
import { sessionContext, type TrustedSession } from '@/services/runtime/api'

export type TenantChangeTargets = Readonly<{
  salesScope: string
  targets: PlanVersionDTO[]
}>

export type TenantChangeAction = 'SWITCH' | 'RENEW' | 'STOP_RENEWAL'

export type TenantChangePreviewInput = Readonly<{
  action: TenantChangeAction
  targetPlanCode?: string
  targetPlanVersion?: string | number
  effectiveAt?: string
  reason: string
}>

function requireTenantSession(session: TrustedSession) {
  if (!session.authenticated) throw new Error('请先登录业务账号。')
  if (!session.active_tenant_id) throw new Error('请选择可访问的租户。')
}

function trustedHeaders(session: TrustedSession) {
  requireTenantSession(session)
  return { 'X-Biz-Session-Context': sessionContext(session) }
}

export function createTenantChangeRequestId(prefix = 'tenant-plan-change') {
  return commercialRequestId(prefix)
}

export function listMySubscriptionChangeTargets(session: TrustedSession) {
  return request<TenantChangeTargets>('/v1/tenant/subscription/change-targets', {
    headers: trustedHeaders(session),
  })
}

export function previewMySubscriptionChange(
  session: TrustedSession,
  input: TenantChangePreviewInput,
  requestId = createTenantChangeRequestId('tenant-plan-preview'),
) {
  requireTenantSession(session)
  return mutate<SubscriptionChangePreviewDTO>(
    '/v1/tenant/subscription/change-previews',
    'POST',
    {
      requestId,
      action: input.action,
      targetPlanCode: input.action === 'SWITCH' ? String(input.targetPlanCode ?? '').trim() : '',
      targetPlanVersion: input.action === 'SWITCH' ? String(input.targetPlanVersion ?? '') : '0',
      effectiveAt: input.action === 'STOP_RENEWAL' ? '' : String(input.effectiveAt ?? '').trim(),
      reason: input.reason.trim(),
    },
    { idempotencyKey: requestId, sessionContext: sessionContext(session) },
  )
}

export function getMySubscriptionChangePreview(
  session: TrustedSession,
  changeId: string,
) {
  return request<SubscriptionChangePreviewDTO>(
    `/v1/tenant/subscription/change-previews/${encodeURIComponent(changeId)}`,
    { headers: trustedHeaders(session) },
  )
}

export function confirmMySubscriptionChange(
  session: TrustedSession,
  preview: SubscriptionChangePreviewDTO,
  reason: string,
  requestId = createTenantChangeRequestId('tenant-plan-confirm'),
) {
  requireTenantSession(session)
  return mutate<SubscriptionChangeReceiptDTO>(
    `/v1/tenant/subscription/changes/${encodeURIComponent(preview.changeId)}/confirm`,
    'POST',
    {
      changeId: preview.changeId,
      requestId,
      previewHash: preview.previewHash,
      reason: reason.trim(),
    },
    { idempotencyKey: requestId, sessionContext: sessionContext(session) },
  )
}

export function getMySubscriptionChangeReceipt(session: TrustedSession, changeId: string) {
  return request<SubscriptionChangeReceiptDTO>(
    `/v1/tenant/subscription/changes/${encodeURIComponent(changeId)}`,
    { headers: trustedHeaders(session) },
  )
}

export function needsExternalCommercialApproval(preview: SubscriptionChangePreviewDTO | null) {
  return Boolean(preview && preview.pricingBasis && preview.pricingBasis !== 'NO_PRICE_REFERENCE')
}

export function tenantChangeRuntimeError(error: unknown) {
  if (error instanceof CommercialApiError) {
    if (error.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
    if (error.code === 'forbidden') return '当前账号没有变更套餐的权限。'
    if (error.code === 'conflict') return '套餐状态已变化，请重新生成变更预览。'
    if (error.message.includes('SUBSCRIPTION_CHANGE_EXTERNAL_APPROVAL_REQUIRED')) {
      return '该套餐存在价格引用，需要先完成外部商业或支付审批，当前页面不会绕过审批直接生效。'
    }
    if (error.message.includes('SUBSCRIPTION_CHANGE_PREVIEW_EXPIRED')) {
      return '变更预览已过期，请重新生成预览。'
    }
    if (error.message.includes('SUBSCRIPTION_CHANGE_QUOTA_VALIDATION_REQUIRED')) {
      return '当前额度状态需要权威复核，暂不能确认此次变更。'
    }
    return error.message
  }
  return error instanceof Error ? error.message : '套餐变更服务请求失败。'
}
