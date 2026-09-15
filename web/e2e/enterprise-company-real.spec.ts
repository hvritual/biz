import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_COMPANY_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type Profile = {
  tenantId: string
  name: string
  shortName: string
  industry: string
  companySize: string
  timezone: string
  contactName: string
  phone: string
  email: string
  address: string
  description: string
  logoAssetRef: string
  version: number
}
type Write = { path: string; method: string; headers: Record<string, string>; body: Record<string, unknown> }
type Options = {
  unauthenticated?: boolean
  readStatus?: number
  mutationStatus?: number
  readbackStatus?: number
}

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function mockTenantProfileServer(page: Page, options: Options = {}) {
  let profile: Profile = {
    tenantId: 'tenant-001',
    name: 'CoffeeLink 租赁运营有限公司',
    shortName: 'CoffeeLink',
    industry: '咖啡设备租赁与运营',
    companySize: '50–200 人',
    timezone: 'Asia/Shanghai',
    contactName: 'Alice Chen',
    phone: '021-60000001',
    email: 'owner@coffeelink.test',
    address: '上海市',
    description: '服务端权威企业资料',
    logoAssetRef: 'asset://tenant/logo/default',
    version: 7,
  }
  const writes: Write[] = []

  await page.route('**/api/auth/session', async (route) => {
    if (options.unauthenticated) return json(route, 401, { message: 'unauthenticated' })
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-001',
      active_tenant_id: 'tenant-001',
      csrf_token: 'csrf-real-company',
      tenants: [{ id: 'tenant-001', name: 'CoffeeLink 测试租户' }],
    })
  })

  await page.route('**/api/v1/tenant/profile', async (route) => {
    const request = route.request()
    if (request.method() === 'GET') {
      if (options.readStatus) return json(route, options.readStatus, { message: 'tenant profile denied' })
      if (options.readbackStatus && writes.length > 0) return json(route, options.readbackStatus, { message: 'tenant profile readback failed' })
      return json(route, 200, profile)
    }
    if (request.method() !== 'PATCH') return json(route, 405, { message: 'method not allowed' })
    const body = request.postDataJSON() as Record<string, unknown>
    writes.push({ path: new URL(request.url()).pathname, method: request.method(), headers: request.headers(), body })
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'tenant profile conflict' })
    profile = {
      ...profile,
      ...body,
      tenantId: profile.tenantId,
      version: profile.version + 1,
    } as Profile
    return json(route, 200, profile)
  })

  return { getWrites: () => writes, getProfile: () => profile }
}

async function openRealCompany(page: Page) {
  await page.goto('/#/enterprise/company')
  await expect(page.locator('[data-enterprise-page="company"]')).toBeVisible()
  await expect(page.locator('[data-enterprise-source="api"]')).toBeVisible()
}

test('real company page renders only authoritative Tenant Profile data across CoffeeLink viewports', async ({ page }) => {
  await mockTenantProfileServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [{ width: 1366, height: 768 }, { width: 1440, height: 900 }, { width: 1536, height: 1024 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport)
    await openRealCompany(page)
    await expect(page.getByLabel('企业名称')).toHaveValue('CoffeeLink 租赁运营有限公司')
    await expect(page.getByText('Tenant Profile API')).toBeVisible()
    await expect(page.getByText('上海云迹科技有限公司', { exact: true })).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-company-real-${viewport.width}.png`, fullPage: false })
  }
})

test('company update sends trusted-session headers and idempotency then confirms only after readback', async ({ page }) => {
  const server = await mockTenantProfileServer(page)
  await openRealCompany(page)
  await page.getByLabel('企业简称').fill('CoffeeLink Pro')
  await page.getByLabel('企业邮箱').fill('success@coffeelink.test')
  await page.getByRole('button', { name: '保存并回读确认' }).click()
  await expect(page.getByText('企业资料已由服务端确认 · v8', { exact: true })).toBeVisible()
  await expect(page.getByText('v8', { exact: true })).toBeVisible()

  const write = server.getWrites()[0]!
  expect(write.path).toBe('/api/v1/tenant/profile')
  expect(write.method).toBe('PATCH')
  expect(write.headers['x-csrf-token']).toBe('csrf-real-company')
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-tenant-profile-update-tenant-001-/)
  expect(write.headers['x-biz-session-context']).toContain('tenant-001')
  expect(write.body.tenantId).toBeUndefined()
  expect(write.body.shortName).toBe('CoffeeLink Pro')
  expect(write.body.version).toBe(7)
})

test('company 409 preserves draft and reuses the same idempotency key', async ({ page }) => {
  const server = await mockTenantProfileServer(page, { mutationStatus: 409 })
  await openRealCompany(page)
  await page.getByLabel('企业简称').fill('冲突中的草稿')
  const save = page.getByRole('button', { name: '保存并回读确认' })
  await save.click()
  await expect(page.getByRole('alert')).toContainText('企业资料已被其他操作修改')
  await expect(page.getByLabel('企业简称')).toHaveValue('冲突中的草稿')
  await save.click()
  await expect(page.getByRole('alert')).toContainText('企业资料已被其他操作修改')
  const writes = server.getWrites()
  expect(writes).toHaveLength(2)
  expect(writes[0]?.headers['idempotency-key']).toBe(writes[1]?.headers['idempotency-key'])
  await expect(page.getByRole('status')).toHaveCount(0)
})

test('successful PATCH without GET readback is not presented as confirmed success', async ({ page }) => {
  await mockTenantProfileServer(page, { readbackStatus: 500 })
  await openRealCompany(page)
  await page.getByLabel('企业简称').fill('未确认资料')
  await page.getByRole('button', { name: '保存并回读确认' }).click()
  await expect(page.getByRole('alert')).toContainText('tenant profile readback failed')
  await expect(page.getByRole('status')).toHaveCount(0)
})

test('401 and 403 never fall back to demo company data', async ({ page }) => {
  await mockTenantProfileServer(page, { unauthenticated: true })
  await openRealCompany(page)
  await expect(page.getByRole('alert')).toContainText('登录会话已失效')
  await expect(page.getByText('上海云迹科技有限公司', { exact: true })).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockTenantProfileServer(page, { readStatus: 403 })
  await page.reload()
  await expect(page.getByRole('alert')).toContainText('没有维护企业资料的权限')
  await expect(page.getByText('上海云迹科技有限公司', { exact: true })).toHaveCount(0)
})

test('API mode has no browser DataURL logo upload success path', async ({ page }) => {
  await mockTenantProfileServer(page)
  await openRealCompany(page)
  await expect(page.locator('input[type="file"]')).toHaveCount(0)
  await page.getByRole('button', { name: '资产服务上传' }).click()
  await expect(page.getByText(/API 模式禁止 DataURL Logo/)).toBeVisible()
})
