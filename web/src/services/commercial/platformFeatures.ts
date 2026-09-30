import { commercialRequestId, mutate, request, type CommercialFeatureDTO, type CommercialFeatureModuleReferenceDTO } from './platformCommercial'

const encoded = (value: string) => encodeURIComponent(value.trim())

export async function listPlatformCommercialFeatures() {
  const result = await request<{ features?: CommercialFeatureDTO[] }>('/v1/platform/commercial-features')
  return result.features ?? []
}

export function createPlatformCommercialFeature(input: { featureCode: string; name: string; moduleRefs: CommercialFeatureModuleReferenceDTO[]; reason: string }) {
  const requestId = commercialRequestId('commercial-feature-create')
  return mutate<CommercialFeatureDTO>('/v1/platform/commercial-features', 'POST', { requestId, featureCode: input.featureCode.trim(), name: input.name.trim(), moduleRefs: input.moduleRefs, reason: input.reason.trim() }, { idempotencyKey: requestId })
}

export function transitionPlatformCommercialFeature(feature: CommercialFeatureDTO, action: 'publish' | 'stop-selling' | 'sunset' | 'complete-migration' | 'retire', reason: string, details: { replacementCode?: string; migrationState?: string } = {}) {
  const requestId = commercialRequestId(`commercial-feature-${action}`)
  return mutate<CommercialFeatureDTO>(`/v1/platform/commercial-features/${encoded(feature.featureCode)}/${action}`, 'POST', { requestId, featureCode: feature.featureCode, version: String(feature.version), reason: reason.trim(), replacementCode: details.replacementCode?.trim() || '', migrationState: details.migrationState?.trim() || '' }, { idempotencyKey: requestId })
}
