import { expect, test, type Page, type Route } from '@playwright/test'

async function fulfillJson(route: Route, body: unknown, status = 200) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

function subscription(planCode = 'office-pro', planVersion = '2') {
  return {
    subscriptionId: 'sub-1',
    tenantId: 'tenant-1',
    kind: 'base',
    state: 'ACTIVE',
    planCode,
    planVersion,
    ruleId: 'default-office',
    ruleVersion: '1',
    salesScope: 'default',
    entitlementSourceVersion: '7',
    createdAt: '2026-09-10T00:00:00Z',
    matchExplanation: 'matched default sales scope',
    revision: '4',
    periodStart: '2026-09-10T00:00:00Z',
    periodEnd: '2026-10-10T00:00:00Z',
    renewalStopped: false,
    pendingChangeId: '',
  }
}

function entitlement() {
  return {
    tenantId: 'tenant-1',
    sourceVersion: '7',
    resolverVersion: '4',
    evaluatedAt: '2026-09-12T05:00:00Z',
    validUntil: '',
    nextTransitionAt: '',
    catalogVersions: [{ moduleCode: 'device', version: '3' }],
    decisions: [],
    entitlementVersion: '11',
    catalogRevision: '3',
    permissionVersion: '',
    permissionSubject: '',
  }
}

function targetPlan(code: string, version: string) {
  return {
    planCode: code,
    version,
    revision: '1',
    planRevision: '3',
    state: 'PUBLISHED',
    name: code === 'office-basic' ? '办公室基础版' : '办公室旗舰版',
    terms: {
      modules: [],
      salesScope: ['default'],
      validityMode: 'fixed_days',
      validityDays: 30,
      priceRef: `price:${code}`,
    },
    contentSha256: 'abcdef',
    createdAt: '2026-09-01T00:00:00Z',
    publishedAt: '2026-09-02T00:00:00Z',
    retiredAt: '',
    actorId: 'platform-admin',
    reason: 'published',
  }
}

async function setupWorkspace(page: Page, current = subscription()) {
  await page.route('**/api/v1/platform/modules', (route) => fulfillJson(route, { modules: [] }))
  await page.route('**/api/auth/session', (route) => fulfillJson(route, {
    authenticated: true,
    actor_kind: 'platform',
    csrf_token: 'csrf-subscription-change',
  }))
  await page.route('**/api/v1/platform/tenants/tenant-1/subscription', (route) => fulfillJson(route, current))
  await page.route('**/api/v1/platform/tenants/tenant-1/entitlement-overrides', (route) => {
    if (route.request().method() === 'GET') return fulfillJson(route, { sources: [], sourceVersion: '7' })
    return route.continue()
  })
  await page.route('**/api/v1/platform/tenants/tenant-1/entitlements', (route) => fulfillJson(route, entitlement()))

  await page.goto('/#/platform/commercial/tenant-entitlements')
  await page.getByLabel('租户 ID').fill('tenant-1')
  await page.getByRole('button', { name: '读取权益' }).click()
  await expect(page.getByText('office-pro v2')).toBeVisible()
}

test('TestCE13SubscriptionChangePreviewUsesServerImpactAndMatchingIdempotencyKey', async ({ page }) => {
  await setupWorkspace(page)

  let previewBody: Record<string, unknown> | undefined
  let previewHeaders: Record<string, string> | undefined
  await page.route('**/api/v1/platform/tenants/tenant-1/subscription/change-previews', async (route) => {
    previewBody = route.request().postDataJSON() as Record<string, unknown>
    previewHeaders = route.request().headers()
    await fulfillJson(route, {
      changeId: 'chg-down-1',
      tenantId: 'tenant-1',
      actorId: 'platform-admin',
      requestId: previewBody?.requestId,
      action: 'SWITCH',
      classification: 'DOWNGRADE',
      mode: 'SCHEDULED',
      previewHash: 'preview-hash-down',
      before: subscription(),
      target: targetPlan('office-basic', '1'),
      subscriptionRevision: '4',
      sourceVersion: '7',
      entitlementVersion: '11',
      catalogRevision: '3',
      createdAt: '2026-09-12T05:00:00Z',
      expiresAt: '2099-09-12T05:10:00Z',
      effectiveAt: '2099-10-10T00:00:00Z',
      entitlementExpiresAt: '2099-11-09T00:00:00Z',
      currentEntitlements: entitlement(),
      projectedEntitlements: { ...entitlement(), entitlementVersion: '0' },
      dependencies: [{ moduleCode: 'device', requiresModules: ['customer'] }],
      quotaImpacts: [{
        moduleCode: 'device',
        key: 'device.count',
        beforeLimit: { unlimited: false, value: '100' },
        afterLimit: { unlimited: false, value: '20' },
        usageKnown: false,
        used: '0',
        overLimit: false,
        policy: 'REVALIDATE_AT_EXECUTION',
        evidence: 'CE-19 usage adapter not connected',
      }],
      impacts: ['设备额度从 100 降至 20', '部分能力将在预约执行后失效'],
      pricingBasis: 'price_ref:office-basic',
      quotaValidationRequired: true,
      provisioningRequirements: [{ code: 'sync-device-policy', adapter: 'iot', version: 'v1', maxAttempts: 5 }],
    })
  })

  const workspace = page.getByTestId('ce13-subscription-change')
  await workspace.getByLabel('目标 plan_code').fill('office-basic')
  await workspace.getByLabel('目标版本').fill('1')
  await workspace.getByLabel('预览原因').fill('客户缩减规模，需要下周期降级')
  await workspace.getByRole('button', { name: '生成不可变预览' }).click()

  await expect(workspace.getByText('DOWNGRADE')).toBeVisible()
  await expect(workspace.getByText('SCHEDULED')).toBeVisible()
  await expect(workspace.getByText('确认前仍需额度再校验')).toBeVisible()
  await expect(workspace.getByText('price_ref:office-basic')).toBeVisible()
  await expect(workspace.getByText('REVALIDATE_AT_EXECUTION')).toBeVisible()
  await expect(workspace.getByText(/SCHEDULED 只表示预约已保存/)).toHaveCount(0)

  const requestId = String(previewBody?.requestId)
  expect(requestId).toContain('ce13-subscription-preview-')
  expect(previewHeaders?.['idempotency-key']).toBe(requestId)
  expect(previewHeaders?.['x-csrf-token']).toBe('csrf-subscription-change')
  expect(previewHeaders?.authorization).toBeUndefined()
  expect(previewBody).toMatchObject({
    tenantId: 'tenant-1',
    action: 'SWITCH',
    targetPlanCode: 'office-basic',
    targetPlanVersion: '1',
    reason: '客户缩减规模，需要下周期降级',
  })
})

test('TestCE13SubscriptionChangeConfirmPinsPreviewHashAndRendersImmutableReceipt', async ({ page }) => {
  let current = subscription()
  await setupWorkspace(page, current)

  await page.route('**/api/v1/platform/tenants/tenant-1/subscription/change-previews', async (route) => {
    const body = route.request().postDataJSON() as Record<string, unknown>
    await fulfillJson(route, {
      changeId: 'chg-up-1',
      tenantId: 'tenant-1',
      actorId: 'platform-admin',
      requestId: body.requestId,
      action: 'SWITCH',
      classification: 'UPGRADE',
      mode: 'IMMEDIATE',
      previewHash: 'preview-hash-up',
      before: current,
      target: targetPlan('office-ultimate', '3'),
      subscriptionRevision: '4',
      sourceVersion: '7',
      entitlementVersion: '11',
      catalogRevision: '3',
      createdAt: '2026-09-12T05:00:00Z',
      expiresAt: '2099-09-12T05:10:00Z',
      effectiveAt: '2026-09-12T05:00:00Z',
      entitlementExpiresAt: '',
      currentEntitlements: entitlement(),
      projectedEntitlements: { ...entitlement(), entitlementVersion: '0' },
      dependencies: [],
      quotaImpacts: [],
      impacts: ['新增旗舰能力'],
      pricingBasis: 'price_ref:office-ultimate',
      quotaValidationRequired: false,
      provisioningRequirements: [],
    })
  })

  let confirmBody: Record<string, unknown> | undefined
  let confirmHeaders: Record<string, string> | undefined
  await page.route('**/api/v1/platform/tenants/tenant-1/subscription/changes/chg-up-1/confirm', async (route) => {
    confirmBody = route.request().postDataJSON() as Record<string, unknown>
    confirmHeaders = route.request().headers()
    current = { ...subscription('office-ultimate', '3'), revision: '5', entitlementSourceVersion: '8' }
    await fulfillJson(route, {
      changeId: 'chg-up-1',
      tenantId: 'tenant-1',
      actorId: 'platform-admin',
      requestId: confirmBody?.requestId,
      previewHash: 'preview-hash-up',
      action: 'SWITCH',
      status: 'APPLIED',
      mode: 'IMMEDIATE',
      confirmedAt: '2026-09-12T05:01:00Z',
      effectiveAt: '2026-09-12T05:01:00Z',
      entitlementExpiresAt: '',
      reason: '平台主管确认升级',
      before: subscription(),
      after: current,
      beforeSourceVersion: '7',
      afterSourceVersion: '8',
      beforeEntitlementVersion: '11',
      afterEntitlementVersion: '12',
      quotaValidationRequired: false,
      pricingAuthority: 'PLATFORM_MANUAL_APPROVAL',
      quotaImpacts: [],
      provisioningTaskId: '',
    })
  })

  const workspace = page.getByTestId('ce13-subscription-change')
  await workspace.getByLabel('目标 plan_code').fill('office-ultimate')
  await workspace.getByLabel('目标版本').fill('3')
  await workspace.getByLabel('预览原因').fill('客户批准升级旗舰套餐')
  await workspace.getByRole('button', { name: '生成不可变预览' }).click()
  await expect(workspace.getByText('UPGRADE')).toBeVisible()

  await workspace.getByLabel(/PLATFORM_MANUAL_APPROVAL/).check()
  await workspace.getByLabel('确认原因').fill('平台主管确认升级')
  await workspace.getByRole('button', { name: '确认此 preview_hash' }).click()

  await expect(workspace.getByText('不可变变更回执')).toBeVisible()
  await expect(workspace.getByText('APPLIED').first()).toBeVisible()
  await expect(workspace.getByText('office-ultimate v3')).toBeVisible()
  await expect(workspace.getByText('PLATFORM_MANUAL_APPROVAL')).toBeVisible()

  const requestId = String(confirmBody?.requestId)
  expect(requestId).toContain('ce13-subscription-confirm-')
  expect(confirmHeaders?.['idempotency-key']).toBe(requestId)
  expect(confirmHeaders?.['x-csrf-token']).toBe('csrf-subscription-change')
  expect(confirmHeaders?.authorization).toBeUndefined()
  expect(confirmBody).toMatchObject({
    tenantId: 'tenant-1',
    changeId: 'chg-up-1',
    previewHash: 'preview-hash-up',
    reason: '平台主管确认升级',
  })
})
