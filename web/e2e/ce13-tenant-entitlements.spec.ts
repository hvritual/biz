import { selectUiOption } from './ui.helpers'
import { expect, test, type Page, type Route } from '@playwright/test'
import { observeInitialSubscription, tabToInitialControl, type InitialSubscriptionObservation } from './initial-subscription-visual.helpers'

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


function initialPlanVersion(longContent = false) {
  return {
    planCode: 'office-pro',
    version: '3',
    revision: '1',
    planRevision: '3',
    state: 'PUBLISHED',
    name: longContent ? '办公专业版 · 跨区域租赁与连锁经营 Enterprise Operations and Subscription Management' : '办公专业版',
    terms: {
      modules: [{
        moduleCode: 'device',
        capabilityCodes: ['device.lifecycle'],
        quotas: [{ key: 'device.count', unlimited: false, value: longContent ? '9007199254740993' : '100' }],
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

function initialEntitlement(applied: boolean, quota = '100') {
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
      limit: { unlimited: false, value: quota },
      masked: false,
      sources: [],
    }] : [],
    entitlementVersion: applied ? '2' : '1',
    catalogRevision: '3',
    permissionVersion: '',
    permissionSubject: '',
  }
}

async function mockInitialTargetDiscovery(page: Page, version = initialPlanVersion()) {
  await page.route('**/api/v1/platform/plans/office-pro/versions/3', (route) => fulfillJson(route, version))
  await page.route('**/api/v1/platform/plans?pageSize=100', (route) => fulfillJson(route, {
    plans: [{
      planCode: 'office-pro',
      name: version.name,
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
    versions: [version],
    nextAfterVersion: '',
  }))
  await page.route('**/api/v1/platform/plans/office-pro/versions/3/eligibility', (route) => fulfillJson(route, {
    eligible: true,
    reason: 'eligible',
    version,
  }))
}

type InitialJourneyOptions = {
  observe?: InitialSubscriptionObservation
  afterNavigation?: () => Promise<void>
  keyboard?: boolean
  longContent?: boolean
  recoverResults?: boolean
}

async function runFirstSubscriptionJourney(page: Page, options: InitialJourneyOptions = {}) {
  const target = initialPlanVersion(options.longContent)
  const quota = target.terms.modules[0]!.quotas[0]!.value
  const changeId = options.longContent ? `chg-initial-${'qualification-'.repeat(3)}0123456789` : 'chg-initial-1'
  await mockModules(page)
  await mockInitialTargetDiscovery(page, target)
  let applied = false
  let previewCalls = 0
  let confirmCalls = 0
  let failNextReadback = Boolean(options.recoverResults)
  let previewBody: Record<string, unknown> | undefined
  let confirmBody: Record<string, unknown> | undefined
  const activate = async (name: string) => {
    const control = page.getByRole('button', { name, exact: true })
    if (!options.keyboard) { await control.click(); return }
    await tabToInitialControl(page, control)
    await page.keyboard.press('Enter')
  }
  const enterText = async (label: string, value: string) => {
    // Field labels can include inline help text in their accessible name.
    const control = page.getByLabel(label)
    if (!options.keyboard) { await control.fill(value); return }
    await tabToInitialControl(page, control)
    await page.keyboard.insertText(value)
  }

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
  await page.route('**/api/v1/platform/tenants/tenant-initial/entitlements', (route) => {
    if (applied && failNextReadback) {
      failNextReadback = false
      return fulfillJson(route, { message: 'qualification: final entitlement read failed' }, 503)
    }
    return fulfillJson(route, initialEntitlement(applied, quota))
  })

  await page.route('**/api/v1/platform/tenants/tenant-initial/subscription/change-previews', async (route) => {
    previewCalls += 1
    previewBody = route.request().postDataJSON() as Record<string, unknown>
    await fulfillJson(route, {
      changeId,
      tenantId: 'tenant-initial',
      actorId: 'platform-admin',
      requestId: String(previewBody.requestId),
      action: 'INITIAL',
      classification: 'INITIAL',
      mode: 'IMMEDIATE',
      previewHash: 'a'.repeat(64),
      target,
      subscriptionRevision: '0',
      sourceVersion: '0',
      entitlementVersion: '1',
      catalogRevision: '3',
      createdAt: '2026-10-07T00:00:00Z',
      expiresAt: '2099-10-07T00:10:00Z',
      effectiveAt: '2026-10-07T00:00:00Z',
      entitlementExpiresAt: '2027-10-07T00:00:00Z',
      currentEntitlements: initialEntitlement(false, quota),
      projectedEntitlements: initialEntitlement(true, quota),
      dependencies: [],
      quotaImpacts: [],
      impacts: [],
      impactDetails: [],
      pricingBasis: 'NO_PRICE_REFERENCE',
      quotaValidationRequired: false,
      provisioningRequirements: [],
    })
  })
  const receipt = () => ({
      changeId,
      tenantId: 'tenant-initial',
      actorId: 'platform-admin',
      requestId: String(confirmBody?.requestId),
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
  await page.route(`**/api/v1/platform/tenants/tenant-initial/subscription/changes/${changeId}`, (route) => applied
    ? fulfillJson(route, receipt())
    : fulfillJson(route, { message: 'change receipt not found' }, 404))
  await page.route(`**/api/v1/platform/tenants/tenant-initial/subscription/changes/${changeId}/confirm`, async (route) => {
    confirmCalls += 1
    confirmBody = route.request().postDataJSON() as Record<string, unknown>
    applied = true
    await fulfillJson(route, options.recoverResults ? { message: 'qualification: confirmation result unknown' } : receipt(), options.recoverResults ? 503 : 200)
  })

  await page.goto('/#/platform/commercial/tenant-entitlements')
  await options.afterNavigation?.()
  await enterText('租户编号', 'tenant-initial')
  await activate('读取权益')

  await expect(page.getByRole('heading', { name: '首次开通套餐' })).toBeVisible()
  await expect(page.getByText('完成首次开通不会自动给成员分配角色或操作权限。')).toBeVisible()
  await options.observe?.('entry', page.getByRole('heading', { name: '首次开通套餐' }))

  await enterText('适用范围', 'default')
  await activate('读取套餐目录')
  const planSelect = page.getByLabel('套餐', { exact: true })
  if (options.keyboard) {
    await tabToInitialControl(page, planSelect)
    await page.keyboard.press('Enter')
    await expect(page.getByRole('option').filter({ hasText: target.name })).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(planSelect).toBeFocused()
    await page.keyboard.press('Enter')
    await page.keyboard.press('End')
    await page.keyboard.press('Enter')
    await expect(planSelect).toBeFocused()
    await expect(planSelect).toContainText(target.name)
  } else await selectUiOption(planSelect, 'office-pro')
  await activate('检查已发布版本')
  await expect(page.getByText('exact v3')).toBeVisible()
  await expect(page.locator('.target-detail')).toContainText(quota)
  await options.observe?.('target', page.locator('.target-detail h3'))

  await enterText('首次开通原因', '为新租户开通办公套餐')
  await activate('查看首次开通方案')
  await expect.poll(() => previewBody).toMatchObject({
    tenantId: 'tenant-initial',
    action: 'INITIAL',
    salesScope: 'default',
    targetPlanCode: 'office-pro',
    targetPlanVersion: '3',
  })
  await options.observe?.('preview', page.getByRole('heading', { name: '首次开通方案' }))

  if (options.keyboard) {
    await activate('确认首次开通')
    await expect(page.getByRole('alert')).toContainText('请先确认已核对首次开通影响。')
    expect(confirmCalls).toBe(0)
    await expect(page.getByRole('button', { name: '确认首次开通' })).toBeFocused()
    const acknowledgement = page.getByRole('checkbox', { name: /我已核对 exact 套餐版本/ })
    await tabToInitialControl(page, acknowledgement)
    await page.keyboard.press('Space')
    await expect(acknowledgement).toBeChecked()
  } else await page.getByText(/我已核对 exact 套餐版本/).click()
  await enterText('确认原因', '平台首次开通确认')
  await options.observe?.('confirm', page.getByRole('button', { name: '确认首次开通' }))
  await activate('确认首次开通')

  if (options.recoverResults) {
    await expect(page.getByText('首次开通结果待确认', { exact: true })).toBeVisible()
    await expect(page.getByLabel('适用范围')).toBeDisabled()
    await expect(page.getByRole('button', { name: '确认首次开通' })).toBeDisabled()
    await options.observe?.('unknown', page.getByText('首次开通结果待确认', { exact: true }))
    await activate('读取原变更结果')
    await expect(page.getByText('订阅回执已存在，最终权益仍待确认', { exact: true })).toBeVisible()
    await expect(page.getByText(`变更编号 ${changeId}`, { exact: true })).toBeVisible()
    await expect(page.getByText('首次开通已完成，最终权益已确认', { exact: true })).toHaveCount(0)
    await options.observe?.('readback-failed', page.getByText('订阅回执已存在，最终权益仍待确认', { exact: true }))
    await activate('重新读取结果')
  }

  await expect(page.locator('.subscription-card').getByText(`${target.name} v3`)).toBeVisible()
  await options.observe?.('verified', page.locator('.subscription-card'))
  expect(previewCalls).toBe(1)
  expect(confirmCalls).toBe(1)
  expect(confirmBody).toMatchObject({
    tenantId: 'tenant-initial',
    changeId,
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
    await runFirstSubscriptionJourney(page, {
      observe: observeInitialSubscription(page, testInfo, `${viewport.width}x${viewport.height}`),
    })
    await page.screenshot({
      path: testInfo.outputPath(`ce340-first-subscription-${viewport.width}x${viewport.height}.png`),
      fullPage: true,
    })
  })
}

async function runFirstProvisioningRecovery(page: Page, options: InitialJourneyOptions = {}) {
  const target = initialPlanVersion(options.longContent)
  const quota = target.terms.modules[0]!.quotas[0]!.value
  let confirmed = false
  let confirmCalls = 0
  const pendingSubscription = {
    subscriptionId: 'sub-initial-preparing', tenantId: 'tenant-initial', kind: 'BASE', state: 'PROVISIONING',
    planCode: 'office-pro', planVersion: '3', salesScope: 'default', entitlementSourceVersion: '1',
    revision: '1', pendingChangeId: 'chg-initial-provisioning', renewalStopped: false,
    createdAt: '2026-10-07T00:00:01Z', periodStart: '2026-10-07T00:00:01Z', periodEnd: '2027-10-07T00:00:01Z',
  }
  await mockModules(page)
  await mockInitialTargetDiscovery(page, target)

  await page.route('**/api/auth/session', (route) => fulfillJson(route, { authenticated: true, csrf_token: 'csrf-provisioning' }))
  await page.route('**/api/v1/platform/tenants/tenant-initial/subscription', (route) => confirmed
    ? fulfillJson(route, pendingSubscription)
    : fulfillJson(route, { message: 'subscription not found' }, 404))
  await page.route('**/api/v1/platform/tenants/tenant-initial/entitlement-overrides', (route) => fulfillJson(route, { sources: [], sourceVersion: confirmed ? '1' : '0' }))
  // INITIAL preparation reserves a generation and a pending subscription, but
  // grants no target source. Mirror confirm.go's real preparation contract.
  await page.route('**/api/v1/platform/tenants/tenant-initial/entitlements', (route) => fulfillJson(route, {
    ...initialEntitlement(false), sourceVersion: confirmed ? '1' : '0', entitlementVersion: confirmed ? '2' : '1',
  }))

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
      target,
      sourceVersion: '0',
      entitlementVersion: '1',
      catalogRevision: '3',
      createdAt: '2026-10-07T00:00:00Z',
      expiresAt: '2099-10-07T00:10:00Z',
      effectiveAt: '2026-10-07T00:00:00Z',
      currentEntitlements: initialEntitlement(false, quota),
      projectedEntitlements: initialEntitlement(true, quota),
      dependencies: [],
      quotaImpacts: [],
      impacts: [],
      impactDetails: [],
      pricingBasis: 'NO_PRICE_REFERENCE',
      quotaValidationRequired: false,
      provisioningRequirements: [{ code: 'prepare-device', adapter: 'device-provider', version: 'v1', maxAttempts: 2 }],
    })
  })
  const preparationReceipt = {
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
    after: pendingSubscription,
    beforeSourceVersion: '0',
    afterSourceVersion: '1',
    beforeEntitlementVersion: '1',
    afterEntitlementVersion: '2',
    provisioningTaskId: 'job-initial-1',
    quotaImpacts: [],
  }
  await page.route('**/api/v1/platform/tenants/tenant-initial/subscription/changes/chg-initial-provisioning/confirm', (route) => {
    confirmed = true
    confirmCalls += 1
    return fulfillJson(route, preparationReceipt)
  })
  await page.route('**/api/v1/platform/tenants/tenant-initial/subscription/changes/chg-initial-provisioning', (route) => confirmed
    ? fulfillJson(route, preparationReceipt)
    : fulfillJson(route, { message: 'change receipt not found' }, 404))

  let taskRevision = '4'
  let taskState: 'FAILED' | 'QUEUED' = 'FAILED'
  const preparationTask = () => ({
    taskId: 'job-initial-1',
    tenantId: 'tenant-initial',
    changeId: 'chg-initial-provisioning',
    state: taskState,
    revision: taskRevision,
    targetPlanCode: 'office-pro',
    targetPlanVersion: '3',
    steps: [],
    stepIndex: 0,
    stage: 'PREPARE',
    failureCode: taskState === 'FAILED' ? 'PREPARATION_RETRIES_EXHAUSTED' : '',
    retryAllowed: taskState === 'FAILED',
    cancellationAllowed: false,
    retryCycles: taskState === 'FAILED' ? 1 : 2,
  })
  await page.route('**/api/v1/platform/tenants/tenant-initial/provisioning/tasks/job-initial-1', (route) => fulfillJson(route, preparationTask()))

  let retryBody: Record<string, unknown> | undefined
  await page.route('**/api/v1/platform/tenants/tenant-initial/provisioning/tasks/job-initial-1/retry', async (route) => {
    retryBody = route.request().postDataJSON() as Record<string, unknown>
    taskRevision = '5'
    taskState = 'QUEUED'
    await fulfillJson(route, preparationTask())
  })

  await page.goto('/#/platform/commercial/tenant-entitlements')
  await options.afterNavigation?.()
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
  await expect(page.getByText('目标套餐权益尚未正式生效。系统会继续使用同一准备任务，不会重复创建订阅。')).toBeVisible()
  await options.observe?.('provisioning-failed', page.getByText('处理失败', { exact: true }))
  const retry = page.getByRole('button', { name: '恢复原准备任务' })
  if (options.keyboard) {
    await tabToInitialControl(page, retry)
    await page.keyboard.press('Enter')
  } else await retry.click()
  await expect(page.getByText('已恢复原准备任务；不会创建第二份首次订阅。')).toBeVisible()
  await expect(page.getByText('等待处理', { exact: true })).toBeVisible()
  await expect(retry).toHaveCount(0)
  await expect(page.getByRole('button', { name: '确认首次开通' })).toBeDisabled()
  await options.observe?.('provisioning-queued', page.getByText('等待处理', { exact: true }))
  expect(retryBody).toMatchObject({
    tenantId: 'tenant-initial',
    taskId: 'job-initial-1',
    expectedRevision: '4',
  })
  expect(previewCalls).toBe(1)
  expect(confirmCalls).toBe(1)
  await page.reload()
  await expect(page.getByText('等待处理', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: '确认首次开通' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '查看首次开通方案' })).toHaveCount(0)
  await expect(page.getByLabel('已有开通变更编号')).toHaveValue('chg-initial-provisioning')
  await options.observe?.('provisioning-restored', page.getByText('等待处理', { exact: true }))
  expect(previewCalls).toBe(1)
  expect(confirmCalls).toBe(1)
}

test('TestCE340ProvisioningFailureRetriesSameTaskWithoutSecondSubscription', async ({ page }, info) => {
  await runFirstProvisioningRecovery(page, { observe: observeInitialSubscription(page, info, 'provisioning') })
})

for (const viewport of ce340Viewports) {
  test(`TestCE340KeyboardAndLongContentRecoverOriginalUnknownReadbackAndProvisioning ${viewport.width}x${viewport.height}`, async ({ page }, info) => {
    await page.setViewportSize(viewport)
    const options = {
      keyboard: true, longContent: true, recoverResults: true,
      observe: observeInitialSubscription(page, info, `keyboard-recovery-${viewport.width}x${viewport.height}`),
    }
    await runFirstSubscriptionJourney(page, options)
    await page.goto('about:blank')
    await page.unrouteAll({ behavior: 'wait' })
    await runFirstProvisioningRecovery(page, options)
  })
}
