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

test('TestCE13PlatformCommercialReusesJoinedCoffeeLinkNavigation', async ({ page }) => {
  await mockModules(page)
  await page.goto('/#/dashboard')
  const main = page.getByTestId('main-content')
  const before = await main.boundingBox()

  await page.getByRole('button', { name: '平台商业', exact: true }).click()
  const drawer = page.getByRole('dialog', { name: '平台商业导航' })
  await expect(drawer).toBeVisible()
  await expect(drawer.getByRole('button', { name: '模块目录', exact: true })).toBeVisible()
  await expect(drawer.getByRole('button', { name: '套餐版本', exact: true })).toBeVisible()
  await expect(drawer.getByRole('button', { name: '租户权益', exact: true })).toBeVisible()
  await expect(drawer.getByRole('button', { name: '预览订阅变更', exact: true })).toBeVisible()

  const drawerBox = await drawer.boundingBox()
  expect(drawerBox).not.toBeNull()
  expect(drawerBox!.width).toBeGreaterThanOrEqual(450)
  expect(drawerBox!.width).toBeLessThanOrEqual(481)

  const after = await main.boundingBox()
  expect(after?.x).toBe(before?.x)
  expect(after?.width).toBe(before?.width)

  await page.getByRole('button', { name: '平台商业', exact: true }).click()
  await expect(drawer).toBeHidden()
  await expect(page).toHaveURL(/#\/dashboard$/)

  await page.getByRole('button', { name: '平台商业', exact: true }).click()
  await drawer.getByRole('button', { name: '模块目录', exact: true }).click()
  await expect(page).toHaveURL(/#\/platform\/commercial\/modules$/)
  await expect(page.getByText('平台管理').first()).toBeVisible()
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
  await page.route('**/api/v1/platform/modules/customer/sales-status', async (route) => {
    mutationBody = route.request().postDataJSON() as Record<string, unknown>
    mutationHeaders = route.request().headers()
    await fulfillJson(route, {
      ...readyModule,
      salesStatus: 'MODULE_SALES_STATUS_RETIRED',
      version: '2',
    })
  })

  await page.goto('/#/platform/commercial/modules')
  await page.getByRole('button', { name: '查看详情' }).click()
  const dialog = page.getByRole('dialog', { name: /模块详情/ })
  await expect(dialog.getByText('customer.read')).toBeVisible()
  await expect(dialog.getByText('tenant')).toBeVisible()
  await dialog.getByLabel('变更原因').fill('CE-13 销售状态收口验证')
  await dialog.getByRole('button', { name: '停售销售' }).click()

  await expect(dialog.getByText('模块已停售；技术状态未被自动修改。')).toBeVisible()
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
