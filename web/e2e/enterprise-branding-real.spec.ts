import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_BRANDING_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type Branding = {
  tenantId: string
  preset: 'blue' | 'emerald' | 'violet' | 'amber' | 'custom'
  primary: string
  version: number
  canManage: boolean
}
type Write = { tenantId: string; headers: Record<string, string>; body: Record<string, unknown> }
type Options = {
  readStatus?: number
  mutationStatus?: number
  readbackStatus?: number
  canManage?: boolean
}

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function useZhLocale(page: Page) {
  await page.addInitScript(() => localStorage.setItem('coffeelink.locale', 'zh-CN'))
}

async function mockBrandingServer(page: Page, options: Options = {}) {
  let activeTenant = 'tenant-a'
  const states: Record<string, Branding> = {
    'tenant-a': { tenantId: 'tenant-a', preset: 'violet', primary: '', version: 7, canManage: options.canManage ?? true },
    'tenant-b': { tenantId: 'tenant-b', preset: 'emerald', primary: '', version: 4, canManage: options.canManage ?? true },
  }
  const writes: Write[] = []

  await page.route('**/api/auth/session', (route) => json(route, 200, {
    authenticated: true,
    actor_kind: 'tenant',
    user_id: 'branding-user',
    active_tenant_id: activeTenant,
    csrf_token: 'csrf-branding',
    tenants: [
      { id: 'tenant-a', name: 'Tenant A' },
      { id: 'tenant-b', name: 'Tenant B' },
    ],
  }))

  await page.route('**/api/auth/session/tenant', async (route) => {
    const body = route.request().postDataJSON() as { tenant_id?: string }
    if (!body.tenant_id || !states[body.tenant_id]) return json(route, 400, { message: 'unknown tenant' })
    activeTenant = body.tenant_id
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'branding-user',
      active_tenant_id: activeTenant,
      csrf_token: 'csrf-branding',
      tenants: [
        { id: 'tenant-a', name: 'Tenant A' },
        { id: 'tenant-b', name: 'Tenant B' },
      ],
    })
  })

  await page.route('**/api/auth/authorization', async (route) => {
    const buttonCodes = options.canManage === false
      ? ['tenant.branding.get']
      : ['tenant.branding.get', 'tenant.branding.update']
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'branding-user',
      tenant_id: activeTenant,
      tenant_name: activeTenant === 'tenant-a' ? 'Tenant A' : 'Tenant B',
      timezone: 'Asia/Shanghai',
      roles: ['branding'],
      grants: [],
      data_policies: [],
      site_ids: [],
      permission_version: 'sha256:branding-e2e',
      modules: [{ code: 'access-management', allowed: true, reason: 'allowed', actions: buttonCodes }],
      actions: buttonCodes.map((code) => ({ code, permissions: [], permission_mode: 'all' })),
      button_codes: buttonCodes,
    })
  })

  await page.route('**/api/v1/tenant/branding', async (route) => {
    const request = route.request()
    if (request.method() === 'GET') {
      if (options.readStatus) return json(route, options.readStatus, { message: 'branding read denied' })
      if (options.readbackStatus && writes.length > 0) return json(route, options.readbackStatus, { message: 'branding readback failed' })
      return json(route, 200, states[activeTenant])
    }
    if (request.method() !== 'PATCH') return json(route, 405, { message: 'method not allowed' })
    const body = request.postDataJSON() as Record<string, unknown>
    writes.push({ tenantId: activeTenant, headers: request.headers(), body })
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'branding conflict' })
    states[activeTenant] = {
      ...states[activeTenant]!,
      preset: body.preset as Branding['preset'],
      primary: String(body.primary ?? ''),
      version: states[activeTenant]!.version + 1,
    }
    return json(route, 200, states[activeTenant])
  })

  return { writes, states, activeTenant: () => activeTenant }
}

async function openBranding(page: Page) {
  await useZhLocale(page)
  await page.goto('/#/enterprise/branding')
  await expect(page.getByRole('heading', { name: '品牌与主题', level: 1 })).toBeVisible()
  await expect(page.locator('[data-enterprise-page="branding"]')).toBeVisible()
}

async function chooseTenant(page: Page, name: 'Tenant A' | 'Tenant B') {
  await page.getByRole('combobox', { name: '切换企业' }).click()
  await page.getByRole('option', { name, exact: true }).click()
}

test('authoritative branding renders and captures all CoffeeLink viewports', async ({ page }) => {
  await mockBrandingServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await openBranding(page)
    await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'violet')
    await expect(page.getByText('服务端已确认', { exact: true })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-branding-real-${viewport.width}.png`, fullPage: false, animations: 'disabled' })
  }
})

test('custom preview saves with CAS trusted headers and survives reload only after readback', async ({ page }) => {
  const server = await mockBrandingServer(page)
  await openBranding(page)
  await page.getByRole('button', { name: '自定义', exact: true }).click()
  const textColor = page.locator('.custom-primary-field input:not([type="color"])')
  await textColor.fill('#125a75')
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'custom')
  expect(await page.evaluate(() => document.documentElement.style.getPropertyValue('--color-primary'))).toBe('#125a75')
  await page.getByRole('button', { name: '保存主题', exact: true }).click()
  await expect(page.getByText('企业品牌主题已由服务端确认并重新读取。', { exact: true })).toBeVisible()

  expect(server.writes).toHaveLength(1)
  const write = server.writes[0]!
  expect(write.tenantId).toBe('tenant-a')
  expect(write.headers['x-csrf-token']).toBe('csrf-branding')
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-tenant-branding-update/)
  expect(write.headers['x-biz-session-context']).toContain('tenant-a')
  expect(write.body).toEqual({ preset: 'custom', primary: '#125a75', version: 7 })
  expect(write.body.tenantId).toBeUndefined()

  await page.reload()
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'custom')
  expect(await page.evaluate(() => document.documentElement.style.getPropertyValue('--color-primary'))).toBe('#125a75')
})

test('cancel restores authoritative theme and tenant switching isolates A and B themes', async ({ page }) => {
  await mockBrandingServer(page)
  await openBranding(page)
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'violet')
  await page.getByRole('button', { name: 'Amber', exact: false }).click()
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'amber')
  await page.getByRole('button', { name: '取消预览', exact: true }).click()
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'violet')

  await chooseTenant(page, 'Tenant B')
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'emerald')
  await expect(page.locator('.branding-scope dd.mono').filter({ hasText: 'tenant-b' })).toBeVisible()
  await chooseTenant(page, 'Tenant A')
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'violet')
  await expect(page.locator('.branding-scope dd.mono').filter({ hasText: 'tenant-a' })).toBeVisible()
})

test('read-only member applies server theme without exposing write actions', async ({ page }) => {
  await mockBrandingServer(page, { canManage: false })
  await openBranding(page)
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'violet')
  await expect(page.getByRole('note')).toContainText('当前账号仅可查看并使用企业主题，不能修改品牌配置。')
  await expect(page.getByRole('button', { name: '保存主题', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '恢复默认 Blue', exact: true })).toHaveCount(0)
})

test('401 and 403 branding reads stay explicit and never create writable preview state', async ({ page }) => {
  await mockBrandingServer(page, { readStatus: 401 })
  await openBranding(page)
  await expect(page.getByRole('alert')).toContainText('登录会话已失效')
  await expect(page.getByRole('button', { name: '保存主题', exact: true })).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockBrandingServer(page, { readStatus: 403 })
  await page.reload()
  await expect(page.getByRole('alert')).toContainText('当前账号没有维护企业品牌主题的权限')
  await expect(page.getByRole('button', { name: '保存主题', exact: true })).toHaveCount(0)
})

test('409 preserves draft and retries the same idempotency key', async ({ page }) => {
  const server = await mockBrandingServer(page, { mutationStatus: 409 })
  await openBranding(page)
  await page.getByRole('button', { name: 'Amber', exact: false }).click()
  const save = page.getByRole('button', { name: '保存主题', exact: true })
  await save.click()
  await expect(page.getByRole('alert')).toContainText('企业品牌主题已被其他操作修改')
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'amber')
  await save.click()
  await expect.poll(() => server.writes.length).toBe(2)
  expect(server.writes[0]?.headers['idempotency-key']).toBe(server.writes[1]?.headers['idempotency-key'])
  await expect(page.getByText('企业品牌主题已由服务端确认并重新读取。', { exact: true })).toHaveCount(0)
})

test('successful write without branding readback is never presented as confirmed success', async ({ page }) => {
  await mockBrandingServer(page, { readbackStatus: 500 })
  await openBranding(page)
  await page.getByRole('button', { name: 'Amber', exact: false }).click()
  await page.getByRole('button', { name: '保存主题', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('branding readback failed')
  await expect(page.getByText('企业品牌主题已由服务端确认并重新读取。', { exact: true })).toHaveCount(0)
})
