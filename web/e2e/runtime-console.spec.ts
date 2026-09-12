import { expect, test } from '@playwright/test'
import { mkdirSync } from 'node:fs'
test('live member mutation retries one command and never presents preview identity', async ({ page }) => {
  const keys: string[] = []
  let attempts = 0
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path === '/api/auth/session')
      return route.fulfill({
        json: {
          authenticated: true,
          actor_kind: 'tenant',
          user_id: 'real-user',
          active_tenant_id: 'tenant-1',
          tenants: [{ id: 'tenant-1', name: '真实企业' }],
          csrf_token: 'csrf',
        },
      })
    if (path === '/api/v1/tenant/members')
      return route.fulfill({
        json: {
          members: [
            {
              userId: 'member-1',
              email: 'member@example.com',
              status: 'TENANT_MEMBER_STATUS_ACTIVE',
              version: '9007199254740993',
            },
          ],
        },
      })
    if (path.endsWith('/member-1/suspend')) {
      keys.push(route.request().headers()['idempotency-key'] ?? '')
      expect(JSON.parse(route.request().headers()['x-biz-session-context'] ?? '{}').active_tenant_id).toBe(
        'tenant-1',
      )
      attempts++
      return route.fulfill({
        status: attempts === 1 ? 503 : 200,
        json:
          attempts === 1 ? { message: '暂时不可用' } : { userId: 'member-1', version: '9007199254740994' },
      })
    }
    return route.fulfill({ status: 404, json: { message: path } })
  })
  await page.goto('/#/workspace/members')
  await expect(page.getByRole('heading', { name: '业务成员', exact: true, level: 1 })).toBeVisible()
  await expect(page.getByText('真实企业', { exact: true })).toHaveCount(1)
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)
  mkdirSync('test-results/screenshots', { recursive: true })
  for (const [width, height] of [
    [1536, 1024],
    [1366, 768],
    [390, 844],
  ]) {
    await page.setViewportSize({ width: width!, height: height! })
    await page.screenshot({ path: `test-results/screenshots/runtime-${width}.png` })
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width!)
  }
  await page.setViewportSize({ width: 1366, height: 768 })
  await page
    .getByRole('row')
    .filter({ hasText: 'member@example.com' })
    .getByRole('button', { name: '停用', exact: true })
    .click()
  await page.getByRole('button', { name: '确认操作', exact: true }).click()
  await expect(page.getByRole('button', { name: '重试相同操作' })).toBeVisible()
  await page.getByRole('button', { name: '重试相同操作' }).click()
  await expect(page.getByRole('status')).toContainText('操作已提交')
  expect(keys).toHaveLength(2)
  expect(keys[0]).toBeTruthy()
  expect(keys[0]).toBe(keys[1])
})
test('tenant-only sessions do not load platform tenant records', async ({ page }) => {
  let lists = 0
  await page.route('**/api/**', async (route) => {
    if (route.request().url().endsWith('/auth/session'))
      return route.fulfill({
        json: { authenticated: true, actor_kind: 'tenant', user_id: 'u', active_tenant_id: 't', tenants: [] },
      })
    lists++
    return route.fulfill({ status: 403, json: { message: 'forbidden' } })
  })
  await page.goto('/#/platform/tenants')
  await expect(page.getByText('租户管理需要平台身份。')).toBeVisible()
  expect(lists).toBe(0)
})

test('platform account replacement invalidates the previous draft', async ({ page }) => {
  let account = 'platform-a',
    writes = 0
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/auth/session'))
      return route.fulfill({
        json: { authenticated: true, actor_kind: 'platform', platform_subject: account, csrf_token: 'csrf' },
      })
    if (route.request().method() !== 'GET') {
      writes++
      return route.fulfill({ json: {} })
    }
    return route.fulfill({
      json: { tenants: [{ id: 't', name: '真实租户', status: 'TENANT_STATUS_ACTIVE', version: '1' }] },
    })
  })
  await page.goto('/#/platform/tenants')
  await page
    .getByRole('row')
    .filter({ hasText: '真实租户' })
    .getByRole('button', { name: '停用', exact: true })
    .click()
  account = 'platform-b'
  await page.getByRole('button', { name: '确认操作', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('会话已变化')
  expect(writes).toBe(0)
})
test('authoritative plan discovery feeds the existing version workspace', async ({ page }) => {
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url())
    if (url.pathname === '/api/v1/platform/plans')
      return route.fulfill({ json: { plans: [{ planCode: 'real-plan', name: '权威套餐' }] } })
    if (url.pathname.endsWith('/versions')) return route.fulfill({ json: { versions: [] } })
    return route.fulfill({ json: { modules: [] } })
  })
  await page.goto('/#/platform/commercial/plans')
  await page.getByRole('button', { name: '读取套餐目录' }).click()
  await page.getByLabel('选择套餐', { exact: true }).selectOption('real-plan')
  await expect(page.locator('#plan-code')).toHaveValue('real-plan')
  await expect(page.getByText('没有找到 real-plan 的版本记录')).toBeVisible()
})
