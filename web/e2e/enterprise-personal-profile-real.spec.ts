import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { installApiFailFast, selectUiOption } from './ui.helpers'

test.skip(!process.env.ENTERPRISE_PERSONAL_PROFILE_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type Profile = {
  userId: string
  username: string
  name: string
  tenantId: string
  tenantName: string
  roles: Array<{ roleId: string; roleName: string; roleStatus: string }>
  registeredAt: string
  joinedAt: string
  email: string
  phone: string
  avatarAssetRef: string
  version: number
  employeeId: string
  position: string
  departmentId: string
}

type Write = {
  tenantId: string
  headers: Record<string, string>
  body: Record<string, unknown>
}

type Options = {
  unauthenticated?: boolean
  authorizationDenied?: boolean
  readStatus?: number
  mutationStatus?: number
  confirmationStatus?: number
  delayTenantAProfile?: boolean
}

const avatarOptions = [
  { assetRef: 'avatar:coffee-blue', name: 'Coffee Blue', tone: 'blue' },
  { assetRef: 'avatar:coffee-violet', name: 'Coffee Violet', tone: 'violet' },
  { assetRef: 'avatar:coffee-emerald', name: 'Coffee Emerald', tone: 'emerald' },
  { assetRef: 'avatar:coffee-amber', name: 'Coffee Amber', tone: 'amber' },
]

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function useZhLocale(page: Page) {
  await page.addInitScript(() => localStorage.setItem('coffeelink.locale', 'zh-CN'))
}

async function mockPersonalProfileApi(page: Page, options: Options = {}) {
  await installApiFailFast(page)

  let activeTenant = 'tenant-a'
  let delayedTenantA = false
  let releaseTenantA: (() => void) | null = null
  const tenantAGate = new Promise<void>((resolve) => {
    releaseTenantA = resolve
  })

  const profiles: Record<string, Profile> = {
    'tenant-a': {
      userId: 'user-shared',
      username: 'alice.coffee',
      name: 'Alice A',
      tenantId: 'tenant-a',
      tenantName: 'Tenant A',
      roles: [{ roleId: 'role-a', roleName: '运营负责人', roleStatus: 'active' }],
      registeredAt: '2026-01-02T03:04:05Z',
      joinedAt: '2026-02-03T04:05:06Z',
      email: 'a***@example.invalid',
      phone: '***4567',
      avatarAssetRef: 'avatar:coffee-violet',
      version: 7,
      employeeId: 'EMP-A',
      position: '运营负责人',
      departmentId: 'dept-a',
    },
    'tenant-b': {
      userId: 'user-shared',
      username: 'alice.coffee',
      name: 'Alice B',
      tenantId: 'tenant-b',
      tenantName: 'Tenant B',
      roles: [{ roleId: 'role-b', roleName: '门店运营', roleStatus: 'active' }],
      registeredAt: '2026-01-02T03:04:05Z',
      joinedAt: '2026-03-04T05:06:07Z',
      email: '',
      phone: '',
      avatarAssetRef: 'avatar:coffee-emerald',
      version: 4,
      employeeId: 'EMP-B',
      position: '门店运营',
      departmentId: 'dept-b',
    },
  }
  const writes: Write[] = []

  await page.route('**/api/auth/login**', async (route) => {
    await route.fulfill({ status: 200, contentType: 'text/html', body: '<!doctype html><title>Login</title>' })
  })

  await page.route('**/api/auth/session', (route) => {
    if (options.unauthenticated) return json(route, 401, { message: 'unauthenticated' })
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-shared',
      active_tenant_id: activeTenant,
      active_tenant_timezone: 'Asia/Shanghai',
      context_version: activeTenant === 'tenant-a' ? 11 : 12,
      csrf_token: 'csrf-personal-profile',
      tenants: [
        { id: 'tenant-a', name: 'Tenant A', timezone: 'Asia/Shanghai' },
        { id: 'tenant-b', name: 'Tenant B', timezone: 'Asia/Shanghai' },
      ],
    })
  })

  await page.route('**/api/auth/session/tenant', async (route) => {
    const body = route.request().postDataJSON() as { tenant_id?: string }
    if (!body.tenant_id || !profiles[body.tenant_id]) return json(route, 400, { message: 'unknown tenant' })
    activeTenant = body.tenant_id
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-shared',
      active_tenant_id: activeTenant,
      active_tenant_timezone: 'Asia/Shanghai',
      context_version: activeTenant === 'tenant-a' ? 11 : 12,
      csrf_token: 'csrf-personal-profile',
      tenants: [
        { id: 'tenant-a', name: 'Tenant A', timezone: 'Asia/Shanghai' },
        { id: 'tenant-b', name: 'Tenant B', timezone: 'Asia/Shanghai' },
      ],
    })
  })

  await page.route('**/api/auth/authorization', (route) => {
    if (options.unauthenticated) return json(route, 401, { message: 'unauthenticated' })
    const buttonCodes = options.authorizationDenied
      ? []
      : [
          'tenant.member.personal_profile.get',
          'tenant.member.personal_profile.avatar_options',
          'tenant.member.personal_profile.avatar_update',
        ]
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-shared',
      tenant_id: activeTenant,
      tenant_name: profiles[activeTenant]!.tenantName,
      timezone: 'Asia/Shanghai',
      roles: ['member'],
      grants: options.authorizationDenied
        ? []
        : [{ permission: 'tenant.personal_profile.self', role_id: `membership:${activeTenant}:user-shared`, role_name: '', scope: 'self' }],
      data_policies: [],
      site_ids: [],
      permission_version: `sha256:personal-profile-${activeTenant}`,
      modules: [{ code: 'access-management', allowed: !options.authorizationDenied, reason: options.authorizationDenied ? 'denied' : 'allowed', actions: buttonCodes }],
      actions: buttonCodes.map((code) => ({
        code,
        permissions: ['tenant.personal_profile.self'],
        permission_mode: 'all',
      })),
      button_codes: buttonCodes,
    })
  })

  await page.route('**/api/v1/tenant/me/profile', async (route) => {
    const requestTenant = activeTenant
    if (options.delayTenantAProfile && requestTenant === 'tenant-a' && !delayedTenantA) {
      delayedTenantA = true
      await tenantAGate
    }
    if (options.readStatus) return json(route, options.readStatus, { message: 'personal profile read failed' })
    if (options.confirmationStatus && writes.length > 0) return json(route, options.confirmationStatus, { message: 'personal profile confirmation failed' })
    return json(route, 200, profiles[requestTenant])
  })

  await page.route('**/api/v1/tenant/me/avatar-options', (route) => {
    return json(route, 200, { options: avatarOptions })
  })

  await page.route('**/api/v1/tenant/me/avatar', async (route) => {
    if (route.request().method() !== 'PATCH') return json(route, 405, { message: 'method not allowed' })
    const requestTenant = activeTenant
    const body = route.request().postDataJSON() as Record<string, unknown>
    writes.push({ tenantId: requestTenant, headers: route.request().headers(), body })
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'personal profile conflict' })
    profiles[requestTenant] = {
      ...profiles[requestTenant]!,
      avatarAssetRef: String(body.avatarAssetRef ?? ''),
      version: profiles[requestTenant]!.version + 1,
    }
    return json(route, 200, profiles[requestTenant])
  })

  return {
    profiles,
    writes,
    activeTenant: () => activeTenant,
    releaseTenantA: () => releaseTenantA?.(),
  }
}

async function openPersonalProfile(page: Page) {
  await useZhLocale(page)
  await page.goto('/#/enterprise/personal-profile')
  await expect(page.locator('[data-enterprise-page="personal-profile"]')).toBeVisible()
}

test('personal profile renders current self data across four CoffeeLink viewports', async ({ page }) => {
  await mockPersonalProfileApi(page)
  mkdirSync('screenshots/enterprise181-personal-profile', { recursive: true })

  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await openPersonalProfile(page)
    await expect(page.getByRole('heading', { name: '个人中心', level: 1 })).toBeVisible()
    await expect(page.getByText('Alice A', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('Tenant A', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('a***@example.invalid', { exact: true })).toBeVisible()
    await expect(page.getByText('***4567', { exact: true })).toBeVisible()
    await expect(page.getByText(/alice\.a-|\+49170/)).toHaveCount(0)
    await expect(page.locator('[aria-label="当前账号"] [data-avatar-ref="avatar:coffee-violet"]')).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width)
    await page.screenshot({
      path: `screenshots/enterprise181-personal-profile/profile-${viewport.width}.png`,
      fullPage: false,
      animations: 'disabled',
    })
  }
})

test('keyboard avatar selection uses trusted CAS write and updates header only after refresh', async ({ page }) => {
  const api = await mockPersonalProfileApi(page)
  await openPersonalProfile(page)

  const amber = page.getByRole('button', { name: '选择头像 Coffee Amber' })
  await amber.focus()
  await amber.press('Space')
  await expect(amber).toHaveAttribute('aria-pressed', 'true')

  const save = page.getByRole('button', { name: '保存头像', exact: true })
  await save.focus()
  await save.press('Enter')
  await expect(page.getByText('头像已保存并完成服务端回读。', { exact: true })).toBeVisible()

  expect(api.writes).toHaveLength(1)
  const write = api.writes[0]!
  expect(write.tenantId).toBe('tenant-a')
  expect(write.body).toEqual({ version: 7, avatarAssetRef: 'avatar:coffee-amber' })
  expect(write.body.userId).toBeUndefined()
  expect(write.body.tenantId).toBeUndefined()
  expect(write.headers['x-csrf-token']).toBe('csrf-personal-profile')
  expect(write.headers['x-biz-session-context']).toContain('tenant-a')
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-personal-profile-avatar-tenant-a-/)
  await expect(page.locator('[aria-label="当前账号"] [data-avatar-ref="avatar:coffee-amber"]')).toBeVisible()
})

test('successful avatar PATCH without follow-up profile refresh never updates the header', async ({ page }) => {
  await mockPersonalProfileApi(page, { confirmationStatus: 500 })
  await openPersonalProfile(page)

  await page.getByRole('button', { name: '选择头像 Coffee Amber' }).click()
  await page.getByRole('button', { name: '保存头像', exact: true }).click()

  await expect(page.getByRole('alert').first()).toBeVisible()
  await expect(page.getByText('头像已保存并完成服务端回读。', { exact: true })).toHaveCount(0)
  await expect(page.locator('[aria-label="当前账号"] [data-avatar-ref="avatar:coffee-violet"]')).toBeVisible()
  await expect(page.locator('[aria-label="当前账号"] [data-avatar-ref="avatar:coffee-amber"]')).toHaveCount(0)
})

test('tenant switch discards delayed tenant A profile and renders tenant B empty contacts', async ({ page }) => {
  const api = await mockPersonalProfileApi(page, { delayTenantAProfile: true })
  await openPersonalProfile(page)

  await selectUiOption(page.getByRole('combobox', { name: '切换企业' }), 'tenant-b')
  await expect(page.getByText('Alice B', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('Tenant B', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('未绑定', { exact: true })).toHaveCount(2)
  await expect(page.locator('[aria-label="当前账号"] [data-avatar-ref="avatar:coffee-emerald"]')).toBeVisible()

  api.releaseTenantA()
  await page.waitForTimeout(150)

  await expect(page.getByText('Alice B', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('Alice A', { exact: true })).toHaveCount(0)
  await expect(page.locator('[aria-label="当前账号"] [data-avatar-ref="avatar:coffee-emerald"]')).toBeVisible()
  expect(api.activeTenant()).toBe('tenant-b')
})

test('unauthenticated and unauthorized personal profile routes fail closed', async ({ page }) => {
  await mockPersonalProfileApi(page, { unauthenticated: true })
  await page.goto('/#/enterprise/personal-profile')
  await expect(page).toHaveURL(/\/api\/auth\/login\?return_to=/)
  await expect(page.locator('[data-enterprise-page="personal-profile"]')).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockPersonalProfileApi(page, { authorizationDenied: true })
  await page.goto('/#/enterprise/personal-profile')
  await expect(page).toHaveURL(/#\/authorization-state/)
  await expect(page.getByRole('heading', { name: '没有访问权限' })).toBeVisible()
  await expect(page.locator('[data-enterprise-page="personal-profile"]')).toHaveCount(0)
})
