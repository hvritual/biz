import { selectUiOption } from './ui.helpers'
import { expect, test, type Page, type Route } from '@playwright/test'

async function fulfillJson(route: Route, body: unknown, status = 200) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

const moduleCatalog = {
  modules: [{
    moduleCode: 'device',
    name: '设备管理',
    category: 'business',
    salesScope: ['default'],
    technicalStatus: 'MODULE_TECHNICAL_STATUS_READY',
    salesStatus: 'MODULE_SALES_STATUS_SELLABLE',
    capabilityCodes: ['device.lifecycle'],
    quotaSchemaKeys: ['device.count'],
    fieldPolicySchemaKeys: ['device.serial'],
    dependencies: [],
    version: '3',
  }],
}

async function mockModules(page: Page) {
  await page.route('**/api/v1/platform/modules', (route) => fulfillJson(route, moduleCatalog))
}

function subscription() {
  return {
    subscriptionId: 'sub-1',
    tenantId: 'tenant-1',
    kind: 'base',
    state: 'ACTIVE',
    planCode: 'office-pro',
    planVersion: '2',
    ruleId: 'default-office',
    ruleVersion: '1',
    salesScope: 'default',
    entitlementSourceVersion: '6',
    createdAt: '2026-09-10T00:00:00Z',
    matchExplanation: 'matched default sales scope',
    revision: '4',
    periodStart: '2026-09-10T00:00:00Z',
    periodEnd: '',
    renewalStopped: false,
    pendingChangeId: '',
  }
}

function entitlement(sourceVersion = '7') {
  return {
    tenantId: 'tenant-1',
    sourceVersion,
    resolverVersion: '4',
    evaluatedAt: '2026-09-12T05:00:00Z',
    validUntil: '',
    nextTransitionAt: '',
    catalogVersions: [{ moduleCode: 'device', version: '3' }],
    decisions: [{
      kind: 'capability',
      moduleCode: 'device',
      key: 'device.lifecycle',
      fieldAction: '',
      allowed: true,
      reason: 'GRANTED',
      masked: false,
      sources: [{
        id: 'plan:office-pro:2',
        sourceKind: 'plan',
        effect: 'grant',
        state: 'active',
        disposition: 'applied',
        reason: '套餐基础能力',
        actorId: 'platform-admin',
      }],
    }],
    entitlementVersion: '11',
    catalogRevision: '3',
    permissionVersion: '',
    permissionSubject: '',
  }
}

test('TestCE13TenantEntitlementWorkspaceUsesExplicitTenantAndServerExplanation', async ({ page }) => {
  await mockModules(page)
  let tenantDirectoryCalls = 0
  await page.route('**/api/v1/tenants*', async (route) => {
    tenantDirectoryCalls += 1
    await fulfillJson(route, { message: 'browser must not call API-key-only tenant directory' }, 500)
  })
  await page.route('**/api/auth/session', (route) => fulfillJson(route, { authenticated: true, csrf_token: 'csrf-ent' }))
  await page.route('**/api/v1/platform/tenants/tenant-1/subscription', (route) => fulfillJson(route, subscription()))
  await page.route('**/api/v1/platform/tenants/tenant-1/entitlement-overrides', (route) => fulfillJson(route, { sources: [], sourceVersion: '7' }))

  let explainBody: Record<string, unknown> | undefined
  await page.route('**/api/v1/platform/tenants/tenant-1/entitlements', async (route) => {
    explainBody = route.request().postDataJSON() as Record<string, unknown>
    await fulfillJson(route, entitlement())
  })

  await page.goto('/#/platform/commercial/tenant-entitlements')
  await expect(page.getByText('租户权益工作台', { exact: true })).toBeVisible()
  await page.getByLabel('租户编号').fill('tenant-1')
  await page.getByLabel(/能力过滤/).fill(' device.lifecycle ')
  await page.getByRole('button', { name: '读取权益' }).click()

  await expect(page.locator('.subscription-card').getByText('办公专业版 v2')).toBeVisible()
  await expect(page.getByText('设备生命周期').first()).toBeVisible()
  await expect(page.getByText('套餐基础能力')).toBeVisible()
  await expect(page.getByText('来源记录已保留')).toBeVisible()
  await expect(page.getByText('11').first()).toBeVisible()
  expect(explainBody).toMatchObject({ tenantId: 'tenant-1', capabilityCodes: ['device.lifecycle'] })
  expect(tenantDirectoryCalls).toBe(0)
})

test('TestCE13TenantOverrideCreateAndRevokeUseSourceVersionCas', async ({ page }) => {
  await mockModules(page)
  let currentSourceVersion = '7'
  let currentSources: Record<string, unknown>[] = []
  await page.route('**/api/auth/session', (route) => fulfillJson(route, { authenticated: true, csrf_token: 'csrf-entitlement-write' }))
  await page.route('**/api/v1/platform/tenants/tenant-1/subscription', (route) => fulfillJson(route, subscription()))
  await page.route('**/api/v1/platform/tenants/tenant-1/entitlements', (route) => fulfillJson(route, entitlement(currentSourceVersion)))

  let createBody: Record<string, unknown> | undefined
  let createHeaders: Record<string, string> | undefined
  await page.route('**/api/v1/platform/tenants/tenant-1/entitlement-overrides', async (route) => {
    if (route.request().method() === 'GET') {
      await fulfillJson(route, { sources: currentSources, sourceVersion: currentSourceVersion })
      return
    }
    createBody = route.request().postDataJSON() as Record<string, unknown>
    createHeaders = route.request().headers()
    currentSourceVersion = '8'
    currentSources = [{
      id: 'ov-new',
      tenantId: 'tenant-1',
      moduleCode: 'device',
      target: 'ENTITLEMENT_TARGET_CAPABILITY',
      key: 'device.lifecycle',
      fieldAction: '',
      effect: 'ENTITLEMENT_EFFECT_GRANT',
      effectiveAt: '',
      expiresAt: '',
      revokedAt: '',
      reason: '临时开放设备生命周期能力',
      actorId: 'platform-admin',
      version: '1',
      sourceKind: 'override',
    }]
    await fulfillJson(route, { sourceVersion: currentSourceVersion, source: currentSources[0] })
  })

  let revokeBody: Record<string, unknown> | undefined
  let revokeHeaders: Record<string, string> | undefined
  await page.route('**/api/v1/platform/tenants/tenant-1/entitlement-overrides/ov-new/revoke', async (route) => {
    revokeBody = route.request().postDataJSON() as Record<string, unknown>
    revokeHeaders = route.request().headers()
    currentSourceVersion = '9'
    currentSources = [{ ...currentSources[0], revokedAt: '2026-09-12T05:30:00Z' }]
    await fulfillJson(route, { sourceVersion: currentSourceVersion, source: currentSources[0] })
  })

  await page.goto('/#/platform/commercial/tenant-entitlements')
  await page.getByLabel('租户编号').fill('tenant-1')
  await page.getByRole('button', { name: '读取权益' }).click()
  await page.getByRole('button', { name: '新增专项权益' }).click()

  const createDialog = page.getByRole('dialog', { name: '新增专项权益' })
  await selectUiOption(createDialog.getByLabel('模块', { exact: true }), 'device')
  await selectUiOption(createDialog.getByLabel('授权范围', { exact: true }), 'ENTITLEMENT_TARGET_CAPABILITY')
  await selectUiOption(createDialog.getByLabel('具体项目', { exact: true }), 'device.lifecycle')
  await selectUiOption(createDialog.getByLabel('授权结果', { exact: true }), 'ENTITLEMENT_EFFECT_GRANT')
  await createDialog.getByLabel('原因', { exact: true }).fill('临时开放设备生命周期能力')
  await createDialog.getByRole('button', { name: '创建专项权益' }).click()

  await expect(page.getByText('临时开放设备生命周期能力', { exact: true })).toBeVisible()
  await expect(page.getByText('专项权益记录已保留', { exact: true })).toBeVisible()
  expect(createBody).toMatchObject({ tenantId: 'tenant-1', expectedVersion: '7', key: 'device.lifecycle' })
  expect(createHeaders?.['x-csrf-token']).toBe('csrf-entitlement-write')
  expect(createHeaders?.authorization).toBeUndefined()

  await page.getByRole('button', { name: '撤销' }).click()
  const revokeDialog = page.locator('.revoke-dialog')
  await expect(revokeDialog.getByRole('heading', { name: '撤销专项权益' })).toBeVisible()
  await revokeDialog.getByLabel('撤销原因', { exact: true }).fill('临时授权结束')
  await revokeDialog.getByRole('button', { name: '确认撤销' }).click()

  await expect(page.getByText('已撤销')).toBeVisible()
  expect(revokeBody).toMatchObject({ tenantId: 'tenant-1', id: 'ov-new', expectedVersion: '8', reason: '临时授权结束' })
  expect(revokeHeaders?.['x-csrf-token']).toBe('csrf-entitlement-write')
  expect(revokeHeaders?.authorization).toBeUndefined()
})


function initialPlanVersion() {
  return {
    planCode: 'office-pro',
    version: '3',
    revision: '1',
    planRevision: '3',
    state: 'PUBLISHED',
    name: '办公专业版',
    terms: {
      modules: [{
        moduleCode: 'device',
        capabilityCodes: ['device.lifecycle'],
        quotas: [{ key: 'device.count', unlimited: false, value: '100' }],
        fields: [],
      }],
      salesScope: ['default'],
      validityMode: 'fixed_days',
      validityDays: 365,
      priceRef: '',
    },
    contentSha256: 'published-office-pro-v3',
    createdAt: '2026-10-01T00:00:00Z',
    publishedAt: '2026-10-02T00:00:00Z',
    retiredAt: '',
    actorId: 'platform-admin',
    reason: '发布办公专业版',
  }
}

function initialEntitlement(applied: boolean) {
  return {
    tenantId: 'tenant-initial',
    sourceVersion: applied ? '1' : '0',
    resolverVersion: '4',
    evaluatedAt: '2026-10-07T00:00:00Z',
    validUntil: '',
    nextTransitionAt: '',
    catalogVersions: [{ moduleCode: 'device', version: '3' }],
    decisions: applied ? [{
      kind: 'module',
      moduleCode: 'device',
      key: 'device',
      fieldAction: '',
      allowed: true,
      reason: 'ALLOWED',
      masked: false,
      sources: [],
    }, {
      kind: 'capability',
      moduleCode: 'device',
      key: 'device.lifecycle',
      fieldAction: '',
      allowed: true,
      reason: 'GRANTED',
      masked: false,
      sources: [],
    }, {
      kind: 'quota',
      moduleCode: 'device',
      key: 'device.count',
      fieldAction: '',
      allowed: true,
      reason: 'PLAN_LIMIT',
      limit: { unlimited: false, value: '100' },
      masked: false,
      sources: [],
    }] : [],
    entitlementVersion: applied ? '2' : '1',
    catalogRevision: '3',
    permissionVersion: '',
    permissionSubject: '',
  }
}

async function mockInitialTargetDiscovery(page: Page) {
  await page.route('**/api/v1/platform/plans?pageSize=100', (route) => fulfillJson(route, {
    plans: [{
      planCode: 'office-pro',
      name: '办公专业版',
      latestVersion: '3',
      latestRevision: '1',
      planRevision: '3',
      state: 'PUBLISHED',
      salesScope: ['default'],
      createdAt: '2026-10-01T00:00:00Z',
      publishedAt: '2026-10-02T00:00:00Z',
      retiredAt: '',
    }],
    nextAfterPlanCode: '',
  }))
  await page.route('**/api/v1/platform/plans/office-pro/versions?pageSize=50', (route) => fulfillJson(route, {
    versions: [initialPlanVersion()],
    nextAfterVersion: '',
  }))
  await page.route('**/api/v1/platform/plans/office-pro/versions/3/eligibility', (route) => fulfillJson(route, {
    eligible: true,
    reason: 'eligible',
    version: initialPlanVersion(),
  }))
}

async function runFirstSubscriptionJourney(page: Page) {
  await mockModules(page)
  await mockInitialTargetDiscovery(page)
  let applied = false
  let previewBody: Record<string, unknown> | undefined
  let confirmBody: Record<string, unknown> | undefined

  await page.route('**/api/auth/session', (route) => fulfillJson(route, { authenticated: true, csrf_token: 'csrf-initial' }))
  await page.route('**/api/v1/platform/tenants/tenant-initial/subscription', (route) => {
    if (!applied) return fulfillJson(route, { message: 'subscription not found' }, 404)
    return fulfillJson(route, {
      subscriptionId: 'sub-initial',
      tenantId: 'tenant-initial',
      kind: 'BASE',
      state: 'ACTIVE',
      planCode: 'office-pro',
      planVersion: '3',
      ruleId: '',
      ruleVersion: '0',
      salesScope: 'default',
      entitlementSourceVersion: '1',
      createdAt: '2026-10-07T00:00:00Z',
      matchExplanation: '首次开通',
      revision: '1',
      periodStart: '2026-10-07T00:00:00Z',
      periodEnd: '2027-10-07T00:00:00Z',
      renewalStopped: false,
      pendingChangeId: '',
    })
  })
  await page.route('**/api/v1/platform/tenants/tenant-initial/entitlement-overrides', (route) => fulfillJson(route, { sources: [], sourceVersion: applied ? '1' : '0' }))
  await page.route('**/api/v1/platform/tenants/tenant-initial/entitlements', (route) => fulfillJson(route, initialEntitlement(applied)))

  await page.route('**/api/v1/platform/tenants/tenant-initial/subscription/change-previews', async (route) => {
    previewBody = route.request().postDataJSON() as Record<string, unknown>
    await fulfillJson(route, {
      changeId: 'chg-initial-1',
      tenantId: 'tenant-initial',
      actorId: 'platform-admin',
      requestId: String(previewBody.requestId),
      action: 'INITIAL',
      classification: 'INITIAL',
      mode: 'IMMEDIATE',
      previewHash: 'a'.repeat(64),
      target: initialPlanVersion(),
      subscriptionRevision: '0',
      sourceVersion: '0',
      entitlementVersion: '1',
      catalogRevision: '3',
      createdAt: '2026-10-07T00:00:00Z',
      expiresAt: '2099-10-07T00:10:00Z',
      effectiveAt: '2026-10-07T00:00:00Z',
      entitlementExpiresAt: '2027-10-07T00:00:00Z',
      currentEntitlements: initialEntitlement(false),
      projectedEntitlements: initialEntitlement(true),
      dependencies: [],
      quotaImpacts: [],
      impacts: [],
      impactDetails: [],
      pricingBasis: 'NO_PRICE_REFERENCE',
      quotaValidationRequired: false,
      provisioningRequirements: [],
    })
  })
  await page.route('**/api/v1/platform/tenants/tenant-initial/subscription/changes/chg-initial-1/confirm', async (route) => {
    confirmBody = route.request().postDataJSON() as Record<string, unknown>
    applied = true
    await fulfillJson(route, {
      changeId: 'chg-initial-1',
      tenantId: 'tenant-initial',
      actorId: 'platform-admin',
      requestId: String(confirmBody.requestId),
      previewHash: 'a'.repeat(64),
      action: 'INITIAL',
      status: 'APPLIED',
      mode: 'IMMEDIATE',
      confirmedAt: '2026-10-07T00:00:01Z',
      effectiveAt: '2026-10-07T00:00:01Z',
      entitlementExpiresAt: '2027-10-07T00:00:01Z',
      reason: '平台首次开通确认',
      after: {
        subscriptionId: 'sub-initial',
        tenantId: 'tenant-initial',
        kind: 'BASE',
        state: 'ACTIVE',
        planCode: 'office-pro',
        planVersion: '3',
        salesScope: 'default',
        entitlementSourceVersion: '1',
        revision: '1',
      },
      beforeSourceVersion: '0',
      afterSourceVersion: '1',
      beforeEntitlementVersion: '1',
      afterEntitlementVersion: '2',
      quotaValidationRequired: false,
      pricingAuthority: 'PLATFORM_MANUAL_APPROVAL',
      quotaImpacts: [],
      provisioningTaskId: '',
      failureCode: '',
    })
  })

  await page.goto('/#/platform/commercial/tenant-entitlements')
  await page.getByLabel('租户编号').fill('tenant-initial')
  await page.getByRole('button', { name: '读取权益' }).click()

  await expect(page.getByRole('heading', { name: '首次开通套餐' })).toBeVisible()
  await expect(page.getByText('完成首次开通不会自动给成员分配角色或操作权限。')).toBeVisible()

  await page.getByLabel('适用范围').fill('default')
  await page.getByRole('button', { name: '读取套餐目录' }).click()
  await selectUiOption(page.getByLabel('套餐', { exact: true }), 'office-pro')
  await page.getByRole('button', { name: '检查已发布版本' }).click()
  await expect(page.getByText('exact v3')).toBeVisible()

  await page.getByLabel('首次开通原因').fill('为新租户开通办公套餐')
  await page.getByRole('button', { name: '查看首次开通方案' }).click()
  await expect.poll(() => previewBody).toMatchObject({
    tenantId: 'tenant-initial',
    action: 'INITIAL',
    salesScope: 'default',
    targetPlanCode: 'office-pro',
    targetPlanVersion: '3',
  })

  await page.getByText(/我已核对 exact 套餐版本/).click()
  await page.getByLabel('确认原因').fill('平台首次开通确认')
  await page.getByRole('button', { name: '确认首次开通' }).click()

  await expect(page.locator('.subscription-card').getByText('办公专业版 v3')).toBeVisible()
  expect(confirmBody).toMatchObject({
    tenantId: 'tenant-initial',
    changeId: 'chg-initial-1',
    previewHash: 'a'.repeat(64),
  })
}

const ce340Viewports = [
  { width: 1366, height: 768 },
  { width: 1440, height: 900 },
  { width: 1536, height: 1024 },
  { width: 390, height: 844 },
]

for (const viewport of ce340Viewports) {
  test(`TestCE340FirstSubscriptionRequiresExactPublishedVersionAndFinalEntitlementReadback ${viewport.width}x${viewport.height}`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport)
    await runFirstSubscriptionJourney(page)
    await page.screenshot({
      path: testInfo.outputPath(`ce340-first-subscription-${viewport.width}x${viewport.height}.png`),
      fullPage: true,
    })
  })
}

test('TestCE340ProvisioningFailureRetriesSameTaskWithoutSecondSubscription', async ({ page }) => {
  await mockModules(page)
  await mockInitialTargetDiscovery(page)

  await page.route('**/api/auth/session', (route) => fulfillJson(route, { authenticated: true, csrf_token: 'csrf-provisioning' }))
  await page.route('**/api/v1/platform/tenants/tenant-initial/subscription', (route) => fulfillJson(route, { message: 'subscription not found' }, 404))
  await page.route('**/api/v1/platform/tenants/tenant-initial/entitlement-overrides', (route) => fulfillJson(route, { sources: [], sourceVersion: '0' }))
  await page.route('**/api/v1/platform/tenants/tenant-initial/entitlements', (route) => fulfillJson(route, initialEntitlement(false)))

  let previewCalls = 0
  await page.route('**/api/v1/platform/tenants/tenant-initial/subscription/change-previews', async (route) => {
    previewCalls += 1
    await fulfillJson(route, {
      changeId: 'chg-initial-provisioning',
      tenantId: 'tenant-initial',
      actorId: 'platform-admin',
      requestId: 'preview-provisioning',
      action: 'INITIAL',
      classification: 'INITIAL',
      mode: 'IMMEDIATE',
      previewHash: 'b'.repeat(64),
      target: initialPlanVersion(),
      sourceVersion: '0',
      entitlementVersion: '1',
      catalogRevision: '3',
      createdAt: '2026-10-07T00:00:00Z',
      expiresAt: '2099-10-07T00:10:00Z',
      effectiveAt: '2026-10-07T00:00:00Z',
      currentEntitlements: initialEntitlement(false),
      projectedEntitlements: initialEntitlement(true),
      dependencies: [],
      quotaImpacts: [],
      impacts: [],
      impactDetails: [],
      pricingBasis: 'NO_PRICE_REFERENCE',
      quotaValidationRequired: false,
      provisioningRequirements: [{ code: 'prepare-device', adapter: 'device-provider', version: 'v1', maxAttempts: 2 }],
    })
  })
  await page.route('**/api/v1/platform/tenants/tenant-initial/subscription/changes/chg-initial-provisioning/confirm', (route) => fulfillJson(route, {
    changeId: 'chg-initial-provisioning',
    tenantId: 'tenant-initial',
    actorId: 'platform-admin',
    requestId: 'confirm-provisioning',
    previewHash: 'b'.repeat(64),
    action: 'INITIAL',
    status: 'PROVISIONING',
    mode: 'IMMEDIATE',
    confirmedAt: '2026-10-07T00:00:01Z',
    effectiveAt: '2026-10-07T00:00:01Z',
    reason: '需要设备侧准备',
    afterSourceVersion: '1',
    afterEntitlementVersion: '2',
    provisioningTaskId: 'job-initial-1',
    quotaImpacts: [],
  }))

  let taskRevision = '4'
  await page.route('**/api/v1/platform/tenants/tenant-initial/provisioning/tasks/job-initial-1', (route) => fulfillJson(route, {
    taskId: 'job-initial-1',
    tenantId: 'tenant-initial',
    changeId: 'chg-initial-provisioning',
    state: 'FAILED',
    revision: taskRevision,
    targetPlanCode: 'office-pro',
    targetPlanVersion: '3',
    steps: [],
    stepIndex: 0,
    stage: 'PREPARE',
    failureCode: 'PREPARATION_RETRIES_EXHAUSTED',
    retryAllowed: true,
    cancellationAllowed: false,
    retryCycles: 1,
  }))

  let retryBody: Record<string, unknown> | undefined
  await page.route('**/api/v1/platform/tenants/tenant-initial/provisioning/tasks/job-initial-1/retry', async (route) => {
    retryBody = route.request().postDataJSON() as Record<string, unknown>
    taskRevision = '5'
    await fulfillJson(route, {
      taskId: 'job-initial-1',
      tenantId: 'tenant-initial',
      changeId: 'chg-initial-provisioning',
      state: 'QUEUED',
      revision: taskRevision,
      targetPlanCode: 'office-pro',
      targetPlanVersion: '3',
      steps: [],
      retryAllowed: false,
      cancellationAllowed: false,
      retryCycles: 2,
    })
  })

  await page.goto('/#/platform/commercial/tenant-entitlements')
  await page.getByLabel('租户编号').fill('tenant-initial')
  await page.getByRole('button', { name: '读取权益' }).click()
  await page.getByLabel('适用范围').fill('default')
  await page.getByRole('button', { name: '读取套餐目录' }).click()
  await selectUiOption(page.getByLabel('套餐', { exact: true }), 'office-pro')
  await page.getByRole('button', { name: '检查已发布版本' }).click()
  await page.getByLabel('首次开通原因').fill('需要设备准备')
  await page.getByRole('button', { name: '查看首次开通方案' }).click()
  await page.getByText(/我已核对 exact 套餐版本/).click()
  await page.getByLabel('确认原因').fill('批准带准备任务的首次开通')
  await page.getByRole('button', { name: '确认首次开通' }).click()

  await expect(page.getByText('处理失败')).toBeVisible()
  await page.getByRole('button', { name: '恢复原准备任务' }).click()
  await expect(page.getByText('已恢复原准备任务；不会创建第二份首次订阅。')).toBeVisible()
  expect(retryBody).toMatchObject({
    tenantId: 'tenant-initial',
    taskId: 'job-initial-1',
    expectedRevision: '4',
  })
  expect(previewCalls).toBe(1)
})
