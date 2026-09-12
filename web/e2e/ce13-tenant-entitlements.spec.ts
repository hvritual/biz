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
  await expect(page.getByText('Issue #64').first()).toBeVisible()
  await page.getByLabel('租户 ID').fill('tenant-1')
  await page.getByLabel(/能力过滤/).fill(' device.lifecycle ')
  await page.getByRole('button', { name: '读取权益' }).click()

  await expect(page.locator('.subscription-card').getByText('office-pro v2')).toBeVisible()
  await expect(page.getByText('device.lifecycle').first()).toBeVisible()
  await expect(page.getByText('套餐基础能力')).toBeVisible()
  await expect(page.getByText('plan:office-pro:2')).toBeVisible()
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
  await page.getByLabel('租户 ID').fill('tenant-1')
  await page.getByRole('button', { name: '读取权益' }).click()
  await page.getByRole('button', { name: '新增专项来源' }).click()

  const createDialog = page.getByRole('dialog', { name: '新增专项权益来源' })
  await createDialog.getByLabel('模块', { exact: true }).selectOption('device')
  await createDialog.getByLabel('目标类型', { exact: true }).selectOption('ENTITLEMENT_TARGET_CAPABILITY')
  await createDialog.getByLabel('目标 key', { exact: true }).selectOption('device.lifecycle')
  await createDialog.getByLabel('效果', { exact: true }).selectOption('ENTITLEMENT_EFFECT_GRANT')
  await createDialog.getByLabel('原因', { exact: true }).fill('临时开放设备生命周期能力')
  await createDialog.getByRole('button', { name: '创建专项来源' }).click()

  await expect(page.getByText('ov-new')).toBeVisible()
  expect(createBody).toMatchObject({ tenantId: 'tenant-1', expectedVersion: '7', key: 'device.lifecycle' })
  expect(createHeaders?.['x-csrf-token']).toBe('csrf-entitlement-write')
  expect(createHeaders?.authorization).toBeUndefined()

  await page.getByRole('button', { name: '撤销' }).click()
  const revokeDialog = page.locator('.revoke-dialog')
  await expect(revokeDialog.getByRole('heading', { name: '撤销专项来源' })).toBeVisible()
  await revokeDialog.getByLabel('撤销原因', { exact: true }).fill('临时授权结束')
  await revokeDialog.getByRole('button', { name: '确认撤销' }).click()

  await expect(page.getByText('已撤销')).toBeVisible()
  expect(revokeBody).toMatchObject({ tenantId: 'tenant-1', id: 'ov-new', expectedVersion: '8', reason: '临时授权结束' })
  expect(revokeHeaders?.['x-csrf-token']).toBe('csrf-entitlement-write')
  expect(revokeHeaders?.authorization).toBeUndefined()
})
