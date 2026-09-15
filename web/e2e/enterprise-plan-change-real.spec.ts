import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_PLAN_CHANGE_E2E, 'runs only against the VITE_DATA_MODE=api build')

type ReceiptStatus = 'APPLIED' | 'SCHEDULED' | 'PROVISIONING'
type Options = {
  paid?: boolean
  receiptStatus?: ReceiptStatus
}
type Captured = {
  previewBodies: Array<Record<string, unknown>>
  previewHeaders: Array<Record<string, string>>
  confirmBodies: Array<Record<string, unknown>>
  confirmHeaders: Array<Record<string, string>>
  targetPaths: string[]
}

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

function subscription(pendingChangeId = '', changed = false) {
  return {
    subscriptionId: 'sub-authoritative-001',
    tenantId: 'tenant-001',
    kind: 'base',
    state: 'active',
    planCode: changed ? 'rental-pro-2026' : 'rental-growth-2026',
    planVersion: changed ? 4 : 3,
    salesScope: 'rental',
    entitlementSourceVersion: changed ? 12 : 11,
    createdAt: '2026-09-01T00:00:00Z',
    revision: changed ? 9 : 8,
    periodStart: '2026-09-01T00:00:00Z',
    periodEnd: '2027-09-01T00:00:00Z',
    renewalStopped: false,
    pendingChangeId,
  }
}

function entitlements() {
  return {
    tenantId: 'tenant-001',
    sourceVersion: 11,
    resolverVersion: 2,
    entitlementVersion: 19,
    catalogRevision: 7,
    permissionVersion: 'permission-fingerprint-001',
    permissionSubject: 'user-001',
    evaluatedAt: '2026-09-15T03:00:00Z',
    decisions: [
      { kind: 'module', moduleCode: 'access-management', key: 'access-management', allowed: true, reason: 'plan grant' },
      { kind: 'module', moduleCode: 'device-operations', key: 'device-operations', allowed: true, reason: 'plan grant' },
      { kind: 'quota', moduleCode: 'access-management', key: 'tenant.members', allowed: true, limit: { unlimited: false, value: 10 }, reason: 'plan limit' },
    ],
  }
}

function target(paid = false) {
  return {
    planCode: 'rental-pro-2026',
    version: 4,
    revision: 2,
    planRevision: 8,
    state: 'published',
    name: paid ? '专业版（需商业审批）' : '专业版',
    terms: {
      modules: [
        { moduleCode: 'access-management', capabilityCodes: ['tenant.member.lifecycle'], quotas: [{ key: 'tenant.members', unlimited: false, value: 30 }], fields: [] },
        { moduleCode: 'device-operations', capabilityCodes: ['device.lifecycle'], quotas: [], fields: [] },
      ],
      salesScope: ['rental'],
      validityMode: 'fixed_days',
      validityDays: 365,
      priceRef: paid ? 'price-rental-pro-annual' : '',
    },
    contentSha256: 'target-content-sha-001',
    createdAt: '2026-08-01T00:00:00Z',
    publishedAt: '2026-08-15T00:00:00Z',
    retiredAt: '',
    actorId: 'platform-admin',
    reason: 'published target',
  }
}

function previewBody(options: Options = {}) {
  const receiptStatus = options.receiptStatus ?? 'APPLIED'
  const scheduled = receiptStatus === 'SCHEDULED'
  const provisioning = receiptStatus === 'PROVISIONING'
  return {
    changeId: 'chg-tenant-preview-001',
    tenantId: 'tenant-001',
    actorId: 'user-001',
    requestId: 'tenant-plan-preview-fixed',
    action: 'SWITCH',
    classification: 'UPGRADE',
    mode: scheduled ? 'SCHEDULED' : 'IMMEDIATE',
    previewHash: 'a'.repeat(64),
    before: subscription(),
    target: target(options.paid),
    subscriptionRevision: 8,
    sourceVersion: 11,
    entitlementVersion: 19,
    catalogRevision: 7,
    createdAt: '2026-09-15T06:00:00Z',
    expiresAt: '2026-09-15T06:10:00Z',
    effectiveAt: scheduled ? '2026-10-01T00:00:00Z' : '2026-09-15T06:00:05Z',
    entitlementExpiresAt: '2027-09-15T06:00:05Z',
    currentEntitlements: entitlements(),
    projectedEntitlements: { ...entitlements(), entitlementVersion: 0, sourceVersion: 12 },
    dependencies: [{ moduleCode: 'device-operations', requiresModules: ['access-management'] }],
    quotaImpacts: [{
      moduleCode: 'access-management',
      key: 'tenant.members',
      beforeLimit: { unlimited: false, value: 10 },
      afterLimit: { unlimited: false, value: 30 },
      usageKnown: true,
      used: 6,
      overLimit: false,
      policy: 'AUTHORITATIVE_USAGE_OK',
      evidence: 'tenant.members authoritative meter',
    }],
    impacts: [
      'Existing tenant data is preserved; this operation never deletes resources.',
      scheduled ? 'Scheduled intent only: existing rights remain unchanged until execution.' : 'Immediate change is revalidated at confirmation.',
      ...(options.paid ? ['This target carries a price reference. Tenant confirmation requires external commercial/payment approval.'] : []),
    ],
    pricingBasis: options.paid ? 'PLATFORM_MANUAL_APPROVAL_REQUIRED' : 'NO_PRICE_REFERENCE',
    quotaValidationRequired: false,
    provisioningRequirements: provisioning ? [{ code: 'external-license', adapter: 'license-adapter', version: 'v1', maxAttempts: 3 }] : [],
  }
}

function receiptBody(status: ReceiptStatus) {
  const pending = status === 'APPLIED' ? '' : 'chg-tenant-preview-001'
  return {
    changeId: 'chg-tenant-preview-001',
    tenantId: 'tenant-001',
    actorId: 'user-001',
    requestId: 'tenant-plan-confirm-fixed',
    previewHash: 'a'.repeat(64),
    action: 'SWITCH',
    status,
    mode: status === 'SCHEDULED' ? 'SCHEDULED' : 'IMMEDIATE',
    confirmedAt: '2026-09-15T06:02:00Z',
    effectiveAt: status === 'SCHEDULED' ? '2026-10-01T00:00:00Z' : '2026-09-15T06:02:00Z',
    entitlementExpiresAt: '2027-09-15T06:02:00Z',
    reason: '租户自服务套餐变更',
    before: subscription(),
    after: subscription(pending, status === 'APPLIED'),
    beforeSourceVersion: 11,
    afterSourceVersion: status === 'APPLIED' ? 12 : 11,
    beforeEntitlementVersion: 19,
    afterEntitlementVersion: status === 'APPLIED' ? 20 : 19,
    quotaValidationRequired: false,
    pricingAuthority: 'TENANT_SELF_SERVICE_NO_PRICE_REFERENCE',
    quotaImpacts: [],
    provisioningTaskId: status === 'PROVISIONING' ? 'task-chg-tenant-preview-001' : '',
  }
}

async function mockServer(page: Page, options: Options = {}): Promise<Captured> {
  const captured: Captured = { previewBodies: [], previewHeaders: [], confirmBodies: [], confirmHeaders: [], targetPaths: [] }
  let confirmed = false
  const status = options.receiptStatus ?? 'APPLIED'

  await page.route('**/api/auth/session', (route) => json(route, 200, {
    authenticated: true,
    actor_kind: 'tenant',
    user_id: 'user-001',
    active_tenant_id: 'tenant-001',
    csrf_token: 'csrf-plan-change',
    tenants: [{ id: 'tenant-001', name: 'CoffeeLink 测试租户' }],
  }))
  await page.route('**/api/v1/tenant/subscription', (route) => {
    const pending = confirmed && status !== 'APPLIED' ? 'chg-tenant-preview-001' : ''
    return json(route, 200, subscription(pending, confirmed && status === 'APPLIED'))
  })
  await page.route('**/api/v1/tenant/entitlements', (route) => json(route, 200, entitlements()))
  await page.route('**/api/v1/tenant/usage', (route) => json(route, 200, {
    usages: [{ moduleCode: 'access-management', key: 'tenant.members', known: true, used: 6, evidence: 'authoritative member meter' }],
  }))
  await page.route('**/api/v1/tenant/subscription/change-targets', (route) => {
    captured.targetPaths.push(new URL(route.request().url()).pathname)
    return json(route, 200, { salesScope: 'rental', targets: [target(options.paid)] })
  })
  await page.route('**/api/v1/tenant/subscription/change-previews', (route) => {
    const request = route.request()
    captured.previewBodies.push(request.postDataJSON() as Record<string, unknown>)
    captured.previewHeaders.push(request.headers())
    return json(route, 200, previewBody(options))
  })
  await page.route('**/api/v1/tenant/subscription/changes/chg-tenant-preview-001/confirm', (route) => {
    const request = route.request()
    captured.confirmBodies.push(request.postDataJSON() as Record<string, unknown>)
    captured.confirmHeaders.push(request.headers())
    if (options.paid) return json(route, 412, { message: 'SUBSCRIPTION_CHANGE_EXTERNAL_APPROVAL_REQUIRED' })
    confirmed = true
    return json(route, 200, receiptBody(status))
  })
  await page.route('**/api/v1/tenant/subscription/changes/chg-tenant-preview-001', (route) => json(route, 200, receiptBody(status)))
  return captured
}

async function openFlow(page: Page) {
  await page.goto('/#/enterprise/plan')
  await expect(page.locator('[data-enterprise-page="plan"]')).toBeVisible()
  await expect(page.locator('[data-enterprise-source="api"]')).toBeVisible()
  await page.locator('[data-plan-change-open]').click()
  const lifecycle = page.locator('[data-plan-change-lifecycle]')
  await lifecycle.scrollIntoViewIfNeeded()
  await expect(lifecycle.getByText(/专业版/).first()).toBeVisible()
  return lifecycle
}

async function selectTargetAndPreview(page: Page) {
  const lifecycle = page.locator('[data-plan-change-lifecycle]')
  await lifecycle.getByRole('button', { name: /专业版/ }).click()
  await lifecycle.locator('[data-plan-change-preview]').click()
  await expect(lifecycle.getByText(/UPGRADE/).first()).toBeVisible()
  await expect(lifecycle.locator('.steps').getByText(/权威预览/).first()).toBeVisible()
}

function screenshot(name: string) {
  mkdirSync('screenshots', { recursive: true })
  return `screenshots/${name}.png`
}

test('tenant change target selection is trusted-tenant scoped and screenshotable', async ({ page }) => {
  const captured = await mockServer(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await openFlow(page)
  await expect.poll(() => captured.targetPaths.length).toBe(1)
  expect(captured.targetPaths).toEqual(['/api/v1/tenant/subscription/change-targets'])
  expect(captured.targetPaths[0]).not.toContain('tenant-001')
  await page.screenshot({ path: screenshot('enterprise-plan-change-targets-1440'), fullPage: false })
})

test('tenant preview carries no tenant authority fields and renders quota impact', async ({ page }) => {
  const captured = await mockServer(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await openFlow(page)
  await selectTargetAndPreview(page)
  await expect.poll(() => captured.previewBodies.length).toBe(1)
  expect(captured.previewBodies[0]?.tenantId).toBeUndefined()
  expect(captured.previewBodies[0]?.salesScope).toBeUndefined()
  expect(captured.previewBodies[0]?.used).toBeUndefined()
  expect(captured.previewHeaders[0]?.['x-biz-session-context']).toContain('tenant-001')
  expect(captured.previewHeaders[0]?.['x-csrf-token']).toBe('csrf-plan-change')
  expect(captured.previewHeaders[0]?.['idempotency-key']).toBeTruthy()
  await expect(page.getByText('已用 6')).toBeVisible()
  await page.locator('[data-plan-change-lifecycle]').scrollIntoViewIfNeeded()
  await page.screenshot({ path: screenshot('enterprise-plan-change-preview-1440'), fullPage: false })
})

test('paid target fails closed at external commercial approval boundary', async ({ page }) => {
  const captured = await mockServer(page, { paid: true })
  await page.setViewportSize({ width: 1440, height: 900 })
  await openFlow(page)
  await selectTargetAndPreview(page)
  await expect(page.locator('[data-plan-change-external-approval]')).toBeVisible()
  await expect(page.locator('[data-plan-change-confirm]')).toBeDisabled()
  expect(captured.confirmBodies).toHaveLength(0)
  await page.locator('[data-plan-change-lifecycle]').scrollIntoViewIfNeeded()
  await page.screenshot({ path: screenshot('enterprise-plan-change-external-approval-1440'), fullPage: false })
})

for (const status of ['APPLIED', 'SCHEDULED', 'PROVISIONING'] as const) {
  test(`tenant no-price change confirms to authoritative ${status} receipt`, async ({ page }) => {
    const captured = await mockServer(page, { receiptStatus: status })
    await page.setViewportSize({ width: 1440, height: 900 })
    await openFlow(page)
    await selectTargetAndPreview(page)
    await page.locator('[data-plan-change-confirm]').click()
    await expect(page.locator('[data-plan-change-receipt]')).toContainText(status)
    await expect.poll(() => captured.confirmBodies.length).toBe(1)
    expect(captured.confirmBodies[0]?.tenantId).toBeUndefined()
    expect(captured.confirmBodies[0]?.previewHash).toBe('a'.repeat(64))
    expect(captured.confirmHeaders[0]?.['x-biz-session-context']).toContain('tenant-001')
    expect(captured.confirmHeaders[0]?.['x-csrf-token']).toBe('csrf-plan-change')
    expect(captured.confirmHeaders[0]?.['idempotency-key']).toBeTruthy()
    if (status !== 'APPLIED') await expect(page.getByText(/待处理 chg-tenant-preview-001/)).toBeVisible()
    await page.locator('[data-plan-change-lifecycle]').scrollIntoViewIfNeeded()
    await page.screenshot({ path: screenshot(`enterprise-plan-change-${status.toLowerCase()}-1440`), fullPage: false })
  })
}
