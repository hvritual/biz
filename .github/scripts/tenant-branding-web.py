from pathlib import Path
import json


def load(path: str) -> str:
    return Path(path).read_text(encoding='utf-8')


def save(path: str, value: str) -> None:
    Path(path).write_text(value, encoding='utf-8')


def replace_once(source: str, old: str, new: str, label: str) -> str:
    count = source.count(old)
    if count != 1:
        raise SystemExit(f'{label}: expected exactly one match, found {count}')
    return source.replace(old, new, 1)

# Enterprise store owns business branding state; ui/base/theme remains the only theme runtime.
path = 'web/src/stores/enterprise.ts'
source = load(path)
source = replace_once(
    source,
    "} from '@/services/enterprise/tenantProfileRuntime'\nimport { loginUrl, type PermissionGrant, type TrustedSession } from '@/services/runtime/api'\n",
    "} from '@/services/enterprise/tenantProfileRuntime'\nimport {\n  getEnterpriseTenantBranding,\n  readEnterpriseTenantBrandingSession,\n  tenantBrandingRequestId,\n  tenantBrandingRuntimeError,\n  updateEnterpriseTenantBranding,\n  type EnterpriseTenantBranding,\n  type EnterpriseTenantBrandingDraft,\n} from '@/services/enterprise/tenantBrandingRuntime'\nimport { loginUrl, type PermissionGrant, type TrustedSession } from '@/services/runtime/api'\n",
    'branding imports',
)
source = replace_once(
    source,
    "  const previewMode = dataSource.kind === 'demo'\n  const sourceKind = dataSource.kind\n  const authenticated = computed(() => previewMode || Boolean(session.value?.authenticated))\n",
    "  const previewMode = dataSource.kind === 'demo'\n  const sourceKind = dataSource.kind\n  const demoBrandingByTenant = new Map<string, EnterpriseTenantBranding>([\n    ['shanghai', { tenantId: 'shanghai', preset: 'blue', primary: '', version: 1, canManage: true }],\n    ['hangzhou', { tenantId: 'hangzhou', preset: 'emerald', primary: '', version: 1, canManage: true }],\n  ])\n  const branding = ref<EnterpriseTenantBranding | null>(previewMode ? { ...demoBrandingByTenant.get(initial.tenantId)! } : null)\n  const brandingLoading = ref(false)\n  const brandingReady = ref(previewMode)\n  const brandingError = ref('')\n  const canManageBranding = computed(() => Boolean(branding.value?.canManage))\n  const authenticated = computed(() => previewMode || Boolean(session.value?.authenticated))\n",
    'branding state',
)
source = replace_once(
    source,
    "  let departmentMutation: { tenantId: string; signature: string; keys: Record<string, string> } | null = null\n\n  function memberMutationSignature",
    "  let departmentMutation: { tenantId: string; signature: string; keys: Record<string, string> } | null = null\n  let brandingMutation: { tenantId: string; signature: string; key: string } | null = null\n\n  async function refreshBranding() {\n    const targetTenant = tenantId.value\n    brandingLoading.value = true\n    brandingError.value = ''\n    try {\n      if (previewMode) {\n        const current = demoBrandingByTenant.get(targetTenant) ?? { tenantId: targetTenant, preset: 'blue', primary: '', version: 1, canManage: true }\n        demoBrandingByTenant.set(targetTenant, current)\n        branding.value = { ...current }\n        brandingReady.value = true\n        return true\n      }\n      const trusted = session.value\n      if (!trusted?.authenticated || !trusted.active_tenant_id || trusted.active_tenant_id !== targetTenant) {\n        branding.value = null\n        brandingReady.value = true\n        return false\n      }\n      const current = await getEnterpriseTenantBranding(trusted)\n      if (tenantId.value !== targetTenant) return false\n      branding.value = current\n      brandingReady.value = true\n      return true\n    } catch (error) {\n      if (tenantId.value === targetTenant) {\n        branding.value = null\n        brandingError.value = tenantBrandingRuntimeError(error)\n        brandingReady.value = true\n      }\n      return false\n    } finally {\n      if (tenantId.value === targetTenant) brandingLoading.value = false\n    }\n  }\n\n  async function stableBrandingSession() {\n    const expected = session.value\n    if (!expected?.authenticated || !expected.active_tenant_id) throw new Error('请先登录并选择可访问租户。')\n    const current = await readEnterpriseTenantBrandingSession()\n    if (!sameTrustedSession(expected, current)) {\n      branding.value = null\n      brandingMutation = null\n      throw new Error('会话或当前租户已变化，请刷新后重新操作。')\n    }\n    return current\n  }\n\n  async function saveBranding(draft: EnterpriseTenantBrandingDraft) {\n    const normalized: EnterpriseTenantBrandingDraft = {\n      preset: draft.preset,\n      primary: draft.preset === 'custom' ? draft.primary.trim().toLowerCase() : '',\n    }\n    if (previewMode) {\n      const current = branding.value ?? { tenantId: tenantId.value, preset: 'blue', primary: '', version: 0, canManage: true }\n      const next: EnterpriseTenantBranding = { ...current, ...normalized, tenantId: tenantId.value, version: Number(current.version) + 1, canManage: true }\n      demoBrandingByTenant.set(tenantId.value, next)\n      branding.value = { ...next }\n      brandingReady.value = true\n      return next\n    }\n    const current = branding.value\n    if (!current) throw new Error('企业品牌主题尚未从服务端加载。')\n    if (!current.canManage) throw new Error('当前账号没有维护企业品牌主题的权限。')\n    const signature = JSON.stringify({ tenantId: tenantId.value, version: current.version, ...normalized })\n    if (!brandingMutation || brandingMutation.tenantId !== tenantId.value || brandingMutation.signature !== signature) {\n      brandingMutation = { tenantId: tenantId.value, signature, key: tenantBrandingRequestId(tenantId.value) }\n    }\n    try {\n      const trusted = await stableBrandingSession()\n      const updated = await updateEnterpriseTenantBranding(trusted, current, normalized, brandingMutation.key)\n      if (updated.tenantId && updated.tenantId !== tenantId.value) throw new Error('企业品牌主题回读不属于当前租户。')\n      branding.value = updated\n      brandingError.value = ''\n      brandingReady.value = true\n      brandingMutation = null\n      return updated\n    } catch (error) {\n      throw new Error(tenantBrandingRuntimeError(error))\n    }\n  }\n\n  function resetBranding() {\n    return saveBranding({ preset: 'blue', primary: '' })\n  }\n\n  function memberMutationSignature",
    'branding business methods',
)
source = replace_once(
    source,
    "      const state = await dataSource.load(tenantId.value || undefined, activeDomains)\n      applySourceState(state, activeDomains)\n      ready.value = true\n",
    "      const previousTenant = tenantId.value\n      const state = await dataSource.load(tenantId.value || undefined, activeDomains)\n      applySourceState(state, activeDomains)\n      if (state.tenantId !== previousTenant) {\n        branding.value = null\n        brandingReady.value = false\n        brandingMutation = null\n        await refreshBranding()\n      }\n      ready.value = true\n",
    'refresh branding lifecycle',
)
source = replace_once(
    source,
    "      departmentMutation = null\n      ready.value = true\n",
    "      departmentMutation = null\n      branding.value = null\n      brandingError.value = ''\n      brandingReady.value = false\n      brandingMutation = null\n      await refreshBranding()\n      ready.value = true\n",
    'switch tenant branding lifecycle',
)
source = replace_once(
    source,
    "    settings,\n    previewMode,\n",
    "    settings,\n    branding,\n    brandingLoading,\n    brandingReady,\n    brandingError,\n    canManageBranding,\n    previewMode,\n",
    'return branding state',
)
source = replace_once(
    source,
    "    saveCompany,\n    saveDepartment,\n",
    "    saveCompany,\n    saveDepartment,\n    refreshBranding,\n    saveBranding,\n    resetBranding,\n",
    'return branding actions',
)
save(path, source)

# AppShell applies server-confirmed branding on active-tenant lifecycle, never from a page mount.
path = 'web/src/features/app-shell/components/AppShell.vue'
source = load(path)
source = replace_once(
    source,
    "import AppIcon from '@/ui/common/AppIcon.vue'\n",
    "import AppIcon from '@/ui/common/AppIcon.vue'\nimport { applyUiTheme, resolveTenantUiTheme } from '@/ui/base/theme'\n",
    'app shell theme import',
)
source = replace_once(
    source,
    "onMounted(() => {\n  viewport()\n",
    "watch(\n  [() => store.tenantId, () => store.branding] as const,\n  ([tenantId, branding]) => {\n    if (!branding || branding.tenantId !== tenantId) {\n      applyUiTheme('blue')\n      return\n    }\n    try {\n      applyUiTheme(resolveTenantUiTheme({ preset: branding.preset, primary: branding.primary || undefined }))\n    } catch {\n      applyUiTheme('blue')\n    }\n  },\n  { immediate: true },\n)\n\nonMounted(() => {\n  viewport()\n",
    'app shell branding watcher',
)
save(path, source)

# Route and enterprise navigation.
path = 'web/src/router/index.ts'
source = load(path)
source = replace_once(
    source,
    "    {\n      path: '/enterprise/logs',\n",
    "    {\n      path: '/enterprise/branding',\n      component: () => import('@/features/enterprise/pages/BrandingView.vue'),\n      meta: { title: '品牌与主题', module: 'enterprise', surface: 'tenant', pageTemplate: 'FormPage' },\n    },\n    {\n      path: '/enterprise/logs',\n",
    'branding route',
)
save(path, source)

path = 'web/src/router/navigation.ts'
source = load(path)
source = replace_once(
    source,
    "  { id: 'company', label: '企业信息', icon: 'company', path: '/enterprise/company' },\n  { id: 'logs', label: '操作日志', icon: 'file', path: '/enterprise/logs' },\n",
    "  { id: 'company', label: '企业信息', icon: 'company', path: '/enterprise/company' },\n  { id: 'branding', label: '品牌与主题', icon: 'settings', path: '/enterprise/branding' },\n  { id: 'logs', label: '操作日志', icon: 'file', path: '/enterprise/logs' },\n",
    'branding navigation',
)
save(path, source)

path = 'web/src/router/navigation.spec.ts'
source = load(path)
source = replace_once(source, "it('keeps enterprise center aligned with the approved six functional entries and terminology'", "it('keeps enterprise center aligned with the approved functional entries and terminology'", 'navigation test title')
source = replace_once(
    source,
    "expect(enterpriseNavigation.map((item) => item.id)).toEqual(['members', 'roles', 'organization', 'plan', 'company', 'logs'])",
    "expect(enterpriseNavigation.map((item) => item.id)).toEqual(['members', 'roles', 'organization', 'plan', 'company', 'branding', 'logs'])",
    'navigation ids',
)
source = replace_once(source, "expect(enterpriseNavigation.map((item) => item.label)).toContain('套餐额度')", "expect(enterpriseNavigation.map((item) => item.label)).toEqual(['成员管理', '角色权限', '组织架构', '套餐额度', '企业信息', '品牌与主题', '操作日志'])", 'navigation labels')
save(path, source)

# i18n extension keeps the large message catalog stable and adds branding as a bounded module.
path = 'web/src/i18n/index.ts'
source = load(path)
source = replace_once(source, "import { memberScopeMessages } from './scope-messages'\n", "import { memberScopeMessages } from './scope-messages'\nimport { brandingMessages } from './branding-messages'\n", 'branding messages import')
source = replace_once(
    source,
    "  'zh-CN': { ...messages['zh-CN'], members: { ...messages['zh-CN'].members, scopes: memberScopeMessages['zh-CN'], profile: memberDetailMessages['zh-CN'] } },\n  'en-US': { ...messages['en-US'], members: { ...messages['en-US'].members, scopes: memberScopeMessages['en-US'], profile: memberDetailMessages['en-US'] } },\n",
    "  'zh-CN': { ...messages['zh-CN'], navigation: { ...messages['zh-CN'].navigation, enterprise: { ...messages['zh-CN'].navigation.enterprise, branding: brandingMessages['zh-CN'].navigation } }, branding: brandingMessages['zh-CN'], members: { ...messages['zh-CN'].members, scopes: memberScopeMessages['zh-CN'], profile: memberDetailMessages['zh-CN'] } },\n  'en-US': { ...messages['en-US'], navigation: { ...messages['en-US'].navigation, enterprise: { ...messages['en-US'].navigation.enterprise, branding: brandingMessages['en-US'].navigation } }, branding: brandingMessages['en-US'], members: { ...messages['en-US'].members, scopes: memberScopeMessages['en-US'], profile: memberDetailMessages['en-US'] } },\n",
    'branding locale merge',
)
save(path, source)

path = 'web/scripts/check-i18n.mjs'
source = load(path)
source = replace_once(source, "  'src/features/enterprise/pages/MembersView.vue',\n", "  'src/features/enterprise/pages/MembersView.vue',\n  'src/features/enterprise/pages/BrandingView.vue',\n", 'i18n branding scope')
save(path, source)

# Page/route contract.
path = 'web/ui-contracts.json'
contract = json.loads(load(path))
if any(route['path'] == '/enterprise/branding' for route in contract['routes']):
    raise SystemExit('branding contract already exists')
company_index = next(i for i, route in enumerate(contract['routes']) if route['path'] == '/enterprise/company')
contract['routes'].insert(company_index + 1, {
    'path': '/enterprise/branding',
    'component': '@/features/enterprise/pages/BrandingView.vue',
    'surface': 'tenant',
    'template': 'FormPage',
    'required_regions': ['page-heading', 'form-workspace', 'form-actions', 'scope'],
})
save(path, json.dumps(contract, ensure_ascii=False, indent=2) + '\n')

path = 'web/e2e/page-patterns.spec.ts'
source = load(path)
source = replace_once(
    source,
    "  { path: '/enterprise/company', pattern: 'FormPage', regions: ['page-heading', 'form-workspace', 'form-actions', 'scope'] },\n",
    "  { path: '/enterprise/company', pattern: 'FormPage', regions: ['page-heading', 'form-workspace', 'form-actions', 'scope'] },\n  { path: '/enterprise/branding', pattern: 'FormPage', regions: ['page-heading', 'form-workspace', 'form-actions', 'scope'] },\n",
    'branding page pattern',
)
save(path, source)

# Read-only users get the authoritative theme without write-shaped controls.
path = 'web/src/features/enterprise/pages/BrandingView.vue'
source = load(path)
source = replace_once(source, '<div class="preset-grid" :aria-label="t(\'branding.presetSection\')">', '<div v-if="canEdit" class="preset-grid" :aria-label="t(\'branding.presetSection\')">', 'readonly preset grid')
source = replace_once(source, '<section class="custom-section">', '<section v-if="canEdit" class="custom-section">', 'readonly custom section')
source = replace_once(
    source,
    "        <div class=\"form-footer\" data-ui-region=\"form-actions\">\n          <span v-if=\"dirty\" class=\"muted flex-1\">{{ t('branding.dirty') }}</span>\n          <UiButton type=\"button\" class=\"btn\" :disabled=\"!canEdit || saving\" @click=\"resetDefault\">\n            {{ t('branding.resetDefault') }}\n          </UiButton>\n          <UiButton type=\"button\" class=\"btn\" :disabled=\"!dirty || saving\" @click=\"cancel\">\n            {{ t('branding.cancel') }}\n          </UiButton>\n          <UiButton class=\"btn btn-primary\" type=\"submit\" :disabled=\"!canEdit || !dirty || !customValid || saving\">\n            <AppIcon name=\"check\" :size=\"15\" />{{ saving ? t('branding.saving') : t('branding.save') }}\n          </UiButton>\n        </div>",
    "        <div class=\"form-footer\" data-ui-region=\"form-actions\">\n          <template v-if=\"canEdit\">\n            <span v-if=\"dirty\" class=\"muted flex-1\">{{ t('branding.dirty') }}</span>\n            <UiButton type=\"button\" class=\"btn\" :disabled=\"saving\" @click=\"resetDefault\">{{ t('branding.resetDefault') }}</UiButton>\n            <UiButton type=\"button\" class=\"btn\" :disabled=\"!dirty || saving\" @click=\"cancel\">{{ t('branding.cancel') }}</UiButton>\n            <UiButton class=\"btn btn-primary\" type=\"submit\" :disabled=\"!dirty || !customValid || saving\"><AppIcon name=\"check\" :size=\"15\" />{{ saving ? t('branding.saving') : t('branding.save') }}</UiButton>\n          </template>\n          <span v-else class=\"muted flex-1\">{{ t('branding.readOnly') }}</span>\n        </div>",
    'readonly form actions',
)
save(path, source)

# Enforce usable primary foreground contrast at the canonical theme authority.
path = 'web/src/ui/base/theme.ts'
source = load(path)
source = replace_once(
    source,
    "  const normalized = normalizeHex(primary)\n  const white = '#ffffff', dark = '#111827'\n  return {\n    primary: normalized,\n    primaryHover: mix(normalized, '#000000', 0.16),\n    primarySoft: mix(normalized, white, 0.92),\n    onPrimary: contrast(normalized, white) >= contrast(normalized, dark) ? white : dark,\n    gradientEnd: mix(normalized, white, 0.58),\n  }\n",
    "  const normalized = normalizeHex(primary)\n  const white = '#ffffff', dark = '#111827'\n  const onPrimary = contrast(normalized, white) >= contrast(normalized, dark) ? white : dark\n  if (contrast(normalized, onPrimary) < 4.5) throw new Error('Brand primary does not provide readable foreground contrast.')\n  return {\n    primary: normalized,\n    primaryHover: mix(normalized, '#000000', 0.16),\n    primarySoft: mix(normalized, white, 0.92),\n    onPrimary,\n    gradientEnd: mix(normalized, white, 0.58),\n  }\n",
    'brand contrast guard',
)
save(path, source)

# Make branding a first-class CoffeeLink qualification slice and visual evidence source.
path = '.github/workflows/coffeelink-web.yml'
source = load(path)
source = replace_once(
    source,
    "          ENTERPRISE_MEMBER_REAL_E2E=1 ENTERPRISE_ROLE_REAL_E2E=1 npx playwright test \\\n            e2e/enterprise-members-real.spec.ts \\\n            e2e/enterprise-roles-real.spec.ts\n",
    "          ENTERPRISE_MEMBER_REAL_E2E=1 ENTERPRISE_ROLE_REAL_E2E=1 ENTERPRISE_BRANDING_REAL_E2E=1 npx playwright test \\\n            e2e/enterprise-members-real.spec.ts \\\n            e2e/enterprise-roles-real.spec.ts \\\n            e2e/enterprise-branding-real.spec.ts\n",
    'branding API e2e command',
)
source = replace_once(
    source,
    "          for title in role_required:\n              assert title in serialized, f'missing EC-RI-03 browser evidence: {title}'\n          print(\n              f'ENTERPRISE_REAL_E2E=PASS expected={stats.get(\"expected\", 0)} '\n              f'ec_ri_02={len(member_required)} ec_ri_03={len(role_required)}'\n          )\n",
    "          branding_required = [\n              'authoritative branding renders and captures all CoffeeLink viewports',\n              'custom preview saves with CAS trusted headers and survives reload only after readback',\n              'cancel restores authoritative theme and tenant switching isolates A and B themes',\n              'read-only member applies server theme without exposing write actions',\n              '401 and 403 branding reads stay explicit and never create writable preview state',\n              '409 preserves draft and retries the same idempotency key',\n              'successful write without branding readback is never presented as confirmed success',\n          ]\n          for title in role_required:\n              assert title in serialized, f'missing EC-RI-03 browser evidence: {title}'\n          for title in branding_required:\n              assert title in serialized, f'missing #107 branding browser evidence: {title}'\n          print(\n              f'ENTERPRISE_REAL_E2E=PASS expected={stats.get(\"expected\", 0)} '\n              f'ec_ri_02={len(member_required)} ec_ri_03={len(role_required)} branding={len(branding_required)}'\n          )\n",
    'branding e2e evidence titles',
)
source = replace_once(source, "          for surface in ['members', 'roles']:\n", "          for surface in ['members', 'roles', 'branding']:\n", 'branding screenshot evidence')
save(path, source)

print('TENANT_BRANDING_WEB_PATCH=PASS store-lifecycle=1 app-shell-authority=1 route=1 contract=1 i18n=1 e2e=1')
