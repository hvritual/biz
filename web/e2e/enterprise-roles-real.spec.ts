import { selectUiOption } from './ui.helpers'
import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'

test.skip(!process.env.ENTERPRISE_ROLE_REAL_E2E, 'runs only against the VITE_DATA_MODE=api build')

type Grant = { permission: string; scope: string }
type Role = {
  id: string
  name: string
  description: string
  roleCode: string
  systemRole: boolean
  memberCount: number
  status: string
  version: number
  permissions: Grant[]
}
type RoleSummary = { roleId: string; roleName: string; roleStatus: string }
type Member = { userId: string; email: string; status: string; version: number; name: string; phone: string; employeeId: string; position: string; departmentId: string; roles: RoleSummary[]; derivedDataScope: string }
type Write = { path: string; method: string; headers: Record<string, string>; body: unknown }
type Options = {
  unauthenticated?: boolean
  listStatus?: number
  mutationStatus?: number
  readbackStatus?: number
  ownerConflict?: boolean
  authorizationStatus?: number
  shrinkCatalogAfterFirstRead?: boolean
}

const active = 'TENANT_ROLE_STATUS_ACTIVE'
const disabled = 'TENANT_ROLE_STATUS_DISABLED'
const activeMember = 'TENANT_MEMBER_STATUS_ACTIVE'

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}
function summary(role: Role): RoleSummary { return { roleId: role.id, roleName: role.name, roleStatus: role.status } }

async function mockRoleServer(page: Page, options: Options = {}) {
  let roles: Role[] = [
    { id: 'tenant-001:owner', name: 'owner', description: '', roleCode: 'tenant_owner', systemRole: true, memberCount: 1, status: active, version: 1, permissions: [
      { permission: 'tenant.member.manage', scope: 'DATA_SCOPE_ALL' },
      { permission: 'tenant.member.read', scope: 'DATA_SCOPE_ALL' },
      { permission: 'tenant.role.manage', scope: 'DATA_SCOPE_ALL' },
      { permission: 'tenant.role.read', scope: 'DATA_SCOPE_ALL' },
    ] },
    { id: 'tenant-001:admin', name: 'tenant_admin', description: '', roleCode: 'tenant_admin', systemRole: true, memberCount: 0, status: active, version: 1, permissions: [] },
    { id: 'role-ops', name: '运营负责人', description: '负责日常运营', roleCode: '', systemRole: false, memberCount: 1, status: active, version: 2, permissions: [{ permission: 'tenant.member.read', scope: 'DATA_SCOPE_SITES' }] },
    { id: 'role-disabled', name: '历史查看者', description: '历史只读角色', roleCode: '', systemRole: false, memberCount: 0, status: disabled, version: 4, permissions: [{ permission: 'tenant.role.read', scope: 'DATA_SCOPE_SELF' }] },
  ]
  let members: Member[] = [
    { userId: 'user-001', email: 'owner@coffeelink.test', status: activeMember, version: 3, name: 'Alice Chen', phone: '', employeeId: 'EMP-1001', position: '企业负责人', departmentId: 'dept-owner', roles: [summary(roles[0]!)], derivedDataScope: 'all' },
    { userId: 'user-002', email: 'ops@coffeelink.test', status: activeMember, version: 2, name: 'Bob Lin', phone: '', employeeId: 'EMP-1002', position: '运营负责人', departmentId: 'dept-ops', roles: [summary(roles[1]!)], derivedDataScope: 'sites' },
  ]
  const writes: Write[] = []
  let actionCatalogReads = 0
  const record = (route: Route) => {
    const request = route.request()
    writes.push({ path: new URL(request.url()).pathname.replace(/^\/api(?=\/)/, ''), method: request.method(), headers: request.headers(), body: request.postDataJSON() })
  }
  const replaceRole = (next: Role) => {
    roles = roles.map((role) => role.id === next.id ? next : role)
    members = members.map((member) => ({ ...member, roles: member.roles.map((item) => item.roleId === next.id ? summary(next) : item) }))
    return next
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
    return json(route, 200, { authenticated: true, actor_kind: 'tenant', user_id: 'user-001', active_tenant_id: 'tenant-001', csrf_token: 'csrf-real-role', tenants: [{ id: 'tenant-001', name: 'CoffeeLink 测试租户' }] })
  })
  await page.route(/\/(?:api\/)?auth\/authorization(?:\?.*)?$/, async (route) => {
    if (options.unauthenticated) return json(route, 401, { message: 'unauthenticated' })
    if (options.authorizationStatus) return json(route, options.authorizationStatus, { message: 'authorization denied' })
    const buttonCodes = [
      'tenant.role.list',
      'tenant.role.create',
      'tenant.role.update',
      'tenant.role.set_permissions',
      'tenant.role.enable',
      'tenant.role.disable',
      'tenant.role.delete',
    ]
    return json(route, 200, {
      authenticated: true,
      actor_kind: 'tenant',
      user_id: 'user-001',
      tenant_id: 'tenant-001',
      tenant_name: 'CoffeeLink 测试租户',
      timezone: 'Asia/Shanghai',
      roles: ['owner'],
      grants: [{ permission: 'tenant.role.manage', role_id: 'tenant-001:owner', role_name: 'owner', scope: 'all' }],
      data_policies: [],
      site_ids: [],
      permission_version: 'sha256:role-e2e',
      modules: [{ code: 'access-management', allowed: true, reason: 'allowed', actions: buttonCodes }],
      actions: buttonCodes.map((code) => ({ code, permissions: [], permission_mode: 'all' })),
      button_codes: buttonCodes,
    })
  })
  await page.route(/\/(?:api\/)?auth\/action-catalog(?:\?.*)?$/, async (route) => {
    actionCatalogReads += 1
    const permissions = [
      { permission: 'tenant.member.read', groups: ['access/tenant_member_lifecycle'], actions: ['tenant.member.get', 'tenant.member.list'] },
      { permission: 'tenant.member.manage', groups: ['access/tenant_member_lifecycle'], actions: ['tenant.member.invite', 'tenant.member.profile.update'] },
      { permission: 'tenant.role.read', groups: ['access/tenant_role_permission'], actions: ['tenant.role.get', 'tenant.role.list'] },
      { permission: 'tenant.role.manage', groups: ['access/tenant_role_permission'], actions: ['tenant.role.create', 'tenant.role.set_permissions'] },
    ].filter((item) => !(options.shrinkCatalogAfterFirstRead && actionCatalogReads > 1 && item.permission === 'tenant.member.read'))
    return json(route, 200, {
      schema_version: 'v1',
      actions: [],
      permissions,
      entitlement: { version: actionCatalogReads, source_version: 1, catalog_revision: 1 },
    })
  })
  await page.route(/\/(?:api\/)?v1\/tenant\/members(?:\?.*)?$/, async (route) => json(route, 200, { members }))
  await page.route(/\/(?:api\/)?v1\/tenant\/roles(?:\/.*)?(?:\?.*)?$/, async (route) => {
    const request = route.request()
    const parts = new URL(request.url()).pathname.split('/').filter(Boolean)
    const index = parts.indexOf('roles')
    const roleId = index >= 0 && parts[index + 1] ? decodeURIComponent(parts[index + 1]!) : ''
    const action = index >= 0 ? parts[index + 2] : undefined
    const role = roles.find((item) => item.id === roleId)

    if (request.method() === 'GET') {
      if (!roleId) {
        if (options.listStatus) return json(route, options.listStatus, { message: 'roles denied' })
        const url = new URL(request.url())
        const query = (url.searchParams.get('query') ?? '').trim().toLowerCase()
        const status = url.searchParams.get('status') ?? ''
        const filtered = roles.filter((item) =>
          (!query || (item.name + item.description).toLowerCase().includes(query)) &&
          (!status || item.status === status),
        )
        return json(route, 200, { roles: filtered })
      }
      if (options.readbackStatus && writes.length) return json(route, options.readbackStatus, { message: 'role readback failed' })
      return role ? json(route, 200, role) : json(route, 404, { message: 'role not found' })
    }

    record(route)
    if (options.mutationStatus) return json(route, options.mutationStatus, { message: 'role conflict' })
    if (!roleId && request.method() === 'POST') {
      const body = request.postDataJSON() as { name: string }
      const created: Role = {
        id: 'role-new',
        name: body.name,
        description: (body as { description?: string }).description ?? '',
        roleCode: '',
        systemRole: false,
        memberCount: 0,
        status: active,
        version: 1,
        permissions: [],
      }
      roles = [...roles, created]
      return json(route, 200, created)
    }
    if (!role) return json(route, 404, { message: 'role not found' })
    if (request.method() === 'PATCH') {
      const body = request.postDataJSON() as { name: string; description?: string }
      if (role.systemRole) return json(route, 409, { message: 'system role protected' })
      return json(route, 200, replaceRole({ ...role, name: body.name, description: body.description ?? '', version: role.version + 1 }))
    }
    if (request.method() === 'PUT' && action === 'permissions') {
      const body = request.postDataJSON() as { permissions: Grant[] }
      return json(route, 200, replaceRole({ ...role, permissions: body.permissions, version: role.version + 1 }))
    }
    if (request.method() === 'POST' && (action === 'enable' || action === 'disable')) {
      if (role.systemRole) return json(route, 409, { message: 'system role protected' })
      if (action === 'disable' && role.memberCount > 0) return json(route, 409, { message: 'role still has members' })
      return json(route, 200, replaceRole({ ...role, status: action === 'enable' ? active : disabled, version: role.version + 1 }))
    }
    if (request.method() === 'DELETE') {
      if (role.systemRole) return json(route, 409, { message: 'system role protected' })
      if (role.memberCount > 0) return json(route, 409, { message: 'role still has members' })
      roles = roles.filter((item) => item.id !== role.id)
      return json(route, 200, role)
    }
    if (request.method() === 'POST' && parts.at(-1) === 'revoke') {
      const userId = decodeURIComponent(parts[index + 3] ?? '')
      if (options.ownerConflict && role.name === 'owner') return json(route, 409, { message: 'last owner protected' })
      const before = members.filter((member) => member.roles.some((item) => item.roleId === role.id)).length
      members = members.map((member) => member.userId === userId ? { ...member, roles: member.roles.filter((item) => item.roleId !== role.id) } : member)
      const after = members.filter((member) => member.roles.some((item) => item.roleId === role.id)).length
      return json(route, 200, replaceRole({ ...role, memberCount: role.memberCount - (before - after) }))
    }
    if (request.method() === 'POST' && action === 'members') {
      const userId = (request.postDataJSON() as { userId: string }).userId
      const before = members.filter((member) => member.roles.some((item) => item.roleId === role.id)).length
      members = members.map((member) => member.userId === userId && !member.roles.some((item) => item.roleId === role.id) ? { ...member, roles: [...member.roles, summary(role)] } : member)
      const after = members.filter((member) => member.roles.some((item) => item.roleId === role.id)).length
      return json(route, 200, replaceRole({ ...role, memberCount: role.memberCount + (after - before) }))
    }
    return json(route, 400, { message: 'unsupported mutation' })
  })
  return { getWrites: () => writes, getMembers: () => members, getActionCatalogReads: () => actionCatalogReads }
}

async function openRealRoles(page: Page) {
  await page.goto('/#/enterprise/roles')
  await expect(page.locator('[data-enterprise-page="roles"]')).toBeVisible()
}
function rowFor(page: Page, name: string) { return page.locator('tbody tr').filter({ hasText: name }) }

test('real role page renders authoritative grants and member counts across CoffeeLink viewports', async ({ page }) => {
  await mockRoleServer(page)
  mkdirSync('screenshots', { recursive: true })
  for (const viewport of [{ width: 1366, height: 768 }, { width: 1440, height: 900 }, { width: 1536, height: 1024 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport)
    await openRealRoles(page)
    await expect(page.getByText('企业所有者', { exact: true })).toBeVisible()
    await expect(page.getByText('企业管理员', { exact: true })).toBeVisible()
    await expect(page.getByText('运营负责人', { exact: true }).first()).toBeVisible()
    await expect(rowFor(page, '运营负责人')).toContainText('1 人')
    await expect(rowFor(page, '运营负责人')).toContainText('指定数据')
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
  await dialog.getByLabel('角色名称').fill('华东运营')
  await dialog.getByLabel('角色说明').fill('负责华东区域日常运营')
  await dialog.getByLabel('企业成员 查看成员', { exact: true }).check()
  await selectUiOption(dialog.getByLabel('数据范围'), 'custom')
  await dialog.getByRole('button', { name: '保存角色' }).click()
  await expect(page.getByRole('status')).toContainText('角色配置已保存并更新。')
  await expect(page.getByText('华东运营', { exact: true })).toBeVisible()
  const create = server.getWrites().find((item) => item.path === '/v1/tenant/roles')!
  const permissions = server.getWrites().find((item) => item.path.endsWith('/role-new/permissions'))!
  expect(create.headers['idempotency-key']).toMatch(/^enterprise-role-create-/)
  expect(permissions.headers['idempotency-key']).toMatch(/^enterprise-role-permissions-/)
  expect(create.headers['idempotency-key']).not.toBe(permissions.headers['idempotency-key'])
  expect(create.headers['x-csrf-token']).toBe('csrf-real-role')
  expect(create.body).toMatchObject({ name: '华东运营', description: '负责华东区域日常运营' })
  expect(permissions.body).toMatchObject({ roleId: 'role-new', version: 1 })
})

test('role permission tree preserves mixed full and none parent states and authoritative reopen', async ({ page }) => {
  await mockRoleServer(page)
  await openRealRoles(page)
  await rowFor(page, '运营负责人').getByRole('button', { name: '编辑' }).click()
  let dialog = page.getByRole('dialog', { name: '编辑角色权限' })
  const memberGroup = dialog.locator('[data-role-permission-group="member"]')
  const parent = memberGroup.getByLabel('企业成员 全选')

  await expect(parent).toHaveAttribute('aria-checked', 'mixed')
  await parent.check()
  await expect(parent).toBeChecked()
  await expect(memberGroup.locator('[data-role-permission-leaf="tenant.member.read"] input')).toBeChecked()
  await expect(memberGroup.locator('[data-role-permission-leaf="tenant.member.manage"] input')).toBeChecked()

  await dialog.getByRole('button', { name: '保存角色' }).click()
  await expect(page.getByRole('status')).toContainText('角色配置已保存并更新。')

  await rowFor(page, '运营负责人').getByRole('button', { name: '编辑' }).click()
  dialog = page.getByRole('dialog', { name: '编辑角色权限' })
  const reopenedGroup = dialog.locator('[data-role-permission-group="member"]')
  await expect(reopenedGroup.getByLabel('企业成员 全选')).toBeChecked()
  await reopenedGroup.locator('[data-role-permission-leaf="tenant.member.manage"] input').uncheck()
  await expect(reopenedGroup.getByLabel('企业成员 全选')).toHaveAttribute('aria-checked', 'mixed')
  await reopenedGroup.getByLabel('企业成员 全选').uncheck()
  await expect(reopenedGroup.getByLabel('企业成员 全选')).not.toBeChecked()
  await expect(reopenedGroup.locator('input[type="checkbox"]:checked')).toHaveCount(0)

  await dialog.getByRole('button', { name: '角色权限' }).focus()
  await dialog.getByRole('button', { name: '企业成员' }).click()
  await expect(reopenedGroup.getByLabel('企业成员 全选')).toBeFocused()
})

test('role save revalidates current permission catalog and rejects retired selection before write', async ({ page }) => {
  const server = await mockRoleServer(page, { shrinkCatalogAfterFirstRead: true })
  await openRealRoles(page)
  await rowFor(page, '运营负责人').getByRole('button', { name: '编辑' }).click()
  const dialog = page.getByRole('dialog', { name: '编辑角色权限' })
  await expect(dialog.locator('[data-role-permission-leaf="tenant.member.read"] input')).toBeChecked()
  await dialog.getByRole('button', { name: '保存角色' }).click()

  await expect(dialog.getByRole('alert')).toContainText('可配置权限已发生变化')
  await expect(dialog.locator('[data-role-permission-leaf="tenant.member.read"]')).toHaveCount(0)
  expect(server.getActionCatalogReads()).toBeGreaterThanOrEqual(2)
  expect(server.getWrites().filter((item) => item.path.endsWith('/role-ops/permissions'))).toHaveLength(0)
  await expect(page.getByText('角色配置已保存并更新。', { exact: true })).toHaveCount(0)
})

test('canonical roles page exposes authoritative member counts while membership changes stay on Members flow', async ({ page }) => {
  const server = await mockRoleServer(page)
  await openRealRoles(page)
  const row = rowFor(page, '运营负责人')
  await expect(row).toContainText('1 人')
  await expect(row.getByRole('button', { name: '编辑' })).toBeVisible()
  await expect(row.getByRole('button', { name: '管理' })).toHaveCount(0)
  expect(server.getWrites()).toHaveLength(0)
})

test('401 redirects to trusted login and 403 blocks roles before protected data loads', async ({ page }) => {
  await mockRoleServer(page, { unauthenticated: true })
  await page.goto('/#/enterprise/roles')
  await expect(page).toHaveURL(/\/api\/auth\/login\?return_to=/)
  await expect(page.locator('[data-enterprise-page="roles"]')).toHaveCount(0)
  await expect(page.getByText('超级管理员', { exact: true })).toHaveCount(0)

  await page.unrouteAll({ behavior: 'ignoreErrors' })
  await mockRoleServer(page, { authorizationStatus: 403 })
  await page.goto('/#/enterprise/roles')
  await expect(page.locator('[data-authorization-state]')).toBeVisible()
  await expect(page.getByRole('heading', { name: '没有访问权限' })).toBeVisible()
  await expect(page.locator('[data-enterprise-page="roles"]')).toHaveCount(0)
  await expect(page.getByText('超级管理员', { exact: true })).toHaveCount(0)
})

test('role 409 preserves the same idempotency key and editor draft for retry', async ({ page }) => {
  const server = await mockRoleServer(page, { mutationStatus: 409 })
  await openRealRoles(page)
  await rowFor(page, '运营负责人').getByRole('button', { name: '编辑' }).click()
  const dialog = page.getByRole('dialog', { name: '编辑角色权限' })
  const save = dialog.getByRole('button', { name: '保存角色' })
  await dialog.getByLabel('角色名称').fill('运营负责人-冲突草稿')
  await save.click()
  await expect(dialog.getByRole('alert')).toContainText('角色版本或所有者保护规则已发生冲突')
  await expect(dialog.getByLabel('角色名称')).toHaveValue('运营负责人-冲突草稿')
  await expect(save).toBeEnabled()
  await save.click()
  await expect(dialog.getByRole('alert')).toContainText('角色版本或所有者保护规则已发生冲突')
  const writes = server.getWrites().filter((item) => item.method === 'PATCH')
  expect(writes).toHaveLength(2)
  expect(writes[0]?.headers['idempotency-key']).toBe(writes[1]?.headers['idempotency-key'])
  await expect(page.getByRole('status')).toHaveCount(0)
})

test('protected owner role is read-only in the canonical role editor', async ({ page }) => {
  const server = await mockRoleServer(page)
  await openRealRoles(page)
  await rowFor(page, '企业所有者').getByRole('button', { name: '查看' }).click()
  const dialog = page.getByRole('dialog', { name: '内置角色详情' })
  await expect(dialog.getByText(/内置角色只读/)).toBeVisible()
  await expect(dialog.getByRole('button', { name: '保存角色' })).toHaveCount(0)
  expect(server.getWrites()).toHaveLength(0)
})

test('a successful role write without readback is not presented as confirmed success', async ({ page }) => {
  await mockRoleServer(page, { readbackStatus: 500 })
  await openRealRoles(page)
  await rowFor(page, '运营负责人').getByRole('button', { name: '编辑' }).click()
  const dialog = page.getByRole('dialog', { name: '编辑角色权限' })
  await dialog.getByLabel('角色名称').fill('未确认角色名')
  await dialog.getByRole('button', { name: '保存角色' }).click()
  await expect(dialog.getByRole('alert')).toContainText('角色权限暂不可用，请稍后重试。')
  await expect(page.getByText('角色配置已保存并更新。', { exact: true })).toHaveCount(0)
})

test('server-side role query and reset preserve canonical role facts', async ({ page }) => {
  await mockRoleServer(page)
  await openRealRoles(page)
  const query = page.getByPlaceholder('搜索角色名称、说明…')
  await query.fill('历史')
  await selectUiOption(page.getByLabel('角色状态'), 'disabled')
  await page.getByRole('button', { name: '查询', exact: true }).click()
  await expect(page.getByText('历史查看者', { exact: true })).toBeVisible()
  await expect(page.getByText('运营负责人', { exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: '重置', exact: true }).click()
  await expect(page.getByText('运营负责人', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('企业管理员', { exact: true })).toBeVisible()
})

test('system roles are read-only and member-bound custom roles cannot be deleted', async ({ page }) => {
  const server = await mockRoleServer(page)
  await openRealRoles(page)
  const ownerRow = rowFor(page, '企业所有者')
  const adminRow = rowFor(page, '企业管理员')
  await expect(ownerRow.getByRole('button', { name: /^删除 / })).toHaveCount(0)
  await expect(adminRow.getByRole('button', { name: /^删除 / })).toHaveCount(0)

  const opsRow = rowFor(page, '运营负责人')
  const deleteOps = opsRow.getByRole('button', { name: '删除 运营负责人' })
  await expect(deleteOps).toBeDisabled()
  expect(server.getWrites().filter((item) => item.method === 'DELETE')).toHaveLength(0)
})

test('unbound custom role deletion requires confirmation and server receipt', async ({ page }) => {
  const server = await mockRoleServer(page)
  await openRealRoles(page)
  const row = rowFor(page, '历史查看者')
  await row.getByRole('button', { name: '删除 历史查看者' }).click()
  const dialog = page.getByRole('dialog', { name: '删除角色' })
  await expect(dialog).toContainText('历史查看者')
  await dialog.getByRole('button', { name: '确认删除' }).click()
  await expect(page.getByText('历史查看者', { exact: true })).toHaveCount(0)
  const writes = server.getWrites().filter((item) => item.method === 'DELETE')
  expect(writes).toHaveLength(1)
  expect(writes[0]?.headers['idempotency-key']).toMatch(/^enterprise-role-delete-/)
})

