import { expect, test, type Page, type Route } from '@playwright/test'

const readyModule = {
  moduleCode: 'customer',
  name: '客户管理',
  category: 'business',
  salesScope: ['default'],
  technicalStatus: 'MODULE_TECHNICAL_STATUS_READY',
  salesStatus: 'MODULE_SALES_STATUS_SELLABLE',
  capabilityCodes: ['customer.read', 'customer.manage'],
  quotaSchemaKeys: ['customer.count'],
  fieldPolicySchemaKeys: ['customer.phone'],
  dependencies: ['tenant'],
  version: '1',
}

async function fulfillJson(route: Route, body: unknown, status = 200) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function mockModules(page: Page) {
  await page.route('**/api/v1/platform/modules', (route) => fulfillJson(route, { modules: [readyModule] }))
}

const readyFeature = {
  featureCode: 'marketing-campaigns', name: '营销活动', version: '1',
  moduleRefs: [{ moduleCode: 'access-management', capabilityCodes: ['tenant.lifecycle'] }],
  productState: 'PUBLISHED', salesState: 'SELLABLE', runtimeState: 'CONTINUING', migrationState: 'NONE', replacementCode: '',
  referenceImpact: { publishedPlans: '1', addOns: '0', activeSubscriptions: '0', entitlementSources: '0' },
}

async function mockFeatures(page: Page) {
  await page.route('**/api/v1/platform/commercial-features', (route) => fulfillJson(route, { features: [readyFeature] }))
}

test('TestCE13PlatformCommercialReusesJoinedCoffeeLinkNavigation', async ({ page }) => {
  await mockModules(page)
  await page.goto('/#/dashboard')
  const main = page.getByTestId('main-content')
  const before = await main.boundingBox()

  await page.getByRole('button', { name: '平台管理', exact: true }).click()
  const drawer = page.getByRole('dialog', { name: '平台管理导航' })
  await expect(drawer).toBeVisible()
  await expect(drawer.getByRole('button', { name: '平台总览', exact: true })).toBeVisible()
  await expect(drawer.getByRole('button', { name: '租户管理', exact: true })).toBeVisible()
  await expect(drawer.getByRole('button', { name: '模块目录', exact: true })).toBeVisible()
  await expect(drawer.getByRole('button', { name: '套餐版本', exact: true })).toBeVisible()
  await expect(drawer.getByRole('button', { name: '租户权益', exact: true })).toBeVisible()
  await expect(drawer.getByRole('button', { name: '打开租户管理', exact: true })).toBeVisible()

  const drawerBox = await drawer.boundingBox()
  expect(drawerBox).not.toBeNull()
  expect(drawerBox!.width).toBeGreaterThanOrEqual(450)
  expect(drawerBox!.width).toBeLessThanOrEqual(481)

  const after = await main.boundingBox()
  expect(after?.x).toBe(before?.x)
  expect(after?.width).toBe(before?.width)

  await page.getByRole('button', { name: '平台管理', exact: true }).click()
  await expect(drawer).toBeHidden()
  await expect(page).toHaveURL(/#\/dashboard$/)

  await page.getByRole('button', { name: '平台管理', exact: true }).click()
  await drawer.getByRole('button', { name: '模块目录', exact: true }).click()
  await expect(page).toHaveURL(/#\/platform\/commercial\/modules$/)
  await expect(page.getByText('平台管理').first()).toBeVisible()
})

test('TestCE289CommercialAuthorityGateHidesPreviewNavigationAndLabelsDirectPreview', async ({ page }) => {
  await mockFeatures(page)
  await page.goto('/#/dashboard')
  await page.getByRole('button', { name: '平台管理', exact: true }).click()
  const drawer = page.getByRole('dialog', { name: '平台管理导航' })
  await expect(drawer).toBeVisible()
  for (const label of ['平台总览', '租户管理', '模块目录', '商业功能', '套餐版本', '租户权益']) {
    await expect(drawer.getByRole('button', { name: label, exact: true })).toBeVisible()
  }
  for (const label of ['增购项', '租户订阅', '套餐变更', '到期与宽限', '授权诊断', '额度管理', '专项授权', '用量计费', '商业审计']) {
    await expect(drawer.getByRole('button', { name: label, exact: true })).toHaveCount(0)
  }
  await expect(drawer.getByRole('button', { name: '查看用量计费', exact: true })).toHaveCount(0)

  await page.keyboard.press('Escape')
  await expect(drawer).toBeHidden()
  await page.goto('/#/platform/commercial/features')
  await expect(page.getByTestId('ce290-commercial-features')).toBeVisible()
  await expect(page.getByRole('heading', { name: '营销活动', exact: true })).toBeVisible()
  await expect(page.getByText('设计预览 · 不显示真实业务数据', { exact: true })).toHaveCount(0)
})

test('TestCE13ModuleSalesStatusUsesTrustedSessionCsrfAndServerVersion', async ({ page }) => {
  await mockModules(page)
  await page.route('**/api/auth/session', (route) => fulfillJson(route, {
    authenticated: true,
    actor_kind: 'platform',
    csrf_token: 'csrf-ce13-module',
  }))

  let mutationBody: Record<string, unknown> | undefined
  let mutationHeaders: Record<string, string> | undefined
  const changedModule = { ...readyModule, salesStatus: 'MODULE_SALES_STATUS_RETIRED', version: '1' }
  await page.route('**/api/v1/platform/modules/customer', (route) => fulfillJson(route, changedModule))
  await page.route('**/api/v1/platform/modules/customer/sales-status', async (route) => {
    mutationBody = route.request().postDataJSON() as Record<string, unknown>
    mutationHeaders = route.request().headers()
    await fulfillJson(route, changedModule)
  })

  await page.goto('/#/platform/commercial/modules')
  await page.getByRole('button', { name: '查看详情' }).click()
  const detail = page.getByRole('dialog', { name: /模块详情/ })
  await expect(detail.getByText('查看客户')).toBeVisible()
  await expect(detail.getByText('租户基础能力')).toBeVisible()
  await detail.getByRole('button', { name: '停售销售' }).click()
  const confirmation = page.getByRole('dialog', { name: '确认模块停售', exact: true })
  await confirmation.getByLabel('变更原因').fill('CE-13 销售状态收口验证')
  await confirmation.getByRole('button', { name: '确认停售', exact: true }).click()

  await expect(page.getByRole('dialog', { name: '模块变更结果' }).getByText('模块已停售；目录版本与技术状态未改变。')).toBeVisible()
  expect(mutationBody).toMatchObject({
    moduleCode: 'customer',
    salesStatus: 'MODULE_SALES_STATUS_RETIRED',
    version: '1',
    reason: 'CE-13 销售状态收口验证',
  })
  expect(mutationHeaders?.['x-csrf-token']).toBe('csrf-ce13-module')
  expect(mutationHeaders?.['idempotency-key']).toMatch(/^ce13-module-sales-/)
  expect(mutationHeaders?.authorization).toBeUndefined()
})
