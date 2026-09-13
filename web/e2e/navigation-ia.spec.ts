import { expect, test, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'

async function ready(page: Page) {
  await page.goto('/#/enterprise/members')
  await expect(page.locator('.member-table')).toBeVisible()
  await page.evaluate(() => document.fonts.ready)
}

async function openDomain(page: Page, selector: string, title: string) {
  const trigger = page.locator(`[data-module="${selector}"]`)
  if (!(await trigger.isVisible())) {
    await page.getByRole('button', { name: '打开主导航', exact: true }).click()
    await expect(trigger).toBeVisible()
  }
  await trigger.click()
  const panel = page.getByRole('dialog', { name: `${title}导航`, exact: true })
  await expect(panel).toBeVisible()
  return panel
}

test('module panel renders grouped business domains without entity-bound global entries', async ({ page }) => {
  await ready(page)

  let panel = await openDomain(page, 'customers', '客户经营')
  await expect(panel.locator('[data-menu-group="客户管理"]')).toContainText('客户总览')
  await expect(panel.locator('[data-menu-group="客户协同"]')).toContainText('客户事项')
  await expect(panel.locator('[data-menu-group="客户成功"]')).toContainText('经营计划')
  await expect(panel.getByText('客户工作区', { exact: true })).toHaveCount(0)
  await expect(panel.getByText('事项看板', { exact: true })).toHaveCount(0)

  panel = await openDomain(page, 'sites', '租赁运营')
  await expect(panel.locator('[data-menu-group="投放与资产"]')).toContainText('合同与续约')
  await expect(panel.locator('[data-menu-group="计费与结算"]')).toContainText('租赁对账')
  await expect(panel.locator('[data-menu-group="履约与服务"]')).toContainText('投放交付')
  await expect(panel.getByTitle('投放交付尚未接入')).toBeDisabled()
  await expect(panel.getByTitle('服务恢复验证尚未接入')).toBeDisabled()
  await expect(panel.getByTitle('回款跟进尚未接入')).toBeDisabled()
  await expect(panel.getByTitle('退租回收尚未接入')).toBeDisabled()

  panel = await openDomain(page, 'devices', '设备运营')
  await expect(panel.locator('[data-menu-group="设备配置"]')).toContainText('饮品配置')
  await expect(panel.getByTitle('饮品配置尚未接入')).toBeDisabled()

  panel = await openDomain(page, 'orders', '经营管理')
  await expect(panel).toContainText('订单管理')
  await expect(panel).toContainText('数据分析')
  await expect(panel.getByText('饮品配置', { exact: true })).toHaveCount(0)
})

test('grouped customer and rental menus capture all CoffeeLink acceptance viewports', async ({ page }) => {
  mkdirSync('test-results/screenshots', { recursive: true })
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await ready(page)

    await openDomain(page, 'customers', '客户经营')
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({
      path: `test-results/screenshots/menu-customer-${viewport.width}.png`,
    })

    await openDomain(page, 'sites', '租赁运营')
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({
      path: `test-results/screenshots/menu-rental-${viewport.width}.png`,
    })
  }
})
