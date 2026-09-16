import { expect, test, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'

const pages = [
  ['features', '商业功能'],
  ['subscriptions', '租户订阅'],
  ['authorization', '授权诊断'],
  ['quotas', '额度管理'],
  ['usage-billing', '用量计费'],
  ['changes', '套餐变更'],
  ['add-ons', '增购项'],
  ['overrides', '专项授权'],
  ['expiry', '到期与宽限'],
  ['audit', '商业审计'],
] as const

const viewports = [
  { width: 1366, height: 768 },
  { width: 1440, height: 900 },
  { width: 1536, height: 1024 },
  { width: 390, height: 844 },
]

async function openLifecycle(page: Page, key: string, title: string) {
  await page.goto(`/#/platform/commercial/${key}`)
  await expect(page.getByRole('heading', { name: title, exact: true, level: 1 })).toBeVisible()
  await expect(page.locator(`[data-lifecycle-page="${key}"]`)).toBeVisible()
  await expect(page.getByText('服务端契约待接入', { exact: true })).toBeVisible()
  await expect(page.locator('[data-ui-region="metrics"] > *')).toHaveCount(4)
  await expect(page.locator('[data-ui-region="lifecycle"]')).toBeVisible()
  await expect(page.locator('[data-ui-region="data"] tbody tr').first()).toBeVisible()
}

test('platform lifecycle pages expose independent management routes and CoffeeLink interaction structure', async ({ page }) => {
  for (const [key, title] of pages) {
    await openLifecycle(page, key, title)
    await page.getByLabel(`搜索${title}`).fill('不存在的记录')
    await expect(page.getByText('没有符合当前条件的记录', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: '重置', exact: true }).click()
    await expect(page.locator('[data-ui-region="data"] tbody tr').first()).toBeVisible()
    await page.getByRole('button', { name: '查看', exact: true }).first().click()
    await expect(page.locator('[data-ui-region="detail"]')).toContainText('Authority / Evidence')
  }
})

test('platform lifecycle management captures all CoffeeLink acceptance viewports', async ({ page }) => {
  mkdirSync('test-results/screenshots', { recursive: true })
  for (const viewport of viewports) {
    await page.setViewportSize(viewport)
    for (const [key, title] of pages) {
      await openLifecycle(page, key, title)
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
      await page.screenshot({
        path: `test-results/screenshots/platform-lifecycle-${key}-${viewport.width}.png`,
        fullPage: true,
      })
    }
  }
})

test('platform lifecycle navigation stays grouped in the joined 480px flyout', async ({ page }) => {
  await page.setViewportSize({ width: 1366, height: 768 })
  await page.goto('/#/dashboard')
  const main = page.getByTestId('main-content')
  const before = await main.boundingBox()
  await page.getByRole('button', { name: '平台管理', exact: true }).click()
  const drawer = page.getByRole('dialog', { name: '平台管理导航' })
  await expect(drawer).toBeVisible()
  for (const group of ['租户生命周期', '产品与定价', '权益与授权', '计量与治理']) {
    await expect(drawer.getByText(group, { exact: true })).toBeVisible()
  }
  await expect(drawer.getByRole('button', { name: '商业功能', exact: true })).toBeVisible()
  await expect(drawer.getByRole('button', { name: '用量计费', exact: true })).toBeVisible()
  const box = await drawer.boundingBox()
  expect(box?.width).toBe(480)
  expect(await main.boundingBox()).toEqual(before)
})
