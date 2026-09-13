import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_ROLE_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type RemoteGrant = { permission: string; scope: string }
type RemoteRole = { id: string; name: string; status: string; version: number; permissions: RemoteGrant[] }
type RemoteRoleSummary = { roleId: string; roleName: string; roleStatus: string }
type RemoteMember = {
  userId: string
  email: string
  status: string
  version: number
  name: string
  phone: string
  employeeId: string
  position: string
  departmentId: string
  roles: RemoteRoleSummary[]
  derivedDataScope: string
}
type WriteRecord = { path: string; method: string; headers: Record<string, string>; body: unknown }
type MockOptions = {
  unauthenticated?: boolean
  listStatus?: number
  mutationStatus?: number
  readbackStatus?: number
  ownerConflict?: boolean
}

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

function summary(role: RemoteRole): RemoteRoleSummary {
  return { roleId: role.id, roleName: role.name, roleStatus: role.status }
}

async function mockRoleServer(page: Page, options: MockOptions = {}) {
  let roles: RemoteRole[] = [
    {
      id: 'tenant-001:owner',
      name: 'owner',
      status: 'TENANT_ROLE_STATUS_ACTIVE',
      version: 1,
      permissions: [
        { permission: 'tenant.member.manage', scope: 'DATA_SCOPE_ALL' },
        { permission: 'tenant.member.read', scope: 'DATA_SCOPE_ALL' },
        { permission: 'tenant.role.manage', scope: 'DATA_SCOPE_ALL' },
        { permission: 'tenant.role.read', scope: 'DATA_SCOPE_ALL' },
      ],
    },
    {
      id: 'role-ops',
      name: '运营负责人',
      status: 'TENANT_ROLE_STATUS_ACTIVE',
      version: 2,
      permissions: [{ permission: 'tenant.member.read', scope: 'DATA_SCOPE_SITES' }],
    },
    {
      id: 'role-disabled',
      name: '历史查看者',
      status: 'TENANT_ROLE_STATUS_DISABLED',
      version: 4,
      permissions: [{ permission: 'tenant.role.read', scope: 'DATA_SCOPE_SELF' }],
    },
  ]
  let members: RemoteMember[] = [
    {
      userId: 'user-001', email: 'owner@coffeelink.test', status: 'TENANT_MEMBER_STATUS_ACTIVE', version: 3,
      name: 'Alice Chen', phone: '', employeeId: 'EMP-1001', position: '企业负责人', departmentId: 'dept-owner',
      roles: [summary(roles[0]!)], derivedDataScope: 'all',
    },
    {
      userId: 'user-002', email: 'ops@coffeelink.test', status: 'TENANT_MEMBER_STATUS_ACTIVE', version: 2,
      name: 'Bob Lin', phone: '', employeeId: 'EMP-1002', position: '运营负责人', departmentId: 'dept-ops',
      roles: [summary(roles[1]!)], derivedDataScope: 'sites',
    },
  ]
  const writes: WriteRecord[] = []

  const record = (route: Route) => {
    const request = route.request()
    writes.push({ path: new URL(request.url()).pathname, method: request.method(), headers: request.headers(), body: request.postDataJSON() })
  }
  const replaceRole = (next: RemoteRole) => {
    roles = roles.map((role) => (role.id === next.id ? next : role))
    members = members.map((member) => ({
      ...member,
      roles: member.roles.map((item) => item.roleId === next.id ? summary(next) : item),
    }))
    return next
  }

  await page.route('**/api/auth/session', async (route) => {
    if (options.unauthenticated) return json(route, 401, { message: 'unauthenticated' })
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-001',
      active_tenant_id: 'tenant-001',
      csrf_token: 'csrf-real-role',
      tenants: [{ id: 'tenant-001', name: 'CoffeeLink 测试租户' }],
    })
  })

  await page.route('**/api/v1/tenant/members', async (route) => {
    return json(route, 200, { members })
  })

  await page.route(/\/api\/v1\/tenant\/roles(?:\/.*)?$/, async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const parts = url.pathname.split('/').filter(Boolean)
    const roleIndex = parts.indexOf('roles')
    const roleId = roleIndex >= 0 && parts[roleIndex + 1] ? decodeURIComponent(parts[roleIndex + 1]!) : ''
    const action = roleIndex >= 0 ? parts[roleIndex + 2] : undefined
    const role = roles.find((item) => item.id === roleId)

    if (request.method() === 'GET') {
      if (!roleId) {
        if (options.listStatus) return json(route, options.listStatus, { message: 'roles denied' })
        return json(route, 200, { roles })
      }
      if (options.readbackStatus && writes.length) return json(route, options.readbackStatus, { message: 'role readback failed' })
      return role ? json(route, 200, role) : json(route, 404, { message: 'role not found' })
    }

    record(route)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'role conflict' })

    if (!roleId && request.method() === 'POST') {
      const body = request.postDataJSON() as { name: string }
      const created: RemoteRole = { id: 'role-new', name: body.name, status: 'TENANT_ROLE_STATUS_ACTIVE', version: 1, permissions: [] }
      roles = [...roles, created]
      return json(route, 200, created)
    }
    if (!role) return json(route, 404, { message: 'role not found' })

    if (request.method() === 'PATCH') {
      const body = request.postDataJSON() as { name: string }
      return json(route, 200, replaceRole({ ...role, name: body.name, version: role.version + 1 }))
    }
    if (request.method() === 'PUT' && action === 'permissions') {
      const body = request.postDataJSON() as { permissions: RemoteGrant[] }
      return json(route, 200, replaceRole({ ...role, permissions: body.permissions, version: role.version + 1 }))
    }
    if (request.method() === 'POST' && (action === 'enable' || action === 'disable')) {
      if (role.name === 'owner' && action === 'disable') return json(route, 409, { message: 'owner protected' })
      return json(route, 200, replaceRole({
        ...role,
        status: action === 'enable' ? 'TENANT_ROLE_STATUS_ACTIVE' : 'TENANT_ROLE_STATUS_DISABLED',
        version: role.version + 1,
      }))
    }
    if (request.method() === 'POST' && action === 'members') {
      const userId = (request.postDataJSON() as { userId: string }).userId
      members = members.map((member) => member.userId === userId && !member.roles.some((item) => item.roleId === role.id)
        ? { ...member, roles: [...member.roles, summary(role)] }
        : member)
      return json(route, 200, role)
    }
    if (request.method() === 'POST' && parts.at(-1) === 'revoke') {
      const userId = decodeURIComponent(parts[roleIndex + 3] ?? '')
      if (options.ownerConflict && role.name === 'owner') return json(route, 409, { message: 'last owner protected' })
      members = members.map((member) => member.userId === userId
        ? { ...member, roles: member.roles.filter((item) => item.roleId !== role.id) }
        : member)
      return json(route, 200, role)
    }
    return json(route, 400, { message: 'unsupported mutation' })
  })

  return { getWrites: () => writes, getRoles: () => roles, getMembers: () => members }
}

async function openRealRoles(page: Page) {
  await page.goto('/#/enterprise/roles')
  await expect(page.locator('[data-enterprise-role-source="server"]')).toBeVisible()
}

function rowFor(page: Page, name: string) {
  return page.locator('tbody tr').filter({ hasText: name })
}

test('real role page renders authoritative grants and member counts across CoffeeLink viewports', async ({ page }) => {
  await mockRoleServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await openRealRoles(page)
    await expect(page.getByText('企业所有者', { exact: true })).toBeVisible()
    await expect(page.getByText('运营负责人', { exact: true }).first()).toBeVisible()
    await expect(page.getByText(/查看成员/).first()).toBeVisible()
    await expect(page.getByText('超级管理员', { exact: true })).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-roles-real-${viewport.width}.png` })
  }
})

test('role create and permission update use independent idempotency keys and confirm from role readback', async ({ page }) => {
  const server = await mockRoleServer(page)
  await openRealRoles(page)
  await page.getByRole('button', { name: '新建角色', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '新建角色' })
  await dialog.locator('[data-role-name]').fill('华东运营')
  await dialog.getByLabel('查看成员', { exact: true }).check()
  await dialog.getByLabel('查看成员 数据范围').selectOption('sites')
  await dialog.getByRole('button', { name: '保存并回读确认' }).click()
  await expect(page.getByRole('status')).toContainText('服务端确认')
  await expect(page.getByText('华东运营', { exact: true })).toBeVisible()
  const writes = server.getWrites()
  const create = writes.find((item) => item.path === '/api/v1/tenant/roles')!
  const permissions = writes.find((item) => item.path.endsWith('/role-new/permissions'))!
  expect(create.headers['idempotency-key']).toMatch(/^enterprise-role-create-/)
  expect(permissions.headers['idempotency-key']).toMatch(/^enterprise-role-permissions-/)
  expect(create.headers['idempotency-key']).not.toBe(permissions.headers['idempotency-key'])
  expect(create.headers['x-csrf-token']).toBe('csrf-real-role')
  expect(permissions.body).toMatchObject({ roleId: 'role-new', version: 1 })
})

test('role member assignment is confirmed from member readback', async ({ page }) => {
  const server = await mockRoleServer(page)
  await openRealRoles(page)
  await rowFor(page, '运营负责人').getByRole('button', { name: '管理' }).click()
  const dialog = page.getByRole('dialog', { name: '管理角色权限' })
  await dialog.getByLabel('角色成员 Alice Chen').check()
  await dialog.getByRole('button', { name: '保存并回读确认' }).click()
  await expect(page.getByRole('status')).toContainText('服务端确认')
  const write = server.getWrites().find((item) => item.path.endsWith('/role-ops/members'))!
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-role-assign-user-001-/)
  expect(server.getMembers().find((member) => member.userId === 'user-001')?.roles.some((role) => role.roleId === 'role-ops')).toBe(true)
})

test('401 and 403 are surfaced and never replaced with preview roles', async ({ page }) => {
  await mockRoleServer(page, { unauthenticated: true })
  await openRealRoles(page)
  await expect(page.getByRole('alert')).toContainText('登录会话已失效')
  await expect(page.getByText('超级管理员', { exact: true })).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockRoleServer(page, { listStatus: 403 })
  await page.reload()
  await expect(page.getByRole('alert')).toContainText('没有管理企业角色与权限的权限')
  await expect(page.getByText('超级管理员', { exact: true })).toHaveCount(0)
})

test('role 409 preserves the same idempotency key and editor draft for retry', async ({ page }) => {
  const server = await mockRoleServer(page, { mutationStatus: 409 })
  await openRealRoles(page)
  await rowFor(page, '运营负责人').getByRole('button', { name: '管理' }).click()
  const dialog = page.getByRole('dialog', { name: '管理角色权限' })
  await dialog.locator('[data-role-name]').fill('运营负责人-冲突草稿')
  await dialog.getByRole('button', { name: '保存并回读确认' }).click()
  await expect(dialog.getByRole('alert')).toContainText('角色版本或所有者保护规则已发生冲突')
  await expect(dialog.locator('[data-role-name]')).toHaveValue('运营负责人-冲突草稿')
  await dialog.getByRole('button', { name: '保存并回读确认' }).click()
  const writes = server.getWrites().filter((item) => item.method === 'PATCH')
  expect(writes).toHaveLength(2)
  expect(writes[0]?.headers['idempotency-key']).toBe(writes[1]?.headers['idempotency-key'])
  await expect(page.getByRole('status')).toHaveCount(0)
})

test('owner invariant conflict is surfaced and never presented as confirmed success', async ({ page }) => {
  await mockRoleServer(page, { ownerConflict: true })
  await openRealRoles(page)
  await rowFor(page, '企业所有者').getByRole('button', { name: '查看与成员' }).click()
  const dialog = page.getByRole('dialog', { name: '企业所有者角色' })
  await dialog.getByLabel('角色成员 Alice Chen').uncheck()
  await dialog.getByRole('button', { name: '保存并回读确认' }).click()
  await expect(dialog.getByRole('alert')).toContainText('角色版本或所有者保护规则已发生冲突')
  await expect(page.getByText(/角色配置已由服务端确认/)).toHaveCount(0)
})

test('a successful role write without readback is not presented as confirmed success', async ({ page }) => {
  await mockRoleServer(page, { readbackStatus: 500 })
  await openRealRoles(page)
  await rowFor(page, '运营负责人').getByRole('button', { name: '管理' }).click()
  const dialog = page.getByRole('dialog', { name: '管理角色权限' })
  await dialog.locator('[data-role-name]').fill('未确认角色名')
  await dialog.getByRole('button', { name: '保存并回读确认' }).click()
  await expect(dialog.getByRole('alert')).toContainText('role readback failed')
  await expect(page.getByText(/角色配置已由服务端确认/)).toHaveCount(0)
})
