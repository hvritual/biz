import { expect, test, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'
async function ready(page: Page) {
  await page.goto('/#/enterprise/members')
  await expect(page.locator('.member-table')).toBeVisible()
  await page.evaluate(() => document.fonts.ready)
}
test('reference body has four summary cards, explicit filters and independent table tools', async ({
  page,
}) => {
  await ready(page)
  await expect(page.locator('.overview-card')).toHaveCount(4)
  await expect(page.locator('.overview-card').nth(1)).toContainText('部门数')
  await expect(page.locator('.quota-card')).toContainText('368')
  await expect(page.locator('.quota-card')).toContainText('500')
  await expect(page.getByRole('button', { name: '批量启用', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: '批量停用', exact: true })).toBeDisabled()
  await page.getByLabel('搜索成员', { exact: true }).fill('lisi@example.com')
  await expect(page.locator('.member-table tbody tr')).toHaveCount(10)
  await page.getByRole('button', { name: '查询', exact: true }).click()
  await expect(page.locator('.member-table tbody tr')).toHaveCount(1)
  await page.getByRole('button', { name: '重置', exact: true }).click()
  await expect(page.getByLabel('搜索成员', { exact: true })).toHaveValue('')
  await expect(page.locator('.member-table tbody tr')).toHaveCount(10)
  await expect(page.locator('.member-table th')).toHaveCount(10)
})
test('view slides out details without a desktop scrim, preserves art and does not select the row', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1536, height: 1024 })
  await ready(page)
  const art = page.locator('.coffee-hero img'),
    src = await art.getAttribute('src')
  await expect(page.getByRole('dialog', { name: '成员详情', exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: '查看 李四', exact: true }).click()
  const drawer = page.getByRole('dialog', { name: '成员详情', exact: true })
  await expect(drawer).toBeVisible()
  await expect(drawer).toHaveAttribute('aria-modal', 'false')
  await expect(page.locator('.member-detail-backdrop')).toHaveCount(0)
  await expect(drawer).toContainText('lisi@example.com')
  await expect(drawer).toContainText('个人信息')
  await expect(drawer).toContainText('组织信息')
  await expect(page.locator('.member-table tr.viewing')).toHaveCount(1)
  await expect(page.getByLabel('选择 李四', { exact: true })).not.toBeChecked()
  await expect(art).toBeVisible()
  await expect(art).toHaveAttribute('src', src!)
  await page.waitForTimeout(350)
  const artBox = await art.boundingBox(),
    detailBox = await drawer.boundingBox()
  expect(artBox!.x + artBox!.width).toBeLessThanOrEqual(detailBox!.x)
  await drawer.getByRole('tab', { name: '角色权限', exact: true }).click()
  await expect(drawer).toContainText('tenant.member.read')
  await drawer.getByRole('tab', { name: '数据权限', exact: true }).click()
  await expect(drawer).toContainText('当前数据范围')
  await drawer.getByRole('tab', { name: '操作日志', exact: true }).click()
  await expect(drawer).toContainText('暂无操作记录')
  await page.keyboard.press('Escape')
  await expect(drawer).toHaveCount(0)
  await expect(page.getByRole('button', { name: '查看 李四', exact: true })).toBeFocused()
  await expect(page.locator('.member-table tr.viewing')).toHaveCount(0)
})
test('switching viewed member refreshes the details and resets the active tab', async ({ page }) => {
  await ready(page)
  await page.getByRole('button', { name: '查看 李四', exact: true }).click()
  const drawer = page.getByRole('dialog', { name: '成员详情', exact: true })
  await drawer.getByRole('tab', { name: '数据权限', exact: true }).click()
  await page.getByRole('button', { name: '查看 王五', exact: true }).click()
  await expect(drawer).toContainText('wangwu@example.com')
  await expect(drawer.getByRole('tab', { name: '基本信息', exact: true })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  await drawer.getByRole('button', { name: '编辑成员资料', exact: true }).click()
  await expect(page.getByRole('dialog', { name: '修改成员信息', exact: true })).toBeVisible()
  await expect(drawer).toHaveCount(0)
})
test('batch disable and enable preserve reason validation and current-page scope', async ({ page }) => {
  await ready(page)
  await page.getByLabel('选择 李四', { exact: true }).check()
  await page.getByLabel('选择 王五', { exact: true }).check()
  await expect(page.locator('.selection-label')).toContainText('2')
  await page.getByRole('button', { name: '批量停用', exact: true }).click()
  let d = page.getByRole('dialog', { name: '批量停用成员', exact: true })
  await d.getByRole('button', { name: '确认批量停用', exact: true }).click()
  await expect(d.getByRole('alert')).toContainText('原因')
  await d.getByLabel('操作原因', { exact: true }).fill('暂时离岗')
  await d.getByRole('checkbox').check()
  await d.getByRole('button', { name: '确认批量停用', exact: true }).click()
  await expect(d).toHaveCount(0)
  await expect(page.locator('[data-member-id="member-2"]')).toContainText('已禁用')
  await expect(page.locator('[data-member-id="member-3"]')).toContainText('已禁用')
  await page.getByLabel('选择 李四', { exact: true }).check()
  await page.getByLabel('选择 王五', { exact: true }).check()
  await page.getByRole('button', { name: '批量启用', exact: true }).click()
  d = page.getByRole('dialog', { name: '批量启用成员', exact: true })
  await d.getByLabel('操作原因', { exact: true }).fill('身份复核完成')
  await d.getByRole('checkbox').check()
  await d.getByRole('button', { name: '确认批量启用', exact: true }).click()
  await expect(page.locator('[data-member-id="member-2"]')).toContainText('启用')
  await expect(page.locator('.selection-label')).toContainText('0')
})
test('detail state is isolated on tenant switch and navigation flyout stays an overlay', async ({ page }) => {
  await ready(page)
  await page.getByRole('button', { name: '查看 李四', exact: true }).click()
  await page.getByLabel('切换企业', { exact: true }).selectOption('hangzhou')
  await expect(page.locator('.member-detail-panel')).toHaveCount(0)
  await expect(page.locator('.member-overview')).toContainText('24')
  await page.locator('[data-module="enterprise"]').click()
  await expect(page.locator('.module-panel')).toBeVisible()
  expect((await page.locator('.module-panel').boundingBox())!.width).toBe(480)
})
test('mobile detail is a dismissible modal with no horizontal page overflow', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await ready(page)
  await page.getByRole('button', { name: '查看 李四', exact: true }).click()
  const d = page.getByRole('dialog', { name: '成员详情', exact: true })
  await expect(d).toHaveAttribute('aria-modal', 'true')
  await expect(d).toContainText('lisi@example.com')
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
  await d.getByRole('button', { name: '关闭成员详情', exact: true }).click()
  await expect(d).toHaveCount(0)
  expect(await page.locator('#app').evaluate((el) => (el as HTMLElement).inert)).toBe(false)
})
test('member reference screenshots from actual browser interactions', async ({ page }) => {
  mkdirSync('screenshots/member-reference', { recursive: true })
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (m) => {
    if (m.type() === 'error') errors.push(m.text())
  })
  for (const viewport of [
    { width: 1536, height: 1024 },
    { width: 1366, height: 768 },
  ]) {
    await page.setViewportSize(viewport)
    await ready(page)
    await page.screenshot({
      path: `screenshots/member-reference/members-${viewport.width}-closed.png`,
      fullPage: true,
    })
    await page.getByRole('button', { name: '查看 李四', exact: true }).click()
    await expect(page.locator('.member-detail-panel')).toBeVisible()
    await page.waitForTimeout(350)
    await page.screenshot({
      path: `screenshots/member-reference/members-${viewport.width}-detail.png`,
      fullPage: true,
    })
    await page.getByRole('button', { name: '关闭成员详情', exact: true }).click()
    await expect(page.locator('.member-detail-panel')).toHaveCount(0)
  }
  expect(errors).toEqual([])
})
