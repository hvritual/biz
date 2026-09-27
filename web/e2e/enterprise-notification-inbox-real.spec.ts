import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { installApiFailFast, selectUiOption } from './ui.helpers'

test.skip(!process.env.ENTERPRISE_NOTIFICATION_INBOX_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type InboxMessage = {
  message_id: string
  type_code: string
  level: 'general' | 'important' | 'urgent'
  reference_kind: string
  reference_id: string
  created_at: string
}

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

function message(id: string, tenant: string, overrides: Partial<InboxMessage> = {}): InboxMessage {
  return {
    message_id: id.repeat(64),
    type_code: 'device.fault',
    level: 'urgent',
    reference_kind: 'device',
    reference_id: `${tenant}-machine`,
    created_at: '2026-09-27T01:00:00Z',
    ...overrides,
  }
}

function notificationPreferences(tenant: string) {
  return {
    tenant_id: tenant,
    user_id: 'user-shared',
    policy: 'optional-notifications-v1',
    sms: { channel: 'sms', state: 'default', allowed: true, version: 0, updated_at: null },
    email: { channel: 'email', state: 'default', allowed: true, version: 0, updated_at: null },
  }
}

async function useZhLocale(page: Page) {
  await page.addInitScript(() => localStorage.setItem('coffeelink.locale', 'zh-CN'))
}

async function mockInboxApi(page: Page, options: { delayFirstTenantAInbox?: boolean } = {}) {
  await installApiFailFast(page)

  let activeTenant = 'tenant-a'
  let tenantAUnread = [
    message('a', 'tenant-a', { reference_id: '<img src=x onerror="globalThis.__inbox_xss=1">' }),
    message('b', 'tenant-a', { type_code: 'device.offline', level: 'important', reference_id: 'tenant-a-machine-b' }),
  ]
  let tenantBUnread = [
    message('c', 'tenant-b', { type_code: 'system.announcement', level: 'general', reference_kind: 'site', reference_id: 'tenant-b-site' }),
  ]
  let delayed = false
  let releaseTenantA: (() => void) | null = null
  const tenantAGate = new Promise<void>((resolve) => { releaseTenantA = resolve })
  const writes: Array<{ tenant: string; headers: Record<string, string>; body: unknown }> = []

  const session = () => ({
    authenticated: true,
    actor_kind: 'user',
    user_id: 'user-shared',
    active_tenant_id: activeTenant,
    active_tenant_timezone: 'Asia/Shanghai',
    context_version: activeTenant === 'tenant-a' ? 11 : 12,
    csrf_token: 'csrf-notification-inbox',
    tenants: [
      { id: 'tenant-a', name: 'Tenant A', timezone: 'Asia/Shanghai' },
      { id: 'tenant-b', name: 'Tenant B', timezone: 'Asia/Shanghai' },
    ],
  })

  await page.route('**/api/auth/login**', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: '<!doctype html><title>Login</title>' }))
  await page.route('**/api/auth/session', (route) => json(route, 200, session()))
  await page.route('**/api/auth/session/tenant', (route) => {
    const body = route.request().postDataJSON() as { tenant_id?: string }
    if (body.tenant_id !== 'tenant-a' && body.tenant_id !== 'tenant-b') return json(route, 400, { error: 'UNKNOWN_TENANT' })
    activeTenant = body.tenant_id
    return json(route, 200, session())
  })
  await page.route('**/api/auth/authorization', (route) => json(route, 200, {
    authenticated: true,
    actor_kind: 'user',
    user_id: 'user-shared',
    tenant_id: activeTenant,
    tenant_name: activeTenant === 'tenant-a' ? 'Tenant A' : 'Tenant B',
    timezone: 'Asia/Shanghai',
    roles: ['member'],
    grants: [{ permission: 'tenant.personal_profile.self', role_id: `membership:${activeTenant}:user-shared`, role_name: '', scope: 'self' }],
    data_policies: [],
    site_ids: [],
    permission_version: `sha256:notification-inbox-${activeTenant}`,
    modules: [{ code: 'access-management', allowed: true, reason: 'allowed', actions: ['tenant.member.personal_profile.get'] }],
    actions: [{ code: 'tenant.member.personal_profile.get', permissions: ['tenant.personal_profile.self'], permission_mode: 'all' }],
    button_codes: ['tenant.member.personal_profile.get'],
  }))
  await page.route('**/api/v1/tenant/me/profile', (route) => json(route, 200, {
    userId: 'user-shared',
    username: 'operator',
    name: activeTenant === 'tenant-a' ? 'Alice A' : 'Alice B',
    tenantId: activeTenant,
    tenantName: activeTenant === 'tenant-a' ? 'Tenant A' : 'Tenant B',
    roles: [{ roleId: 'role-member', roleName: '成员', roleStatus: 'active' }],
    registeredAt: '2026-01-02T03:04:05Z',
    joinedAt: '2026-02-03T04:05:06Z',
    email: 'a***@example.invalid',
    phone: '***4567',
    avatarAssetRef: 'avatar:coffee-blue',
    version: 1,
    employeeId: 'EMP-1',
    position: '运营',
    departmentId: 'dept-1',
  }))
  await page.route('**/api/v1/tenant/me/avatar-options', (route) => json(route, 200, { options: [] }))
  await page.route('**/api/auth/personal/notification-preferences', (route) => {
    if (route.request().method() !== 'GET') return json(route, 405, { error: 'METHOD_NOT_ALLOWED' })
    return json(route, 200, notificationPreferences(activeTenant))
  })

  await page.route('**/api/auth/personal/in-app-notifications/read-all', async (route) => {
    if (route.request().method() !== 'POST') return json(route, 405, { error: 'METHOD_NOT_ALLOWED' })
    const requestTenant = activeTenant
    const current = requestTenant === 'tenant-a' ? tenantAUnread : tenantBUnread
    writes.push({ tenant: requestTenant, headers: route.request().headers(), body: route.request().postDataJSON() })
    const markedCount = current.length
    if (requestTenant === 'tenant-a') {
      tenantAUnread = [message('d', 'tenant-a', {
        type_code: 'device.connection_recovered',
        level: 'general',
        reference_id: 'arrived-during-mark-all',
        created_at: '2026-09-27T01:00:01Z',
      })]
    } else {
      tenantBUnread = []
    }
    return json(route, 200, {
      receipt_id: 'f'.repeat(64),
      tenant_id: requestTenant,
      user_id: 'user-shared',
      marked_count: markedCount,
      read_at: '2026-09-27T01:00:02Z',
    })
  })

  await page.route('**/api/auth/personal/in-app-notifications', async (route) => {
    if (route.request().method() !== 'GET') return json(route, 405, { error: 'METHOD_NOT_ALLOWED' })
    const requestTenant = activeTenant
    if (options.delayFirstTenantAInbox && requestTenant === 'tenant-a' && !delayed) {
      delayed = true
      await tenantAGate
    }
    const items = requestTenant === 'tenant-a' ? tenantAUnread : tenantBUnread
    return json(route, 200, {
      tenant_id: requestTenant,
      user_id: 'user-shared',
      unread_count: items.length,
      messages: items,
      as_of: '2026-09-27T01:00:03Z',
    })
  })

  return {
    writes,
    activeTenant: () => activeTenant,
    setTenantAUnread: (items: InboxMessage[]) => { tenantAUnread = items },
    releaseTenantA: () => releaseTenantA?.(),
  }
}

async function openPersonalProfile(page: Page) {
  await useZhLocale(page)
  await page.goto('/#/enterprise/personal-profile')
  await expect(page.locator('[data-enterprise-page="personal-profile"]')).toBeVisible()
}

test('#186 bell renders real current-owner unread messages across four viewports with an explicit empty-capable panel', async ({ page }) => {
  await mockInboxApi(page)
  mkdirSync('screenshots/enterprise186-notification-inbox', { recursive: true })

  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await openPersonalProfile(page)
    const bell = page.getByRole('button', { name: /站内消息，2 条未读/ })
    await expect(bell).toBeVisible()
    await bell.click()

    const inbox = page.locator('[data-notification-inbox]')
    await expect(inbox).toBeVisible()
    await expect(inbox.getByText('设备故障', { exact: true })).toBeVisible()
    await expect(inbox.getByText('设备通信离线', { exact: true })).toBeVisible()
    await expect(inbox.getByText('<img src=x onerror="globalThis.__inbox_xss=1">', { exact: true })).toBeVisible()
    await expect(page.locator('img[src="x"]')).toHaveCount(0)
    expect(await page.evaluate(() => (globalThis as typeof globalThis & { __inbox_xss?: number }).__inbox_xss)).toBeUndefined()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width)

    await page.screenshot({
      path: `screenshots/enterprise186-notification-inbox/inbox-${viewport.width}.png`,
      fullPage: false,
      animations: 'disabled',
    })
    await page.getByRole('button', { name: '关闭', exact: true }).click()
  }
})

test('#186 keyboard mark-all uses trusted metadata, re-reads authority and preserves messages arriving during the operation', async ({ page }) => {
  const api = await mockInboxApi(page)
  await openPersonalProfile(page)

  const bell = page.getByRole('button', { name: /站内消息，2 条未读/ })
  await bell.focus()
  await bell.press('Enter')
  const markAll = page.getByRole('button', { name: '全部已读', exact: true })
  await markAll.focus()
  await markAll.press('Enter')

  await expect(page.getByText('设备通信恢复', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: /站内消息，1 条未读/ })).toBeVisible()
  await expect(page.getByText(/期间有 1 条新消息到达/)).toBeVisible()

  expect(api.writes).toHaveLength(1)
  expect(api.writes[0]!.tenant).toBe('tenant-a')
  expect(api.writes[0]!.body).toEqual({})
  expect(JSON.stringify(api.writes[0]!.body)).not.toMatch(/tenant|user/)
  expect(api.writes[0]!.headers['x-csrf-token']).toBe('csrf-notification-inbox')
  expect(api.writes[0]!.headers['x-biz-session-context']).toContain('tenant-a')
  expect(api.writes[0]!.headers['idempotency-key']).toMatch(/^notification-read-all-/)
})

test('#186 tenant switch clears the old inbox immediately and ignores a delayed old-tenant response', async ({ page }) => {
  const api = await mockInboxApi(page, { delayFirstTenantAInbox: true })
  await openPersonalProfile(page)

  await selectUiOption(page.getByRole('combobox', { name: '切换企业' }), 'tenant-b')
  await expect(page.getByRole('button', { name: /站内消息，1 条未读/ })).toBeVisible()
  await page.getByRole('button', { name: /站内消息，1 条未读/ }).click()
  const inbox = page.locator('[data-notification-inbox]')
  await expect(inbox.getByText('企业系统通知', { exact: true })).toBeVisible()
  await expect(inbox).toContainText('tenant-b-site')

  api.releaseTenantA()
  await page.waitForTimeout(150)
  await expect(inbox).toContainText('tenant-b-site')
  await expect(inbox).not.toContainText('tenant-a-machine')
  expect(api.activeTenant()).toBe('tenant-b')
})
