import { test, expect } from '@playwright/test'
import { ready, action, fill, commit, snap, state } from './customer.helpers'

test('create customer with first work, follow-up readback and no duplicate on reload', async ({ page }) => {
  const d = await action(page, '/customers', 'create-customer')
  await fill(d, {
    name: '锦云酒店审核样例',
    category: '连锁酒店',
    title: '首批租赁点位需求确认',
    nextAction: '确认首批设备数量与安装条件',
  })
  await commit(d)
  await expect(page.getByRole('heading', { name: '锦云酒店审核样例', exact: true })).toBeVisible()
  await page.reload()
  const s = await state(page)
  expect(s.customers.filter((c: { name: string }) => c.name === '锦云酒店审核样例')).toHaveLength(1)
  expect(s.work.some((w: { title: string }) => w.title === '首批租赁点位需求确认')).toBe(true)
})
test('filter draft cancel and applied filter preserve actual result sets', async ({ page }) => {
  await ready(page, '/customers/work')
  const before = await page.locator('tbody tr').count()
  await page.getByLabel('搜索客户事项', { exact: true }).fill('不存在的事项')
  expect(await page.locator('tbody tr').count()).toBe(before)
  await page.getByRole('button', { name: '查询', exact: true }).click()
  await expect(page.locator('tbody tr')).toHaveCount(0)
  await page.getByRole('button', { name: '重置', exact: true }).click()
  await expect(page.locator('tbody tr')).toHaveCount(before)
})
test('kanban uses validated flow action rather than silent mutation', async ({ page }) => {
  await ready(page, '/customers/work?view=board')
  await page.getByRole('button', { name: '流转 CS-108', exact: true }).click()
  const d = page.getByRole('dialog')
  await fill(d, { status: '处理中', nextAction: '完成点位扩展现场准备' })
  await commit(d)
  expect((await state(page)).work.find((w: { id: string }) => w.id === 'CS-108').status).toBe('处理中')
})
test('tenant switch isolates customer changes and closes dirty dialog', async ({ page }) => {
  const d = await action(page, '/customers/accounts/CUS-0186', 'edit-customer')
  await fill(d, { name: '上海独立客户审核' })
  await commit(d)
  await ready(page, '/customers')
  await page.getByLabel('切换企业', { exact: true }).selectOption('hangzhou')
  await expect(page.locator('.customer-area')).not.toContainText('上海独立客户审核')
  await page.getByLabel('切换企业', { exact: true }).selectOption('shanghai')
  await expect(page.locator('.customer-area')).toContainText('上海独立客户审核')
})
test('safe archive restore and shared view revoke have real preview state', async ({ page }) => {
  const d = await action(page, '/customers/accounts/CUS-0176', 'archive')
  await fill(d, { reason: '历史业务均已完成处置，归档后保留记录。', confirm: true })
  await commit(d)
  expect((await state(page)).customers.find((c: { id: string }) => c.id === 'CUS-0176').archived).toBe(true)
  const restore = await action(page, '/customers/accounts/CUS-0176', 'restore')
  await fill(restore, { reason: '恢复客户经营入口，保留原关系状态。' })
  await commit(restore)
  const revoke = await action(page, '/customers/sharing?target=SH-031', 'revoke-share')
  await fill(revoke, { reason: '验收协作结束，撤销该预览授权。' })
  await commit(revoke)
  await ready(page, '/customers/client/SH-031')
  await expect(page.locator('.customer-area')).toContainText('授权')
  await expect(page.getByRole('button', { name: '确认交付', exact: true })).toHaveCount(0)
})
test('base pages and responsive viewports contain real content and no overflow', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (msg) => {
    if (msg.type() === 'error') errors.push(msg.text())
  })
  await page.setViewportSize({ width: 1536, height: 1024 })
  for (const [path, name] of [
    ['/customers', '39-customer-overview'],
    ['/customers/work?view=board', '40-issue-board'],
    ['/customers/work?view=calendar', '41-issue-calendar'],
    ['/customers/contracts', '42-contracts'],
    ['/customers/plans', '43-plans'],
    ['/customers/automation', '44-automation-rules'],
    ['/customers/sharing', '45-client-sharing'],
  ]) {
    await ready(page, path!)
    await snap(page, name!)
  }
  await ready(page, '/customers')
  await page.getByRole('button', { name: '收起一级菜单', exact: true }).click()
  await page.locator('[data-module="customers"]').click()
  const foldedNav = (await page.locator('.primary-nav').boundingBox())!
  expect((await page.locator('.module-panel').boundingBox())!.x).toBe(foldedNav.x + foldedNav.width)
  await snap(page, '46-collapsed-connected-nav')
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: '展开一级菜单', exact: true }).click()
  for (const [width, height] of [
    [1366, 768],
    [1440, 900],
    [390, 844],
  ]) {
    await page.setViewportSize({ width: width!, height: height! })
    await ready(page, '/customers')
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width!)
    await snap(page, `47-customer-${width}`)
    await ready(page, '/customers/work/CS-104')
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width!)
    await snap(page, `48-delivery-${width}`)
  }
  expect(errors).toEqual([])
})

test('other customer professional details show missing sources rather than unrelated records', async ({
  page,
}) => {
  await ready(page, '/customers/work/CS-113')
  await expect(page.locator('.customer-area')).toContainText('未关联')
  await expect(page.locator('.customer-area')).not.toContainText('AR-0901')
  await expect(page.locator('.customer-area')).not.toContainText('12,000')
  await ready(page, '/customers')
  await page.getByRole('button', { name: '存在风险', exact: true }).click()
  await expect(page.locator('tbody tr')).toHaveCount(3)
  await expect(page.locator('tbody')).not.toContainText('无已识别风险')
})
