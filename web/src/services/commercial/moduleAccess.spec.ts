import { describe, expect, it } from 'vitest'
import { moduleAccessDefinitions, moduleReadbackMatches, operationDefinition } from './moduleAccess'
import type { ModuleDTO } from './platformCommercial'
import type { ActionCatalogAction } from '@/services/runtime/api'

const module: ModuleDTO = {
  moduleCode: 'access-management', name: '成员与权限', category: 'access', salesScope: ['default'],
  technicalStatus: 'MODULE_TECHNICAL_STATUS_READY', salesStatus: 'MODULE_SALES_STATUS_SELLABLE',
  capabilityCodes: ['tenant.lifecycle', 'tenant.member.lifecycle', 'tenant.role.permission'],
  quotaSchemaKeys: [], fieldPolicySchemaKeys: [], dependencies: [], version: '9',
}

function action(
  code: string,
  options: Partial<ActionCatalogAction> & Pick<ActionCatalogAction, 'classification'>,
): ActionCatalogAction {
  return {
    code,
    domain: 'test',
    application: 'test',
    use_case: code,
    tenant_required: options.tenant_required ?? true,
    authentication: ['web-session'],
    permissions: [],
    permission_mode: 'all',
    http: [{ method: 'GET', path: `/test/${code}` }],
    ...options,
  }
}

const catalogActions: ActionCatalogAction[] = [
  action('commercial.module.create', {
    classification: 'platform_management',
    tenant_required: false,
    application: 'module_catalog',
    permissions: ['platform.module.manage'],
  }),
  action('commercial.module.update', {
    classification: 'platform_management',
    tenant_required: false,
    application: 'module_catalog',
    permissions: ['platform.module.manage'],
  }),
  action('commercial.module.set_technical_status', {
    classification: 'platform_management',
    tenant_required: false,
    application: 'module_catalog',
    permissions: ['platform.module.technical.manage'],
  }),
  action('tenant.member.list', {
    classification: 'tenant_business',
    module_code: 'access-management',
    capability_codes: ['tenant.member.lifecycle'],
  }),
  action('tenant.role.list', {
    classification: 'tenant_business',
    module_code: 'access-management',
    capability_codes: ['tenant.role.permission'],
  }),
  action('tenant.profile.get', {
    classification: 'tenant_business',
    module_code: 'access-management',
    capability_codes: ['tenant.lifecycle'],
  }),
  action('device.list', {
    classification: 'tenant_business',
    module_code: 'device-operations',
    capability_codes: ['device.lifecycle'],
  }),
]

describe('module definition inspection is not an authorization catalog', () => {
  it('reads server-provided operation contract metadata, including the separate technical permission', () => {
    expect(operationDefinition(catalogActions, 'commercial.module.create')?.permissions).toEqual(['platform.module.manage'])
    expect(operationDefinition(catalogActions, 'commercial.module.update')?.permissions).toEqual(['platform.module.manage'])
    expect(operationDefinition(catalogActions, 'commercial.module.set_technical_status')?.permissions).toEqual(['platform.module.technical.manage'])
    expect(operationDefinition(catalogActions, 'nonexistent.operation')).toBeUndefined()
  })
  it('joins exact entry operations, not navigation modules or URL prefixes', () => {
    const result = moduleAccessDefinitions(module, [
      { path: '/enterprise/members', meta: { title: '成员管理', module: 'enterprise', authorizationActions: ['tenant.member.list'] } },
      { path: '/enterprise/unbound', meta: { module: 'access-management' } },
      { path: '/elsewhere', meta: { authorizationActions: ['device.list'] } },
      { path: '/recovery', meta: { authorizationActions: ['tenant.member.list'], authorizationPublic: true } },
    ], catalogActions)
    expect(result.pages.map((page) => page.path)).toEqual(['/enterprise/members'])
    expect(result.pages[0]?.navigationModule).toBe('enterprise')
    expect(result.operations.some((item) => item.operationId === 'tenant.member.list')).toBe(true)
    expect(result.operations.some((item) => item.operationId === 'commercial.module.update')).toBe(false)
    expect(result).not.toHaveProperty('allowed')
  })
  it('keeps missing and mismatched contract facts explicit', () => {
    const result = moduleAccessDefinitions({ ...module, capabilityCodes: ['unknown.capability'] }, [], catalogActions)
    expect(result.unmappedCapabilities).toEqual(['unknown.capability'])
    expect(result.catalogMismatch).toBe(true)
    expect(moduleAccessDefinitions({ ...module, moduleCode: 'unknown-module' }, [], catalogActions).pages).toEqual([])
  })
})

describe('authoritative module readback', () => {
  const receipt = { ...module, name: '权限管理', salesStatus: 'MODULE_SALES_STATUS_RETIRED' as const, version: '10' }
  it('requires exact module and version, including uint64 strings', () => {
    expect(moduleReadbackMatches(receipt, { ...receipt }, 'metadata')).toBe(true)
    expect(moduleReadbackMatches(receipt, { ...receipt, moduleCode: 'other' }, 'metadata')).toBe(false)
    expect(moduleReadbackMatches(receipt, { ...receipt, version: '11' }, 'metadata')).toBe(false)
    const huge = { ...receipt, version: '18446744073709551614' }
    expect(moduleReadbackMatches(huge, { ...huge, version: '18446744073709551615' }, 'metadata')).toBe(false)
  })
  it('compares the affected fields and scope set rather than a success toast', () => {
    expect(moduleReadbackMatches(receipt, { ...receipt, name: 'stale' }, 'metadata')).toBe(false)
    expect(moduleReadbackMatches(receipt, { ...receipt, salesScope: ['other'] }, 'metadata')).toBe(false)
    const scopes = { ...receipt, salesScope: ['office', 'default'] }
    expect(moduleReadbackMatches(scopes, { ...scopes, salesScope: ['default', 'office'] }, 'metadata')).toBe(true)
    expect(moduleReadbackMatches(receipt, { ...receipt, salesStatus: 'MODULE_SALES_STATUS_SELLABLE' }, 'sales')).toBe(false)
    expect(moduleReadbackMatches(receipt, { ...receipt, technicalStatus: 'MODULE_TECHNICAL_STATUS_DISABLED' }, 'technical')).toBe(false)
    expect(moduleReadbackMatches(receipt, { ...receipt }, 'create')).toBe(true)
  })
})
