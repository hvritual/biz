import { readFileSync, writeFileSync } from 'node:fs'

function replaceOnce(source, from, to, label) {
  if (!source.includes(from)) throw new Error(`missing patch anchor: ${label}`)
  return source.replace(from, to)
}

// 1) Canonical Members page declares every domain it actually renders.
const membersViewPath = 'web/src/features/enterprise/pages/MembersView.vue'
let membersView = readFileSync(membersViewPath, 'utf8')
membersView = replaceOnce(
  membersView,
  `onMounted(() => void store.ensureDomains(['members', 'roles']).catch(() => undefined))`,
  `onMounted(() => void store.ensureDomains(['members', 'roles', 'departments']).catch(() => undefined))`,
  'members domains',
)
writeFileSync(membersViewPath, membersView)

// 2) Store: same logical member mutation reuses idempotency keys until authoritative readback succeeds.
const storePath = 'web/src/stores/enterprise.ts'
let store = readFileSync(storePath, 'utf8')
store = replaceOnce(
  store,
  `  memberRequestId,\n  memberRoleRequestId,\n  readEnterpriseMemberSession,`,
  `  memberRequestId,\n  memberRoleRequestId,\n  memberRuntimeError,\n  readEnterpriseMemberSession,`,
  'member runtime error import',
)
store = replaceOnce(
  store,
  `  let companyMutation: { tenantId: string; signature: string; key: string } | null = null`,
  `  let companyMutation: { tenantId: string; signature: string; key: string } | null = null\n  let memberMutation: { tenantId: string; signature: string; keys: Record<string, string> } | null = null\n\n  function memberMutationSignature(draft: Member, action: MemberAction, expectedVersion: number) {\n    return JSON.stringify({\n      action,\n      expectedVersion,\n      id: draft.id,\n      email: draft.email.trim().toLowerCase(),\n      name: draft.name.trim(),\n      phone: draft.phone.trim(),\n      employeeId: draft.employeeId.trim(),\n      departmentId: draft.departmentId,\n      position: draft.position.trim(),\n      roleIds: [...draft.roleIds].sort(),\n      scope: draft.scope,\n    })\n  }\n\n  function memberMutationKey(signature: string, slot: string, create: () => string) {\n    if (!memberMutation || memberMutation.tenantId !== tenantId.value || memberMutation.signature !== signature) {\n      memberMutation = { tenantId: tenantId.value, signature, keys: {} }\n    }\n    memberMutation.keys[slot] ??= create()\n    return memberMutation.keys[slot]!\n  }`,
  'member mutation state',
)
store = replaceOnce(
  store,
  `      companyMutation = null\n      ready.value = true`,
  `      companyMutation = null\n      memberMutation = null\n      ready.value = true`,
  'tenant switch clears member mutation',
)
store = replaceOnce(
  store,
  `  async function saveMember(draft: Member, action: MemberAction, expectedVersion: number) {\n    const current = members.value.find((member) => member.id === draft.id)\n    if (current && current.version !== expectedVersion) throw new Error('成员资料已被其他操作修改，请重新打开后重试。')`,
  `  async function saveMember(draft: Member, action: MemberAction, expectedVersion: number) {\n    const current = members.value.find((member) => member.id === draft.id)\n    if (!previewMode && current && action === 'edit' && draft.email.trim().toLowerCase() !== current.email.trim().toLowerCase()) {\n      throw new Error('登录邮箱由成员关系与身份服务管理，当前成员资料接口不支持修改。')\n    }\n    if (current && current.version !== expectedVersion) throw new Error('成员资料已被其他操作修改，请重新打开后重试。')`,
  'immutable API email',
)
const apiStart = `    const trusted = await stableMemberSession()\n    if (action === 'role') {`
const apiEnd = `\n    throw new Error('该成员操作不应通过资料保存入口执行。')`
const startIndex = store.indexOf(apiStart)
const endIndex = store.indexOf(apiEnd, startIndex)
if (startIndex < 0 || endIndex < 0) throw new Error('missing saveMember API block')
const replacement = `    const signature = memberMutationSignature(draft, action, expectedVersion)\n    const key = (slot: string, create: () => string) => memberMutationKey(signature, slot, create)\n\n    try {\n      const trusted = await stableMemberSession()\n      if (action === 'role') {\n        if (!current) throw new Error('成员不存在。')\n        if (draft.scope !== current.scope) {\n          throw new Error('真实服务的数据范围由角色权限派生，当前不支持按成员单独覆盖数据范围。')\n        }\n        const add = draft.roleIds.filter((roleId) => !current.roleIds.includes(roleId))\n        const remove = current.roleIds.filter((roleId) => !draft.roleIds.includes(roleId))\n        for (const roleId of add) {\n          await assignEnterpriseMemberRole(\n            trusted,\n            current.id,\n            roleId,\n            key(\`role-assign-\${roleId}\`, () => memberRoleRequestId('assign')),\n          )\n        }\n        for (const roleId of remove) {\n          await revokeEnterpriseMemberRole(\n            trusted,\n            current.id,\n            roleId,\n            key(\`role-revoke-\${roleId}\`, () => memberRoleRequestId('revoke')),\n          )\n        }\n        await getEnterpriseMember(trusted, current.id)\n        await refresh(['members', 'roles', 'departments'])\n        memberMutation = null\n        return\n      }\n\n      if (action === 'create' || action === 'invite') {\n        let receipt = await inviteEnterpriseMember(trusted, draft.email, key('invite', () => memberRequestId('invite')))\n        receipt = await updateEnterpriseMemberProfile(\n          trusted,\n          receipt,\n          {\n            name: draft.name,\n            phone: draft.phone,\n            employeeId: draft.employeeId,\n            position: draft.position,\n            departmentId: draft.departmentId,\n          },\n          key('profile', () => memberRequestId('profile')),\n        )\n        for (const roleId of draft.roleIds) {\n          await assignEnterpriseMemberRole(\n            trusted,\n            receipt.userId,\n            roleId,\n            key(\`role-assign-\${roleId}\`, () => memberRoleRequestId('assign')),\n          )\n        }\n        if (action === 'create') {\n          const invited = await getEnterpriseMember(trusted, receipt.userId)\n          await activateEnterpriseMember(trusted, invited, key('activate', () => memberRequestId('activate')))\n        }\n        await getEnterpriseMember(trusted, receipt.userId)\n        await refresh(['members', 'roles', 'departments'])\n        memberMutation = null\n        return\n      }\n\n      if (action === 'edit') {\n        if (!current) throw new Error('成员不存在。')\n        const receipt = await updateEnterpriseMemberProfile(\n          trusted,\n          asServerMember(current),\n          {\n            name: draft.name,\n            phone: draft.phone,\n            employeeId: draft.employeeId,\n            position: draft.position,\n            departmentId: draft.departmentId,\n          },\n          key('profile', () => memberRequestId('profile')),\n        )\n        await getEnterpriseMember(trusted, receipt.userId)\n        await refresh(['members', 'roles', 'departments'])\n        memberMutation = null\n        return\n      }\n\n      throw new Error('该成员操作不应通过资料保存入口执行。')\n    } catch (error) {\n      throw new Error(memberRuntimeError(error))\n    }`
store = store.slice(0, startIndex) + replacement + store.slice(endIndex + apiEnd.length)
writeFileSync(storePath, store)

// 3) Canonical dialog: derive defaults from authoritative catalogs and expose unsupported API fields as read-only/explicit.
const dialogPath = 'web/src/features/enterprise/components/members/MemberActionDialog.vue'
let dialog = readFileSync(dialogPath, 'utf8')
const watchAnchor = `watch(\n  () => [props.open, props.action, props.member] as const,`
const helper = `function newMemberDraft(action: MemberAction): Member {\n  const department = store.departments.find((item) => item.enabled)\n  const role = store.roles.find((item) => item.enabled && !item.builtin) ?? store.roles.find((item) => item.enabled)\n  return {\n    id: crypto.randomUUID(),\n    name: '',\n    email: '',\n    phone: '',\n    employeeId: '',\n    departmentId: department?.id ?? '',\n    position: '',\n    roleIds: role ? [role.id] : [],\n    scope: role?.scope ?? 'custom',\n    status: action === 'invite' ? 'invited' : 'active',\n    online: false,\n    joinedAt: store.previewMode ? new Date().toISOString().slice(0, 10) : '',\n    lastLogin: null,\n    version: 0,\n    mfa: false,\n    note: '',\n  }\n}\n`
dialog = replaceOnce(dialog, watchAnchor, helper + watchAnchor, 'new member draft helper')
const oldDraft = `    draft.value = props.member\n      ? (JSON.parse(JSON.stringify(props.member)) as Member)\n      : {\n          id: crypto.randomUUID(),\n          name: '',\n          email: '',\n          phone: '',\n          employeeId: '',\n          departmentId: 'operations',\n          position: '业务专员',\n          roleIds: ['role-2'],\n          scope: 'department',\n          status: props.action === 'invite' ? 'invited' : 'active',\n          online: false,\n          joinedAt: new Date().toISOString().slice(0, 10),\n          lastLogin: null,\n          version: 0,\n          mfa: false,\n          note: '',\n        }`
dialog = replaceOnce(
  dialog,
  oldDraft,
  `    draft.value = props.member\n      ? (JSON.parse(JSON.stringify(props.member)) as Member)\n      : newMemberDraft(props.action)`,
  'remove demo IDs from new member draft',
)
const oldEmail = `<UiInput\n                v-model="draft.email"\n                class="input"\n                type="email"\n                required\n                placeholder="name@example.com"\n              /><small v-if="action === 'edit'">预览资料修改不等于变更已验证的登录凭据。</small>`
const newEmail = `<UiInput\n                v-model="draft.email"\n                class="input"\n                type="email"\n                required\n                :readonly="!store.previewMode && action === 'edit'"\n                placeholder="name@example.com"\n              /><small v-if="action === 'edit' && store.previewMode">预览资料修改不等于变更已验证的登录凭据。</small\n              ><small v-else-if="action === 'edit'">登录邮箱由成员关系与身份服务管理，当前资料接口不支持修改。</small>`
dialog = replaceOnce(dialog, oldEmail, newEmail, 'API email read-only')
const oldDateScope = `<label class="field"\n              ><span>加入日期</span><UiInput v-model="draft.joinedAt" class="input" type="date" /></label\n            ><label class="field"\n              ><span>数据范围</span\n              ><UiSelect v-model="draft.scope" class="select">\n                <UiOption v-for="(label, key) in scopeLabels" :key="key" :value="key">{{ label }}</UiOption>\n              </UiSelect></label\n            >`
const newDateScope = `<label class="field"\n              ><span>加入日期</span\n              ><UiInput v-if="store.previewMode" v-model="draft.joinedAt" class="input" type="date"\n              /><UiInput v-else class="input" value="服务端未提供" readonly\n              /><small v-if="!store.previewMode">当前成员资料接口未提供加入日期写入能力。</small></label\n            ><label class="field"\n              ><span>数据范围</span\n              ><UiSelect v-if="store.previewMode" v-model="draft.scope" class="select">\n                <UiOption v-for="(label, key) in scopeLabels" :key="key" :value="key">{{ label }}</UiOption>\n              </UiSelect\n              ><UiInput\n                v-else\n                class="input"\n                :value="member ? scopeLabels[draft.scope] : '保存后由角色权限服务派生'"\n                readonly\n              /><small v-if="!store.previewMode">真实数据范围由角色权限服务派生，不能按成员直接覆盖。</small></label\n            >`
dialog = replaceOnce(dialog, oldDateScope, newDateScope, 'API joined date and scope read-only')
const oldNote = `<section class="form-section">\n          <label class="field"\n            ><span>备注</span\n            ><UiTextarea\n              v-model="draft.note"\n              class="textarea"\n              rows="2"\n              maxlength="500"\n              placeholder="补充成员职责或说明"\n            />\n          </label>\n        </section>`
const newNote = `<section class="form-section">\n          <label v-if="store.previewMode" class="field"\n            ><span>备注</span\n            ><UiTextarea\n              v-model="draft.note"\n              class="textarea"\n              rows="2"\n              maxlength="500"\n              placeholder="补充成员职责或说明"\n            />\n          </label>\n          <div v-else class="notice-box">当前成员资料 API 未提供备注写入字段，本页面不提供不可持久化的编辑入口。</div>\n        </section>`
dialog = replaceOnce(dialog, oldNote, newNote, 'API note unsupported')
dialog = replaceOnce(
  dialog,
  `          />角色权限与数据范围分别配置。变更后按最新授权重新计算访问范围，不要求成员重新登录来激活权限。`,
  `          />{{\n            store.previewMode\n              ? '角色权限与数据范围分别配置。变更后按最新授权重新计算访问范围，不要求成员重新登录来激活权限。'\n              : '真实数据范围由角色权限服务派生；此处只修改角色，保存后以服务端权威回读范围为准。'\n          }}`,
  'role scope explanation',
)
const oldRoleScope = `<label class="field"\n          ><span>目标数据范围</span\n          ><UiSelect v-model="draft.scope" class="select">\n            <UiOption v-for="(label, key) in scopeLabels" :key="key" :value="key">{{ label }}</UiOption>\n          </UiSelect></label\n        >`
const newRoleScope = `<label class="field"\n          ><span>目标数据范围</span\n          ><UiSelect v-if="store.previewMode" v-model="draft.scope" class="select">\n            <UiOption v-for="(label, key) in scopeLabels" :key="key" :value="key">{{ label }}</UiOption>\n          </UiSelect\n          ><UiInput v-else class="input" value="保存后由角色权限服务派生" readonly /></label\n        >`
dialog = replaceOnce(dialog, oldRoleScope, newRoleScope, 'role scope read-only')
dialog = replaceOnce(
  dialog,
  `<p>{{ scopeLabels[draft.scope as DataScope] }}</p>`,
  `<p>{{ store.previewMode ? scopeLabels[draft.scope as DataScope] : '保存后由服务端派生' }}</p>`,
  'role change preview scope',
)
dialog = replaceOnce(
  dialog,
  `<div v-if="action === 'invite'" class="notice-box">\n        <AppIcon name="mail" />邀请记录将在本地预览中创建。邮件投递、有效期和激活凭证需由服务端提供。\n      </div>`,
  `<div v-if="action === 'invite'" class="notice-box">\n        <AppIcon name="mail" />{{\n          store.previewMode\n            ? '邀请记录将在本地预览中创建。邮件投递、有效期和激活凭证需由服务端提供。'\n            : '邀请会调用真实成员服务；只有资料、角色关系与成员回读全部完成后才显示成功。'\n        }}\n      </div>`,
  'invite API explanation',
)
writeFileSync(dialogPath, dialog)

// 4) Detail read model: do not present synthetic demo-only facts as server facts.
const detailPath = 'web/src/features/enterprise/components/members/MemberDetailOverview.vue'
let detail = readFileSync(detailPath, 'utf8')
detail = replaceOnce(
  detail,
  `<dd class="numeric">{{ member.joinedAt }}</dd>`,
  `<dd class="numeric">{{ member.joinedAt || (store.previewMode ? '未记录' : '服务端未提供') }}</dd>`,
  'joined date fallback',
)
detail = replaceOnce(
  detail,
  `<dd>{{ member.note || '暂无备注' }}</dd>`,
  `<dd>{{ member.note || (store.previewMode ? '暂无备注' : '服务端未提供') }}</dd>`,
  'note fallback',
)
detail = replaceOnce(
  detail,
  `<div class="notice-box">示例权限仅用于界面预览；实际授权以服务端校验为准。</div>`,
  `<div class="notice-box">{{\n    store.previewMode\n      ? '示例权限仅用于界面预览；实际授权以服务端校验为准。'\n      : '数据范围由服务端角色权限派生，本页只展示权威回读结果。'\n  }}</div>`,
  'detail scope source notice',
)
detail = replaceOnce(
  detail,
  `<li>\n        <span>{{ member.joinedAt }}</span\n        ><strong>加入当前企业</strong><small>成员关系记录 · 示例数据</small>\n      </li>`,
  `<li v-if="store.previewMode">\n        <span>{{ member.joinedAt }}</span\n        ><strong>加入当前企业</strong><small>成员关系记录 · 示例数据</small>\n      </li>`,
  'hide synthetic API timeline item',
)
writeFileSync(detailPath, detail)

// 5) Replace RealView-oriented Members E2E with canonical product interactions.
const testPath = 'web/e2e/enterprise-members-real.spec.ts'
let test = readFileSync(testPath, 'utf8')
test = replaceOnce(
  test,
  `import { expect, test, type Page, type Route } from '@playwright/test'`,
  `import { expect, test, type Page, type Route } from '@playwright/test'\nimport { selectUiOption } from './ui.helpers'`,
  'UI select helper import',
)
test = replaceOnce(
  test,
  `type RemoteRole = { id: string; name: string; status: string; version: number; permissions: unknown[] }`,
  `type RemoteRole = { id: string; name: string; status: string; version: number; permissions: unknown[]; protectedOwner?: boolean }\ntype RemoteDepartment = { departmentId: string; name: string; parentId: string; leaderUserId: string; email: string; phone: string; status: string; sort: number; version: number }`,
  'department test type',
)
test = replaceOnce(
  test,
  `const roleCatalog: RemoteRole[] = [\n  { id: 'role-ops', name: '运营负责人', status: 'TENANT_ROLE_STATUS_ACTIVE', version: 2, permissions: [] },\n  { id: 'role-viewer', name: '经营查看者', status: 'TENANT_ROLE_STATUS_ACTIVE', version: 1, permissions: [] },\n  { id: 'role-legacy', name: '历史角色', status: 'TENANT_ROLE_STATUS_DISABLED', version: 4, permissions: [] },\n]`,
  `const roleCatalog: RemoteRole[] = [\n  { id: 'role-ops', name: '运营负责人', status: 'TENANT_ROLE_STATUS_ACTIVE', version: 2, permissions: [] },\n  { id: 'role-viewer', name: '经营查看者', status: 'TENANT_ROLE_STATUS_ACTIVE', version: 1, permissions: [] },\n  { id: 'role-legacy', name: '历史角色', status: 'TENANT_ROLE_STATUS_DISABLED', version: 4, permissions: [] },\n]\n\nconst departmentCatalog: RemoteDepartment[] = [\n  { departmentId: 'dept-success', name: '客户成功部', parentId: '', leaderUserId: 'user-001', email: 'success@coffeelink.test', phone: '', status: 'TENANT_DEPARTMENT_STATUS_ACTIVE', sort: 10, version: 3 },\n  { departmentId: 'dept-rental', name: '租赁运营部', parentId: '', leaderUserId: 'user-001', email: 'rental@coffeelink.test', phone: '', status: 'TENANT_DEPARTMENT_STATUS_ACTIVE', sort: 20, version: 2 },\n]`,
  'department fixture catalog',
)
test = replaceOnce(
  test,
  `  await page.route('**/api/v1/tenant/roles', async (route) => {\n    return json(route, 200, { roles: roleCatalog })\n  })`,
  `  await page.route('**/api/v1/tenant/roles', async (route) => {\n    return json(route, 200, { roles: roleCatalog })\n  })\n\n  await page.route('**/api/v1/tenant/departments', async (route) => {\n    return json(route, 200, { departments: departmentCatalog })\n  })`,
  'department API fixture',
)
const marker = `async function openRealMembers(page: Page) {`
const markerIndex = test.indexOf(marker)
if (markerIndex < 0) throw new Error('missing Members E2E marker')
const prefix = test.slice(0, markerIndex)
const tail = `async function openCanonicalMembers(page: Page) {\n  await page.goto('/#/enterprise/members')\n  await expect(page.locator('[data-enterprise-page="members"]')).toBeVisible()\n  await expect(page.locator('[data-enterprise-source="api"]')).toBeVisible()\n}\n\ntest('canonical member page renders authoritative member, role, department and scope across CoffeeLink viewports', async ({ page }) => {\n  await mockMemberServer(page)\n  mkdirSync('screenshots', { recursive: true })\n  for (const viewport of [\n    { width: 1366, height: 768 },\n    { width: 1440, height: 900 },\n    { width: 1536, height: 1024 },\n    { width: 390, height: 844 },\n  ]) {\n    await page.setViewportSize(viewport)\n    await openCanonicalMembers(page)\n    const row = page.locator('[data-member-id="user-001"]')\n    await expect(row).toBeVisible()\n    await expect(row).toContainText('Alice Chen')\n    await expect(row).toContainText('客户成功部')\n    await expect(row).toContainText('运营负责人')\n    await expect(row).toContainText('指定数据')\n    await expect(page.getByText('张三', { exact: true })).toHaveCount(0)\n    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)\n    await page.screenshot({ path: \`screenshots/enterprise-members-real-\${viewport.width}.png\` })\n  }\n})\n\ntest('canonical invite uses loaded role and department, trusted headers and authoritative readback', async ({ page }) => {\n  const server = await mockMemberServer(page)\n  await openCanonicalMembers(page)\n  await page.getByRole('button', { name: '邀请成员', exact: true }).click()\n  const dialog = page.getByRole('dialog', { name: '邀请成员' })\n  await dialog.getByLabel('姓名').fill('Invitee User')\n  await dialog.getByLabel('邮箱').fill('invitee@coffeelink.test')\n  await expect(dialog.getByLabel('所属部门')).toContainText('客户成功部')\n  await expect(dialog.getByLabel('数据范围')).toHaveValue('保存后由角色权限服务派生')\n  await expect(dialog.getByLabel('运营负责人')).toBeChecked()\n  await dialog.getByRole('button', { name: '创建邀请', exact: true }).click()\n  await expect(page.getByRole('status')).toContainText('服务端确认')\n  await expect(page.locator('[data-member-id="user-003"]')).toContainText('Invitee User')\n  const invite = server.getWrites().find((item) => item.path === '/api/v1/tenant/members' && item.method === 'POST')!\n  expect(invite.headers['x-csrf-token']).toBe('csrf-real-member')\n  expect(invite.headers['idempotency-key']).toMatch(/^enterprise-member-invite-/)\n  expect(invite.headers['x-biz-session-context']).toContain('tenant-001')\n  expect(server.getMembers().find((member) => member.userId === 'user-003')).toMatchObject({\n    name: 'Invitee User',\n    departmentId: 'dept-success',\n  })\n})\n\ntest('canonical profile edit exposes only supported fields and sends authoritative version', async ({ page }) => {\n  const server = await mockMemberServer(page)\n  await openCanonicalMembers(page)\n  await page.getByRole('button', { name: '编辑 Alice Chen', exact: true }).click()\n  const dialog = page.getByRole('dialog', { name: '修改成员信息' })\n  await expect(dialog.getByLabel('邮箱')).toHaveAttribute('readonly', '')\n  await expect(dialog.getByLabel('加入日期')).toHaveValue('服务端未提供')\n  await expect(dialog.getByLabel('数据范围')).toHaveValue('指定数据')\n  await expect(dialog.getByText(/成员资料 API 未提供备注写入字段/)).toBeVisible()\n  await dialog.getByLabel('姓名').fill('Alice Updated')\n  await dialog.getByLabel('手机号').fill('+886900000009')\n  await dialog.getByLabel('员工编号').fill('EMP-2009')\n  await dialog.getByLabel('岗位').fill('租赁运营负责人')\n  await selectUiOption(dialog.getByLabel('所属部门'), 'dept-rental')\n  await dialog.getByRole('button', { name: '保存变更', exact: true }).click()\n  await expect(page.getByRole('status')).toContainText('服务端确认')\n  const row = page.locator('[data-member-id="user-001"]')\n  await expect(row).toContainText('Alice Updated')\n  await expect(row).toContainText('租赁运营部')\n  const write = server.getWrites().find((item) => item.path.endsWith('/profile'))!\n  expect(write.method).toBe('PATCH')\n  expect(write.headers['x-csrf-token']).toBe('csrf-real-member')\n  expect(write.headers['idempotency-key']).toMatch(/^enterprise-member-profile-/)\n  expect(write.headers['x-biz-session-context']).toContain('tenant-001')\n  expect(write.body).toMatchObject({ userId: 'user-001', version: 3, departmentId: 'dept-rental', employeeId: 'EMP-2009' })\n  expect(write.body).not.toHaveProperty('email')\n  expect(write.body).not.toHaveProperty('scope')\n  expect(write.body).not.toHaveProperty('joinedAt')\n  expect(write.body).not.toHaveProperty('note')\n})\n\ntest('canonical role change is idempotent, scope remains server-derived and readback confirms membership', async ({ page }) => {\n  const server = await mockMemberServer(page)\n  await openCanonicalMembers(page)\n  await page.getByRole('button', { name: 'Alice Chen 更多操作', exact: true }).click()\n  await page.getByRole('button', { name: '角色与数据权限变更', exact: true }).click()\n  const dialog = page.getByRole('dialog', { name: '角色变更与权限调整' })\n  await expect(dialog.getByLabel('目标数据范围')).toHaveValue('保存后由角色权限服务派生')\n  await dialog.getByLabel('经营查看者').check()\n  await dialog.getByRole('button', { name: '保存变更', exact: true }).click()\n  await expect(page.getByRole('status')).toContainText('服务端确认')\n  await expect(page.locator('[data-member-id="user-001"]')).toContainText('经营查看者')\n  const write = server.getWrites().find((item) => item.path.endsWith('/roles/role-viewer/members'))!\n  expect(write.method).toBe('POST')\n  expect(write.headers['idempotency-key']).toMatch(/^enterprise-member-role-assign-/)\n  expect(write.headers['x-csrf-token']).toBe('csrf-real-member')\n})\n\ntest('401 and 403 remain explicit and never replace API members with preview data', async ({ page }) => {\n  await mockMemberServer(page, { unauthenticated: true })\n  await openCanonicalMembers(page)\n  await expect(page.getByText('unauthenticated', { exact: true })).toBeVisible()\n  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)\n\n  await page.unrouteAll({ behavior: 'ignoreErrors' })\n  await mockMemberServer(page, { listStatus: 403 })\n  await page.reload()\n  await expect(page.getByText('list denied', { exact: true })).toBeVisible()\n  await expect(page.getByText('张三', { exact: true })).toHaveCount(0)\n})\n\ntest('canonical profile 409 preserves draft and reuses the same idempotency key', async ({ page }) => {\n  const server = await mockMemberServer(page, { mutationStatus: 409 })\n  await openCanonicalMembers(page)\n  await page.getByRole('button', { name: '编辑 Alice Chen', exact: true }).click()\n  const dialog = page.getByRole('dialog', { name: '修改成员信息' })\n  await dialog.getByLabel('姓名').fill('conflicting change')\n  const save = dialog.getByRole('button', { name: '保存变更', exact: true })\n  await save.click()\n  await expect(dialog.getByRole('alert')).toContainText('成员状态或请求版本已发生变化')\n  await expect(dialog.getByLabel('姓名')).toHaveValue('conflicting change')\n  await save.click()\n  await expect(dialog.getByRole('alert')).toContainText('成员状态或请求版本已发生变化')\n  const writes = server.getWrites().filter((item) => item.path.endsWith('/profile'))\n  expect(writes).toHaveLength(2)\n  expect(writes[0]?.headers['idempotency-key']).toBe(writes[1]?.headers['idempotency-key'])\n  await expect(page.getByText(/变更已由服务端确认并完成权威回读/)).toHaveCount(0)\n})\n\ntest('successful member write without GET readback is never presented as canonical success', async ({ page }) => {\n  await mockMemberServer(page, { readbackStatus: 500 })\n  await openCanonicalMembers(page)\n  await page.getByRole('button', { name: '编辑 Alice Chen', exact: true }).click()\n  const dialog = page.getByRole('dialog', { name: '修改成员信息' })\n  await dialog.getByLabel('姓名').fill('unconfirmed change')\n  await dialog.getByRole('button', { name: '保存变更', exact: true }).click()\n  await expect(dialog.getByRole('alert')).toContainText('readback failed')\n  await expect(page.getByText(/变更已由服务端确认并完成权威回读/)).toHaveCount(0)\n})\n`
writeFileSync(testPath, prefix + tail)

console.log('Members canonical API convergence patch applied.')
