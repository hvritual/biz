import { expect, test, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'

const evidenceDir = 'test-results/screenshots'
const viewports = [
  [1366, 768],
  [1440, 900],
  [1536, 1024],
  [390, 844],
] as const

const moduleFixture = [
  {
    moduleCode: 'device-operations',
    name: '设备运营',
    category: 'operations',
    salesScope: ['default'],
    technicalStatus: 'MODULE_TECHNICAL_STATUS_READY',
    salesStatus: 'MODULE_SALES_STATUS_SELLABLE',
    capabilityCodes: ['device.lifecycle', 'device.telemetry'],
    quotaSchemaKeys: ['device.count'],
    fieldPolicySchemaKeys: [],
    dependencies: [],
    version: 12,
  },
  {
    moduleCode: 'customer-operations',
    name: '客户经营',
    category: 'crm',
    salesScope: ['default'],
    technicalStatus: 'MODULE_TECHNICAL_STATUS_READY',
    salesStatus: 'MODULE_SALES_STATUS_SELLABLE',
    capabilityCodes: ['customer.view'],
    quotaSchemaKeys: ['customer.count'],
    fieldPolicySchemaKeys: [],
    dependencies: ['device-operations'],
    version: 7,
  },
  {
    moduleCode: 'marketing',
    name: '营销能力',
    category: 'growth',
    salesScope: ['enterprise'],
    technicalStatus: 'MODULE_TECHNICAL_STATUS_NOT_READY',
    salesStatus: 'MODULE_SALES_STATUS_RETIRED',
    capabilityCodes: ['marketing.campaign'],
    quotaSchemaKeys: [],
    fieldPolicySchemaKeys: [],
    dependencies: ['customer-operations'],
    version: 3,
  },
]

const planVersionFixture = {
  planCode: 'office-pro',
  version: 3,
  revision: 8,
  planRevision: 5,
  state: 'PUBLISHED',
  name: '办公室专业版',
  terms: {
    modules: [{ moduleCode: 'device-operations', capabilityCodes: ['device.lifecycle'], quotas: [], fields: [] }],
    salesScope: ['default'],
    validityMode: 'unlimited',
    validityDays: 0,
    priceRef: 'price-office-pro',
  },
  contentSha256: 'visual-fixture',
  createdAt: '2026-09-01T08:00:00Z',
  publishedAt: '2026-09-02T08:00:00Z',
  retiredAt: '',
  actorId: 'platform-admin',
  reason: 'visual contract fixture',
}

async function installPlatformFixture(page: Page) {
  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = url.pathname
    const method = request.method()
    if (path === '/api/auth/session') {
      return route.fulfill({ json: { authenticated: true, actor_kind: 'platform', platform_subject: 'platform-admin', csrf_token: 'csrf' } })
    }
    if (path === '/api/v1/tenants' && method === 'GET') {
      return route.fulfill({ json: { tenants: [
        { id: 'tenant-a', name: '北区运营公司', status: 'TENANT_STATUS_ACTIVE', version: '12' },
        { id: 'tenant-b', name: '共享办公项目组', status: 'TENANT_STATUS_SUSPENDED', version: '7' },
        { id: 'tenant-c', name: '历史测试租户', status: 'TENANT_STATUS_CLOSED', version: '4' },
      ] } })
    }
    if (path === '/api/v1/platform/modules' && method === 'GET') {
      return route.fulfill({ json: { modules: moduleFixture } })
    }
    if (path === '/api/v1/platform/plans' && method === 'GET') {
      return route.fulfill({ json: { plans: [{ planCode: 'office-pro', name: '办公室专业版' }], nextAfterPlanCode: '' } })
    }
    if (path === '/api/v1/platform/plans/office-pro/versions' && method === 'GET') {
      return route.fulfill({ json: { versions: [planVersionFixture], nextAfterVersion: '' } })
    }
    if (path === '/api/v1/platform/tenants/tenant-a/subscription' && method === 'GET') {
      return route.fulfill({ json: {
        subscriptionId: 'sub-a', tenantId: 'tenant-a', kind: 'PLAN', state: 'ACTIVE', planCode: 'office-pro', planVersion: 3,
        ruleId: '', ruleVersion: 0, salesScope: 'default', entitlementSourceVersion: 19, createdAt: '2026-09-01T08:00:00Z',
        matchExplanation: '平台人工批准', revision: 4, periodStart: '2026-09-01T00:00:00Z', periodEnd: '2027-09-01T00:00:00Z',
        renewalStopped: false, pendingChangeId: '',
      } })
    }
    if (path === '/api/v1/platform/tenants/tenant-a/entitlement-overrides' && method === 'GET') {
      return route.fulfill({ json: { sourceVersion: 19, sources: [{
        id: 'override-a', tenantId: 'tenant-a', moduleCode: 'device-operations', target: 'ENTITLEMENT_TARGET_CAPABILITY',
        key: 'device.lifecycle', fieldAction: '', effect: 'ENTITLEMENT_EFFECT_GRANT', effectiveAt: '', expiresAt: '', revokedAt: '',
        reason: '客户合同专项能力', actorId: 'platform-admin', version: 2, sourceKind: 'override',
      }] } })
    }
    if (path === '/api/v1/platform/tenants/tenant-a/entitlements' && method === 'POST') {
      return route.fulfill({ json: {
        tenantId: 'tenant-a', sourceVersion: 19, resolverVersion: 6, evaluatedAt: '2026-09-13T06:00:00Z',
        validUntil: '2026-10-01T00:00:00Z', nextTransitionAt: '2026-10-01T00:00:00Z', entitlementVersion: 27, catalogRevision: 12,
        permissionVersion: '9', permissionSubject: 'platform-admin', catalogVersions: [{ moduleCode: 'device-operations', version: 12 }],
        decisions: [{ kind: 'CAPABILITY', moduleCode: 'device-operations', key: 'device.lifecycle', fieldAction: '', allowed: true,
          reason: '套餐 + 专项来源允许', masked: false, sources: [{ id: 'override-a', sourceKind: 'override', effect: 'GRANT', state: 'ACTIVE', disposition: 'applied', reason: '客户合同专项能力', actorId: 'platform-admin' }] }],
      } })
    }
    return route.fulfill({ status: 404, json: { message: `${method} ${path}` } })
  })
}

async function expectNoHorizontalOverflow(page: Page, width: number) {
  await page.evaluate(() => document.fonts.ready)
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
}

async function openOverviewPage(page: Page, width: number, height: number) {
  await page.setViewportSize({ width, height })
  await page.goto('/#/platform/overview')
  await expect(page.getByRole('heading', { name: '平台管理', exact: true, level: 1 })).toBeVisible()
  await expect(page.getByTestId('platform-overview')).toBeVisible()
  await expect(page.getByRole('link', { name: /租户管理/ })).toBeVisible()
  await expectNoHorizontalOverflow(page, width)
}

async function openTenantPage(page: Page, width: number, height: number) {
  await page.setViewportSize({ width, height })
  await page.goto('/#/platform/tenants')
  await expect(page.getByRole('heading', { name: '租户管理', exact: true, level: 1 })).toBeVisible()
  await expect(page.getByTestId('platform-tenant-management')).toBeVisible()
  await expect(page.getByText('北区运营公司', { exact: true })).toBeVisible()
  await expectNoHorizontalOverflow(page, width)
}

async function openModulePage(page: Page, width: number, height: number) {
  await page.setViewportSize({ width, height })
  await page.goto('/#/platform/commercial/modules')
  await expect(page.getByRole('heading', { name: '模块目录', exact: true, level: 1 })).toBeVisible()
  await expect(page.getByTestId('ce13-module-catalog')).toHaveAttribute('data-ui-template', 'ListPage')
  await expect(page.getByText('设备运营', { exact: true })).toBeVisible()
  await expect(page.locator('thead th')).toHaveCount(6)
  await expect(page.locator('.commercial-tabs')).toHaveCount(0)
  await expectNoHorizontalOverflow(page, width)
}

async function openPlanPage(page: Page, width: number, height: number) {
  await page.setViewportSize({ width, height })
  await page.goto('/#/platform/commercial/plans')
  await expect(page.getByRole('heading', { name: '套餐版本', exact: true, level: 1 })).toBeVisible()
  await page.getByLabel('套餐代码').fill('office-pro')
  await page.getByRole('button', { name: '读取版本', exact: true }).click()
  await expect(page.getByText('办公室专业版', { exact: true }).first()).toBeVisible()
  await expect(page.getByTestId('ce13-plan-management')).toHaveAttribute('data-ui-template', 'WorkbenchPage')
  await expect(page.locator('[data-ui-region="history"]')).toBeVisible()
  await expect(page.locator('[data-ui-region="detail"]')).toBeVisible()
  await expect(page.locator('.commercial-tabs')).toHaveCount(0)
  await expectNoHorizontalOverflow(page, width)
}

async function openEntitlementPage(page: Page, width: number, height: number) {
  await page.setViewportSize({ width, height })
  await page.goto('/#/platform/commercial/tenant-entitlements')
  await expect(page.getByRole('heading', { name: '租户权益', exact: true, level: 1 })).toBeVisible()
  await page.getByLabel('租户 ID').fill('tenant-a')
  await page.getByRole('button', { name: '读取权益', exact: true }).click()
  await expect(page.locator('.subscription-card')).toBeVisible()
  await expect(page.getByText('office-pro v3', { exact: true })).toBeVisible()
  await expect(page.getByTestId('ce13-tenant-entitlements')).toHaveAttribute('data-ui-template', 'WorkbenchPage')
  await expect(page.locator('[data-ui-region="decisions"]')).toBeVisible()
  await expect(page.locator('[data-ui-region="overrides"]')).toBeVisible()
  await expect(page.locator('.commercial-tabs')).toHaveCount(0)
  await expectNoHorizontalOverflow(page, width)
}

test.beforeEach(async ({ page }) => {
  mkdirSync(evidenceDir, { recursive: true })
  await installPlatformFixture(page)
})

test('platform overview WorkbenchPage captures the four CoffeeLink V1.1 viewports', async ({ page }) => {
  for (const [width, height] of viewports) {
    await openOverviewPage(page, width, height)
    await page.screenshot({ path: `${evidenceDir}/platform-overview-${width}.png`, fullPage: false })
  }
})

test('platform tenant ListPage captures the four CoffeeLink V1.1 viewports', async ({ page }) => {
  for (const [width, height] of viewports) {
    await openTenantPage(page, width, height)
    await page.screenshot({ path: `${evidenceDir}/platform-tenants-${width}.png`, fullPage: false })
  }
})

test('platform module catalog ListPage captures the four CoffeeLink V1.1 viewports', async ({ page }) => {
  for (const [width, height] of viewports) {
    await openModulePage(page, width, height)
    await page.screenshot({ path: `${evidenceDir}/platform-modules-${width}.png`, fullPage: false })
  }
})

test('platform plan WorkbenchPage captures the four CoffeeLink V1.1 viewports', async ({ page }) => {
  for (const [width, height] of viewports) {
    await openPlanPage(page, width, height)
    await page.screenshot({ path: `${evidenceDir}/platform-plans-${width}.png`, fullPage: false })
  }
})

test('platform entitlement WorkbenchPage captures the four CoffeeLink V1.1 viewports', async ({ page }) => {
  for (const [width, height] of viewports) {
    await openEntitlementPage(page, width, height)
    await page.screenshot({ path: `${evidenceDir}/platform-entitlements-${width}.png`, fullPage: false })
  }
})

test('platform navigation overlay never pushes content and supports collapsed rail evidence', async ({ page }) => {
  await openTenantPage(page, 1366, 768)
  const content = page.getByTestId('main-content')
  const before = await content.boundingBox()
  expect(before).not.toBeNull()

  await page.locator('[data-module="platform-commercial"]').click()
  await expect(page.locator('.module-panel')).toBeVisible()
  const after = await content.boundingBox()
  expect(after).not.toBeNull()
  expect(after!.x).toBeCloseTo(before!.x, 0)
  expect(after!.width).toBeCloseTo(before!.width, 0)
  await page.screenshot({ path: `${evidenceDir}/platform-tenants-menu-1366.png`, fullPage: false })

  await page.keyboard.press('Escape')
  await expect(page.locator('.module-panel')).toHaveCount(0)
  await page.getByRole('button', { name: '收起一级菜单' }).click()
  await page.locator('[data-module="platform-commercial"]').click()
  await expect(page.locator('.module-panel')).toBeVisible()
  await page.screenshot({ path: `${evidenceDir}/platform-tenants-collapsed-menu-1366.png`, fullPage: false })
})
