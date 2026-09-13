import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_MEMBER_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type RemoteMember = { userId: string; email: string; status: string; version: number }

type MockOptions = {
  listStatus?: number
  mutationStatus?: number
  readbackStatus?: number
  unauthenticated?: boolean
}

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function mockMemberServer(page: Page, options: MockOptions = {}) {
  let members: RemoteMember[] = [
    { userId: 'user-001', email: 'owner@coffeelink.test', status: 'TENANT_MEMBER_STATUS_ACTIVE', version: 3 },
    { userId: 'user-002', email: 'new@coffeelink.test', status: 'TENANT_MEMBER_STATUS_INVITED', version: 1 },
  ]
  let lastWriteHeaders: Record<string, string> = {}
  const writeHeaderHistory: Record<string, string>[] = []

  await page.route('**/api/auth/session', async (route) => {
    if (options.unauthenticated) return json(route, 401, { message: 'unauthenticated' })
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-001',
      active_tenant_id: 'tenant-001',
      csrf_token: 'csrf-real-member',
      tenants: [{ id: 'tenant-001', name: 'CoffeeLink 测试租户' }],
    })
  })

  await page.route('**/api/v1/tenant/members', async (route) => {
    const request = route.request()
    if (request.method() === 'GET') {
      if (options.listStatus) return json(route, options.listStatus, { message: 'list denied' })
      return json(route, 200, { members })
    }
    lastWriteHeaders = request.headers()
    writeHeaderHistory.push(lastWriteHeaders)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'mutation conflict' })
    const body = request.postDataJSON() as { email: string }
    const created: RemoteMember = {
      userId: 'user-003',
      email: body.email,
      status: 'TENANT_MEMBER_STATUS_INVITED',
      version: 1,
    }
    members = [...members, created]
    return json(route, 200, created)
  })

  await page.route(/\/api\/v1\/tenant\/members\/[^/]+(?:\/(?:activate|suspend|remove))?$/, async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const parts = url.pathname.split('/')
    const userId = decodeURIComponent(parts[parts.indexOf('members') + 1] || '')
    const action = parts.at(-1)
    const current = members.find((member) => member.userId === userId)
    if (!current) return json(route, 404, { message: 'not found' })

    if (request.method() === 'GET') {
      if (options.readbackStatus) return json(route, options.readbackStatus, { message: 'readback failed' })
      return json(route, 200, current)
    }

    lastWriteHeaders = request.headers()
    writeHeaderHistory.push(lastWriteHeaders)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'mutation conflict' })
    const nextStatus =
      action === 'activate'
        ? 'TENANT_MEMBER_STATUS_ACTIVE'
        : action === 'suspend'
          ? 'TENANT_MEMBER_STATUS_SUSPENDED'
          : 'TENANT_MEMBER_STATUS_REMOVED'
    const updated = { ...current, status: nextStatus, version: current.version + 1 }
    members = members.map((member) => (member.userId === userId ? updated : member))
    return json(route, 200, updated)
  })

  return {
    getMembers: () => members,
    getLastWriteHeaders: () => lastWriteHeaders,
    getWriteHeaderHistory: () => writeHeaderHistory,
  }
}

async function openRealMembers(page: Page) {
  await page.goto('/#/enterprise/members')
  await expect(page.locator('[data-enterprise-member-source="server"]')).toBeVisible()
}

test('real member page reads authoritative server fields and captures the four CoffeeLink viewports', async ({ page }) => {
  await mockMemberServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await openRealMembers(page)
    await expect(page.getByText('owner@coffeelink.test', { exact: true })).toBeVisible()
    await expect(page.getByText('服务端列表回读', { exact: true })).toBeVisible()
    await expect(page.getByText('上海咖啡科技有限公司', { exact: true })).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-members-real-${viewport.width}.png` })
  }
})

test('invite uses csrf, idempotency and session context then confirms with server readback', async ({ page }) => {
  const server = await mockMemberServer(page)
  await openRealMembers(page)
  await page.getByRole('button', { name: '邀请成员', exact: true }).click()
  await page.getByLabel('成员邮箱').fill('invitee@coffeelink.test')
  await page.getByRole('button', { name: '确认操作', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('服务端确认')
  await expect(page.getByText('invitee@coffeelink.test', { exact: true })).toBeVisible()
  const headers = server.getLastWriteHeaders()
  expect(headers['x-csrf-token']).toBe('csrf-real-member')
  expect(headers['idempotency-key']).toMatch(/^enterprise-member-invite-/)
  expect(headers['x-biz-session-context']).toContain('tenant-001')
})

test('401 and 403 are surfaced and never replaced with preview members', async ({ page }) => {
  await mockMemberServer(page, { unauthenticated: true })
  await openRealMembers(page)
  await expect(page.getByRole('alert')).toContainText('登录会话已失效')
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockMemberServer(page, { listStatus: 403 })
  await page.reload()
  await expect(page.getByRole('alert')).toContainText('没有管理企业成员的权限')
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)
})

test('409 preserves the action and the same idempotency key for retry instead of reporting success', async ({ page }) => {
  const server = await mockMemberServer(page, { mutationStatus: 409 })
  await openRealMembers(page)
  await page.getByRole('button', { name: '停用', exact: true }).first().click()
  const dialog = page.getByRole('dialog', { name: '停用成员' })
  await expect(dialog).toContainText('owner@coffeelink.test')
  await dialog.getByRole('button', { name: '确认操作', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText('成员状态或请求版本已发生变化')
  const retry = dialog.getByRole('button', { name: '重试相同操作', exact: true })
  await expect(retry).toBeVisible()
  await retry.click()
  await expect(dialog.getByRole('alert')).toContainText('成员状态或请求版本已发生变化')
  const writes = server.getWriteHeaderHistory()
  expect(writes).toHaveLength(2)
  expect(writes[0]?.['idempotency-key']).toBe(writes[1]?.['idempotency-key'])
  await expect(page.getByRole('status')).toHaveCount(0)
})

test('a successful write without readback is not presented as confirmed success', async ({ page }) => {
  await mockMemberServer(page, { readbackStatus: 500 })
  await openRealMembers(page)
  await page.getByRole('button', { name: '停用', exact: true }).first().click()
  const dialog = page.getByRole('dialog', { name: '停用成员' })
  await dialog.getByRole('button', { name: '确认操作', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText('readback failed')
  await expect(page.getByText(/服务端确认/)).toHaveCount(0)
})
