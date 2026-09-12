export type ModuleTechnicalStatus =
  | 'MODULE_TECHNICAL_STATUS_UNSPECIFIED'
  | 'MODULE_TECHNICAL_STATUS_NOT_READY'
  | 'MODULE_TECHNICAL_STATUS_READY'
  | 'MODULE_TECHNICAL_STATUS_DISABLED'

export type ModuleSalesStatus =
  | 'MODULE_SALES_STATUS_UNSPECIFIED'
  | 'MODULE_SALES_STATUS_SELLABLE'
  | 'MODULE_SALES_STATUS_RETIRED'

export interface ModuleDTO {
  moduleCode: string
  name: string
  category: string
  salesScope: string[]
  technicalStatus: ModuleTechnicalStatus
  salesStatus: ModuleSalesStatus
  capabilityCodes: string[]
  quotaSchemaKeys: string[]
  fieldPolicySchemaKeys: string[]
  dependencies: string[]
  version: string | number
}

export interface PlanQuota {
  key: string
  unlimited: boolean
  value: string | number
}

export interface PlanField {
  key: string
  action: string
  mode: string
}

export interface PlanModule {
  moduleCode: string
  capabilityCodes: string[]
  quotas: PlanQuota[]
  fields: PlanField[]
}

export interface PlanTerms {
  modules: PlanModule[]
  salesScope: string[]
  validityMode: string
  validityDays: number
  priceRef: string
}

export interface PlanVersionDTO {
  planCode: string
  version: string | number
  revision: string | number
  planRevision: string | number
  state: string
  name: string
  terms: PlanTerms
  contentSha256: string
  createdAt: string
  publishedAt: string
  retiredAt: string
  actorId: string
  reason: string
}

export interface PlanEligibilityDTO {
  eligible: boolean
  reason: string
  version?: PlanVersionDTO
}

export interface ListPlanVersionsResult {
  versions: PlanVersionDTO[]
  nextAfterVersion: string | number
}

export interface CreatePlanDraftInput {
  requestId: string
  planCode: string
  name: string
  terms: PlanTerms
  reason: string
}

export interface CreatePlanVersionInput {
  requestId: string
  fromVersion: string | number
  expectedPlanRevision: string | number
  reason: string
}

export interface UpdatePlanDraftInput {
  requestId: string
  expectedRevision: string | number
  name: string
  terms: PlanTerms
  reason: string
}

export interface ChangePlanStateInput {
  requestId: string
  expectedRevision: string | number
  reason: string
}

export type EntitlementTarget =
  | 'ENTITLEMENT_TARGET_MODULE'
  | 'ENTITLEMENT_TARGET_CAPABILITY'
  | 'ENTITLEMENT_TARGET_QUOTA'
  | 'ENTITLEMENT_TARGET_FIELD'

export type EntitlementEffect =
  | 'ENTITLEMENT_EFFECT_GRANT'
  | 'ENTITLEMENT_EFFECT_DENY'
  | 'ENTITLEMENT_EFFECT_QUOTA_ADD'
  | 'ENTITLEMENT_EFFECT_QUOTA_REPLACE'
  | 'ENTITLEMENT_EFFECT_SAFETY_DENY'
  | 'ENTITLEMENT_EFFECT_SAFETY_MASK'

export interface EntitlementLimit {
  unlimited: boolean
  value: string | number
}

export interface EntitlementSourceExplanation {
  id: string
  sourceKind: string
  effect: string
  state: string
  disposition: string
  reason: string
  actorId: string
}

export interface EntitlementDecisionDTO {
  kind: string
  moduleCode: string
  key: string
  fieldAction: string
  allowed: boolean
  reason: string
  limit?: EntitlementLimit
  masked: boolean
  sources: EntitlementSourceExplanation[]
}

export interface EntitlementCatalogVersion {
  moduleCode: string
  version: string | number
}

export interface EntitlementView {
  tenantId: string
  sourceVersion: string | number
  resolverVersion: string | number
  evaluatedAt: string
  validUntil: string
  nextTransitionAt: string
  catalogVersions: EntitlementCatalogVersion[]
  decisions: EntitlementDecisionDTO[]
  entitlementVersion: string | number
  catalogRevision: string | number
  permissionVersion: string
  permissionSubject: string
}

export interface EntitlementOverrideDTO {
  id: string
  tenantId: string
  moduleCode: string
  target: EntitlementTarget
  key: string
  fieldAction: string
  effect: EntitlementEffect
  limit?: EntitlementLimit
  effectiveAt: string
  expiresAt: string
  revokedAt: string
  reason: string
  actorId: string
  version: string | number
  sourceKind: string
}

export interface ListEntitlementOverridesResult {
  sources: EntitlementOverrideDTO[]
  sourceVersion: string | number
}

export interface EntitlementOverrideReceipt {
  source?: EntitlementOverrideDTO
  sourceVersion: string | number
}

export interface CreateEntitlementOverrideInput {
  requestId: string
  expectedVersion: string | number
  moduleCode: string
  target: EntitlementTarget
  key: string
  fieldAction: string
  effect: EntitlementEffect
  limit?: EntitlementLimit
  effectiveAt: string
  expiresAt: string
  reason: string
}

export interface TenantSubscriptionDTO {
  subscriptionId: string
  tenantId: string
  kind: string
  state: string
  planCode: string
  planVersion: string | number
  ruleId: string
  ruleVersion: string | number
  salesScope: string
  entitlementSourceVersion: string | number
  createdAt: string
  matchExplanation: string
  revision: string | number
  periodStart: string
  periodEnd: string
  renewalStopped: boolean
  pendingChangeId: string
}

export type SubscriptionChangeAction = 'SWITCH' | 'RENEW' | 'STOP_RENEWAL'

export interface ProvisioningRequirementDTO {
  code: string
  adapter: string
  version: string
  maxAttempts: number
}

export interface SubscriptionChangeDependency {
  moduleCode: string
  requiresModules: string[]
}

export interface SubscriptionChangeQuotaImpact {
  moduleCode: string
  key: string
  beforeLimit?: EntitlementLimit
  afterLimit?: EntitlementLimit
  usageKnown: boolean
  used: string | number
  overLimit: boolean
  policy: string
  evidence: string
}

export interface SubscriptionChangePreviewDTO {
  changeId: string
  tenantId: string
  actorId: string
  requestId: string
  action: SubscriptionChangeAction | string
  classification: string
  mode: string
  previewHash: string
  before?: TenantSubscriptionDTO
  target?: PlanVersionDTO
  subscriptionRevision: string | number
  sourceVersion: string | number
  entitlementVersion: string | number
  catalogRevision: string | number
  createdAt: string
  expiresAt: string
  effectiveAt: string
  entitlementExpiresAt: string
  currentEntitlements?: EntitlementView
  projectedEntitlements?: EntitlementView
  dependencies: SubscriptionChangeDependency[]
  quotaImpacts: SubscriptionChangeQuotaImpact[]
  impacts: string[]
  pricingBasis: string
  quotaValidationRequired: boolean
  provisioningRequirements: ProvisioningRequirementDTO[]
}

export interface SubscriptionChangeReceiptDTO {
  changeId: string
  tenantId: string
  actorId: string
  requestId: string
  previewHash: string
  action: SubscriptionChangeAction | string
  status: string
  mode: string
  confirmedAt: string
  effectiveAt: string
  entitlementExpiresAt: string
  reason: string
  before?: TenantSubscriptionDTO
  after?: TenantSubscriptionDTO
  beforeSourceVersion: string | number
  afterSourceVersion: string | number
  beforeEntitlementVersion: string | number
  afterEntitlementVersion: string | number
  quotaValidationRequired: boolean
  pricingAuthority: string
  quotaImpacts: SubscriptionChangeQuotaImpact[]
  provisioningTaskId: string
}

export interface PreviewSubscriptionChangeInput {
  requestId: string
  action: SubscriptionChangeAction
  targetPlanCode: string
  targetPlanVersion: string | number
  effectiveAt: string
  reason: string
}

export interface ConfirmSubscriptionChangeInput {
  requestId: string
  previewHash: string
  reason: string
}

interface ListModulesResponse {
  modules?: ModuleDTO[]
}

interface ListPlanVersionsResponse {
  versions?: PlanVersionDTO[]
  nextAfterVersion?: string | number
}

interface ListEntitlementOverridesResponse {
  sources?: EntitlementOverrideDTO[]
  sourceVersion?: string | number
}

interface TrustedSessionResponse {
  authenticated?: boolean
  csrf_token?: string
}

export class CommercialApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: 'unauthenticated' | 'forbidden' | 'conflict' | 'http' | 'invalid-response',
  ) {
    super(message)
  }
}

const configuredBase = import.meta.env.VITE_API_BASE_URL ?? '/api'
const baseUrl = configuredBase.endsWith('/') ? configuredBase.slice(0, -1) : configuredBase

function encoded(value: string | number) {
  return encodeURIComponent(String(value).trim())
}

async function parseError(response: Response) {
  const fallback = `商业平台接口请求失败（HTTP ${response.status}）`
  try {
    const body = (await response.json()) as { message?: string; error?: string }
    return body.message || body.error || fallback
  } catch {
    return fallback
  }
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const message = await parseError(response)
    if (response.status === 401) throw new CommercialApiError(message, 401, 'unauthenticated')
    if (response.status === 403) throw new CommercialApiError(message, 403, 'forbidden')
    if (response.status === 409) throw new CommercialApiError(message, 409, 'conflict')
    throw new CommercialApiError(message, response.status, 'http')
  }

  if (response.status === 204) return undefined as T
  try {
    return (await response.json()) as T
  } catch {
    throw new CommercialApiError('商业平台接口返回了无法解析的数据', response.status, 'invalid-response')
  }
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    credentials: 'include',
    headers,
  })
  return parseResponse<T>(response)
}

async function trustedCsrfToken(): Promise<string> {
  const session = await request<TrustedSessionResponse>('/auth/session')
  if (!session.authenticated || !session.csrf_token) {
    throw new CommercialApiError('当前可信会话不可执行平台写操作', 401, 'unauthenticated')
  }
  return session.csrf_token
}

export async function mutate<T>(
  path: string,
  method: 'POST' | 'PATCH' | 'PUT' | 'DELETE',
  body: unknown,
  options: { idempotencyKey?: string; sessionContext?: string } = {},
): Promise<T> {
  const csrf = await trustedCsrfToken()
  const headers = new Headers({
    'Content-Type': 'application/json',
    'X-CSRF-Token': csrf,
  })
  if (options.idempotencyKey) headers.set('Idempotency-Key', options.idempotencyKey)
  if (options.sessionContext) headers.set('X-Biz-Session-Context', options.sessionContext)
  return request<T>(path, {
    method,
    headers,
    body: JSON.stringify(body),
  })
}

export function commercialRequestId(prefix: string) {
  const random = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `${prefix}-${random}`
}

export async function listPlatformModules(): Promise<ModuleDTO[]> {
  const result = await request<ListModulesResponse>('/v1/platform/modules')
  return Array.isArray(result.modules) ? result.modules : []
}

export async function listPlanVersions(
  planCode: string,
  options: { afterVersion?: string | number; pageSize?: number } = {},
): Promise<ListPlanVersionsResult> {
  const query = new URLSearchParams()
  if (options.afterVersion !== undefined && String(options.afterVersion) !== '') {
    query.set('afterVersion', String(options.afterVersion))
  }
  if (options.pageSize !== undefined) query.set('pageSize', String(options.pageSize))
  const suffix = query.size ? `?${query.toString()}` : ''
  const result = await request<ListPlanVersionsResponse>(`/v1/platform/plans/${encoded(planCode)}/versions${suffix}`)
  return {
    versions: Array.isArray(result.versions) ? result.versions : [],
    nextAfterVersion: result.nextAfterVersion ?? '',
  }
}

export function getPlanVersion(planCode: string, version: string | number) {
  return request<PlanVersionDTO>(`/v1/platform/plans/${encoded(planCode)}/versions/${encoded(version)}`)
}

export function createPlanDraft(input: CreatePlanDraftInput) {
  return mutate<PlanVersionDTO>('/v1/platform/plans', 'POST', input, { idempotencyKey: input.requestId })
}

export function createPlanVersion(planCode: string, input: CreatePlanVersionInput) {
  return mutate<PlanVersionDTO>(`/v1/platform/plans/${encoded(planCode)}/versions`, 'POST', {
    ...input,
    planCode,
  }, { idempotencyKey: input.requestId })
}

export function updatePlanDraft(planCode: string, version: string | number, input: UpdatePlanDraftInput) {
  return mutate<PlanVersionDTO>(`/v1/platform/plans/${encoded(planCode)}/versions/${encoded(version)}`, 'PATCH', {
    ...input,
    planCode,
    version: String(version),
  }, { idempotencyKey: input.requestId })
}

export function publishPlanVersion(planCode: string, version: string | number, input: ChangePlanStateInput) {
  return mutate<PlanVersionDTO>(`/v1/platform/plans/${encoded(planCode)}/versions/${encoded(version)}/publish`, 'POST', {
    ...input,
    planCode,
    version: String(version),
  }, { idempotencyKey: input.requestId })
}

export function retirePlanVersion(planCode: string, version: string | number, input: ChangePlanStateInput) {
  return mutate<PlanVersionDTO>(`/v1/platform/plans/${encoded(planCode)}/versions/${encoded(version)}/retire`, 'POST', {
    ...input,
    planCode,
    version: String(version),
  }, { idempotencyKey: input.requestId })
}

export function checkPlanEligibility(planCode: string, version: string | number, salesScope: string) {
  return mutate<PlanEligibilityDTO>(`/v1/platform/plans/${encoded(planCode)}/versions/${encoded(version)}/eligibility`, 'POST', {
    planCode,
    version: String(version),
    salesScope: salesScope.trim(),
  })
}

export function getTenantSubscription(tenantId: string) {
  return request<TenantSubscriptionDTO>(`/v1/platform/tenants/${encoded(tenantId)}/subscription`)
}

export async function listEntitlementOverrides(tenantId: string): Promise<ListEntitlementOverridesResult> {
  const result = await request<ListEntitlementOverridesResponse>(`/v1/platform/tenants/${encoded(tenantId)}/entitlement-overrides`)
  return {
    sources: Array.isArray(result.sources) ? result.sources : [],
    sourceVersion: result.sourceVersion ?? 0,
  }
}

export function explainTenantEntitlements(tenantId: string, capabilityCodes: string[] = []) {
  return mutate<EntitlementView>(`/v1/platform/tenants/${encoded(tenantId)}/entitlements`, 'POST', {
    tenantId,
    capabilityCodes: capabilityCodes.map((item) => item.trim()).filter(Boolean),
  })
}

export function createEntitlementOverride(tenantId: string, input: CreateEntitlementOverrideInput) {
  return mutate<EntitlementOverrideReceipt>(`/v1/platform/tenants/${encoded(tenantId)}/entitlement-overrides`, 'POST', {
    ...input,
    tenantId,
  }, { idempotencyKey: input.requestId })
}

export function revokeEntitlementOverride(
  tenantId: string,
  overrideId: string,
  input: { requestId: string; expectedVersion: string | number; reason: string },
) {
  return mutate<EntitlementOverrideReceipt>(
    `/v1/platform/tenants/${encoded(tenantId)}/entitlement-overrides/${encoded(overrideId)}/revoke`,
    'POST',
    {
      ...input,
      tenantId,
      id: overrideId,
    },
    { idempotencyKey: input.requestId },
  )
}

export function previewSubscriptionChange(tenantId: string, input: PreviewSubscriptionChangeInput) {
  const requestId = input.requestId.trim()
  return mutate<SubscriptionChangePreviewDTO>(
    `/v1/platform/tenants/${encoded(tenantId)}/subscription/change-previews`,
    'POST',
    {
      ...input,
      tenantId,
      requestId,
      targetPlanVersion: String(input.targetPlanVersion),
    },
    { idempotencyKey: requestId },
  )
}

export function confirmSubscriptionChange(
  tenantId: string,
  changeId: string,
  input: ConfirmSubscriptionChangeInput,
) {
  const requestId = input.requestId.trim()
  return mutate<SubscriptionChangeReceiptDTO>(
    `/v1/platform/tenants/${encoded(tenantId)}/subscription/changes/${encoded(changeId)}/confirm`,
    'POST',
    {
      ...input,
      tenantId,
      changeId,
      requestId,
    },
    { idempotencyKey: requestId },
  )
}

export function getSubscriptionChangePreview(tenantId: string, changeId: string) {
  return request<SubscriptionChangePreviewDTO>(
    `/v1/platform/tenants/${encoded(tenantId)}/subscription/change-previews/${encoded(changeId)}`,
  )
}

export function getSubscriptionChangeReceipt(tenantId: string, changeId: string) {
  return request<SubscriptionChangeReceiptDTO>(
    `/v1/platform/tenants/${encoded(tenantId)}/subscription/changes/${encoded(changeId)}`,
  )
}
