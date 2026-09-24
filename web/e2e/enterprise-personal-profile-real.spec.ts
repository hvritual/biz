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
    await expect(page.locator('[data-ui-region="scope"] h2')).toHaveText('Tenant A')
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
  await expect(page.getByText('头像已保存，并与当前账号状态同步。', { exact: true })).toBeVisible()

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

// #182 extends the existing API-mode personal-center suite. These fixtures
// verify browser contracts, not supplier delivery or live backend authority.
type SecurityApiOptions = { requestError?: string; completeError?: string; failReadback?: boolean; delayed?: boolean }
async function mockPersonalSecurityApi(page: Page, options: SecurityApiOptions = {}) {
  const api = await mockPersonalProfileApi(page)
  const calls: Array<{ path: string; body: Record<string, unknown>; headers: Record<string, string> }> = []
  let completed = false, release: (() => void) | undefined
  const delayed = new Promise<void>((resolve) => { release = resolve })
  await page.route('**/api/auth/personal/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    const body = route.request().postDataJSON() as Record<string, unknown>
    const headers = route.request().headers()
    calls.push({ path, body, headers })
    if (path.endsWith('/request')) {
      if (options.delayed) await delayed
      if (options.requestError) return json(route, 409, { error: options.requestError })
      return json(route, 200, {
        challenge_id: 'challenge-182', flow_id: 'flow-182', masked_destination: body.channel === 'email' ? 'n***@example.invalid' : '***8899',
        expires_at: '2099-01-01T00:00:00Z', delivery_state: 'PENDING', version: api.profiles[api.activeTenant()]!.version, resend_after_seconds: 60,
      })
    }
    if (options.completeError) return json(route, 409, { error: options.completeError })
    if (body.otp_code !== '123456') return json(route, 400, { error: 'VERIFICATION_INVALID' })
    if (completed) return json(route, 409, { error: 'VERIFICATION_CONSUMED' })
    completed = true
    const current = api.profiles[api.activeTenant()]!
    if (path.includes('tenant-deletion')) {
      return json(route, 200, { tenant_id: current.tenantId, user_id: current.userId, version: current.version + 1,
        deleted_at: '2026-09-24T00:00:00Z', notification_event_id: 'notice-delete', notification_state: 'PENDING', reauthentication_required: true })
    }
    if (body.channel === 'email') current.email = 'n***@example.invalid'
    else current.phone = '***8899'
    current.version++
    return json(route, 200, { tenant_id: current.tenantId, user_id: current.userId, email: current.email, phone: current.phone,
      version: current.version, notification_event_id: 'notice-contact', notification_state: 'PENDING' })
  })
  await page.route('**/api/v1/tenant/me/profile', (route) => options.failReadback && completed
    ? json(route, 503, { error: 'unavailable' }) : json(route, 200, api.profiles[api.activeTenant()]))
  return { ...api, calls, release: () => release?.() }
}
async function beginContact(page: Page, channel: 'email' | 'sms' = 'email') {
  await page.getByRole('button', { name: channel === 'email' ? '绑定或更换邮箱' : '绑定或更换手机号', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '验证并更换联系方式' })
  await dialog.getByLabel(/新的联系方式/).fill(channel === 'email' ? 'new@example.invalid' : '+49170888899')
  await dialog.getByLabel('当前登录密码', { exact: true }).fill('CoffeePass9A')
  await dialog.getByRole('button', { name: '获取验证码', exact: true }).click()
  return dialog
}

for (const channel of ['email', 'sms'] as const) {
  test(`#182 ${channel} contact change confirms masked data and clears credentials`, async ({ page }) => {
    const api = await mockPersonalSecurityApi(page)
    await openPersonalProfile(page)
    const dialog = await beginContact(page, channel)
    await expect(dialog.getByLabel('当前登录密码', { exact: true })).toHaveValue('')
    await dialog.getByLabel('验证码', { exact: true }).fill('123456')
    const save = dialog.getByRole('button', { name: '验证并保存', exact: true })
    await save.focus(); await save.press('Enter')
    await expect(dialog.getByText('联系方式已更新，并与当前企业资料同步。', { exact: true })).toBeVisible()
    expect(api.calls).toHaveLength(2)
    const write = api.calls[1]!
    expect(write.body.version).toBe(7)
    expect(write.body).not.toHaveProperty('user_id'); expect(write.body).not.toHaveProperty('tenant_id')
    expect(write.headers['x-csrf-token']).toBe('csrf-personal-profile')
    expect(write.headers['x-biz-session-context']).toContain('tenant-a')
    expect(write.headers['idempotency-key']).toMatch(/^personal-security-complete-/)
    expect(await page.evaluate(() => JSON.stringify(localStorage) + JSON.stringify(sessionStorage))).not.toMatch(/CoffeePass9A|new@example.invalid|123456/)
    await expect(dialog.getByLabel('验证码', { exact: true })).toHaveCount(0)
    await dialog.getByRole('button', { name: '关闭', exact: true }).click()
    await expect(page.locator('.contact-card')).toContainText(channel === 'email' ? 'n***@example.invalid' : '***8899')
  })
}

test('#182 sixty-second resend window follows response and never reports pending notification delivered', async ({ page }) => {
  await page.clock.install()
  const api = await mockPersonalSecurityApi(page); await openPersonalProfile(page)
  const dialog = await beginContact(page)
  await expect(dialog.getByText('通知正在处理中，尚未确认送达。', { exact: false })).toBeVisible()
  await dialog.getByLabel('当前登录密码', { exact: true }).fill('CoffeePass9A')
  await expect(dialog.getByRole('button', { name: /秒后可重新发送/ })).toBeDisabled()
  await page.clock.fastForward(61000)
  await dialog.getByRole('button', { name: '重新发送验证码', exact: true }).click()
  await expect.poll(() => api.calls.length).toBe(2)
  expect(api.calls[0]!.headers['idempotency-key']).not.toBe(api.calls[1]!.headers['idempotency-key'])
})

for (const [code, text] of [
  ['CURRENT_PASSWORD_INVALID', '当前密码不正确，请重新输入。'],
  ['CONTACT_CONFLICT', '该联系方式已被当前企业的其他账号使用。'],
] as const) {
  test(`#182 request rejection ${code} preserves profile`, async ({ page }) => {
    const api = await mockPersonalSecurityApi(page, { requestError: code }); await openPersonalProfile(page)
    const dialog = await beginContact(page)
    await expect(dialog.getByRole('alert')).toHaveText(text)
    await expect(dialog.getByLabel('验证码', { exact: true })).toHaveCount(0)
    expect(api.profiles['tenant-a']!.version).toBe(7)
  })
}

test('#182 wrong OTP and Owner protection do not report success or leave the page', async ({ page }) => {
  const options: SecurityApiOptions = {}
  const api = await mockPersonalSecurityApi(page, options); await openPersonalProfile(page)
  let dialog = await beginContact(page)
  await dialog.getByLabel('验证码', { exact: true }).fill('000000')
  await dialog.getByRole('button', { name: '验证并保存', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText('验证码或验证信息不正确')
  expect(api.profiles['tenant-a']!.version).toBe(7)
  await dialog.getByRole('button', { name: '取消', exact: true }).click()
  options.requestError = 'LAST_OWNER_PROTECTED'
  await page.getByRole('button', { name: '注销本企业账号', exact: true }).click()
  dialog = page.getByRole('dialog', { name: '确认注销本企业账号' })
  await dialog.getByRole('button', { name: '获取验证码', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText('最后一位所有者')
  await expect(page).toHaveURL(/enterprise\/personal-profile/)
})

test('#182 accepted change with failed refresh recovers with GET only', async ({ page }) => {
  const options = { failReadback: true }
  const api = await mockPersonalSecurityApi(page, options); await openPersonalProfile(page)
  const dialog = await beginContact(page)
  await dialog.getByLabel('验证码', { exact: true }).fill('123456')
  await dialog.getByRole('button', { name: '验证并保存', exact: true }).click()
  await expect(dialog.getByRole('button', { name: '重新确认最新资料', exact: true })).toBeVisible()
  await expect(page.locator('.contact-card')).toContainText('a***@example.invalid')
  await expect(dialog.getByRole('button', { name: '验证并保存', exact: true })).toHaveCount(0)
  options.failReadback = false
  await dialog.getByRole('button', { name: '重新确认最新资料', exact: true }).click()
  await expect(dialog.getByText('联系方式已更新，并与当前企业资料同步。', { exact: true })).toBeVisible()
  expect(api.calls.filter((c) => c.path.endsWith('/complete'))).toHaveLength(1)
})

test('#182 deletion requires explicit affected-tenant confirmation and returns to login', async ({ page }) => {
  const api = await mockPersonalSecurityApi(page); await openPersonalProfile(page)
  await page.getByRole('button', { name: '注销本企业账号', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '确认注销本企业账号' })
  await expect(dialog).toContainText('Tenant A')
  await expect(dialog).toContainText('其他企业中的账号关系和全局登录凭据不受影响')
  await dialog.getByRole('button', { name: '获取验证码', exact: true }).click()
  await dialog.getByLabel('验证码', { exact: true }).fill('123456')
  await expect(dialog.getByRole('button', { name: '确认注销', exact: true })).toBeDisabled()
  await dialog.getByRole('checkbox').check()
  await dialog.getByRole('button', { name: '确认注销', exact: true }).click()
  await expect(page).toHaveURL(/\/api\/auth\/login\?return_to=/)
  expect(api.calls[1]!.body).toMatchObject({ confirm_tenant_id: 'tenant-a', confirm_irreversible: true, version: 7 })
  expect(api.calls[1]!.body).not.toHaveProperty('destination')
  expect(api.profiles['tenant-b']!.version).toBe(4)
})

test('#182 session invalidation discards late verification response and clears the form', async ({ page }) => {
  const api = await mockPersonalSecurityApi(page, { delayed: true }); await openPersonalProfile(page)
  await beginContact(page)
  await expect.poll(() => api.calls.length).toBe(1)
  await page.evaluate(() => window.dispatchEvent(new StorageEvent('storage', {
    key: '__coffeelink_session_context_signal_v1', newValue: JSON.stringify({ type: 'session-context-changed', contextVersion: 12, nonce: 'test-182-session-change' }),
  })))
  await expect(page.getByRole('dialog', { name: '验证并更换联系方式' })).toHaveCount(0)
  api.release()
  await selectUiOption(page.getByRole('combobox', { name: '切换企业' }), 'tenant-b')
  await expect(page.locator('[data-ui-region="scope"] h2')).toHaveText('Tenant B')
  await expect(page.getByRole('button', { name: '注销本企业账号', exact: true })).toBeDisabled()
  await expect(page.getByText('暂无已绑定的联系方式，请先绑定邮箱或手机号。', { exact: true })).toBeVisible()
})

test('#182 four viewport contact and deletion review with keyboard cancellation', async ({ page }) => {
  await mockPersonalSecurityApi(page)
  mkdirSync('screenshots/enterprise181-personal-profile', { recursive: true })
  for (const viewport of [{ width: 1366, height: 768 }, { width: 1440, height: 900 }, { width: 1536, height: 1024 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport); await openPersonalProfile(page)
    const opener = page.getByRole('button', { name: '绑定或更换邮箱', exact: true })
    await opener.click()
    await expect(page.getByRole('dialog', { name: '验证并更换联系方式' })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise181-personal-profile/enterprise182-contact-${viewport.width}.png`, animations: 'disabled' })
    await page.keyboard.press('Escape'); await expect(opener).toBeFocused()
    await page.getByRole('button', { name: '注销本企业账号', exact: true }).click()
    await expect(page.getByRole('dialog', { name: '确认注销本企业账号' })).toBeVisible()
    await page.screenshot({ path: `screenshots/enterprise181-personal-profile/enterprise182-deletion-${viewport.width}.png`, animations: 'disabled' })
    await page.keyboard.press('Escape')
  }
})
