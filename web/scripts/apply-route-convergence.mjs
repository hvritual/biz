import { existsSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve('.')

function read(path) {
  return readFileSync(resolve(root, path), 'utf8')
}

function write(path, content) {
  writeFileSync(resolve(root, path), content)
}

function replaceOnce(source, from, to, path) {
  if (!source.includes(from)) throw new Error(`${path}: expected migration token not found: ${from.slice(0, 80)}`)
  return source.replace(from, to)
}

function migrate(path, transform) {
  const source = read(path)
  const next = transform(source)
  if (source === next) throw new Error(`${path}: migration produced no change`)
  write(path, next)
}

migrate('src/features/enterprise/pages/MembersView.vue', (source) => {
  let next = replaceOnce(
    source,
    "import MemberBulkDialog from '@/features/enterprise/components/members/MemberBulkDialog.vue'",
    "import MemberBulkDialog from '@/features/enterprise/components/members/MemberBulkDialog.vue'\nimport EnterpriseSourceBanner from '@/features/enterprise/components/EnterpriseSourceBanner.vue'",
    'MembersView.vue',
  )
  next = replaceOnce(next, '<div class="members-view" :class=', '<div class="members-view" data-ui-template="ListPage" :class=', 'MembersView.vue')
  next = replaceOnce(next, '          compact\n        />\n        <MemberOverview />', '          compact\n        />\n        <EnterpriseSourceBanner />\n        <MemberOverview />', 'MembersView.vue')
  next = next.replace("downloadCsv('成员列表-界面预览.csv'", "downloadCsv('成员列表.csv'")
  next = next.replace("store.audit('成员管理', '导出成员列表', `${output.length} 条预览数据`)", "store.audit('成员管理', '导出成员列表', `${output.length} 条数据`)")
  next = next.replace('ui.toast(`已导出 ${output.length} 条预览记录。`)', 'ui.toast(`已导出 ${output.length} 条记录。`)')
  return next
})

migrate('src/features/enterprise/pages/RolesView.vue', (source) => {
  let next = replaceOnce(
    source,
    "import RoleEditor from '@/features/enterprise/components/roles/RoleEditor.vue'",
    "import RoleEditor from '@/features/enterprise/components/roles/RoleEditor.vue'\nimport EnterpriseSourceBanner from '@/features/enterprise/components/EnterpriseSourceBanner.vue'",
    'RolesView.vue',
  )
  next = replaceOnce(next, '<div class="page-stack">', '<div class="page-stack" data-ui-template="ListPage">', 'RolesView.vue')
  next = replaceOnce(
    next,
    '<PageHeading title="角色权限" description="以最小必要权限分配职责，独立控制功能权限与数据范围" />',
    '<PageHeading title="角色权限" description="以最小必要权限分配职责，独立控制功能权限与数据范围" />\n    <EnterpriseSourceBanner />',
    'RolesView.vue',
  )
  return next
})

migrate('src/features/enterprise/pages/OrganizationView.vue', (source) => {
  let next = source.replace("import { computed, ref } from 'vue'", "import { computed, ref, watch } from 'vue'")
  next = replaceOnce(
    next,
    "import AppPagination from '@/ui/common/AppPagination.vue'",
    "import AppPagination from '@/ui/common/AppPagination.vue'\nimport EnterpriseSourceBanner from '@/features/enterprise/components/EnterpriseSourceBanner.vue'",
    'OrganizationView.vue',
  )
  next = next.replace("const selected = ref('operations'),", "const selected = ref(''),")
  next = next.replace("leaderId: 'member-1',", "leaderId: '',")
  next = next.replace("leaderId: 'member-1',\n          code:", "leaderId: store.members.find((member) => member.status === 'active')?.id ?? '',\n          code:")
  next = next.replace('function save() {', 'async function save() {')
  next = next.replace('    store.saveDepartment(draft.value)', '    await store.saveDepartment(draft.value)')
  next = next.replace("ui.toast('组织调整已保存到当前企业预览。')", "ui.toast(store.sourceKind === 'api' ? '组织调整已由服务端确认并回读。' : '组织调整已保存到当前企业预览。')")
  next = replaceOnce(next, '</script>', `watch(\n  () => store.departments.map((department) => department.id).join(','),\n  () => {\n    if (selected.value && store.departments.some((department) => department.id === selected.value)) return\n    selected.value = store.departments[0]?.id ?? ''\n    page.value = 1\n  },\n  { immediate: true },\n)\n</script>`, 'OrganizationView.vue')
  next = replaceOnce(next, '<div class="page-stack">', '<div class="page-stack" data-ui-template="WorkbenchPage">', 'OrganizationView.vue')
  next = replaceOnce(
    next,
    '<PageHeading title="组织架构" description="管理部门与汇报关系，让组织协作与数据边界保持清晰" />',
    '<PageHeading title="组织架构" description="管理部门与汇报关系，让组织协作与数据边界保持清晰" />\n    <EnterpriseSourceBanner />',
    'OrganizationView.vue',
  )
  return next
})

migrate('src/features/enterprise/pages/CompanyView.vue', (source) => {
  let next = source.replace("import { computed, ref } from 'vue'", "import { computed, ref, watch } from 'vue'")
  next = replaceOnce(
    next,
    "import brand from '@/assets/brand-mark.png'",
    "import brand from '@/assets/brand-mark.png'\nimport EnterpriseSourceBanner from '@/features/enterprise/components/EnterpriseSourceBanner.vue'",
    'CompanyView.vue',
  )
  next = next.replace('function save() {', 'async function save() {')
  next = next.replace('    store.saveCompany(draft.value)', '    await store.saveCompany(draft.value)')
  next = next.replace("ui.toast('企业资料已保存到本地预览。')", "ui.toast(store.sourceKind === 'api' ? '企业资料已由服务端确认并回读。' : '企业资料已保存到本地预览。')")
  next = replaceOnce(next, '</script>', `watch(\n  () => store.company,\n  (value) => {\n    draft.value = { ...value }\n  },\n  { immediate: true },\n)\n</script>`, 'CompanyView.vue')
  next = replaceOnce(next, '<div class="page-stack">', '<div class="page-stack" data-ui-template="FormPage">', 'CompanyView.vue')
  next = replaceOnce(
    next,
    '<PageHeading title="企业信息" description="维护企业基本资料与联系信息，统一团队的身份与展示" />',
    '<PageHeading title="企业信息" description="维护企业基本资料与联系信息，统一团队的身份与展示" />\n    <EnterpriseSourceBanner />',
    'CompanyView.vue',
  )
  next = next.replace('<dd>标准版（示例）</dd>', `<dd>{{ store.sourceKind === 'api' ? '由套餐服务提供' : '标准版（示例）' }}</dd>`)
  return next
})

migrate('src/features/enterprise/components/members/MemberActionDialog.vue', (source) => {
  let next = source.replace(
    '      store.saveMember(draft.value, props.action, props.member?.version ?? 0)',
    '      await store.saveMember(draft.value, props.action, props.member?.version ?? 0)',
  )
  next = next.replace(
    '      store.changeStatus(\n',
    '      await store.changeStatus(\n',
  )
  const resetStart = `    else if (props.action === 'reset') {\n      store.audit(\n        '成员管理',\n        '创建密码重置请求（预览）',\n        draft.value.name,\n        '未请求',\n        '待接入身份服务',\n        reason.value,\n        'high',\n      )\n    }`
  next = replaceOnce(next, resetStart, `    else if (props.action === 'reset') {\n      await store.requestPasswordReset(draft.value, reason.value)\n    }`, 'MemberActionDialog.vue')
  const messageStart = `    const message =\n      props.action === 'reset'\n        ? '已记录预览重置请求；未发送邮件，也未改变真实密码。'\n        : props.action === 'invite'\n          ? '已创建预览邀请记录；未实际发送邀请邮件。'\n          : '变更已保存到当前企业的本地预览数据。'`
  next = replaceOnce(next, messageStart, `    const message = store.previewMode\n      ? props.action === 'reset'\n        ? '已记录预览重置请求；未发送邮件，也未改变真实密码。'\n        : props.action === 'invite'\n          ? '已创建预览邀请记录；未实际发送邀请邮件。'\n          : '变更已保存到当前企业的本地预览数据。'\n      : '变更已由服务端确认并完成权威回读。'`, 'MemberActionDialog.vue')
  return next
})

migrate('src/features/enterprise/components/members/MemberBulkDialog.vue', (source) => {
  let next = source.replace('function submit() {', 'async function submit() {')
  next = next.replace('    store.changeStatuses(props.targets, props.action, reason.value.trim())', '    await store.changeStatuses(props.targets, props.action, reason.value.trim())')
  next = next.replace('ui.toast(`已${label.value} ${props.targets.length} 位成员（本地预览），未修改真实账号。`)', "ui.toast(store.previewMode ? `已${label.value} ${props.targets.length} 位成员（本地预览），未修改真实账号。` : `已${label.value} ${props.targets.length} 位成员，并完成服务端回读。`)")
  return next
})

migrate('src/features/enterprise/pages/PlansView.vue', (source) => {
  let next = source.replace('if (plan.serverChangeContext) lifecycleOpen.value = true\n      else requestOpen.value = true', "if (plan.isServerBacked) lifecycleOpen.value = true\n      else requestOpen.value = true")
  next = next.replace('if (plan.serverChangeContext) lifecycleOpen.value = true\n  else requestOpen.value = true', "if (plan.isServerBacked) lifecycleOpen.value = true\n  else requestOpen.value = true")
  return next
})

const scopesPath = 'src/features/component-scopes.json'
const scopes = JSON.parse(read(scopesPath))
scopes['enterprise/components/EnterpriseSourceBanner.vue'] = {
  scenario: '企业中心 · 数据源状态与租户上下文 · EnterpriseSourceBanner',
  scope: '仅用于企业中心 canonical 页面展示统一 demo/API 数据源状态、真实会话错误、租户切换与刷新；不得作为另一套 API 页面入口。',
}
for (const key of Object.keys(scopes)) {
  if (key.startsWith('enterprise/components/server/')) delete scopes[key]
}
write(scopesPath, `${JSON.stringify(scopes, null, 2)}\n`)

const contractPath = 'ui-contracts.json'
const uiContract = JSON.parse(read(contractPath))
const enterpriseRoutes = [
  { path: '/enterprise/members', component: '@/features/enterprise/pages/MembersView.vue', surface: 'tenant', template: 'ListPage', required_regions: [] },
  { path: '/enterprise/roles', component: '@/features/enterprise/pages/RolesView.vue', surface: 'tenant', template: 'ListPage', required_regions: [] },
  { path: '/enterprise/organization', component: '@/features/enterprise/pages/OrganizationView.vue', surface: 'tenant', template: 'WorkbenchPage', required_regions: [] },
  { path: '/enterprise/plan', component: '@/features/enterprise/pages/PlansView.vue', surface: 'tenant', template: 'WorkbenchPage', required_regions: [] },
  { path: '/enterprise/company', component: '@/features/enterprise/pages/CompanyView.vue', surface: 'tenant', template: 'FormPage', required_regions: [] },
]
const enterprisePaths = new Set(enterpriseRoutes.map((route) => route.path))
uiContract.routes = [...uiContract.routes.filter((route) => !enterprisePaths.has(route.path)), ...enterpriseRoutes]
uiContract.rules.enterprise_canonical_routes_required = true
uiContract.rules.product_shell_forbids_data_mode_ui_branch = true
write(contractPath, `${JSON.stringify(uiContract, null, 2)}\n`)

const removeFiles = [
  'src/features/enterprise/pages/CompanyEntryView.vue',
  'src/features/enterprise/pages/CompanyRealView.vue',
  'src/features/enterprise/pages/MembersEntryView.vue',
  'src/features/enterprise/pages/MembersRealView.vue',
  'src/features/enterprise/pages/OrganizationEntryView.vue',
  'src/features/enterprise/pages/OrganizationRealView.vue',
  'src/features/enterprise/pages/PlansEntryView.vue',
  'src/features/enterprise/pages/PlansRealView.vue',
  'src/features/enterprise/pages/RolesEntryView.vue',
  'src/features/enterprise/pages/RolesRealView.vue',
  'src/router/dataMode.ts',
  'src/router/dataMode.spec.ts',
]
for (const path of removeFiles) {
  const file = resolve(root, path)
  if (existsSync(file)) rmSync(file)
}

const serverComponents = resolve(root, 'src/features/enterprise/components/server')
if (existsSync(serverComponents)) rmSync(serverComponents, { recursive: true, force: true })

console.log('Applied one-shot route convergence migration.')
