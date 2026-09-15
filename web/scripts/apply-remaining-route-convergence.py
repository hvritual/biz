from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

def read(path: str) -> str:
    return (ROOT / path).read_text(encoding='utf-8')

def write(path: str, text: str) -> None:
    (ROOT / path).write_text(text, encoding='utf-8')

def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise RuntimeError(f'missing migration anchor: {label}')
    return text.replace(old, new, 1)

# 1. Standardize API source failure visibility.
path = 'web/src/features/enterprise/components/EnterpriseSourceBanner.vue'
text = read(path)
text = replace_once(
    text,
    '''        <span v-if="store.sourceError" class="source-error">{{ store.sourceError }}</span>\n        <span v-else-if="store.loading">正在读取服务端数据…</span>\n        <span v-else-if="store.authenticated">当前页面保持统一产品界面，数据与写操作由服务端确认。</span>\n        <span v-else>API 模式不会回退到本地示例数据。</span>''',
    '''        <span v-if="store.sourceError" class="source-error" role="alert">{{ store.sourceError }}</span>\n        <span v-else-if="store.loading">正在读取服务端数据…</span>\n        <span v-else-if="store.authenticated">当前页面保持统一产品界面，数据与写操作由服务端确认。</span>\n        <span v-else-if="store.ready" class="source-error" role="alert">登录会话已失效或尚未登录；API 模式不会回退到本地示例数据。</span>\n        <span v-else>正在确认登录会话…</span>''',
    'source banner alert state',
)
write(path, text)

# 2. Presentation projection for protected owner role.
path = 'web/src/services/enterprise/dataSource.ts'
text = read(path)
text = replace_once(
    text,
    '''    name: value.name,\n    description: '',\n    builtin: value.protectedOwner,''',
    '''    name: value.protectedOwner && value.name === 'owner' ? '企业所有者' : value.name,\n    description: '',\n    builtin: value.protectedOwner,''',
    'owner role display projection',
)
write(path, text)

# 3. Enterprise store: domain-specific errors, stable retry keys, authoritative readback.
path = 'web/src/stores/enterprise.ts'
text = read(path)
text = replace_once(text, "  enableEnterpriseRole,\n  readEnterpriseRoleSession,", "  enableEnterpriseRole,\n  getEnterpriseRole,\n  readEnterpriseRoleSession,", 'role get import')
text = replace_once(text, "  roleRequestId,\n  setEnterpriseRolePermissions,", "  roleRequestId,\n  roleRuntimeError,\n  setEnterpriseRolePermissions,", 'role error import')
text = replace_once(text, "  departmentRequestId,\n  updateEnterpriseDepartment,", "  departmentRequestId,\n  departmentRuntimeError,\n  updateEnterpriseDepartment,", 'department error import')
text = replace_once(
    text,
    '''  let companyMutation: { tenantId: string; signature: string; key: string } | null = null\n  let memberMutation: { tenantId: string; signature: string; keys: Record<string, string> } | null = null''',
    '''  let companyMutation: { tenantId: string; signature: string; key: string } | null = null\n  let memberMutation: { tenantId: string; signature: string; keys: Record<string, string> } | null = null\n  let roleMutation: { tenantId: string; signature: string; keys: Record<string, string> } | null = null\n  let departmentMutation: { tenantId: string; signature: string; keys: Record<string, string> } | null = null''',
    'mutation states',
)
anchor = '''  function applySourceState(state: EnterpriseSourceState, domains = state.loadedDomains, replace = false) {'''
helpers = '''  function roleMutationSignature(role: Role, current?: Role) {\n    return JSON.stringify({\n      id: role.id,\n      name: role.name.trim(),\n      enabled: role.enabled,\n      scope: role.scope,\n      permissions: [...role.permissions].sort(),\n      version: current?.runtimeVersion ?? 0,\n    })\n  }\n\n  function roleMutationKey(signature: string, slot: string, create: () => string) {\n    if (!roleMutation || roleMutation.tenantId !== tenantId.value || roleMutation.signature !== signature) {\n      roleMutation = { tenantId: tenantId.value, signature, keys: {} }\n    }\n    roleMutation.keys[slot] ??= create()\n    return roleMutation.keys[slot]!\n  }\n\n  function departmentMutationSignature(value: Department, current?: Department) {\n    return JSON.stringify({\n      id: value.id,\n      name: value.name.trim(),\n      parentId: value.parentId ?? '',\n      leaderId: value.leaderId,\n      email: value.email ?? '',\n      phone: value.phone ?? '',\n      sort: value.sort ?? 0,\n      enabled: value.enabled,\n      version: current?.runtimeVersion ?? 0,\n    })\n  }\n\n  function departmentMutationKey(signature: string, slot: string, create: () => string) {\n    if (!departmentMutation || departmentMutation.tenantId !== tenantId.value || departmentMutation.signature !== signature) {\n      departmentMutation = { tenantId: tenantId.value, signature, keys: {} }\n    }\n    departmentMutation.keys[slot] ??= create()\n    return departmentMutation.keys[slot]!\n  }\n\n'''
if anchor not in text:
    raise RuntimeError('missing mutation helper insertion anchor')
text = text.replace(anchor, helpers + anchor, 1)
text = replace_once(
    text,
    '''  function errorMessage(error: unknown) {\n    return error instanceof Error ? error.message : '企业数据服务请求失败。'\n  }''',
    '''  function errorMessage(error: unknown, domains: EnterpriseDomain[] = activeDomains) {\n    const primary = domains[0]\n    if (primary === 'members') return memberRuntimeError(error)\n    if (primary === 'roles') return roleRuntimeError(error)\n    if (primary === 'departments') return departmentRuntimeError(error)\n    if (primary === 'company') return tenantProfileRuntimeError(error)\n    return error instanceof Error ? error.message : '企业数据服务请求失败。'\n  }''',
    'domain error mapping',
)
text = replace_once(text, '      sourceError.value = errorMessage(error)', '      sourceError.value = errorMessage(error, activeDomains)', 'refresh domain error mapping')
text = replace_once(
    text,
    '''      companyMutation = null\n      memberMutation = null\n      ready.value = true''',
    '''      companyMutation = null\n      memberMutation = null\n      roleMutation = null\n      departmentMutation = null\n      ready.value = true''',
    'tenant mutation reset',
)
start = text.index('  async function saveRole(role: Role) {')
end = text.index('\n  async function stableDepartmentSession()', start)
role_block = '''  async function saveRole(role: Role) {\n    if (!role.name.trim()) throw new Error('请填写角色名称。')\n    if (roles.value.some((value) => value.id !== role.id && value.name === role.name)) throw new Error('角色名称已存在。')\n    const old = roles.value.find((value) => value.id === role.id)\n    if (old?.builtin) throw new Error('内置角色不可直接修改，请复制为自定义角色。')\n\n    if (previewMode) {\n      const updated = { ...role, updatedAt: timestamp() }\n      snapshot.value.roles = old\n        ? roles.value.map((value) => value.id === role.id ? updated : value)\n        : [...roles.value, updated]\n      audit(\n        '角色权限',\n        old ? '更新角色权限' : '新建角色',\n        role.name,\n        old?.permissions.join(', ') ?? '',\n        role.permissions.join(', '),\n        '界面预览操作',\n        'high',\n      )\n      return\n    }\n\n    const signature = roleMutationSignature(role, old)\n    const key = (slot: string, create: () => string) => roleMutationKey(signature, slot, create)\n    try {\n      const trusted = await stableRoleSession()\n      let serverRole: EnterpriseTenantRole\n      if (!old) {\n        serverRole = await createEnterpriseRole(trusted, role.name, key('create', () => roleRequestId('create'))) as EnterpriseTenantRole\n      } else {\n        serverRole = asServerRole(old)\n        if (old.name !== role.name) {\n          serverRole = await updateEnterpriseRole(trusted, serverRole, role.name, key('update', () => roleRequestId('update')))\n        }\n      }\n      const grants: PermissionGrant[] = role.permissions.map((permission) => ({\n        permission,\n        scope: grantScope(role.scope),\n      }))\n      serverRole = await setEnterpriseRolePermissions(\n        trusted,\n        serverRole,\n        grants,\n        key('permissions', () => roleRequestId('permissions')),\n      )\n      const active = serverRole.status === 'TENANT_ROLE_STATUS_ACTIVE'\n      if (role.enabled !== active) {\n        serverRole = role.enabled\n          ? await enableEnterpriseRole(trusted, serverRole, key('enable', () => roleRequestId('enable')))\n          : await disableEnterpriseRole(trusted, serverRole, key('disable', () => roleRequestId('disable')))\n      }\n      await getEnterpriseRole(trusted, serverRole.id)\n      await refresh(['roles', 'members'])\n      roleMutation = null\n    } catch (error) {\n      throw new Error(roleRuntimeError(error))\n    }\n  }\n'''
text = text[:start] + role_block + text[end:]
start = text.index('  async function saveDepartment(value: Department) {')
end = text.index('\n  async function stableProfileSession()', start)
department_block = '''  async function saveDepartment(value: Department) {\n    if (!departmentMoveAllowed(departments.value, value.id, value.parentId)) throw new Error('部门不能移动到自身或下级部门。')\n    if (!value.enabled && members.value.some((member) => member.departmentId === value.id && member.status !== 'removed')) {\n      throw new Error('请先转移部门成员，再停用部门。')\n    }\n    if (!value.name.trim()) throw new Error('请填写部门名称。')\n    const exists = departments.value.find((department) => department.id === value.id)\n\n    if (previewMode) {\n      snapshot.value.departments = exists\n        ? departments.value.map((department) => department.id === value.id ? { ...value } : department)\n        : [...departments.value, { ...value }]\n      audit('组织架构', exists ? '编辑部门' : '新建部门', value.name)\n      return\n    }\n\n    const signature = departmentMutationSignature(value, exists)\n    const key = (slot: string, create: () => string) => departmentMutationKey(signature, slot, create)\n    try {\n      const trusted = await stableDepartmentSession()\n      const draft: EnterpriseDepartmentDraft = {\n        name: value.name,\n        parentId: value.parentId ?? '',\n        leaderUserId: value.leaderId,\n        email: value.email ?? '',\n        phone: value.phone ?? '',\n        sort: value.sort ?? 0,\n        enabled: value.enabled,\n      }\n      let receipt\n      if (exists) {\n        const current = await getEnterpriseDepartment(trusted, exists.id)\n        receipt = await updateEnterpriseDepartment(\n          trusted,\n          current,\n          draft,\n          key('update', () => departmentRequestId('update')),\n        )\n        const active = receipt.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE'\n        if (value.enabled !== active) {\n          receipt = value.enabled\n            ? await enableEnterpriseDepartment(trusted, receipt, key('enable', () => departmentRequestId('enable')))\n            : await disableEnterpriseDepartment(trusted, receipt, key('disable', () => departmentRequestId('disable')))\n        }\n      } else {\n        receipt = await createEnterpriseDepartment(trusted, draft, key('create', () => departmentRequestId('create')))\n        if (!value.enabled && receipt.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE') {\n          receipt = await disableEnterpriseDepartment(trusted, receipt, key('disable', () => departmentRequestId('disable')))\n        }\n      }\n      await getEnterpriseDepartment(trusted, receipt.departmentId)\n      await refresh(['departments', 'members', 'roles'])\n      departmentMutation = null\n    } catch (error) {\n      throw new Error(departmentRuntimeError(error))\n    }\n  }\n'''
text = text[:start] + department_block + text[end:]
write(path, text)

# 4. Organization canonical domain contract + unsupported API fields.
path = 'web/src/features/enterprise/pages/OrganizationView.vue'
text = read(path)
text = replace_once(text, "store.ensureDomains(['departments', 'members'])", "store.ensureDomains(['departments', 'members', 'roles'])", 'organization roles dependency')
text = replace_once(
    text,
    '<span>部门编号</span><UiInput v-model="draft.code" class="input" maxlength="30" /></label',
    '<span>部门编号</span><UiInput v-model="draft.code" class="input" maxlength="30" :readonly="store.sourceKind === \'api\'" :placeholder="store.sourceKind === \'api\' ? \'服务端合同暂未提供部门编号\' : \'\'" /></label',
    'organization code readonly',
)
text = replace_once(
    text,
    '<UiTextarea v-model="draft.description" class="textarea" maxlength="300" /></label',
    '<UiTextarea v-model="draft.description" class="textarea" maxlength="300" :readonly="store.sourceKind === \'api\'" :placeholder="store.sourceKind === \'api\' ? \'服务端合同暂未提供部门职责字段\' : \'\'" /></label',
    'organization description readonly',
)
write(path, text)

# 5. Usage degradation copy is explicit but keeps limits authoritative.
path = 'web/src/stores/enterprisePlan.ts'
text = read(path)
text = replace_once(
    text,
    "      if (model.value.usageError) error.value = model.value.usageError",
    "      if (model.value.usageError) error.value = `用量服务暂不可用：${model.value.usageError}`",
    'plan usage degradation copy',
)
write(path, text)

# 6. Organization E2E -> canonical controls and dependency fixtures.
path = 'web/e2e/enterprise-organization-real.spec.ts'
text = read(path)
text = replace_once(
    text,
    "  await page.route('**/api/v1/tenant/members', async (route) => json(route, 200, { members }))",
    "  await page.route('**/api/v1/tenant/members', async (route) => json(route, 200, { members }))\n  await page.route('**/api/v1/tenant/roles', async (route) => json(route, 200, { roles: [\n    { id: 'owner', name: 'owner', status: 'TENANT_ROLE_STATUS_ACTIVE', version: 1, permissions: [] },\n    { id: 'csm', name: '客户成功', status: 'TENANT_ROLE_STATUS_ACTIVE', version: 1, permissions: [] },\n  ] }))",
    'organization role fixture',
)
text = replace_once(
    text,
    "  await page.locator('.tree-row').filter({ hasText: name }).click()",
    "  await page.getByRole('button', { name: new RegExp(`^${name}`) }).click()",
    'organization canonical tree selector',
)
text = text.replace("page.getByText('运营中心', { exact: true }).first()", "page.getByRole('button', { name: /^运营中心/ })")
text = text.replace("page.getByText('客户成功部', { exact: true }).first()", "page.getByRole('button', { name: /^客户成功部/ })")
text = text.replace("page.getByText('历史业务部', { exact: true }).first()", "page.getByRole('button', { name: /^历史业务部/ })")
text = text.replace("page.getByRole('dialog', { name: '新建部门' })", "page.getByRole('dialog', { name: '部门信息' })")
text = text.replace("page.getByRole('dialog', { name: '编辑部门' })", "page.getByRole('dialog', { name: '部门信息' })")
text = text.replace("getByLabel('部门名称 *')", "getByLabel('部门名称')")
text = text.replace("getByLabel('负责人')", "getByLabel('部门负责人')")
text = text.replace("getByRole('button', { name: '提交并回读确认' })", "getByRole('button', { name: '保存部门' })")
text = text.replace("toMatch(/^enterprise-department-create-new-/)", "toMatch(/^enterprise-department-create-/)")
text = text.replace("page.getByText('市场运营部', { exact: true }).first()", "page.getByRole('button', { name: /^市场运营部/ })")
text = replace_once(
    text,
    "  await selectUiOption(dialog.getByLabel('部门负责人'), 'user-001')\n  await dialog.getByRole('button', { name: '保存部门' }).click()",
    "  await selectUiOption(dialog.getByLabel('部门负责人'), 'user-001')\n  await expect(dialog.getByLabel('部门编号')).toHaveAttribute('readonly', '')\n  await expect(dialog.getByLabel('部门职责')).toHaveAttribute('readonly', '')\n  await dialog.getByRole('button', { name: '保存部门' }).click()",
    'organization API unsupported fields',
)
write(path, text)

# 7. Roles E2E -> canonical RoleEditor. Member-role mutation is already qualified from Members page.
path = 'web/e2e/enterprise-roles-real.spec.ts'
text = read(path)
text = replace_once(text, "import { expect, test, type Page, type Route } from '@playwright/test'", "import { selectUiOption } from './ui.helpers'\nimport { expect, test, type Page, type Route } from '@playwright/test'", 'role select helper import')
text = replace_once(
    text,
    "    await expect(page.getByText(/查看成员/).first()).toBeVisible()",
    "    await expect(rowFor(page, '运营负责人')).toContainText('1 人')\n    await expect(rowFor(page, '运营负责人')).toContainText('授权点位')",
    'role canonical viewport assertions',
)
text = text.replace("dialog.locator('[data-role-name]')", "dialog.getByLabel('角色名称')")
text = text.replace("dialog.getByLabel('查看成员 数据范围').click()\n  await page.getByRole('option', { name: '授权点位', exact: true }).click()", "selectUiOption(dialog.getByLabel('数据范围'), 'custom')")
text = text.replace("dialog.getByRole('button', { name: '保存并回读确认' })", "dialog.getByRole('button', { name: '保存角色' })")
text = replace_once(
    text,
    '''test('role member assignment is confirmed from member readback', async ({ page }) => {\n  const server = await mockRoleServer(page)\n  await openRealRoles(page)\n  await rowFor(page, '运营负责人').getByRole('button', { name: '管理' }).click()\n  const dialog = page.getByRole('dialog', { name: '管理角色权限' })\n  await dialog.getByLabel('角色成员 Alice Chen').check()\n  await dialog.getByRole('button', { name: '保存角色' }).click()\n  await expect(page.getByRole('status')).toContainText('服务端确认')\n  const write = server.getWrites().find((item) => item.path.endsWith('/role-ops/members'))!\n  expect(write.headers['idempotency-key']).toMatch(/^enterprise-role-assign-user-001-/)\n  expect(server.getMembers().find((member) => member.userId === 'user-001')?.roles.some((role) => role.roleId === 'role-ops')).toBe(true)\n})''',
    '''test('canonical roles page exposes authoritative member counts while membership changes stay on Members flow', async ({ page }) => {\n  const server = await mockRoleServer(page)\n  await openRealRoles(page)\n  const row = rowFor(page, '运营负责人')\n  await expect(row).toContainText('1 人')\n  await expect(row.getByRole('button', { name: '编辑' })).toBeVisible()\n  await expect(row.getByRole('button', { name: '管理' })).toHaveCount(0)\n  expect(server.getWrites()).toHaveLength(0)\n})''',
    'role membership canonical boundary',
)
text = text.replace("rowFor(page, '运营负责人').getByRole('button', { name: '管理' }).click()", "rowFor(page, '运营负责人').getByRole('button', { name: '编辑' }).click()")
text = text.replace("page.getByRole('dialog', { name: '管理角色权限' })", "page.getByRole('dialog', { name: '编辑角色权限' })")
text = replace_once(
    text,
    '''test('owner invariant conflict is surfaced and never presented as confirmed success', async ({ page }) => {\n  await mockRoleServer(page, { ownerConflict: true })\n  await openRealRoles(page)\n  await rowFor(page, '企业所有者').getByRole('button', { name: '查看与成员' }).click()\n  const dialog = page.getByRole('dialog', { name: '企业所有者角色' })\n  await dialog.getByLabel('角色成员 Alice Chen').uncheck()\n  await dialog.getByRole('button', { name: '保存角色' }).click()\n  await expect(dialog.getByRole('alert')).toContainText('角色版本或所有者保护规则已发生冲突')\n  await expect(page.getByText(/角色配置已由服务端确认/)).toHaveCount(0)\n})''',
    '''test('protected owner role is read-only in the canonical role editor', async ({ page }) => {\n  const server = await mockRoleServer(page)\n  await openRealRoles(page)\n  await rowFor(page, '企业所有者').getByRole('button', { name: '查看' }).click()\n  const dialog = page.getByRole('dialog', { name: '内置角色详情' })\n  await expect(dialog.getByText(/内置角色只读/)).toBeVisible()\n  await expect(dialog.getByRole('button', { name: '保存角色' })).toHaveCount(0)\n  expect(server.getWrites()).toHaveLength(0)\n})''',
    'protected owner canonical boundary',
)
write(path, text)

# 8. Plan E2E -> current canonical labels and information architecture.
path = 'web/e2e/enterprise-plan-real.spec.ts'
text = read(path)
text = text.replace("await expect(page.getByText('rental-growth-2026', { exact: true })).toBeVisible()", "await expect(page.locator('.current-plan h2')).toContainText('rental-growth-2026')")
text = text.replace("    await expect(page.getByText('Tenant Commercial API')).toBeVisible()\n    await expect(page.getByText('已接入用量 1 项')).toBeVisible()\n", "    await expect(page.getByText('tenant.members', { exact: true }).first()).toBeVisible()\n    await expect(page.getByText('monthly.reports', { exact: true }).first()).toBeVisible()\n")
text = text.replace("getByRole('button', { name: '额度用量' })", "getByRole('button', { name: '使用额度' })")
text = replace_once(
    text,
    "  await expect(memberRow).toContainText('3')\n  await expect(memberRow).toContainText('0')\n  await expect(memberRow).toContainText('额度已用尽')",
    "  await expect(memberRow).toContainText('3')\n  await expect(memberRow).toContainText('100.0%')\n  await expect(memberRow).toContainText('额度已用尽')",
    'finite quota canonical columns',
)
text = replace_once(text, "  await expect(page.getByText(/%/)).toHaveCount(0)\n", "", 'remove old quota percent absence')
text = replace_once(
    text,
    "  await expect(memberRow).toContainText('2')\n  await expect(memberRow).toContainText('1')\n  await expect(memberRow).toContainText('额度可用')",
    "  await expect(memberRow).toContainText('2')\n  await expect(memberRow).toContainText('3')\n  await expect(memberRow).toContainText('66.7%')\n  await expect(memberRow).toContainText('正常')",
    'quota below limit canonical columns',
)
write(path, text)

# 9. Plan change E2E -> open canonical management surface before lifecycle internal control.
path = 'web/e2e/enterprise-plan-change-real.spec.ts'
text = read(path)
text = replace_once(
    text,
    '''  await expect(page.locator('[data-enterprise-page="plan"]')).toBeVisible()\n  await expect(page.locator('[data-enterprise-source="api"]')).toBeVisible()\n  await page.locator('[data-plan-change-open]').click()\n  const lifecycle = page.locator('[data-plan-change-lifecycle]')''',
    '''  await expect(page.locator('[data-enterprise-page="plan"]')).toBeVisible()\n  await expect(page.locator('[data-enterprise-source="api"]')).toBeVisible()\n  await page.getByRole('button', { name: '管理套餐变更', exact: true }).click()\n  const lifecycle = page.locator('[data-plan-change-lifecycle]')\n  await lifecycle.locator('[data-plan-change-open]').click()''',
    'plan change canonical open sequence',
)
text = text.replace("    if (status !== 'APPLIED') await expect(page.getByText(/待处理 chg-tenant-preview-001/)).toBeVisible()\n", "    await expect(page.locator('[data-plan-change-receipt]')).toContainText('chg-tenant-preview-001')\n")
write(path, text)

print('Remaining route convergence migration applied.')
