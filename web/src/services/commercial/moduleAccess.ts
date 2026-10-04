import operationPlans from '../../../../contracts/generated/operation-plans.json'
import commercialMappings from '../../../../contracts/commercial/operation-capabilities.v1.json'
import type { ModuleDTO } from './platformCommercial'

/** Definition metadata only. Never use this index as an authorization decision. */
export interface ModuleOperationDefinition {
  operationId: string
  permissions: string[]
  permissionMode: string
  capabilityCodes: string[]
  http: Array<{ method: string; path: string }>
}

export interface ModuleRouteDefinition {
  path: string
  meta: Record<string, unknown>
}

export interface ModulePageDefinition {
  path: string
  title: string
  surface: string
  navigationModule: string
  requiredOperations: string[]
  matchedOperations: string[]
}

type Mapping = {
  operation_id: string
  classification: string
  module_code?: string
  capability_codes: string[]
}

const mappings: Mapping[] = commercialMappings.operations
type OperationPlan = {
  operationId: string
  security: { permissions?: string[]; permissionMode?: string }
  bindings: { http?: Array<{ method: string; path: string }> }
}
const plans: OperationPlan[] = operationPlans.operations
export const moduleMappingVersion = commercialMappings.mapping_version

export function operationDefinition(operationId: string): ModuleOperationDefinition | undefined {
  const plan = plans.find((item) => item.operationId === operationId)
  if (!plan) return undefined
  const mapping = mappings.find((item) => item.operation_id === operationId)
  return {
    operationId,
    permissions: [...(plan.security.permissions ?? [])],
    permissionMode: plan.security.permissionMode ?? 'all',
    capabilityCodes: [...(mapping?.capability_codes ?? [])],
    http: (plan.bindings.http ?? []).map(({ method, path }) => ({ method, path })),
  }
}

export function moduleAccessDefinitions(module: ModuleDTO, routes: readonly ModuleRouteDefinition[]) {
  const operations = mappings
    .filter((item) => item.module_code === module.moduleCode && item.classification === 'tenant_business')
    .map((item) => operationDefinition(item.operation_id))
    .filter((item): item is ModuleOperationDefinition => Boolean(item?.http.length))
  const operationIds = new Set(operations.map((item) => item.operationId))
  const pages: ModulePageDefinition[] = []
  for (const route of routes) {
    const requiredOperations = Array.isArray(route.meta.authorizationActions)
      ? route.meta.authorizationActions.filter((item): item is string => typeof item === 'string')
      : []
    const matchedOperations = requiredOperations.filter((item) => operationIds.has(item))
    // Navigation grouping is not a commercial capability. Join only by exact operation IDs.
    if (!matchedOperations.length || route.meta.authorizationPublic === true) continue
    pages.push({
      path: route.path,
      title: typeof route.meta.title === 'string' ? route.meta.title : route.path,
      surface: typeof route.meta.surface === 'string' ? route.meta.surface : '未声明',
      navigationModule: typeof route.meta.module === 'string' ? route.meta.module : '未声明',
      requiredOperations,
      matchedOperations,
    })
  }
  const registeredCapabilities = new Set(operations.flatMap((item) => item.capabilityCodes))
  return {
    operations,
    pages: pages.sort((a, b) => a.path.localeCompare(b.path)),
    unmappedCapabilities: (module.capabilityCodes ?? []).filter((code) => !registeredCapabilities.has(code)),
    catalogMismatch: operations.some((item) => item.capabilityCodes.some((code) => !(module.capabilityCodes ?? []).includes(code))),
  }
}

export type ModuleChangeKind = 'create' | 'metadata' | 'sales' | 'technical'
export const moduleChangeOperation: Record<ModuleChangeKind, string> = {
  create: 'commercial.module.create',
  metadata: 'commercial.module.update',
  sales: 'commercial.module.set_sales_status',
  technical: 'commercial.module.set_technical_status',
}

/** A successful command receipt and a matching authoritative GET are separate facts. */
export function moduleReadbackMatches(receipt: ModuleDTO, current: ModuleDTO, kind: ModuleChangeKind) {
  if (receipt.moduleCode !== current.moduleCode || String(receipt.version) !== String(current.version)) return false
  if (kind === 'sales') {
    return receipt.salesStatus === current.salesStatus && receipt.technicalStatus === current.technicalStatus
  }
  if (kind === 'technical') {
    return receipt.technicalStatus === current.technicalStatus && receipt.salesStatus === current.salesStatus
  }
  const metadataMatches = receipt.name === current.name && receipt.category === current.category
    && receipt.technicalStatus === current.technicalStatus && receipt.salesStatus === current.salesStatus
    && [...(receipt.salesScope ?? [])].sort().join('\n') === [...(current.salesScope ?? [])].sort().join('\n')
  if (!metadataMatches || kind !== 'create') return metadataMatches
  const same = (left: string[] = [], right: string[] = []) => [...left].sort().join('\n') === [...right].sort().join('\n')
  return same(receipt.capabilityCodes, current.capabilityCodes)
    && same(receipt.quotaSchemaKeys, current.quotaSchemaKeys)
    && same(receipt.fieldPolicySchemaKeys, current.fieldPolicySchemaKeys)
    && same(receipt.dependencies, current.dependencies)
}
