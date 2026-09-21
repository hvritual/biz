import { t } from './index'

export const backendTermCatalog = {
  plan: {
    'rental-growth-2026': 'rentalGrowth2026',
    'rental-pro-2026': 'rentalPro2026',
    'office-pro': 'officePro',
    'office-basic': 'officeBasic',
    'office-ultimate': 'officeUltimate',
  },
  module: {
    'access-management': 'accessManagement',
    'device-operations': 'deviceOperations',
    'advanced-reporting': 'advancedReporting',
    'customer-management': 'customerManagement',
    marketing: 'marketing',
    analytics: 'analytics',
  },
  entitlementKey: {
    'tenant.members': 'tenantMembers',
    'tenant.devices': 'tenantDevices',
    'monthly.reports': 'monthlyReports',
    'tenant.member.lifecycle': 'tenantMemberLifecycle',
    'device.lifecycle': 'deviceLifecycle',
    'customer.view': 'customerView',
    'customer.count': 'customerCount',
    'member.count': 'memberCount',
    'marketing.campaign.publish': 'marketingCampaignPublish',
  },
  subscriptionState: {
    ACTIVE: 'active', active: 'active',
    TRIAL: 'trial', trial: 'trial',
    GRACE_PERIOD: 'gracePeriod', grace_period: 'gracePeriod',
    SUSPENDED: 'suspended', suspended: 'suspended',
    EXPIRED: 'expired', expired: 'expired',
    TERMINATED: 'terminated', terminated: 'terminated',
  },
  entitlementTarget: {
    ENTITLEMENT_TARGET_MODULE: 'module',
    ENTITLEMENT_TARGET_CAPABILITY: 'capability',
    ENTITLEMENT_TARGET_QUOTA: 'quota',
    ENTITLEMENT_TARGET_FIELD: 'field',
  },
  entitlementEffect: {
    ENTITLEMENT_EFFECT_GRANT: 'grant',
    ENTITLEMENT_EFFECT_DENY: 'deny',
    ENTITLEMENT_EFFECT_QUOTA_ADD: 'quotaAdd',
    ENTITLEMENT_EFFECT_QUOTA_REPLACE: 'quotaReplace',
    ENTITLEMENT_EFFECT_SAFETY_DENY: 'safetyDeny',
    ENTITLEMENT_EFFECT_SAFETY_MASK: 'safetyMask',
    GRANT: 'grant', ALLOW: 'grant', DENY: 'deny',
  },
  fieldAction: { read: 'read', write: 'write', export: 'export' },
  sourceKind: {
    override: 'override', plan: 'plan', addon: 'addon', 'add-on': 'addon', subscription: 'subscription',
  },
  sourceState: {
    ACTIVE: 'active', active: 'active',
    REVOKED: 'revoked', revoked: 'revoked',
    EXPIRED: 'expired', expired: 'expired',
    PENDING: 'pending', pending: 'pending',
    SCHEDULED: 'scheduled', scheduled: 'scheduled',
  },
  disposition: { applied: 'applied', effective: 'effective', used: 'used', ignored: 'ignored', skipped: 'skipped' },
  decisionKind: { capability: 'capability', quota: 'quota', field: 'field', module: 'module' },
  changeClassification: {
    UPGRADE: 'upgrade', DOWNGRADE: 'downgrade', RENEWAL: 'renewal', STOP_RENEWAL: 'stopRenewal', SWITCH: 'switch',
  },
  effectiveMode: { IMMEDIATE: 'immediate', SCHEDULED: 'scheduled', PROVISIONING: 'provisioning' },
  receiptStatus: { APPLIED: 'applied', SCHEDULED: 'scheduled', PROVISIONING: 'provisioning', FAILED: 'failed', PENDING: 'pending' },
  changeAction: { SWITCH: 'switch', RENEW: 'renew', STOP_RENEWAL: 'stopRenewal' },
  salesScope: { rental: 'rental', office: 'office', default: 'default', enterprise: 'enterprise' },
  technicalStatus: {
    MODULE_TECHNICAL_STATUS_READY: 'ready',
    MODULE_TECHNICAL_STATUS_NOT_READY: 'notReady',
    MODULE_TECHNICAL_STATUS_DISABLED: 'disabled',
    MODULE_TECHNICAL_STATUS_UNSPECIFIED: 'unspecified',
  },
  salesStatus: {
    MODULE_SALES_STATUS_SELLABLE: 'sellable',
    MODULE_SALES_STATUS_RETIRED: 'retired',
    MODULE_SALES_STATUS_UNSPECIFIED: 'unspecified',
  },
  memberStatus: {
    TENANT_MEMBER_STATUS_INVITED: 'invited', invited: 'invited',
    TENANT_MEMBER_STATUS_ACTIVE: 'active', active: 'active',
    TENANT_MEMBER_STATUS_SUSPENDED: 'suspended', suspended: 'suspended',
    TENANT_MEMBER_STATUS_REMOVED: 'removed', removed: 'removed',
  },
  roleStatus: {
    TENANT_ROLE_STATUS_ACTIVE: 'active', active: 'active',
    TENANT_ROLE_STATUS_DISABLED: 'disabled', disabled: 'disabled',
  },
  departmentStatus: {
    TENANT_DEPARTMENT_STATUS_ACTIVE: 'active', active: 'active',
    TENANT_DEPARTMENT_STATUS_DISABLED: 'disabled', disabled: 'disabled',
  },
  dataScope: {
    DATA_SCOPE_NONE: 'none', none: 'none',
    DATA_SCOPE_SELF: 'self', self: 'self',
    DATA_SCOPE_SITES: 'sites', sites: 'sites',
    DATA_SCOPE_ALL: 'all', all: 'all',
  },
  permission: {
    'tenant.member.read': 'tenantMemberRead',
    'tenant.member.manage': 'tenantMemberManage',
    'tenant.organization.read': 'tenantOrganizationRead',
    'tenant.organization.manage': 'tenantOrganizationManage',
    'tenant.role.read': 'tenantRoleRead',
    'tenant.role.manage': 'tenantRoleManage',
    'tenant.delegation.read': 'tenantDelegationRead',
    'tenant.delegation.manage': 'tenantDelegationManage',
    'tenant.audit.read': 'tenantAuditRead',
    'tenant.audit.export': 'tenantAuditExport',
    'tenant.profile.read': 'tenantProfileRead',
    'tenant.profile.manage': 'tenantProfileManage',
    'tenant.branding.read': 'tenantBrandingRead',
    'tenant.branding.manage': 'tenantBrandingManage',
    'device.read': 'deviceRead',
    'device.create': 'deviceCreate',
    'device.update': 'deviceUpdate',
    'device.delete': 'deviceDelete',
    'site.read': 'siteRead',
    'tenant.entitlement.read': 'tenantEntitlementRead',
    'commercial.catalog.read': 'commercialCatalogRead',
  },
  permissionGroup: {
    member: 'member',
    organization: 'organization',
    role: 'role',
    delegation: 'delegation',
    audit: 'audit',
    enterpriseInfo: 'enterpriseInfo',
    deviceOperations: 'deviceOperations',
    enterpriseEntitlements: 'enterpriseEntitlements',
    uncategorized: 'uncategorized',
  },
  permissionAction: {
    read: 'read',
    manage: 'manage',
    assign: 'assign',
    export: 'export',
    create: 'create',
    update: 'update',
    delete: 'delete',
  },
  reason: {
    'plan grant': 'planGrant',
    'not in plan': 'notInPlan',
    'plan limit': 'planLimit',
    allowed: 'allowed',
    denies: 'denied',
    denied: 'denied',
  },
} as const

export type BackendTermKind = keyof typeof backendTermCatalog
export type BackendErrorScope = 'profile' | 'branding' | 'member' | 'role' | 'department' | 'audit' | 'planChange' | 'planRead' | 'commercial'

function rawValue(value: unknown) {
  return String(value ?? '').trim()
}

export function backendTermKnown(kind: BackendTermKind, value: unknown) {
  const raw = rawValue(value)
  return Boolean(raw && Object.hasOwn(backendTermCatalog[kind], raw))
}

export function backendTermKey(kind: BackendTermKind, value: unknown) {
  const raw = rawValue(value)
  const semantic = raw ? (backendTermCatalog[kind] as Record<string, string>)[raw] : undefined
  return semantic ? `backendTerms.${kind}.${semantic}` : `backendTerms.fallback.${kind}`
}

export function backendTermLabel(kind: BackendTermKind, value: unknown) {
  return t(backendTermKey(kind, value))
}

export function backendErrorFallback(scope: BackendErrorScope) {
  return t(`backendTerms.errorFallback.${scope}`)
}

const engineeringText = /(?:^[A-Z][A-Z0-9_]{2,}$|[a-z0-9]+(?:[._-][a-z0-9]+){1,}|\b(?:server|readback|preview|receipt|resolver|catalog|idempotency|runtime)\b)/i

export function backendBusinessText(value: unknown) {
  const raw = rawValue(value)
  if (!raw) return backendTermLabel('reason', raw)
  if (backendTermKnown('reason', raw)) return backendTermLabel('reason', raw)
  if (/[\u3400-\u9fff]/u.test(raw) && !engineeringText.test(raw)) return raw
  return backendTermLabel('reason', raw)
}
