import { selectUiOption } from './ui.helpers'
import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_ORGANIZATION_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type Department = {
  departmentId: string
  name: string
  parentId: string
  leaderUserId: string
  email: string
  phone: string
  status: string
  sort: number
  version: number
}
type Member = {
  userId: string
  email: string
  status: string
  version: number
  name: string
  phone: string
  employeeId: string
  position: string
  departmentId: string
  roles: Array<{ roleId: string; roleName: string; roleStatus: string }>
  derivedDataScope: string
}
type Write = { path: string; method: string; headers: Record<string, string>; body: unknown }
type Options = {
  unauthenticated?: boolean
  listStatus?: number
  mutationStatus?: number
  readbackStatus?: number
}

const active = 'TENANT_DEPARTMENT_STATUS_ACTIVE'
const disabled = 'TENANT_DEPARTMENT_STATUS_DISABLED'

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

async function mockOrganizationServer(page: Page, options: Options = {}) {
  let departments: Department[] = [
    { departmentId: 'dept-root', name: '运营中心', parentId: '', leaderUserId: 'user-001', email: 'ops@coffeelink.test', phone: '021-60000001', status: active, sort: 10, version: 2 },
    { departmentId: 'dept-success', name: '客户成功部', parentId: 'dept-root', leaderUserId: 'user-002', email: 'success@coffeelink.test', phone: '', status: active, sort: 20, version: 3 },
    { departmentId: 'dept-history', name: '历史业务部', parentId: '', leaderUserId: '', email: '', phone: '', status: disabled, sort: 90, version: 4 },
  ]
  const members: Member[] = [
    { userId: 'user-001', email: 'owner@coffeelink.test', status: 'TENANT_MEMBER_STATUS_ACTIVE', version: 4, name: 'Alice Chen', phone: '', employeeId: 'EMP-1001', position: '运营负责人', departmentId: 'dept-root', roles: [{ roleId: 'owner', roleName: 'owner', roleStatus: 'active' }], derivedDataScope: 'all' },
    { userId: 'user-002', email: 'csm@coffeelink.test', status: 'TENANT_MEMBER_STATUS_ACTIVE', version: 3, name: 'Bob Lin', phone: '', employeeId: 'EMP-1002', position: '客户成功负责人', departmentId: 'dept-success', roles: [{ roleId: 'csm', roleName: '客户成功', roleStatus: 'active' }], derivedDataScope: 'sites' },
    { userId: 'user-003', email: 'history@coffeelink.test', status: 'TENANT_MEMBER_STATUS_ACTIVE', version: 2, name: 'Carol Wu', phone: '', employeeId: 'EMP-1003', position: '顾问', departmentId: 'dept-history', roles: [], derivedDataScope: 'self' },
  ]
  const writes: Write[] = []
  const record = (route: Route) => {
    const request = route.request()
    writes.push({ path: new URL(request.url()).pathname, method: request.method(), headers: request.headers(), body: request.postDataJSON() })
  }
  const replace = (next: Department) => {
    departments = departments.map((item) => item.departmentId === next.departmentId ? next : item)
    return next
  }

  await page.route('**/api/auth/session', async (route) => {
    if (options.unauthenticated) return json(route, 401, { message: 'unauthenticated' })
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-001',
      active_tenant_id: 'tenant-001',
      csrf_token: 'csrf-real-organization',
      tenants: [{ id: 'tenant-001', name: 'CoffeeLink 测试租户' }],
    })
  })
  await page.route('**/api/v1/tenant/members', async (route) => json(route, 200, { members }))
  await page.route(/\/api\/v1\/tenant\/departments(?:\/.*)?$/, async (route) => {
    const request = route.request()
    const parts = new URL(request.url()).pathname.split('/').filter(Boolean)
    const index = parts.indexOf('departments')
    const departmentId = index >= 0 && parts[index + 1] ? decodeURIComponent(parts[index + 1]!) : ''
    const action = index >= 0 ? parts[index + 2] : undefined
    const department = departments.find((item) => item.departmentId === departmentId)

    if (request.method() === 'GET') {
      if (!departmentId) {
        if (options.listStatus) return json(route, options.listStatus, { message: 'organization denied' })
        return json(route, 200, { departments })
      }
      if (options.readbackStatus && writes.length) return json(route, options.readbackStatus, { message: 'department readback failed' })
      return department ? json(route, 200, department) : json(route, 404, { message: 'department not found' })
    }

    record(route)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'department conflict' })

    if (!departmentId && request.method() === 'POST') {
      const body = request.postDataJSON() as Partial<Department>
      const created: Department = {
        departmentId: 'dept-new',
        name: body.name ?? '',
        parentId: body.parentId ?? '',
        leaderUserId: body.leaderUserId ?? '',
        email: body.email ?? '',
        phone: body.phone ?? '',
        status: active,
        sort: Number(body.sort ?? 0),
        version: 1,
      }
      departments = [...departments, created]
      return json(route, 200, created)
    }

    if (!department) return json(route, 404, { message: 'department not found' })
    if (request.method() === 'PATCH') {
      const body = request.postDataJSON() as Partial<Department>
      return json(route, 200, replace({
        ...department,
        name: body.name ?? department.name,
        parentId: body.parentId ?? '',
        leaderUserId: body.leaderUserId ?? '',
        email: body.email ?? '',
        phone: body.phone ?? '',
        sort: Number(body.sort ?? 0),
        version: department.version + 1,
      }))
    }
    if (request.method() === 'POST' && (action === 'enable' || action === 'disable')) {
      return json(route, 200, replace({
        ...department,
        status: action === 'enable' ? active : disabled,
        version: department.version + 1,
      }))
    }
    return json(route, 400, { message: 'unsupported mutation' })
  })
  return { getWrites: () => writes }
}

async function openRealOrganization(page: Page) {
  await page.goto('/#/enterprise/organization')
  await expect(page.locator('[data-enterprise-organization-source="server"]')).toBeVisible()
}

async function selectDepartment(page: Page, name: string) {
  await page.locator('.tree-row').filter({ hasText: name }).click()
}

test('real organization page renders authoritative hierarchy across CoffeeLink viewports', async ({ page }) => {
  await mockOrganizationServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [{ width: 1366, height: 768 }, { width: 1440, height: 900 }, { width: 1536, height: 1024 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport)
    await openRealOrganization(page)
    await expect(page.getByText('运营中心', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('客户成功部', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('历史业务部', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('张三', { exact: true })).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-organization-real-${viewport.width}.png` })
  }
})

test('department create uses CSRF and idempotency then confirms from server readback', async ({ page }) => {
  const server = await mockOrganizationServer(page)
  await openRealOrganization(page)
  await page.getByRole('button', { name: '新建部门', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '新建部门' })
  await dialog.getByLabel('部门名称 *').fill('市场运营部')
  await selectUiOption(dialog.getByLabel('负责人'), 'user-001')
  await dialog.getByRole('button', { name: '提交并回读确认' }).click()
  await expect(page.getByRole('status')).toContainText('服务端确认')
  await expect(page.getByText('市场运营部', { exact: true }).first()).toBeVisible()
  const write = server.getWrites().find((item) => item.path === '/api/v1/tenant/departments')!
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-department-create-new-/)
  expect(write.headers['x-csrf-token']).toBe('csrf-real-organization')
  expect(write.headers['x-biz-session-context']).toContain('tenant-001')
})

test('department 409 preserves editor draft and the same idempotency key for retry', async ({ page }) => {
  const server = await mockOrganizationServer(page, { mutationStatus: 409 })
  await openRealOrganization(page)
  await selectDepartment(page, '客户成功部')
  await page.getByRole('button', { name: '编辑部门' }).click()
  const dialog = page.getByRole('dialog', { name: '编辑部门' })
  await dialog.getByLabel('部门名称 *').fill('客户增长部')
  const save = dialog.getByRole('button', { name: '提交并回读确认' })
  await save.click()
  await expect(dialog.getByRole('alert')).toContainText('部门版本、层级、负责人或成员归属规则发生冲突')
  await expect(dialog.getByLabel('部门名称 *')).toHaveValue('客户增长部')
  await expect(save).toBeEnabled()
  await save.click()
  await expect(dialog.getByRole('alert')).toContainText('部门版本、层级、负责人或成员归属规则发生冲突')
  const patches = server.getWrites().filter((item) => item.method === 'PATCH')
  expect(patches).toHaveLength(2)
  expect(patches[0]?.headers['idempotency-key']).toBe(patches[1]?.headers['idempotency-key'])
  await expect(page.getByRole('status')).toHaveCount(0)
})

test('401 and 403 are surfaced without demo organization fallback', async ({ page }) => {
  await mockOrganizationServer(page, { unauthenticated: true })
  await openRealOrganization(page)
  await expect(page.getByRole('alert')).toContainText('登录会话已失效')
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)
  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockOrganizationServer(page, { listStatus: 403 })
  await page.reload()
  await expect(page.getByRole('alert')).toContainText('没有管理企业组织架构的权限')
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)
})

test('successful department write without readback is never shown as confirmed success', async ({ page }) => {
  await mockOrganizationServer(page, { readbackStatus: 500 })
  await openRealOrganization(page)
  await selectDepartment(page, '客户成功部')
  await page.getByRole('button', { name: '编辑部门' }).click()
  const dialog = page.getByRole('dialog', { name: '编辑部门' })
  await dialog.getByLabel('部门名称 *').fill('未确认部门')
  await dialog.getByRole('button', { name: '提交并回读确认' }).click()
  await expect(dialog.getByRole('alert')).toContainText('department readback failed')
  await expect(page.getByText(/部门配置已由服务端确认/)).toHaveCount(0)
})
