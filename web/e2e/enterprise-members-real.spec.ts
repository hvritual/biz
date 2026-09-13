import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_MEMBER_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

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
type RemoteRole = { id: string; name: string; status: string; version: number; permissions: unknown[] }

type MockOptions = {
  listStatus?: number
  mutationStatus?: number
  readbackStatus?: number
  unauthenticated?: boolean
}

type WriteRecord = { path: string; method: string; headers: Record<string, string>; body: unknown }

const roleCatalog: RemoteRole[] = [
  { id: 'role-ops', name: '运营负责人', status: 'TENANT_ROLE_STATUS_ACTIVE', version: 2, permissions: [] },
  { id: 'role-viewer', name: '经营查看者', status: 'TENANT_ROLE_STATUS_ACTIVE', version: 1, permissions: [] },
  { id: 'role-legacy', name: '历史角色', status: 'TENANT_ROLE_STATUS_DISABLED', version: 4, permissions: [] },
]

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

function roleSummary(role: RemoteRole): RemoteRoleSummary {
  return { roleId: role.id, roleName: role.name, roleStatus: role.status }
}

function deriveScope(member: RemoteMember) {
  if (member.roles.some((role) => role.roleId === 'role-ops' && role.roleStatus === 'TENANT_ROLE_STATUS_ACTIVE')) return 'sites'
  if (member.roles.some((role) => role.roleId === 'role-viewer' && role.roleStatus === 'TENANT_ROLE_STATUS_ACTIVE')) return 'self'
  return 'none'
}

async function mockMemberServer(page: Page, options: MockOptions = {}) {
  let members: RemoteMember[] = [
    {
      userId: 'user-001',
      email: 'owner@coffeelink.test',
      status: 'TENANT_MEMBER_STATUS_ACTIVE',
      version: 3,
      name: 'Alice Chen',
      phone: '+886900000001',
      employeeId: 'EMP-1001',
      position: '客户成功负责人',
      departmentId: 'dept-success',
      roles: [roleSummary(roleCatalog[0]!)],
      derivedDataScope: 'sites',
    },
    {
      userId: 'user-002',
      email: 'new@coffeelink.test',
      status: 'TENANT_MEMBER_STATUS_INVITED',
      version: 1,
      name: '',
      phone: '',
      employeeId: '',
      position: '',
      departmentId: '',
      roles: [],
      derivedDataScope: 'none',
    },
  ]
  const writes: WriteRecord[] = []

  const recordWrite = (route: Route) => {
    const request = route.request()
    writes.push({
      path: new URL(request.url()).pathname,
      method: request.method(),
      headers: request.headers(),
      body: request.postDataJSON(),
    })
  }

  await page.route('**/api/auth/session', async (route) => {
    if (options.unauthenticated) return json(route, 401, { message: 'unauthenticated' })
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-001',
      active_tenant_id: 'tenant-001',
      csrf_token: 'csrf-real-member',
      tenants: [{ id: 'tenant-001', name: 'CoffeeLink 测试租户' }],
    })
  })

  await page.route('**/api/v1/tenant/roles', async (route) => {
    return json(route, 200, { roles: roleCatalog })
  })

  await page.route(/\/api\/v1\/tenant\/roles\/[^/]+\/members(?:\/[^/]+\/revoke)?$/, async (route) => {
    recordWrite(route)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'mutation conflict' })
    const request = route.request()
    const url = new URL(request.url())
    const parts = url.pathname.split('/')
    const roleId = decodeURIComponent(parts[parts.indexOf('roles') + 1] || '')
    const body = request.postDataJSON() as { userId?: string }
    const userId = body.userId || decodeURIComponent(parts[parts.indexOf('members') + 1] || '')
    const role = roleCatalog.find((item) => item.id === roleId)
    if (!role) return json(route, 404, { message: 'role not found' })
    const revoke = url.pathname.endsWith('/revoke')
    members = members.map((member) => {
      if (member.userId !== userId) return member
      const nextRoles = revoke
        ? member.roles.filter((item) => item.roleId !== roleId)
        : member.roles.some((item) => item.roleId === roleId)
          ? member.roles
          : [...member.roles, roleSummary(role)]
      const next = { ...member, roles: nextRoles }
      return { ...next, derivedDataScope: deriveScope(next) }
    })
    return json(route, 200, role)
  })

  await page.route('**/api/v1/tenant/members', async (route) => {
    const request = route.request()
    if (request.method() === 'GET') {
      if (options.listStatus) return json(route, options.listStatus, { message: 'list denied' })
      return json(route, 200, { members })
    }
    recordWrite(route)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'mutation conflict' })
    const body = request.postDataJSON() as { email: string }
    const created: RemoteMember = {
      userId: 'user-003',
      email: body.email,
      status: 'TENANT_MEMBER_STATUS_INVITED',
      version: 1,
      name: '',
      phone: '',
      employeeId: '',
      position: '',
      departmentId: '',
      roles: [],
      derivedDataScope: 'none',
    }
    members = [...members, created]
    return json(route, 200, created)
  })

  await page.route(/\/api\/v1\/tenant\/members\/[^/]+\/profile$/, async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const parts = url.pathname.split('/')
    const userId = decodeURIComponent(parts[parts.indexOf('members') + 1] || '')
    const current = members.find((member) => member.userId === userId)
    if (!current) return json(route, 404, { message: 'not found' })
    recordWrite(route)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'mutation conflict' })
    const body = request.postDataJSON() as Partial<RemoteMember>
    const updated: RemoteMember = {
      ...current,
      name: String(body.name ?? '').trim(),
      phone: String(body.phone ?? '').trim(),
      employeeId: String(body.employeeId ?? '').trim(),
      position: String(body.position ?? '').trim(),
      departmentId: String(body.departmentId ?? '').trim(),
      version: current.version + 1,
    }
    members = members.map((member) => (member.userId === userId ? updated : member))
    return json(route, 200, updated)
  })

  await page.route(/\/api\/v1\/tenant\/members\/[^/]+(?:\/(?:activate|suspend|remove))?$/, async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const parts = url.pathname.split('/')
    const userId = decodeURIComponent(parts[parts.indexOf('members') + 1] || '')
    const action = parts.at(-1)
    const current = members.find((member) => member.userId === userId)
    if (!current) return json(route, 404, { message: 'not found' })

    if (request.method() === 'GET') {
      if (options.readbackStatus) return json(route, options.readbackStatus, { message: 'readback failed' })
      return json(route, 200, current)
    }

    recordWrite(route)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'mutation conflict' })
    const nextStatus =
      action === 'activate'
        ? 'TENANT_MEMBER_STATUS_ACTIVE'
        : action === 'suspend'
          ? 'TENANT_MEMBER_STATUS_SUSPENDED'
          : 'TENANT_MEMBER_STATUS_REMOVED'
    const updated = { ...current, status: nextStatus, version: current.version + 1 }
    members = members.map((member) => (member.userId === userId ? updated : member))
    return json(route, 200, updated)
  })

  return {
    getMembers: () => members,
    getWrites: () => writes,
  }
}

async function openRealMembers(page: Page) {
  await page.goto('/#/enterprise/members')
  await expect(page.locator('[data-enterprise-member-source="server"]')).toBeVisible()
}

test('real member page renders authoritative profile, role and scope fields across CoffeeLink viewports', async ({ page }) => {
  await mockMemberServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await openRealMembers(page)
    await expect(page.getByText('Alice Chen', { exact: true })).toBeVisible()
    await expect(page.getByText('EMP-1001', { exact: true })).toBeVisible()
    await expect(page.getByText('dept-success', { exact: true })).toBeVisible()
    await expect(page.getByText('运营负责人', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('授权点位', { exact: true })).toBeVisible()
    await expect(page.getByText('上海咖啡科技有限公司', { exact: true })).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-members-real-${viewport.width}.png` })
  }
})

test('invite uses csrf, idempotency and session context then confirms with server readback', async ({ page }) => {
  const server = await mockMemberServer(page)
  await openRealMembers(page)
  await page.getByRole('button', { name: '邀请成员', exact: true }).click()
  await page.getByLabel('成员邮箱').fill('invitee@coffeelink.test')
  await page.getByRole('button', { name: '确认操作', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('服务端确认')
  await expect(page.getByText('invitee@coffeelink.test', { exact: true })).toBeVisible()
  const write = server.getWrites().at(-1)!
  expect(write.headers['x-csrf-token']).toBe('csrf-real-member')
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-member-invite-/)
  expect(write.headers['x-biz-session-context']).toContain('tenant-001')
})

test('profile PATCH carries authoritative version and confirms only after member readback', async ({ page }) => {
  const server = await mockMemberServer(page)
  await openRealMembers(page)
  await page.getByRole('button', { name: '档案', exact: true }).first().click()
  const dialog = page.getByRole('dialog', { name: '编辑成员档案' })
  await dialog.getByLabel('姓名').fill('Alice Updated')
  await dialog.getByLabel('手机号').fill('+886900000009')
  await dialog.getByLabel('工号').fill('EMP-2009')
  await dialog.getByLabel('岗位').fill('租赁运营负责人')
  await dialog.getByLabel('部门引用').fill('dept-rental')
  await dialog.getByRole('button', { name: '确认操作', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('服务端确认')
  await expect(page.getByText('Alice Updated', { exact: true })).toBeVisible()
  const write = server.getWrites().find((item) => item.path.endsWith('/profile'))!
  expect(write.method).toBe('PATCH')
  expect(write.headers['x-csrf-token']).toBe('csrf-real-member')
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-member-profile-/)
  expect(write.headers['x-biz-session-context']).toContain('tenant-001')
  expect(write.body).toMatchObject({ userId: 'user-001', version: 3, departmentId: 'dept-rental' })
})

test('role binding is a single idempotent operation and is confirmed from member readback', async ({ page }) => {
  const server = await mockMemberServer(page)
  await openRealMembers(page)
  await page.getByRole('button', { name: '角色', exact: true }).first().click()
  const dialog = page.getByRole('dialog', { name: '管理成员角色' })
  const viewerRow = dialog.locator('.role-row').filter({ hasText: '经营查看者' })
  await viewerRow.getByRole('button', { name: '绑定', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('角色关系已由服务端确认')
  await expect(viewerRow.getByText('已绑定', { exact: true })).toBeVisible()
  const write = server.getWrites().find((item) => item.path.endsWith('/roles/role-viewer/members'))!
  expect(write.method).toBe('POST')
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-member-role-assign-/)
  expect(write.headers['x-csrf-token']).toBe('csrf-real-member')
})

test('401 and 403 are surfaced and never replaced with preview members', async ({ page }) => {
  await mockMemberServer(page, { unauthenticated: true })
  await openRealMembers(page)
  await expect(page.getByRole('alert')).toContainText('登录会话已失效')
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockMemberServer(page, { listStatus: 403 })
  await page.reload()
  await expect(page.getByRole('alert')).toContainText('没有管理企业成员的权限')
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)
})

test('profile 409 preserves the same idempotency key for retry', async ({ page }) => {
  const server = await mockMemberServer(page, { mutationStatus: 409 })
  await openRealMembers(page)
  await page.getByRole('button', { name: '档案', exact: true }).first().click()
  const dialog = page.getByRole('dialog', { name: '编辑成员档案' })
  await dialog.getByLabel('姓名').fill('conflicting change')
  await dialog.getByRole('button', { name: '确认操作', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText('成员状态或请求版本已发生变化')
  const retry = dialog.getByRole('button', { name: '重试相同操作', exact: true })
  await retry.click()
  await expect(dialog.getByRole('alert')).toContainText('成员状态或请求版本已发生变化')
  const writes = server.getWrites().filter((item) => item.path.endsWith('/profile'))
  expect(writes).toHaveLength(2)
  expect(writes[0]?.headers['idempotency-key']).toBe(writes[1]?.headers['idempotency-key'])
  await expect(page.getByRole('status')).toHaveCount(0)
})

test('a successful write without readback is not presented as confirmed success', async ({ page }) => {
  await mockMemberServer(page, { readbackStatus: 500 })
  await openRealMembers(page)
  await page.getByRole('button', { name: '档案', exact: true }).first().click()
  const dialog = page.getByRole('dialog', { name: '编辑成员档案' })
  await dialog.getByLabel('姓名').fill('unconfirmed change')
  await dialog.getByRole('button', { name: '确认操作', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText('readback failed')
  await expect(page.getByText(/服务端确认/)).toHaveCount(0)
})
