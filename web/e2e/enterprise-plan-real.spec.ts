import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_PLAN_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type Options = {
  unauthenticated?: boolean
  subscriptionStatus?: number
  entitlementStatus?: number
}

type Captured = {
  subscriptionPaths: string[]
  entitlementBodies: Array<Record<string, unknown>>
  entitlementHeaders: Array<Record<string, string>>
}

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function mockPlanServer(page: Page, options: Options = {}): Promise<Captured> {
  const captured: Captured = { subscriptionPaths: [], entitlementBodies: [], entitlementHeaders: [] }
  await page.route('**/api/auth/session', async (route) => {
    if (options.unauthenticated) return json(route, 401, { message: 'unauthenticated' })
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-001',
      active_tenant_id: 'tenant-001',
      csrf_token: 'csrf-plan-read',
      tenants: [{ id: 'tenant-001', name: 'CoffeeLink 测试租户' }],
    })
  })

  await page.route('**/api/v1/tenant/subscription', async (route) => {
    captured.subscriptionPaths.push(new URL(route.request().url()).pathname)
    if (options.subscriptionStatus) return json(route, options.subscriptionStatus, { message: 'subscription denied' })
    return json(route, 200, {
      subscriptionId: 'sub-authoritative-001',
      tenantId: 'tenant-001',
      kind: 'base',
      state: 'active',
      planCode: 'rental-growth-2026',
      planVersion: 3,
      salesScope: 'rental',
      entitlementSourceVersion: 11,
      createdAt: '2026-09-01T00:00:00Z',
      revision: 8,
      periodStart: '2026-09-01T00:00:00Z',
      periodEnd: '2027-09-01T00:00:00Z',
      renewalStopped: false,
      pendingChangeId: '',
    })
  })

  await page.route('**/api/v1/tenant/entitlements', async (route) => {
    const request = route.request()
    captured.entitlementBodies.push(request.postDataJSON() as Record<string, unknown>)
    captured.entitlementHeaders.push(request.headers())
    if (options.entitlementStatus) return json(route, options.entitlementStatus, { message: 'entitlements denied' })
    return json(route, 200, {
      tenantId: 'tenant-001',
      sourceVersion: 11,
      resolverVersion: 2,
      entitlementVersion: 19,
      catalogRevision: 7,
      permissionVersion: 'permission-fingerprint-001',
      permissionSubject: 'user-001',
      evaluatedAt: '2026-09-15T03:00:00Z',
      decisions: [
        { kind: 'module', moduleCode: 'tenant-access', key: 'tenant-access', allowed: true, reason: 'plan grant' },
        { kind: 'module', moduleCode: 'device-ops', key: 'device-ops', allowed: true, reason: 'plan grant' },
        { kind: 'module', moduleCode: 'advanced-reporting', key: 'advanced-reporting', allowed: false, reason: 'not in plan' },
        { kind: 'capability', moduleCode: 'tenant-access', key: 'member.manage', allowed: true, reason: 'plan grant' },
        { kind: 'quota', moduleCode: 'tenant-access', key: 'members', allowed: true, limit: { unlimited: false, value: 37 }, reason: 'plan limit' },
        { kind: 'quota', moduleCode: 'device-ops', key: 'devices', allowed: true, limit: { unlimited: true, value: 0 }, reason: 'plan limit' },
      ],
    })
  })
  return captured
}

async function openRealPlan(page: Page) {
  await page.goto('/#/enterprise/plan')
  await expect(page.locator('[data-enterprise-plan-source="server"]')).toBeVisible()
}

test('real plan page renders only authoritative subscription and entitlement facts across CoffeeLink viewports', async ({ page }) => {
  await mockPlanServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [{ width: 1366, height: 768 }, { width: 1440, height: 900 }, { width: 1536, height: 1024 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport)
    await openRealPlan(page)
    await expect(page.getByText('rental-growth-2026', { exact: true })).toBeVisible()
    await expect(page.getByText('Tenant Commercial API')).toBeVisible()
    await expect(page.getByText('标准版', { exact: true })).toHaveCount(0)
    await expect(page.getByText('500 GB', { exact: true })).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-plan-real-${viewport.width}.png`, fullPage: false })
  }
})

test('tenant plan read never sends an arbitrary tenant id and binds entitlement read to trusted session', async ({ page }) => {
  const captured = await mockPlanServer(page)
  await openRealPlan(page)
  expect(captured.subscriptionPaths).toContain('/api/v1/tenant/subscription')
  expect(captured.subscriptionPaths.some((path) => path.includes('tenant-001'))).toBe(false)
  expect(captured.entitlementBodies).toHaveLength(1)
  expect(captured.entitlementBodies[0]?.tenantId).toBeUndefined()
  expect(captured.entitlementBodies[0]?.capabilityCodes).toEqual([])
  expect(captured.entitlementHeaders[0]?.['x-biz-session-context']).toContain('tenant-001')
  expect(captured.entitlementHeaders[0]?.['x-csrf-token']).toBe('csrf-plan-read')
})

test('quota limits are authoritative while unknown usage is never rendered as zero or a percentage', async ({ page }) => {
  await mockPlanServer(page)
  await openRealPlan(page)
  await page.getByRole('button', { name: '额度上限' }).click()
  await expect(page.getByRole('cell', { name: '37', exact: true })).toBeVisible()
  await expect(page.getByRole('cell', { name: '无限', exact: true })).toBeVisible()
  await expect(page.getByText('权威用量未接入')).toHaveCount(2)
  await expect(page.getByText(/0\s*\/\s*37/)).toHaveCount(0)
  await expect(page.getByText(/%/)).toHaveCount(0)
})

test('401 and 403 are explicit and never fall back to demo plan data', async ({ page }) => {
  await mockPlanServer(page, { unauthenticated: true })
  await openRealPlan(page)
  await expect(page.getByRole('alert')).toContainText('登录会话已失效')
  await expect(page.getByText('标准版', { exact: true })).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockPlanServer(page, { subscriptionStatus: 403 })
  await page.reload()
  await expect(page.getByRole('alert')).toContainText('没有查看套餐与权益的权限')
  await expect(page.getByText('标准版', { exact: true })).toHaveCount(0)
})

test('entitlement authority failure stays visible instead of substituting preview quotas', async ({ page }) => {
  await mockPlanServer(page, { entitlementStatus: 500 })
  await openRealPlan(page)
  await expect(page.getByRole('alert')).toContainText('entitlements denied')
  await expect(page.getByText('成员账号', { exact: true })).toHaveCount(0)
  await expect(page.getByText('500 GB', { exact: true })).toHaveCount(0)
})
