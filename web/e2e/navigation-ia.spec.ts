import { expect, test, type Locator, type Page } from '@playwright/test'
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

async function expectBeforeFooter(item: Locator, panel: Locator) {
  const itemBox = await item.boundingBox()
  const footerBox = await panel.locator('.menu-art').boundingBox()
  expect(itemBox).not.toBeNull()
  expect(footerBox).not.toBeNull()
  expect(itemBox!.y + itemBox!.height).toBeLessThanOrEqual(footerBox!.y)
}

async function expectScrimFillsRightViewport(page: Page) {
  const scrim = page.locator('.navigation-scrim')
  await expect(scrim).toBeVisible()
  const box = await scrim.boundingBox()
  const viewport = page.viewportSize()
  const headerHeight = await page.evaluate(() =>
    Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--header-height')),
  )
  expect(box).not.toBeNull()
  expect(viewport).not.toBeNull()
  expect(box!.y).toBeCloseTo(headerHeight, 0)
  expect(box!.height).toBeCloseTo(viewport!.height - headerHeight, 0)
  expect(box!.y + box!.height).toBeCloseTo(viewport!.height, 0)
}

test('module panel renders grouped business domains without entity-bound global entries', async ({ page }) => {
  await ready(page)

  let panel = await openDomain(page, 'customers', '客户经营')
  await expect(panel.locator('[data-menu-group="customer-management"]')).toContainText('客户总览')
  await expect(panel.locator('[data-menu-group="customer-collaboration"]')).toContainText('客户事项')
  await expect(panel.locator('[data-menu-group="customer-success"]')).toContainText('经营计划')
  await expect(panel.getByText('客户工作区', { exact: true })).toHaveCount(0)
  await expect(panel.getByText('事项看板', { exact: true })).toHaveCount(0)

  panel = await openDomain(page, 'sites', '租赁运营')
  await expect(panel.locator('[data-menu-group="deployment-assets"]')).toContainText('合同与续约')
  await expect(panel.locator('[data-menu-group="billing-settlement"]')).toContainText('租赁对账')
  await expect(panel.locator('[data-menu-group="fulfillment-service"]')).toContainText('投放交付')
  for (const label of ['投放交付', '服务恢复验证', '回款跟进', '退租回收']) {
    await expect(panel.getByRole('button', { name: label, exact: true })).toBeEnabled()
  }

  panel = await openDomain(page, 'devices', '设备运营')
  await expect(panel.locator('[data-menu-group="device-configuration"]')).toContainText('饮品配置')
  await expect(panel.getByRole('button', { name: /饮品配置/ })).toBeDisabled()

  panel = await openDomain(page, 'orders', '经营管理')
  await expect(panel).toContainText('订单管理')
  await expect(panel).toContainText('数据分析')
  await expect(panel.getByText('饮品配置', { exact: true })).toHaveCount(0)
})

test('rental operation entries open scoped collection workspaces and preserve rental navigation context', async ({ page }) => {
  const cases = [
    ['投放交付', '/#/rental/delivery', '苏州三点位投放交付'],
    ['服务恢复验证', '/#/rental/service', '大堂设备恢复验证'],
    ['回款跟进', '/#/rental/payment', '9 月应收回款跟进'],
    ['退租回收', '/#/rental/returns', '虹桥点位退租回收'],
  ] as const

  for (const [label, url, item] of cases) {
    await ready(page)
    const panel = await openDomain(page, 'sites', '租赁运营')
    await panel.getByRole('button', { name: label, exact: true }).click()
    await expect(page).toHaveURL(new RegExp(url.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '$'))
    await expect(page.getByRole('heading', { name: `${label}工作台`, exact: true })).toBeVisible()
    await expect(page.getByText(item, { exact: true })).toBeVisible()
    await expect(page.locator('[data-module="sites"]')).toHaveClass(/active/)
    await expect(page.locator('[data-work-scope]')).toHaveAttribute('data-work-scope')
  }
})

test('navigation scrim fills the complete right-side viewport below the header', async ({ page }) => {
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
  ]) {
    await page.setViewportSize(viewport)
    await ready(page)
    await openDomain(page, 'enterprise', '企业中心')
    await expectScrimFillsRightViewport(page)
    await page.keyboard.press('Escape')
  }
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

    let panel = await openDomain(page, 'customers', '客户经营')
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await expectBeforeFooter(panel.getByRole('button', { name: '经营结果', exact: true }), panel)
    await page.screenshot({ path: `test-results/screenshots/menu-customer-${viewport.width}.png` })

    panel = await openDomain(page, 'sites', '租赁运营')
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await expectBeforeFooter(panel.getByRole('button', { name: '退租回收', exact: true }), panel)
    await page.screenshot({ path: `test-results/screenshots/menu-rental-${viewport.width}.png` })
  }
})

test('rental collection workspaces capture all CoffeeLink acceptance viewports', async ({ page }) => {
  mkdirSync('test-results/screenshots', { recursive: true })
  const flows = [
    ['delivery', '投放交付'],
    ['service', '服务恢复验证'],
    ['payment', '回款跟进'],
    ['returns', '退租回收'],
  ] as const
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    for (const [path, label] of flows) {
      await page.goto(`/#/rental/${path}`)
      await expect(page.getByRole('heading', { name: `${label}工作台`, exact: true })).toBeVisible()
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
      await page.screenshot({ path: `test-results/screenshots/rental-${path}-${viewport.width}.png` })
    }
  }
})
