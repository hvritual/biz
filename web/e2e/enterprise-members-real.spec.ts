import { expect, test, type Page, type Route } from '@playwright/test'
import { selectUiOption } from './ui.helpers'
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
type RemoteRole = { id: string; name: string; status: string; version: number; permissions: unknown[]; protectedOwner?: boolean }
type RemoteDepartment = { departmentId: string; name: string; parentId: string; leaderUserId: string; email: string; phone: string; status: string; sort: number; version: number }

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

const departmentCatalog: RemoteDepartment[] = [
  { departmentId: 'dept-success', name: '客户成功部', parentId: '', leaderUserId: 'user-001', email: 'success@coffeelink.test', phone: '', status: 'TENANT_DEPARTMENT_STATUS_ACTIVE', sort: 10, version: 3 },
  { departmentId: 'dept-rental', name: '租赁运营部', parentId: '', leaderUserId: 'user-001', email: 'rental@coffeelink.test', phone: '', status: 'TENANT_DEPARTMENT_STATUS_ACTIVE', sort: 20, version: 2 },
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

  await page.route('**/api/v1/tenant/departments', async (route) => {
    return json(route, 200, { departments: departmentCatalog })
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

async function openCanonicalMembers(page: Page) {
  await page.goto('/#/enterprise/members')
  await expect(page.locator('[data-enterprise-page="members"]')).toBeVisible()
  await expect(page.locator('[data-enterprise-source="api"]')).toBeVisible()
}

test('canonical member page renders authoritative member, role, department and scope across CoffeeLink viewports', async ({ page }) => {
  await mockMemberServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await openCanonicalMembers(page)
    const row = page.locator('[data-member-id="user-001"]')
    await expect(row).toBeVisible()
    await expect(row).toContainText('Alice Chen')
    await expect(row).toContainText('客户成功部')
    await expect(row).toContainText('运营负责人')
    await expect(row).toContainText('指定数据')
    await expect(page.getByText('张三', { exact: true })).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/enterprise-members-real-${viewport.width}.png` })
  }
})

test('canonical invite uses loaded role and department, trusted headers and authoritative readback', async ({ page }) => {
  const server = await mockMemberServer(page)
  await openCanonicalMembers(page)
  await page.getByRole('button', { name: '邀请成员', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '邀请成员' })
  await dialog.getByLabel('姓名').fill('Invitee User')
  await dialog.getByLabel('邮箱').fill('invitee@coffeelink.test')
  await expect(dialog.getByLabel('所属部门')).toContainText('客户成功部')
  await expect(dialog.getByLabel('数据范围')).toHaveValue('保存后由角色权限服务派生')
  await expect(dialog.getByLabel('运营负责人')).toBeChecked()
  await dialog.getByRole('button', { name: '创建邀请', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('服务端确认')
  await expect(page.locator('[data-member-id="user-003"]')).toContainText('Invitee User')
  const invite = server.getWrites().find((item) => item.path === '/api/v1/tenant/members' && item.method === 'POST')!
  expect(invite.headers['x-csrf-token']).toBe('csrf-real-member')
  expect(invite.headers['idempotency-key']).toMatch(/^enterprise-member-invite-/)
  expect(invite.headers['x-biz-session-context']).toContain('tenant-001')
  expect(server.getMembers().find((member) => member.userId === 'user-003')).toMatchObject({
    name: 'Invitee User',
    departmentId: 'dept-success',
  })
})

test('canonical profile edit exposes only supported fields and sends authoritative version', async ({ page }) => {
  const server = await mockMemberServer(page)
  await openCanonicalMembers(page)
  await page.getByRole('button', { name: '编辑 Alice Chen', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '修改成员信息' })
  await expect(dialog.getByLabel('邮箱')).toHaveAttribute('readonly', '')
  await expect(dialog.getByLabel('加入日期')).toHaveValue('服务端未提供')
  await expect(dialog.getByLabel('数据范围')).toHaveValue('指定数据')
  await expect(dialog.getByText(/成员资料 API 未提供备注写入字段/)).toBeVisible()
  await dialog.getByLabel('姓名').fill('Alice Updated')
  await dialog.getByLabel('手机号').fill('+886900000009')
  await dialog.getByLabel('员工编号').fill('EMP-2009')
  await dialog.getByLabel('岗位').fill('租赁运营负责人')
  await selectUiOption(dialog.getByLabel('所属部门'), 'dept-rental')
  await dialog.getByRole('button', { name: '保存变更', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('服务端确认')
  const row = page.locator('[data-member-id="user-001"]')
  await expect(row).toContainText('Alice Updated')
  await expect(row).toContainText('租赁运营部')
  const write = server.getWrites().find((item) => item.path.endsWith('/profile'))!
  expect(write.method).toBe('PATCH')
  expect(write.headers['x-csrf-token']).toBe('csrf-real-member')
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-member-profile-/)
  expect(write.headers['x-biz-session-context']).toContain('tenant-001')
  expect(write.body).toMatchObject({ userId: 'user-001', version: 3, departmentId: 'dept-rental', employeeId: 'EMP-2009' })
  expect(write.body).not.toHaveProperty('email')
  expect(write.body).not.toHaveProperty('scope')
  expect(write.body).not.toHaveProperty('joinedAt')
  expect(write.body).not.toHaveProperty('note')
})

test('canonical role change is idempotent, scope remains server-derived and readback confirms membership', async ({ page }) => {
  const server = await mockMemberServer(page)
  await openCanonicalMembers(page)
  await page.getByRole('button', { name: 'Alice Chen 更多操作', exact: true }).click()
  await page.getByRole('button', { name: '角色与数据权限变更', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '角色变更与权限调整' })
  await expect(dialog.getByLabel('目标数据范围')).toHaveValue('保存后由角色权限服务派生')
  await dialog.getByLabel('经营查看者').check()
  await dialog.getByRole('button', { name: '保存变更', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('服务端确认')
  await expect(page.locator('[data-member-id="user-001"]')).toContainText('经营查看者')
  const write = server.getWrites().find((item) => item.path.endsWith('/roles/role-viewer/members'))!
  expect(write.method).toBe('POST')
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-member-role-assign-/)
  expect(write.headers['x-csrf-token']).toBe('csrf-real-member')
})

test('401 and 403 remain explicit and never replace API members with preview data', async ({ page }) => {
  await mockMemberServer(page, { unauthenticated: true })
  await openCanonicalMembers(page)
  await expect(page.getByText('unauthenticated', { exact: true })).toBeVisible()
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockMemberServer(page, { listStatus: 403 })
  await page.reload()
  await expect(page.getByText('list denied', { exact: true })).toBeVisible()
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)
})

test('canonical profile 409 preserves draft and reuses the same idempotency key', async ({ page }) => {
  const server = await mockMemberServer(page, { mutationStatus: 409 })
  await openCanonicalMembers(page)
  await page.getByRole('button', { name: '编辑 Alice Chen', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '修改成员信息' })
  await dialog.getByLabel('姓名').fill('conflicting change')
  const save = dialog.getByRole('button', { name: '保存变更', exact: true })
  await save.click()
  await expect(dialog.getByRole('alert')).toContainText('成员状态或请求版本已发生变化')
  await expect(dialog.getByLabel('姓名')).toHaveValue('conflicting change')
  await save.click()
  await expect(dialog.getByRole('alert')).toContainText('成员状态或请求版本已发生变化')
  const writes = server.getWrites().filter((item) => item.path.endsWith('/profile'))
  expect(writes).toHaveLength(2)
  expect(writes[0]?.headers['idempotency-key']).toBe(writes[1]?.headers['idempotency-key'])
  await expect(page.getByText(/变更已由服务端确认并完成权威回读/)).toHaveCount(0)
})

test('successful member write without GET readback is never presented as canonical success', async ({ page }) => {
  await mockMemberServer(page, { readbackStatus: 500 })
  await openCanonicalMembers(page)
  await page.getByRole('button', { name: '编辑 Alice Chen', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '修改成员信息' })
  await dialog.getByLabel('姓名').fill('unconfirmed change')
  await dialog.getByRole('button', { name: '保存变更', exact: true }).click()
  await expect(dialog.getByRole('alert')).toContainText('readback failed')
  await expect(page.getByText(/变更已由服务端确认并完成权威回读/)).toHaveCount(0)
})
