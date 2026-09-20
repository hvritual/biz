import { expect, test, type Page, type Route } from '@playwright/test'
import { selectUiOption } from './ui.helpers'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_MEMBER_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type RemoteRoleSummary = { roleId: string; roleName: string; roleStatus: string }
type RemoteMember = {
  userId: string
  username: string
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
  authorizationStatus?: number
  unauthenticated?: boolean
  memberCount?: number
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
      username: 'alice.owner',
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
      username: 'new.member',
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
  const requestedCount = Math.max(members.length, options.memberCount ?? members.length)
  for (let index = members.length; index < requestedCount; index += 1) {
    members.push({
      userId: `user-extra-${String(index + 1).padStart(2, '0')}`,
      username: `member.${String(index + 1).padStart(2, '0')}`,
      email: `member-${index + 1}@coffeelink.test`,
      status: index % 2 === 0 ? 'TENANT_MEMBER_STATUS_ACTIVE' : 'TENANT_MEMBER_STATUS_INVITED',
      version: 1,
      name: `Member ${String(index + 1).padStart(2, '0')}`,
      phone: '',
      employeeId: `EMP-${2000 + index}`,
      position: '',
      departmentId: index % 2 === 0 ? 'dept-success' : 'dept-rental',
      roles: [roleSummary(index % 2 === 0 ? roleCatalog[0]! : roleCatalog[1]!)],
      derivedDataScope: index % 2 === 0 ? 'sites' : 'self',
    })
  }
  let removedMembers: RemoteMember[] = [{
    userId: 'user-removed-001',
    username: 'former.member',
    email: 'former@coffeelink.test',
    status: 'TENANT_MEMBER_STATUS_REMOVED',
    version: 5,
    name: 'Former Member',
    phone: '',
    employeeId: 'EMP-OLD',
    position: 'Former Operator',
    departmentId: 'dept-success',
    roles: [],
    derivedDataScope: 'none',
  }]
  const writes: WriteRecord[] = []
  const listReads: string[] = []

  const recordWrite = (route: Route) => {
    const request = route.request()
    writes.push({
      path: new URL(request.url()).pathname.replace(/^\/api(?=\/)/, ''),
      method: request.method(),
      headers: request.headers(),
      body: request.postDataJSON(),
    })
  }

  await page.route(/\/(?:api\/)?(?:auth|v1)\//, async (route) => {
    const request = route.request()
    throw new Error(`Unhandled API request: ${request.method()} ${new URL(request.url()).pathname}`)
  })

  await page.route(/\/(?:api\/)?auth\/login(?:\?.*)?$/, async (route) => {
    await route.fulfill({ status: 200, contentType: 'text/html', body: '<!doctype html><title>Login</title>' })
  })

  await page.route(/\/(?:api\/)?auth\/session(?:\?.*)?$/, async (route) => {
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

  await page.route(/\/(?:api\/)?auth\/authorization(?:\?.*)?$/, async (route) => {
    if (options.unauthenticated) return json(route, 401, { message: 'unauthenticated' })
    if (options.authorizationStatus) return json(route, options.authorizationStatus, { message: 'authorization denied' })
    const buttonCodes = [
      'tenant.member.list',
      'tenant.member.create',
      'tenant.member.update',
      'tenant.member.invite',
      'tenant.member.profile.update',
      'tenant.member.activate',
      'tenant.member.suspend',
      'tenant.member.remove',
      'tenant.member.list_removed',
      'tenant.member.restore',
      'tenant.role.list',
      'tenant.department.list',
      'tenant.role.assign_member',
      'tenant.role.revoke_member',
    ]
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-001',
      tenant_id: 'tenant-001',
      tenant_name: 'CoffeeLink 测试租户',
      timezone: 'Asia/Shanghai',
      roles: ['operator'],
      grants: [{ permission: 'tenant.member.manage', role_id: 'role-ops', role_name: 'operator', scope: 'all' }],
      data_policies: [],
      site_ids: [],
      permission_version: 'sha256:member-e2e',
      modules: [{ code: 'access-management', allowed: true, reason: 'allowed', actions: buttonCodes }],
      actions: buttonCodes.map((code) => ({ code, permissions: [], permission_mode: 'all' })),
      button_codes: buttonCodes,
    })
  })

  await page.route(/\/(?:api\/)?v1\/tenant\/roles(?:\?.*)?$/, async (route) => {
    return json(route, 200, { roles: roleCatalog })
  })

  await page.route(/\/(?:api\/)?v1\/tenant\/departments(?:\?.*)?$/, async (route) => {
    return json(route, 200, { departments: departmentCatalog })
  })

  await page.route(/\/(?:api\/)?v1\/tenant\/roles\/[^/]+\/members(?:\/[^/]+\/revoke)?$/, async (route) => {
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

  await page.route(/\/(?:api\/)?v1\/tenant\/members\/removed(?:\?.*)?$/, async (route) => {
    if (route.request().method() !== 'GET') return json(route, 405, { message: 'method not allowed' })
    return json(route, 200, { members: removedMembers, total: removedMembers.length })
  })

  await page.route(/\/(?:api\/)?v1\/tenant\/members\/create$/, async (route) => {
    recordWrite(route)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'mutation conflict' })
    const body = route.request().postDataJSON() as {
      username: string
      email: string
      phone: string
      name: string
      employeeId: string
      position: string
      departmentId: string
      roleIds: string[]
      activationMode: string
    }
    const roles = body.roleIds
      .map((roleId) => roleCatalog.find((role) => role.id === roleId))
      .filter((role): role is RemoteRole => Boolean(role))
      .map(roleSummary)
    const created: RemoteMember = {
      userId: 'user-003',
      username: body.username,
      email: body.email,
      status: 'TENANT_MEMBER_STATUS_INVITED',
      version: 1,
      name: body.name,
      phone: body.phone,
      employeeId: body.employeeId,
      position: body.position,
      departmentId: body.departmentId,
      roles,
      derivedDataScope: 'none',
    }
    created.derivedDataScope = deriveScope(created)
    members = [...members, created]
    return json(route, 200, {
      member: created,
      activationMode: body.activationMode,
      notificationEventId: 'activation-event-003',
      deliveryState: 'PENDING',
      maskedDestination: body.email ? 'i***@coffeelink.test' : '+886****0003',
    })
  })

  await page.route(/\/(?:api\/)?v1\/tenant\/members(?:\?.*)?$/, async (route) => {
    const request = route.request()
    if (request.method() === 'GET') {
      if (options.listStatus) return json(route, options.listStatus, { message: 'list denied' })
      const url = new URL(request.url())
      listReads.push(url.toString())
      const keyword = (url.searchParams.get('query') ?? '').trim().toLowerCase()
      const roleId = url.searchParams.get('role_id') ?? ''
      const departmentId = url.searchParams.get('department_id') ?? ''
      const status = url.searchParams.get('status') ?? ''
      const pageNumber = Math.max(1, Number(url.searchParams.get('page') ?? '1'))
      const pageSize = Math.max(1, Number(url.searchParams.get('page_size') ?? '20'))
      const filtered = members.filter((member) => {
        if (member.status === 'TENANT_MEMBER_STATUS_REMOVED') return false
        if (keyword && !`${member.name} ${member.email} ${member.phone} ${member.employeeId}`.toLowerCase().includes(keyword)) return false
        if (roleId && !member.roles.some((role) => role.roleId === roleId)) return false
        if (departmentId && member.departmentId !== departmentId) return false
        if (status && member.status !== status) return false
        return true
      })
      const offset = (pageNumber - 1) * pageSize
      return json(route, 200, { members: filtered.slice(offset, offset + pageSize), total: filtered.length })
    }
    recordWrite(route)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'mutation conflict' })
    const body = request.postDataJSON() as { email: string }
    const created: RemoteMember = {
      userId: 'user-003',
      username: 'legacy.invite',
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

  await page.route(/\/(?:api\/)?v1\/tenant\/members\/[^/]+\/profile$/, async (route) => {
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

  await page.route(/\/(?:api\/)?v1\/tenant\/members\/(?!create(?:\/|$)|removed(?:\/|$))[^/]+(?:\/(?:activate|suspend|remove|restore))?$/, async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const parts = url.pathname.split('/')
    const userId = decodeURIComponent(parts[parts.indexOf('members') + 1] || '')
    const action = parts.at(-1)
    const current = members.find((member) => member.userId === userId) ?? removedMembers.find((member) => member.userId === userId)
    if (!current) return json(route, 404, { message: 'not found' })

    if (request.method() === 'GET') {
      if (options.readbackStatus) return json(route, options.readbackStatus, { message: 'readback failed' })
      return json(route, 200, current)
    }

    recordWrite(route)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'mutation conflict' })
    if (request.method() === 'PATCH') {
      const body = request.postDataJSON() as Partial<RemoteMember> & { roleIds?: string[] }
      const roles = (body.roleIds ?? current.roles.map((role) => role.roleId))
        .map((roleId) => roleCatalog.find((role) => role.id === roleId))
        .filter((role): role is RemoteRole => Boolean(role))
        .map(roleSummary)
      const updated: RemoteMember = {
        ...current,
        email: String(body.email ?? current.email).trim(),
        phone: String(body.phone ?? current.phone).trim(),
        name: String(body.name ?? current.name).trim(),
        employeeId: String(body.employeeId ?? current.employeeId).trim(),
        position: String(body.position ?? current.position).trim(),
        departmentId: String(body.departmentId ?? current.departmentId).trim(),
        roles,
        version: current.version + 1,
      }
      updated.derivedDataScope = deriveScope(updated)
      members = members.map((member) => member.userId === userId ? updated : member)
      return json(route, 200, updated)
    }
    const nextStatus =
      action === 'activate' || action === 'restore'
        ? 'TENANT_MEMBER_STATUS_ACTIVE'
        : action === 'suspend'
          ? 'TENANT_MEMBER_STATUS_SUSPENDED'
          : 'TENANT_MEMBER_STATUS_REMOVED'
    const updated = { ...current, status: nextStatus, version: current.version + 1 }
    if (action === 'restore') {
      removedMembers = removedMembers.filter((member) => member.userId !== userId)
      members = [...members.filter((member) => member.userId !== userId), updated]
    } else if (action === 'remove') {
      members = members.filter((member) => member.userId !== userId)
      removedMembers = [...removedMembers.filter((member) => member.userId !== userId), updated]
    } else {
      members = members.map((member) => (member.userId === userId ? updated : member))
    }
    return json(route, 200, updated)
  })

  return {
    getMembers: () => members,
    getWrites: () => writes,
    getListReads: () => listReads,
    getRemovedMembers: () => removedMembers,
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

test('canonical members use server-side filters, pagination and authoritative total', async ({ page }) => {
  const server = await mockMemberServer(page, { memberCount: 12 })
  await openCanonicalMembers(page)
  await expect(page.getByText(/12/).first()).toBeVisible()
  expect(server.getListReads().at(-1)).toContain('page=1')
  expect(server.getListReads().at(-1)).toContain('page_size=10')

  await page.getByRole('button', { name: '下一页' }).click()
  await expect.poll(() => server.getListReads().at(-1) ?? '').toContain('page=2')
  await expect(page.locator('[data-member-id="user-extra-11"]')).toBeVisible()

  await page.getByTestId('main-content').getByRole('textbox', { name: '搜索成员', exact: true }).fill('Alice')
  await page.getByRole('button', { name: '查询', exact: true }).click()
  await expect.poll(() => server.getListReads().at(-1) ?? '').toContain('query=Alice')
  await expect(page.locator('[data-member-id="user-001"]')).toBeVisible()
  await expect(page.locator('[data-member-id^="user-extra-"]')).toHaveCount(0)
})

test('canonical invite uses one atomic create write with activation choice and authoritative readback', async ({ page }) => {
  const server = await mockMemberServer(page)
  await openCanonicalMembers(page)
  await page.getByRole('button', { name: '邀请成员', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '邀请成员' })
  await dialog.getByLabel('登录账号').fill('invitee.user')
  await dialog.getByLabel('姓名').fill('Invitee User')
  await dialog.getByLabel('邮箱', { exact: true }).fill('invitee@coffeelink.test')
  await selectUiOption(dialog.getByLabel('激活方式'), 'activation_link')
  await expect(dialog.getByLabel('所属部门')).toContainText('客户成功部')
  await expect(dialog.getByLabel('数据范围')).toHaveValue('保存后由角色权限服务派生')
  await expect(dialog.getByLabel('运营负责人')).toBeChecked()
  await dialog.getByRole('button', { name: '创建邀请', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('服务端确认')
  await expect(page.locator('[data-member-id="user-003"]')).toContainText('Invitee User')
  await expect(page.locator('[data-member-id="user-003"]')).toContainText('@invitee.user')

  const writes = server.getWrites()
  const create = writes.find((item) => item.path === '/v1/tenant/members/create' && item.method === 'POST')!
  expect(create).toBeTruthy()
  expect(writes.filter((item) => item.path.startsWith('/v1/tenant/members'))).toHaveLength(1)
  expect(create.headers['x-csrf-token']).toBe('csrf-real-member')
  expect(create.headers['idempotency-key']).toMatch(/^enterprise-member-create-/)
  expect(create.headers['x-biz-session-context']).toContain('tenant-001')
  expect(create.body).toMatchObject({
    username: 'invitee.user',
    email: 'invitee@coffeelink.test',
    departmentId: 'dept-success',
    roleIds: ['role-ops'],
    activationMode: 'TENANT_MEMBER_ACTIVATION_MODE_ACTIVATION_LINK',
  })
  expect(server.getMembers().find((member) => member.userId === 'user-003')).toMatchObject({
    username: 'invitee.user',
    name: 'Invitee User',
    departmentId: 'dept-success',
    status: 'TENANT_MEMBER_STATUS_INVITED',
  })
})

test('canonical member edit is one atomic update and preserves immutable username', async ({ page }) => {
  const server = await mockMemberServer(page)
  await openCanonicalMembers(page)
  await page.getByRole('button', { name: '编辑 Alice Chen', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '修改成员信息' })
  await expect(dialog.getByLabel('登录账号')).toHaveValue('alice.owner')
  await expect(dialog.getByLabel('登录账号')).toHaveAttribute('readonly', '')
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

  const writes = server.getWrites().filter((item) => item.path === '/v1/tenant/members/user-001')
  expect(writes).toHaveLength(1)
  const write = writes[0]!
  expect(write.method).toBe('PATCH')
  expect(write.headers['x-csrf-token']).toBe('csrf-real-member')
  expect(write.headers['idempotency-key']).toMatch(/^enterprise-member-update-/)
  expect(write.headers['x-biz-session-context']).toContain('tenant-001')
  expect(write.body).toMatchObject({
    userId: 'user-001',
    version: 3,
    departmentId: 'dept-rental',
    employeeId: 'EMP-2009',
    email: 'owner@coffeelink.test',
    roleIds: ['role-ops'],
  })
  expect(write.body).not.toHaveProperty('username')
  expect(write.body).not.toHaveProperty('scope')
  expect(write.body).not.toHaveProperty('joinedAt')
  expect(write.body).not.toHaveProperty('note')
})

test('canonical role change is one atomic member update and scope remains server-derived', async ({ page }) => {
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

  const writes = server.getWrites().filter((item) => item.path === '/v1/tenant/members/user-001')
  expect(writes).toHaveLength(1)
  expect(writes[0]?.method).toBe('PATCH')
  expect(writes[0]?.headers['idempotency-key']).toMatch(/^enterprise-member-update-/)
  expect(writes[0]?.headers['x-csrf-token']).toBe('csrf-real-member')
  expect(writes[0]?.body).toMatchObject({ roleIds: ['role-ops', 'role-viewer'], version: 3 })
})

test('401 redirects to trusted login and 403 blocks members before protected data loads', async ({ page }) => {
  await mockMemberServer(page, { unauthenticated: true })
  await page.goto('/#/enterprise/members')
  await expect(page).toHaveURL(/\/api\/auth\/login\?return_to=/)
  await expect(page.locator('[data-enterprise-page="members"]')).toHaveCount(0)
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockMemberServer(page, { authorizationStatus: 403 })
  await page.goto('/#/enterprise/members')
  await expect(page.locator('[data-authorization-state]')).toBeVisible()
  await expect(page.getByRole('heading', { name: '没有访问权限' })).toBeVisible()
  await expect(page.locator('[data-enterprise-page="members"]')).toHaveCount(0)
  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)
})

test('canonical recycle bin restores removed member with one authoritative write and readback', async ({ page }) => {
  const server = await mockMemberServer(page)
  await openCanonicalMembers(page)
  await page.getByRole('button', { name: '成员回收站', exact: true }).click()
  const recycle = page.getByRole('dialog', { name: '成员回收站' })
  await expect(recycle.getByText('Former Member', { exact: true })).toBeVisible()
  await recycle.getByRole('button', { name: '恢复成员', exact: true }).click()
  const restore = page.getByRole('dialog', { name: '恢复 Former Member' })
  await restore.getByLabel('恢复原因').fill('employment restored')
  await restore.getByRole('button', { name: '确认恢复', exact: true }).click()
  await expect(page.getByRole('status')).toContainText('成员已恢复')
  await expect(page.locator('[data-member-id="user-removed-001"]')).toContainText('Former Member')
  expect(server.getRemovedMembers()).toHaveLength(0)

  const writes = server.getWrites().filter((item) => item.path === '/v1/tenant/members/user-removed-001/restore')
  expect(writes).toHaveLength(1)
  expect(writes[0]?.method).toBe('POST')
  expect(writes[0]?.headers['x-csrf-token']).toBe('csrf-real-member')
  expect(writes[0]?.headers['idempotency-key']).toMatch(/^enterprise-member-restore-/)
  expect(writes[0]?.body).toMatchObject({
    userId: 'user-removed-001',
    version: 5,
    reason: 'employment restored',
  })
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
  const writes = server.getWrites().filter((item) => item.path === '/v1/tenant/members/user-001')
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
