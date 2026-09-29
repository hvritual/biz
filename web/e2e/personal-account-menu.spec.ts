import { expect, test, type Page } from '@playwright/test'

async function ready(page: Page) {
  await page.goto('/#/enterprise/members')
  await expect(page.locator('.member-table')).toBeVisible()
}

test('avatar account menu is the personal-center entry and exposes account actions', async ({ page }) => {
  await ready(page)
  const account = page.getByLabel('当前账号', { exact: true })
  await account.hover()
  const menu = page.getByRole('menu', { name: '账号菜单', exact: true })
  await expect(menu.getByRole('menuitem', { name: '个人中心', exact: true })).toBeVisible()
  await expect(menu.getByRole('menuitem', { name: '版本变更', exact: true })).toBeVisible()
  await expect(menu.getByRole('menuitem', { name: '退出登录', exact: true })).toBeVisible()
  await page.mouse.move(640, 500)
  await expect(menu).toBeVisible()
  await page.locator('.global-search').click()
  await expect(menu).toHaveCount(0)
  await account.click()
  await expect(menu).toBeVisible()
  await menu.getByRole('menuitem', { name: '个人中心', exact: true }).click()
  await expect(page).toHaveURL(/#\/enterprise\/personal-profile$/)
})
