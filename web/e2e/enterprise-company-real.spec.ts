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

  await page.route(/\/(?:api\/)?(?:auth|v1)\//, async (route) => {
    const request = route.request()
    throw new Error(`Unhandled API request: ${request.method()} ${new URL(request.url()).pathname}`)
  })

  await page.route(/\/(?:api\/)?auth\/login(?:\?.*)?$/, async (route) => {
    await route.fulfill({ status: 200, contentType: 'text/html', body: '<!doctype html><title>Login</title>' })
  })

  await page.route(/\/(?:api\/)?auth\/session(?:\?.*)?$/, async (route) => {
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
  await page.route(/\/(?:api\/)?auth\/authorization(?:\?.*)?$/, async (route) => {
    const buttonCodes = ["tenant.profile.get","tenant.profile.update"]
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-001',
      tenant_id: 'tenant-001',
      tenant_name: 'CoffeeLink 测试租户',
      timezone: 'Asia/Shanghai',
      roles: ['operator'],
      grants: [],
      data_policies: [],
      site_ids: [],
      permission_version: 'sha256:e2e',
      modules: [{ code: 'access-management', allowed: true, reason: 'allowed', actions: buttonCodes }],
      actions: buttonCodes.map((code) => ({ code, permissions: [], permission_mode: 'all' })),
      button_codes: buttonCodes,
    })
  })

  await page.route(/\/(?:api\/)?v1\/tenant\/profile(?:\?.*)?$/, async (route) => {
    const request = route.request()
    if (request.method() === 'GET') {
      if (options.readStatus) return json(route, options.readStatus, { message: 'tenant profile denied' })
      if (options.readbackStatus && writes.length > 0) return json(route, options.readbackStatus, { message: 'tenant profile readback failed' })
      return json(route, 200, profile)
    }
    if (request.method() !== 'PATCH') return json(route, 405, { message: 'method not allowed' })
    const body = request.postDataJSON() as Record<string, unknown>
    writes.push({ path: new URL(request.url()).pathname.replace(/^\/api(?=\/)/, ''), method: request.method(), headers: request.headers(), body })
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
}

test('canonical company page renders authoritative profile across CoffeeLink viewports', async ({ page }) => {
  await mockTenantProfileServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await openRealCompany(page)
    await expect(page.getByLabel('企业名称')).toHaveValue('CoffeeLink 租赁运营有限公司')
    await expect(page.getByLabel('企业简称')).toHaveValue('CoffeeLink')
    await expect(page.getByText('上海云迹科技有限公司', { exact: true })).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-company-real-${viewport.width}.png`, fullPage: false })
  }
})

test('canonical company save carries trusted headers and confirms only after readback', async ({ page }) => {
  const server = await mockTenantProfileServer(page)
  await openRealCompany(page)
  await page.getByLabel('企业简称').fill('CoffeeLink Pro')
  await page.getByLabel('企业邮箱').fill('success@coffeelink.test')
  await page.getByRole('button', { name: '保存修改', exact: true }).click()
  await expect(page.getByText('企业资料已保存。', { exact: true })).toBeVisible()
  await expect(page.getByLabel('企业简称')).toHaveValue('CoffeeLink Pro')

  const write = server.getWrites()[0]!
  expect(write.path).toBe('/v1/tenant/profile')
  expect(write.method).toBe('PATCH')
  expect(write.headers['x-csrf-token']).toBe('csrf-real-company')
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-tenant-profile-update-tenant-001-/)
  expect(write.headers['x-biz-session-context']).toContain('tenant-001')
  expect(write.body.tenantId).toBeUndefined()
  expect(write.body.shortName).toBe('CoffeeLink Pro')
  expect(write.body.version).toBe(7)
})

test('company 409 preserves the canonical draft and reuses one idempotency key', async ({ page }) => {
  const server = await mockTenantProfileServer(page, { mutationStatus: 409 })
  await openRealCompany(page)
  await page.getByLabel('企业简称').fill('冲突中的草稿')
  const save = page.getByRole('button', { name: '保存修改', exact: true })
  await save.click()
  await expect(page.getByRole('alert')).toContainText('企业资料已被其他操作修改')
  await expect(page.getByLabel('企业简称')).toHaveValue('冲突中的草稿')
  await save.click()
  await expect(page.getByRole('alert')).toContainText('企业资料已被其他操作修改')
  const writes = server.getWrites()
  expect(writes).toHaveLength(2)
  expect(writes[0]?.headers['idempotency-key']).toBe(writes[1]?.headers['idempotency-key'])
  await expect(page.getByText('企业资料已保存。', { exact: true })).toHaveCount(0)
})

test('successful PATCH without GET readback is not presented as canonical success', async ({ page }) => {
  await mockTenantProfileServer(page, { readbackStatus: 500 })
  await openRealCompany(page)
  await page.getByLabel('企业简称').fill('未确认资料')
  await page.getByRole('button', { name: '保存修改', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('tenant profile readback failed')
  await expect(page.getByText('企业资料已保存。', { exact: true })).toHaveCount(0)
})

test('401 exits to trusted login while downstream 403 never falls back to demo company data', async ({ page }) => {
  await mockTenantProfileServer(page, { unauthenticated: true })
  await page.goto('/#/enterprise/company')
  await expect(page).toHaveURL(/\/api\/auth\/login\?return_to=/)
  await expect(page.locator('[data-enterprise-page="company"]')).toHaveCount(0)
  await expect(page.getByText('上海云迹科技有限公司', { exact: true })).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockTenantProfileServer(page, { readStatus: 403 })
  await page.goto('/#/enterprise/company')
  await expect(page.getByRole('alert')).toContainText('当前账号没有维护企业资料的权限')
  await expect(page.getByText('上海云迹科技有限公司', { exact: true })).toHaveCount(0)
})

test('API company page exposes server asset reference instead of browser DataURL upload', async ({ page }) => {
  await mockTenantProfileServer(page)
  await openRealCompany(page)
  await expect(page.locator('input[type="file"]')).toHaveCount(0)
  const logoState = page.getByLabel('企业 Logo 状态')
  await expect(logoState).toBeVisible()
  await expect(logoState).toContainText('已配置')
  await expect(page.getByText(/DataURL|API 模式|资产服务/)).toHaveCount(0)
})
