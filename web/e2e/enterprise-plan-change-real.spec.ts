import { expect, test, type Page, type Route } from '@playwright/test'
import { installApiFailFast } from './ui.helpers'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_PLAN_CHANGE_E2E, 'runs only against the VITE_DATA_MODE=api build')

type ReceiptStatus = 'APPLIED' | 'SCHEDULED' | 'PROVISIONING'
type Options = {
  paid?: boolean
  receiptStatus?: ReceiptStatus
  readbackStatus?: ReceiptStatus
  pendingStatus?: ReceiptStatus
  structuredImpacts?: boolean
  quotaValidationRequired?: boolean
  confirmError?: string
}
type Captured = {
  previewBodies: Array<Record<string, unknown>>
  previewHeaders: Array<Record<string, string>>
  confirmBodies: Array<Record<string, unknown>>
  confirmHeaders: Array<Record<string, string>>
  targetPaths: string[]
  receiptReads: string[]
  historyReads: string[]
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
      currency: paid ? 'CNY' : '',
      amountMinor: paid ? 19900 : 0,
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
    impactDetails: options.structuredImpacts ? [
      { code: 'DATA_PRESERVED', severity: 'INFO', subject: 'tenant_data', before: '', after: '', usageKnown: false, currentUsage: '0', blocking: false, actionRequired: '', messageKey: 'subscription_change.data_preserved', messageParameters: {} },
      { code: 'QUOTA_REVALIDATION', severity: 'WARNING', subject: 'quota', before: '10', after: '30', usageKnown: true, currentUsage: '6', blocking: true, actionRequired: 'REVALIDATE_QUOTA', messageKey: 'subscription_change.quota_revalidation', messageParameters: {} },
    ] : [],
    pricingBasis: options.paid ? 'PLATFORM_MANUAL_APPROVAL_REQUIRED' : 'NO_PRICE_REFERENCE',
    quotaValidationRequired: Boolean(options.quotaValidationRequired),
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
  await installApiFailFast(page)
  const captured: Captured = { previewBodies: [], previewHeaders: [], confirmBodies: [], confirmHeaders: [], targetPaths: [], receiptReads: [], historyReads: [] }
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
  await page.route('**/api/auth/authorization', async (route) => {
    const buttonCodes = ["commercial.subscription.get_my","commercial.subscription.get_my_usage","commercial.entitlement.get_my","commercial.subscription.change.targets_my","commercial.subscription.change.preview_my","commercial.subscription.change.preview_my.get","commercial.subscription.change.confirm_my","commercial.subscription.change.get_my"]
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-001',
      tenant_id: 'tenant-001',
      tenant_name: 'CoffeeLink 测试租户',
      timezone: 'Asia/Shanghai',
      roles: ['operator'],
      grants: [],
      data_policies: [],
      site_ids: [],
      permission_version: 'sha256:e2e',
      modules: [{ code: 'access-management', allowed: true, reason: 'allowed', actions: buttonCodes }],
      actions: buttonCodes.map((code) => ({ code, permissions: [], permission_mode: 'all' })),
      button_codes: buttonCodes,
    })
  })
  await page.route('**/api/v1/tenant/subscription', (route) => {
    const pending = options.pendingStatus ?? (confirmed && status !== 'APPLIED' ? status : undefined)
    return json(route, 200, subscription(pending && pending !== 'APPLIED' ? 'chg-tenant-preview-001' : '', confirmed && status === 'APPLIED'))
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
    if (options.confirmError) return json(route, 409, { message: options.confirmError })
    confirmed = true
    return json(route, 200, receiptBody(status))
  })
  await page.route('**/api/v1/tenant/subscription/changes/chg-tenant-preview-001', (route) => {
    captured.receiptReads.push(route.request().method())
    return json(route, 200, receiptBody(options.readbackStatus ?? options.pendingStatus ?? status))
  })
  await page.route(/\/api\/v1\/tenant\/subscription\/changes(?:\?.*)?$/, (route) => {
    captured.historyReads.push(route.request().method())
    return json(route, 200, { receipts: [receiptBody('APPLIED')], nextBeforeConfirmedAt: '', nextBeforeChangeId: '' })
  })
  return captured
}

async function openFlow(page: Page) {
  await page.goto('/#/enterprise/plan')
  await expect(page.locator('[data-enterprise-page="plan"]')).toBeVisible()
  await page.getByRole('button', { name: '管理套餐变更', exact: true }).click()
  const lifecycle = page.locator('[data-plan-change-lifecycle]')
  await lifecycle.locator('[data-plan-change-open]').click()
  await lifecycle.scrollIntoViewIfNeeded()
  await expect(lifecycle.getByText(/专业版/).first()).toBeVisible()
  return lifecycle
}

async function selectTargetAndPreview(page: Page) {
  const lifecycle = page.locator('[data-plan-change-lifecycle]')
  await lifecycle.getByRole('button', { name: /专业版/ }).click()
  await lifecycle.locator('[data-plan-change-preview]').click()
  await expect(lifecycle.getByText('升级', { exact: true }).first()).toBeVisible()
  await expect(lifecycle.locator('.steps').getByText(/确认方案/).first()).toBeVisible()
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

test('tenant can inspect selectable plan core terms before creating a preview', async ({ page }) => {
  await mockServer(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  const lifecycle = await openFlow(page)
  const target = lifecycle.getByRole('button', { name: /专业版/ })
  await expect(target).toContainText('免费开通')
  await expect(target).toContainText('365 天')
  await expect(target).toContainText('2 个模块')
  await target.click()
  const terms = lifecycle.locator('[data-plan-target-details]')
  await expect(terms).toContainText('成员与权限')
  await expect(terms).toContainText('设备管理')
  await expect(terms).toContainText('成员生命周期')
  await expect(terms).toContainText('成员额度：30')
  await expect(terms).toContainText('设备生命周期')
})

test('tenant preview carries no tenant authority fields and renders quota impact', async ({ page }) => {
  const captured = await mockServer(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await openFlow(page)
  await selectTargetAndPreview(page)
  await expect.poll(() => captured.previewBodies.length).toBe(1)
  expect(captured.previewBodies[0]?.tenantId).toBeUndefined()
  expect(captured.previewBodies[0]?.reason).toBeUndefined()
  expect(captured.previewBodies[0]?.salesScope).toBeUndefined()
  expect(captured.previewBodies[0]?.used).toBeUndefined()
  expect(captured.previewHeaders[0]?.['x-biz-session-context']).toContain('tenant-001')
  expect(captured.previewHeaders[0]?.['x-csrf-token']).toBe('csrf-plan-change')
  expect(captured.previewHeaders[0]?.['idempotency-key']).toBeTruthy()
  await expect(page.getByText('已用 6')).toBeVisible()
  await expect(page.getByText('现有租户数据将被保留，本次操作不会删除资源。')).toBeVisible()
  await expect(page.getByText('Existing tenant data is preserved; this operation never deletes resources.')).toHaveCount(0)
  await page.locator('[data-plan-change-lifecycle]').scrollIntoViewIfNeeded()
  await page.screenshot({ path: screenshot('enterprise-plan-change-preview-1440'), fullPage: false })
})

test('tenant renders structured impacts instead of legacy free-text impact strings', async ({ page }) => {
  await mockServer(page, { structuredImpacts: true })
  const lifecycle = await openFlow(page)
  await selectTargetAndPreview(page)
  await expect(lifecycle.getByText('现有数据保留', { exact: true })).toBeVisible()
  await expect(lifecycle.getByText('需要重新核对额度', { exact: true })).toBeVisible()
  await expect(lifecycle.getByText('当前用量：6', { exact: true })).toBeVisible()
  await expect(lifecycle.getByText('Existing tenant data is preserved; this operation never deletes resources.')).toHaveCount(0)
})

test('tenant history renders receipt facts from the tenant-scoped history authority', async ({ page }) => {
  const captured = await mockServer(page)
  await page.goto('/#/enterprise/plan')
  await page.getByRole('tab', { name: '变更记录', exact: true }).click()
  await expect(page.getByText('切换套餐 · 已生效', { exact: true })).toBeVisible()
  await expect(page.getByText('chg-tenant-preview-001', { exact: false })).toBeVisible()
  expect(captured.historyReads).toEqual(['GET'])
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

test('tenant cannot submit a change while quota revalidation is required', async ({ page }) => {
  await mockServer(page, { quotaValidationRequired: true })
  await page.setViewportSize({ width: 1440, height: 900 })
  await openFlow(page)
  await selectTargetAndPreview(page)
  const lifecycle = page.locator('[data-plan-change-lifecycle]')
  await expect(lifecycle).toContainText('确认前需要复核')
  await expect(lifecycle.locator('[data-plan-change-confirm]')).toBeDisabled()
})

test('tenant sees a recoverable prompt when the preview has expired', async ({ page }) => {
  const captured = await mockServer(page, { confirmError: 'SUBSCRIPTION_CHANGE_PREVIEW_EXPIRED' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await openFlow(page)
  await selectTargetAndPreview(page)
  await page.locator('[data-plan-change-confirm]').click()
  await page.locator('[data-plan-change-confirm-dialog-submit]').click()
  await expect(page.locator('[data-plan-change-lifecycle]')).toContainText('变更方案已过期，请重新生成方案。')
  expect(captured.confirmBodies).toHaveLength(1)
  await expect(page.locator('[data-plan-change-receipt]')).toHaveCount(0)
})

test('tenant refreshes a scheduled result until the applied receipt is read back', async ({ page }) => {
  const captured = await mockServer(page, { receiptStatus: 'SCHEDULED', readbackStatus: 'APPLIED' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await openFlow(page)
  await selectTargetAndPreview(page)
  await page.locator('[data-plan-change-confirm]').click()
  await page.locator('[data-plan-change-confirm-dialog-submit]').click()
  const receipt = page.locator('[data-plan-change-receipt]')
  await expect(receipt).toContainText('已预约')
  await receipt.locator('[data-plan-change-receipt-refresh]').click()
  await expect(receipt).toContainText('已生效')
  expect(captured.receiptReads).toEqual(['GET'])
})

test('tenant restores a pending change from trusted subscription state after reload', async ({ page }) => {
  const captured = await mockServer(page, { pendingStatus: 'SCHEDULED' })
  await page.goto('/#/enterprise/plan')
  const lifecycle = page.locator('[data-plan-change-lifecycle]')
  const receipt = lifecycle.locator('[data-plan-change-receipt]')
  await expect(receipt).toContainText('已预约')
  await expect(receipt).toContainText('chg-tenant-preview-001')
  expect(captured.receiptReads).toEqual(['GET'])
})

test('tenant confirms the selected preview in a final summary dialog', async ({ page }) => {
  const captured = await mockServer(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await openFlow(page)
  await selectTargetAndPreview(page)
  await page.locator('[data-plan-change-confirm]').click()
  const dialog = page.locator('[data-plan-change-confirm-dialog]')
  await expect(dialog).toContainText('专业版')
  await expect(dialog).toContainText('免费开通')
  expect(captured.confirmBodies).toHaveLength(0)
  await page.locator('[data-plan-change-confirm-dialog-submit]').click()
  await expect(page.locator('[data-plan-change-receipt]')).toContainText('已生效')
})

for (const [status, displayStatus] of [
  ['APPLIED', '已生效'],
  ['SCHEDULED', '已预约'],
  ['PROVISIONING', '处理中'],
] as const) {
  test(`tenant no-price change shows ${displayStatus} result`, async ({ page }) => {
    const captured = await mockServer(page, { receiptStatus: status })
    await page.setViewportSize({ width: 1440, height: 900 })
    await openFlow(page)
    await selectTargetAndPreview(page)
    await page.locator('[data-plan-change-confirm]').click()
    await page.locator('[data-plan-change-confirm-dialog-submit]').click()
    await expect(page.locator('[data-plan-change-receipt]')).toContainText(displayStatus)
    await expect.poll(() => captured.confirmBodies.length).toBe(1)
    expect(captured.confirmBodies[0]?.tenantId).toBeUndefined()
    expect(captured.confirmBodies[0]?.reason).toBeUndefined()
    expect(captured.confirmBodies[0]?.previewHash).toBe('a'.repeat(64))
    expect(captured.confirmHeaders[0]?.['x-biz-session-context']).toContain('tenant-001')
    expect(captured.confirmHeaders[0]?.['x-csrf-token']).toBe('csrf-plan-change')
    expect(captured.confirmHeaders[0]?.['idempotency-key']).toBeTruthy()
    await expect(page.locator('[data-plan-change-receipt]')).toContainText(displayStatus)
    await expect(page.locator('[data-plan-change-lifecycle]')).toContainText('chg-tenant-preview-001')
    await page.locator('[data-plan-change-lifecycle]').scrollIntoViewIfNeeded()
    await page.screenshot({ path: screenshot(`enterprise-plan-change-${status.toLowerCase()}-1440`), fullPage: false })
  })
}
