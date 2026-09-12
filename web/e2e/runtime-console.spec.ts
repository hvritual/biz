import { expect, test } from '@playwright/test'
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
