import { expect, test, type Page } from '@playwright/test'

async function ready(page: Page) {
  await page.goto('/#/enterprise/members')
  await expect(page.locator('.member-table')).toBeVisible()
}

async function openDomain(page: Page, selector: string, title: string) {
  await page.locator(`[data-module="${selector}"]`).click()
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
