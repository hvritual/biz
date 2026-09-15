import { expect, test, type Page, type Route } from '@playwright/test'

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
    actor_kind: 'tenant_user',
    user_id: 'user-001',
    active_tenant_id: 'tenant-001',
    csrf_token: 'csrf-audit-real',
    tenants: [{ id: 'tenant-001', name: 'CoffeeLink 测试租户' }],
  }))

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
    if (options.readStatus) return json(route, options.readStatus, { message: 'audit read denied' })
    const url = new URL(route.request().url())
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
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await openAudit(page)
    await expect(page.getByText('tenant.member.suspend', { exact: true })).toBeVisible()
    await expect(page.getByText('user_id:user-002', { exact: true })).toBeVisible()
    await expect(page.getByText('当前记录存储于本地预览')).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
  }
})

test('detail is read from the server and exposes trace references without inventing before/after payloads', async ({ page }) => {
  await mockAuditServer(page)
  await openAudit(page)
  await page.getByRole('button', { name: '查看审计日志 tenant.member.suspend' }).click()
  await expect(page.getByText('audit-001', { exact: true })).toBeVisible()
  await expect(page.getByText('sha256:session-ref', { exact: true })).toBeVisible()
  await expect(page.getByText('digest-001', { exact: true })).toBeVisible()
  await expect(page.getByText(/未提供 before\/after 正文时/)).toBeVisible()
})

test('audit export uses trusted session, CSRF and idempotency and reports only server-confirmed success', async ({ page }) => {
  const server = await mockAuditServer(page)
  await openAudit(page)
  await page.getByRole('button', { name: '导出日志' }).click()
  await expect(page.getByRole('status')).toContainText('服务端导出已完成：1 条')
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
  expect(server.writes).toHaveLength(2)
  expect(server.writes[0]?.headers['idempotency-key']).toBe(server.writes[1]?.headers['idempotency-key'])
})

test('403 read failure does not fall back to local audit preview data', async ({ page }) => {
  await mockAuditServer(page, { readStatus: 403 })
  await openAudit(page)
  await expect(page.getByRole('alert')).toContainText('没有读取或导出企业审计日志的权限')
  await expect(page.getByText('tenant.member.suspend', { exact: true })).toHaveCount(0)
  await expect(page.getByText('当前记录存储于本地预览')).toHaveCount(0)
})
