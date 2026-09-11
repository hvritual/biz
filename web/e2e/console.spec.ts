import { test, expect, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'
const screenDir = 'screenshots'
async function openMenu(page: Page) {
  await page.locator('[data-module="enterprise"]').click()
  await expect(page.locator('.module-panel')).toBeVisible()
}
async function memberAction(page: Page, name: string, action: string) {
  await page.getByRole('button', { name: `${name} 更多操作`, exact: true }).click()
  await page.getByRole('dialog').getByRole('button', { name: action, exact: true }).click()
}
async function ready(page: Page, path = '/enterprise/members') {
  await page.goto('/#' + path)
  await expect(page.locator('h1')).toBeVisible()
  await page.evaluate(() => document.fonts.ready)
}

test('480px flyout is joined, overlays without reflow, no submenu arrows, and supports Escape', async ({
  page,
}) => {
  await ready(page)
  const main = page.getByTestId('main-content')
  const before = await main.boundingBox()
  await openMenu(page)
  const drawer = await page.locator('.module-panel').boundingBox()
  const primary = await page.locator('.primary-nav').boundingBox()
  expect(drawer!.width).toBe(480)
  expect(drawer!.x).toBe(primary!.x + primary!.width)
  expect(await main.boundingBox()).toEqual(before)
  expect(await page.locator('.sub-link.active').evaluate((el) => el.clientWidth)).toBeLessThan(180)
  await expect(page.locator('.sub-link .lucide-chevron-right')).toHaveCount(0)
  const left = await page.locator('.sub-navigation').boundingBox(),
    right = await page.locator('.quick-navigation').boundingBox()
  expect(right!.x).toBeGreaterThan(left!.x)
  await page.keyboard.press('Escape')
  await expect(page.locator('.module-panel')).toHaveCount(0)
  await expect(page.locator('[data-module="enterprise"]')).toBeFocused()
})
test('collapse and expand preserve working navigation', async ({ page }) => {
  await ready(page)
  const expandedWidth = (await page.locator('.primary-nav').boundingBox())!.width
  await page.getByRole('button', { name: '收起一级菜单', exact: true }).click()
  expect((await page.locator('.primary-nav').boundingBox())!.width).toBe(64)
  await openMenu(page)
  const foldedNav = (await page.locator('.primary-nav').boundingBox())!
  expect((await page.locator('.module-panel').boundingBox())!.x).toBe(foldedNav.x + foldedNav.width)
  await page.getByRole('button', { name: '关闭模块菜单', exact: true }).click()
  await page.getByRole('button', { name: '展开一级菜单', exact: true }).click()
  expect((await page.locator('.primary-nav').boundingBox())!.width).toBe(expandedWidth)
})
test('hovering a module navigation item opens its side menu', async ({ page }) => {
  await ready(page)
  await page.locator('[data-module="devices"]').hover()
  await expect(page.locator('.module-panel')).toBeVisible()
  await expect(page.locator('.module-panel')).toContainText('设备管理')
})
test('module menu close button closes the side menu', async ({ page }) => {
  await ready(page)
  await openMenu(page)
  await page.getByRole('button', { name: '关闭模块菜单', exact: true }).click()
  await expect(page.locator('.module-panel')).toHaveCount(0)
})
test('filters, empty state, pagination and current-page selection work', async ({ page }) => {
  await ready(page)
  await expect(page.locator('.member-table tbody tr')).toHaveCount(10)
  await page.getByRole('checkbox', { name: '选择当前页全部成员', exact: true }).check()
  await expect(page.getByRole('button', { name: '导出已选', exact: true })).toBeVisible()
  await page.getByLabel('搜索成员', { exact: true }).fill('nobody-impossible')
  await expect(page.locator('.member-table tbody tr')).toHaveCount(0)
  await page.getByLabel('搜索成员', { exact: true }).fill('lisi@example.com')
  await expect(page.locator('.member-table tbody tr')).toHaveCount(1)
  await expect(page.getByRole('button', { name: '导出列表', exact: true })).toBeVisible()
})
test('member information edit persists and is audited', async ({ page }) => {
  await ready(page)
  await page.getByRole('button', { name: '编辑 李四', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '修改成员信息' })
  await dialog.getByLabel('姓名', { exact: true }).fill('李四 QA')
  await dialog.getByRole('button', { name: '保存变更', exact: true }).click()
  await expect(dialog).toHaveCount(0)
  await page.reload()
  await expect(page.getByRole('button', { name: '编辑 李四 QA', exact: true })).toBeVisible()
  await ready(page, '/enterprise/logs')
  await expect(page.locator('tbody tr').first()).toContainText('修改成员信息')
})
test('invite creates pending local record without external mail requests', async ({ page }) => {
  await ready(page)
  await page.getByRole('button', { name: '邀请成员', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '邀请成员', exact: true })
  await dialog.getByLabel('姓名', { exact: true }).fill('邀请测试')
  await dialog.getByLabel('邮箱', { exact: true }).fill('invited@example.com')
  await dialog.getByRole('button', { name: '创建邀请', exact: true }).click()
  await expect(page.locator('.member-table tbody tr').first()).toContainText('待激活')
  await expect(page.getByRole('status')).toContainText('未实际发送邀请邮件')
})
test('role changes show before-after preview and persist', async ({ page }) => {
  await ready(page)
  await memberAction(page, '李四', '角色与数据权限变更')
  const d = page.getByRole('dialog', { name: '角色变更与权限调整' })
  await d.getByRole('checkbox', { name: '运营管理员', exact: true }).uncheck()
  await d.getByRole('checkbox', { name: '销售经理', exact: true }).check()
  await expect(d.locator('.change-preview')).toContainText('销售经理')
  await d.getByRole('button', { name: '保存变更', exact: true }).click()
  await expect(page.locator('.member-table tbody tr').nth(1)).toContainText('销售经理')
})
test('suspend and activate require reason and update state', async ({ page }) => {
  await ready(page)
  await memberAction(page, '李四', '禁用当前企业访问')
  let d = page.getByRole('dialog', { name: '禁用成员', exact: true })
  await d.getByRole('checkbox').check()
  await d.getByRole('button', { name: '确认禁用', exact: true }).click()
  await expect(d.getByRole('alert')).toContainText('操作原因')
  await d.getByLabel('操作原因', { exact: true }).fill('岗位调整，暂时停用')
  await d.getByRole('button', { name: '确认禁用', exact: true }).click()
  await expect(page.locator('.member-table tbody tr').nth(1)).toContainText('已禁用')
  await memberAction(page, '李四', '重新启用成员')
  d = page.getByRole('dialog', { name: '重新启用成员', exact: true })
  await d.getByLabel('操作原因', { exact: true }).fill('身份和角色已复核')
  await d.getByRole('button', { name: '确认启用', exact: true }).click()
  await expect(page.locator('.member-table tbody tr').nth(1).getByText('启用', { exact: true })).toBeVisible()
})
test('last owner is protected in the UI', async ({ page }) => {
  await ready(page)
  await memberAction(page, '张三', '禁用当前企业访问')
  const d = page.getByRole('dialog', { name: '禁用成员', exact: true })
  await expect(d).toContainText('最后一位企业所有者')
  await expect(d.getByRole('button', { name: '确认禁用', exact: true })).toBeDisabled()
})
test('password reset records explicit preview request, no plaintext password', async ({ page }) => {
  await ready(page)
  await memberAction(page, '李四', '密码重置')
  const d = page.getByRole('dialog', { name: '密码重置', exact: true })
  await expect(d).toContainText('不执行真实重置')
  await expect(d.locator('input[type="password"]')).toHaveCount(0)
  await d.getByRole('checkbox').check()
  await d.getByRole('button', { name: '记录重置请求', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('未发送邮件')
  await ready(page, '/enterprise/logs')
  await expect(page.locator('tbody tr').first()).toContainText('创建密码重置请求')
})
test('tenant switch clears queries, selections, dialogs and isolates data', async ({ page }) => {
  await ready(page)
  await page.getByRole('button', { name: '编辑 李四', exact: true }).click()
  const d = page.getByRole('dialog')
  await d.getByLabel('姓名', { exact: true }).fill('上海独立成员')
  await d.getByRole('button', { name: '保存变更', exact: true }).click()
  await page.getByLabel('搜索成员', { exact: true }).fill('上海独立成员')
  await page.getByLabel('切换企业', { exact: true }).selectOption('hangzhou')
  await expect(page.getByLabel('搜索成员', { exact: true })).toHaveValue('')
  await expect(page.locator('.metric-value').first()).toContainText('24')
  await expect(page.getByRole('button', { name: '编辑 李四', exact: true })).toBeVisible()
  await page.getByLabel('切换企业', { exact: true }).selectOption('shanghai')
  await expect(page.getByRole('button', { name: '编辑 上海独立成员', exact: true })).toBeVisible()
})
test('role creation uses real permissions matrix, builtin role read-only', async ({ page }) => {
  await ready(page, '/enterprise/roles')
  await page.getByRole('button', { name: '新建角色', exact: true }).click()
  const d = page.getByRole('dialog', { name: '新建角色', exact: true })
  await d.getByLabel('角色名称', { exact: true }).fill('华东只读 QA')
  await d.getByRole('checkbox', { name: '成员管理 查看', exact: true }).check()
  await d.getByRole('button', { name: '保存角色', exact: true }).click()
  await expect(page.locator('table')).toContainText('华东只读 QA')
})
test('company edits persist and validate required name', async ({ page }) => {
  await ready(page, '/enterprise/company')
  await page.getByLabel('企业简介', { exact: true }).fill('团队管理 QA 预览')
  await page.getByRole('button', { name: '保存修改', exact: true }).click()
  await page.reload()
  await expect(page.getByLabel('企业简介', { exact: true })).toHaveValue('团队管理 QA 预览')
})
test('security policy validates numeric boundaries', async ({ page }) => {
  await ready(page, '/system/security')
  await page.getByLabel('空闲会话时长', { exact: true }).fill('1')
  await page.getByRole('button', { name: '保存策略草稿', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('5–480')
  await page.getByLabel('空闲会话时长', { exact: true }).fill('45')
  await page.getByRole('button', { name: '保存策略草稿', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('尚未应用到身份服务')
})

test('visual gallery: native viewport, menu, collapsed and all implemented pages', async ({ page }) => {
  mkdirSync(screenDir, { recursive: true })
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (msg) => {
    if (msg.type() === 'error') errors.push(msg.text())
  })
  await page.setViewportSize({ width: 1536, height: 1024 })
  await ready(page)
  await page.screenshot({ path: `${screenDir}/01-members-desktop.png`, fullPage: true })
  await openMenu(page)
  await page.screenshot({ path: `${screenDir}/02-overlay-menu-480.png` })
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: '收起一级菜单', exact: true }).click()
  await openMenu(page)
  await page.screenshot({ path: `${screenDir}/03-collapsed-menu.png` })
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: '展开一级菜单', exact: true }).click()
  for (const [path, file] of [
    ['/dashboard', '04-workbench'],
    ['/enterprise/roles', '05-roles'],
    ['/enterprise/organization', '06-organization'],
    ['/enterprise/plan', '07-plan'],
    ['/enterprise/company', '08-company'],
    ['/enterprise/logs', '09-audit'],
    ['/system/general', '10-settings'],
    ['/system/security', '11-security'],
    ['/system/notifications', '12-notifications'],
    ['/system/integrations', '13-integrations'],
    ['/system/dictionary', '14-dictionary'],
  ]) {
    await ready(page, path!)
    await page.screenshot({ path: `${screenDir}/${file}.png`, fullPage: true })
  }
  await ready(page)
  await memberAction(page, '李四', '角色与数据权限变更')
  await page.screenshot({ path: `${screenDir}/15-member-role-change.png` })
  await page.getByRole('button', { name: '关闭弹窗', exact: true }).click()
  await memberAction(page, '李四', '密码重置')
  await page.screenshot({ path: `${screenDir}/16-password-reset.png` })
  await page.getByRole('button', { name: '关闭弹窗', exact: true }).click()
  await page.setViewportSize({ width: 1366, height: 768 })
  await page.screenshot({ path: `${screenDir}/17-laptop-1366.png` })
  await openMenu(page)
  await page.screenshot({ path: `${screenDir}/18-laptop-menu.png` })
  await page.keyboard.press('Escape')
  await page.setViewportSize({ width: 390, height: 844 })
  await page.screenshot({ path: `${screenDir}/19-mobile-members.png` })
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390)
  await page.getByRole('button', { name: '打开主导航', exact: true }).click()
  await openMenu(page)
  await page.screenshot({ path: `${screenDir}/20-mobile-menu.png` })
  expect((await page.locator('.module-panel').boundingBox())!.width).toBe(310)
  expect(errors).toEqual([])
})
