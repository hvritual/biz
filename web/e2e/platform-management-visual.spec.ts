import { expect, test, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'

const evidenceDir = 'test-results/screenshots'

async function installPlatformFixture(page: Page) {
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url())
    if (url.pathname === '/api/auth/session') {
      return route.fulfill({
        json: {
          authenticated: true,
          actor_kind: 'platform',
          platform_subject: 'platform-admin',
          csrf_token: 'csrf',
        },
      })
    }
    if (url.pathname === '/api/v1/tenants' && route.request().method() === 'GET') {
      return route.fulfill({
        json: {
          tenants: [
            { id: 'tenant-a', name: '北区运营公司', status: 'TENANT_STATUS_ACTIVE', version: '12' },
            { id: 'tenant-b', name: '共享办公项目组', status: 'TENANT_STATUS_SUSPENDED', version: '7' },
            { id: 'tenant-c', name: '历史测试租户', status: 'TENANT_STATUS_CLOSED', version: '4' },
          ],
        },
      })
    }
    return route.fulfill({ status: 404, json: { message: url.pathname } })
  })
}

async function openTenantPage(page: Page, width: number, height: number) {
  await page.setViewportSize({ width, height })
  await page.goto('/#/platform/tenants')
  await expect(page.getByRole('heading', { name: '租户管理', exact: true, level: 1 })).toBeVisible()
  await expect(page.getByTestId('platform-tenant-management')).toBeVisible()
  await expect(page.getByText('北区运营公司', { exact: true })).toBeVisible()
  await page.evaluate(() => document.fonts.ready)
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
}

test.beforeEach(async ({ page }) => {
  mkdirSync(evidenceDir, { recursive: true })
  await installPlatformFixture(page)
})

test('platform tenant ListPage captures the four CoffeeLink V1.1 viewports', async ({ page }) => {
  for (const [width, height] of [
    [1366, 768],
    [1440, 900],
    [1536, 1024],
    [390, 844],
  ] as const) {
    await openTenantPage(page, width, height)
    await page.screenshot({ path: `${evidenceDir}/platform-tenants-${width}.png`, fullPage: false })
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
  await page.screenshot({
    path: `${evidenceDir}/platform-tenants-collapsed-menu-1366.png`,
    fullPage: false,
  })
})
