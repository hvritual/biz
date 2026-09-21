import { expect, test, type Page, type Route } from '@playwright/test'
import { installApiFailFast } from './ui.helpers'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_AUDIT_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type AuditRecord = {
  auditId: string
  occurredAt: string
  actorSubject: string
  actorUserId: string
  authMethod: string
  authChannel: string
  sessionRef: string
  requestId: string
  idempotencyRef: string
  operationId: string
  module: string
  target: string
  result: string
  risk: string
  receiptRef: string
  reason: string
  requestDigest: string
}

type ExportWrite = {
  headers: Record<string, string>
  body: Record<string, unknown>
}

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function mockAuditServer(page: Page, options: { readStatus?: number; exportStatus?: number } = {}) {
  await installApiFailFast(page)
  const writes: ExportWrite[] = []
  const records: AuditRecord[] = [
    {
      auditId: 'audit-001',
      occurredAt: '2026-09-15T10:00:00Z',
      actorSubject: 'user:user-001',
      actorUserId: 'user-001',
      authMethod: 'session',
      authChannel: 'web-session',
      sessionRef: 'sha256:session-ref',
      requestId: 'req-001',
      idempotencyRef: 'sha256:idem-ref',
      operationId: 'tenant.member.suspend',
      module: 'access',
      target: 'user_id:user-002',
      result: 'success',
      risk: 'high',
      receiptRef: 'request:req-001',
      reason: '成员离职',
      requestDigest: 'digest-001',
    },
  ]

  await page.route('**/api/auth/session', async (route) => json(route, 200, {
    authenticated: true,
    actor_kind: 'user',
    user_id: 'user-001',
    active_tenant_id: 'tenant-001',
    csrf_token: 'csrf-audit-real',
    tenants: [{ id: 'tenant-001', name: 'CoffeeLink 测试租户' }],
  }))
  await page.route('**/api/auth/authorization', async (route) => {
    const buttonCodes = ["access.audit.list","access.audit.get","access.audit.export"]
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'user',
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

  await page.route('**/api/v1/tenant/audit-logs/exports', async (route) => {
    const request = route.request()
    writes.push({ headers: request.headers(), body: request.postDataJSON() as Record<string, unknown> })
    if (options.exportStatus) return json(route, options.exportStatus, { message: 'audit export denied' })
    return json(route, 200, {
      exportId: 'export-001',
      generatedAt: '2026-09-15T10:01:00Z',
      records,
    })
  })

  await page.route('**/api/v1/tenant/audit-logs/audit-001', async (route) => {
    if (options.readStatus) return json(route, options.readStatus, { message: 'audit read denied' })
    return json(route, 200, records[0])
  })

  await page.route('**/api/v1/tenant/audit-logs**', async (route) => {
    const url = new URL(route.request().url())
    if (url.pathname !== '/api/v1/tenant/audit-logs') return route.fallback()
    if (options.readStatus) return json(route, options.readStatus, { message: 'audit read denied' })
    return json(route, 200, {
      records,
      total: 1,
      page: Number(url.searchParams.get('page') || 1),
      pageSize: Number(url.searchParams.get('page_size') || 20),
    })
  })

  return { writes }
}

async function openAudit(page: Page) {
  await page.goto('/#/enterprise/logs')
  await expect(page.locator('[data-enterprise-audit-source="server"]')).toBeVisible()
}

test('API mode renders authoritative audit data across CoffeeLink viewports without demo fallback', async ({ page }) => {
  await mockAuditServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await openAudit(page)
    await expect(page.getByText('权限与成员', { exact: true })).toBeVisible()
    await expect(page.getByText('禁用成员', { exact: true })).toBeVisible()
    await expect(page.getByText('成员 user-002', { exact: true })).toBeVisible()
    await expect(page.getByText(/tenant\.member\.suspend|user_id:/)).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-audit-real-${viewport.width}.png`, fullPage: false })
  }
})

test('detail is read from the server and exposes trace references without inventing before/after payloads', async ({ page }) => {
  await mockAuditServer(page)
  await openAudit(page)
  await page.getByRole('button', { name: '查看操作日志 audit-001' }).click()
  const detail = page.getByRole('dialog', { name: '日志详情' })
  await expect(detail.getByText('audit-001', { exact: true })).toBeVisible()
  await expect(detail.getByText('权限与成员', { exact: true }).first()).toBeVisible()
  await expect(detail.getByText(/禁用成员/).first()).toBeVisible()
  await expect(detail.getByText('成员 user-002', { exact: true })).toBeVisible()
  await expect(detail.getByText(/sha256:session-ref|digest-001|request:req-001/)).toHaveCount(0)
})

test('audit export uses trusted session, CSRF and idempotency and reports only server-confirmed success', async ({ page }) => {
  const server = await mockAuditServer(page)
  await openAudit(page)
  await page.getByRole('button', { name: '导出日志' }).click()
  await expect(page.getByRole('status')).toContainText('日志导出完成：1 条')
  expect(server.writes).toHaveLength(1)
  expect(server.writes[0]?.headers['x-csrf-token']).toBe('csrf-audit-real')
  expect(server.writes[0]?.headers['idempotency-key']).toMatch(/^enterprise-audit-export-/)
  expect(server.writes[0]?.headers['x-biz-session-context']).toContain('tenant-001')
  expect(server.writes[0]?.body.maxRows).toBe(5000)
})

test('failed export preserves the same idempotency key for retry and never shows fake success', async ({ page }) => {
  const server = await mockAuditServer(page, { exportStatus: 409 })
  await openAudit(page)
  await page.getByRole('button', { name: '导出日志' }).click()
  await expect(page.getByRole('alert')).toBeVisible()
  await expect(page.getByRole('status')).toHaveCount(0)
  await page.getByRole('button', { name: '重试导出' }).click()
  await expect.poll(() => server.writes.length).toBe(2)
  expect(server.writes[0]?.headers['idempotency-key']).toBe(server.writes[1]?.headers['idempotency-key'])
})

test('403 read failure does not fall back to local audit preview data', async ({ page }) => {
  await mockAuditServer(page, { readStatus: 403 })
  await openAudit(page)
  await expect(page.getByRole('alert')).toContainText('没有读取或导出企业审计日志的权限')
  await expect(page.getByText('tenant.member.suspend', { exact: true })).toHaveCount(0)
  await expect(page.getByText('当前记录存储于本地预览')).toHaveCount(0)
})
