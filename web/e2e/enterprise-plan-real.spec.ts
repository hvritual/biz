import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_PLAN_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type Options = {
  unauthenticated?: boolean
  subscriptionStatus?: number
  entitlementStatus?: number
  usageStatus?: number
  memberUsed?: number
}

type Captured = {
  subscriptionPaths: string[]
  entitlementBodies: Array<Record<string, unknown>>
  entitlementHeaders: Array<Record<string, string>>
  usagePaths: string[]
  usageHeaders: Array<Record<string, string>>
}

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function mockPlanServer(page: Page, options: Options = {}): Promise<Captured> {
  const captured: Captured = {
    subscriptionPaths: [],
    entitlementBodies: [],
    entitlementHeaders: [],
    usagePaths: [],
    usageHeaders: [],
  }
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
        { kind: 'module', moduleCode: 'access-management', key: 'access-management', allowed: true, reason: 'plan grant' },
        { kind: 'module', moduleCode: 'device-operations', key: 'device-operations', allowed: true, reason: 'plan grant' },
        { kind: 'module', moduleCode: 'advanced-reporting', key: 'advanced-reporting', allowed: false, reason: 'not in plan' },
        { kind: 'capability', moduleCode: 'access-management', key: 'tenant.member.lifecycle', allowed: true, reason: 'plan grant' },
        { kind: 'quota', moduleCode: 'access-management', key: 'tenant.members', allowed: true, limit: { unlimited: false, value: 3 }, reason: 'plan limit' },
        { kind: 'quota', moduleCode: 'device-operations', key: 'tenant.devices', allowed: true, limit: { unlimited: true, value: 0 }, reason: 'plan limit' },
        { kind: 'quota', moduleCode: 'advanced-reporting', key: 'monthly.reports', allowed: true, limit: { unlimited: false, value: 100 }, reason: 'plan limit' },
      ],
    })
  })

  await page.route('**/api/v1/tenant/usage', async (route) => {
    const request = route.request()
    captured.usagePaths.push(new URL(request.url()).pathname)
    captured.usageHeaders.push(request.headers())
    if (options.usageStatus) return json(route, options.usageStatus, { message: 'usage denied' })
    return json(route, 200, {
      usages: [
        {
          moduleCode: 'access-management',
          key: 'tenant.members',
          known: true,
          used: options.memberUsed ?? 3,
          evidence: 'biz_memberships status!=removed (invited+active+suspended consume quota)',
        },
      ],
    })
  })
  return captured
}

async function openRealPlan(page: Page) {
  await page.goto('/#/enterprise/plan')
  await expect(page.locator('[data-enterprise-plan-source="server"]')).toBeVisible()
}

test('real plan page renders authoritative subscription, entitlement and usage facts across CoffeeLink viewports', async ({ page }) => {
  await mockPlanServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [{ width: 1366, height: 768 }, { width: 1440, height: 900 }, { width: 1536, height: 1024 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport)
    await openRealPlan(page)
    await expect(page.getByText('rental-growth-2026', { exact: true })).toBeVisible()
    await expect(page.getByText('Tenant Commercial API')).toBeVisible()
    await expect(page.getByText('已接入用量 1 项')).toBeVisible()
    await expect(page.getByText('标准版', { exact: true })).toHaveCount(0)
    await expect(page.getByText('500 GB', { exact: true })).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-plan-real-${viewport.width}.png`, fullPage: false })
  }
})

test('tenant plan and usage reads never send arbitrary tenant ids and bind to trusted session', async ({ page }) => {
  const captured = await mockPlanServer(page)
  await openRealPlan(page)
  await expect.poll(() => captured.entitlementBodies.length).toBe(1)
  await expect.poll(() => captured.usagePaths.length).toBe(1)
  expect(captured.subscriptionPaths).toContain('/api/v1/tenant/subscription')
  expect(captured.subscriptionPaths.some((path) => path.includes('tenant-001'))).toBe(false)
  expect(captured.entitlementBodies).toHaveLength(1)
  expect(captured.entitlementBodies[0]?.tenantId).toBeUndefined()
  expect(captured.entitlementBodies[0]?.capabilityCodes).toEqual([])
  expect(captured.entitlementHeaders[0]?.['x-biz-session-context']).toContain('tenant-001')
  expect(captured.entitlementHeaders[0]?.['x-csrf-token']).toBe('csrf-plan-read')
  expect(captured.usagePaths).toEqual(['/api/v1/tenant/usage'])
  expect(captured.usagePaths.some((path) => path.includes('tenant-001'))).toBe(false)
  expect(captured.usageHeaders[0]?.['x-biz-session-context']).toContain('tenant-001')
})

test('finite member quota uses authoritative usage while unsupported quotas stay unknown instead of zero', async ({ page }) => {
  await mockPlanServer(page, { memberUsed: 3 })
  await openRealPlan(page)
  await page.getByRole('button', { name: '额度用量' }).click()

  const memberRow = page.getByRole('row').filter({ hasText: 'tenant.members' })
  await expect(memberRow).toContainText('3')
  await expect(memberRow).toContainText('0')
  await expect(memberRow).toContainText('额度已用尽')

  const unknownRow = page.getByRole('row').filter({ hasText: 'monthly.reports' })
  await expect(unknownRow).toContainText('100')
  await expect(unknownRow).toContainText('未知')
  await expect(unknownRow).toContainText('用量未知')
  await expect(unknownRow).not.toContainText('0 / 100')
  await expect(page.getByText(/%/)).toHaveCount(0)
})

test('member quota below limit renders authoritative remaining capacity', async ({ page }) => {
  await mockPlanServer(page, { memberUsed: 2 })
  await openRealPlan(page)
  await page.getByRole('button', { name: '额度用量' }).click()
  const memberRow = page.getByRole('row').filter({ hasText: 'tenant.members' })
  await expect(memberRow).toContainText('2')
  await expect(memberRow).toContainText('1')
  await expect(memberRow).toContainText('额度可用')
})

test('usage authority 5xx keeps quota limits visible but explicitly degrades usage to unknown', async ({ page }) => {
  await mockPlanServer(page, { usageStatus: 500 })
  await openRealPlan(page)
  await expect(page.getByText(/用量服务暂不可用/)).toBeVisible()
  await page.getByRole('button', { name: '额度用量' }).click()
  const memberRow = page.getByRole('row').filter({ hasText: 'tenant.members' })
  await expect(memberRow).toContainText('3')
  await expect(memberRow).toContainText('未知')
  await expect(memberRow).toContainText('用量未知')
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

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockPlanServer(page, { usageStatus: 403 })
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
