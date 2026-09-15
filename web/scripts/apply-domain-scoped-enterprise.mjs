import { readFileSync, writeFileSync, rmSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve('.')
const read = (path) => readFileSync(resolve(root, path), 'utf8')
const write = (path, content) => writeFileSync(resolve(root, path), content)
function replaceOnce(source, from, to, path) {
  if (!source.includes(from)) throw new Error(`${path}: expected token not found: ${from.slice(0, 100)}`)
  return source.replace(from, to)
}
function migrate(path, transform) {
  const source = read(path)
  const next = transform(source)
  if (source === next) throw new Error(`${path}: no migration change`)
  write(path, next)
}

migrate('src/stores/enterprise.ts', (source) => {
  let next = replaceOnce(
    source,
    "  type EnterpriseSourceState,\n} from '@/services/enterprise/dataSource'",
    "  type EnterpriseDomain,\n  type EnterpriseSourceState,\n} from '@/services/enterprise/dataSource'",
    'enterprise.ts import',
  )
  next = replaceOnce(
    next,
    "  let lastPersisted = JSON.stringify(snapshot.value)\n\n  function applySourceState(state: EnterpriseSourceState) {\n    tenantId.value = state.tenantId\n    snapshot.value = state.snapshot\n    session.value = state.session\n    lastPersisted = JSON.stringify(state.snapshot)\n  }",
    "  let lastPersisted = JSON.stringify(snapshot.value)\n  let activeDomains: EnterpriseDomain[] = []\n\n  function applySourceState(state: EnterpriseSourceState, domains = state.loadedDomains, replace = false) {\n    tenantId.value = state.tenantId\n    session.value = state.session\n    if (previewMode || replace) {\n      snapshot.value = state.snapshot\n    } else {\n      const current = snapshot.value\n      snapshot.value = {\n        ...current,\n        members: domains.includes('members') ? state.snapshot.members : current.members,\n        roles: domains.includes('roles') ? state.snapshot.roles : current.roles,\n        departments: domains.includes('departments') ? state.snapshot.departments : current.departments,\n        company: domains.includes('company') ? state.snapshot.company : current.company,\n      }\n    }\n    lastPersisted = JSON.stringify(snapshot.value)\n  }",
    'enterprise.ts applySourceState',
  )
  next = replaceOnce(
    next,
    "  async function refresh() {\n    loading.value = true\n    sourceError.value = ''\n    try {\n      applySourceState(await dataSource.load(tenantId.value || undefined))\n      ready.value = true\n    } catch (error) {\n      sourceError.value = errorMessage(error)\n      ready.value = true\n      throw error\n    } finally {\n      loading.value = false\n    }\n  }",
    "  async function refresh(domains: EnterpriseDomain[] = activeDomains) {\n    activeDomains = [...new Set(domains)]\n    loading.value = true\n    sourceError.value = ''\n    try {\n      const state = await dataSource.load(tenantId.value || undefined, activeDomains)\n      applySourceState(state, activeDomains)\n      ready.value = true\n    } catch (error) {\n      sourceError.value = errorMessage(error)\n      ready.value = true\n      throw error\n    } finally {\n      loading.value = false\n    }\n  }\n\n  async function ensureDomains(domains: EnterpriseDomain[]) {\n    activeDomains = [...new Set(domains)]\n    await refresh(activeDomains)\n  }",
    'enterprise.ts refresh',
  )
  next = replaceOnce(
    next,
    "      applySourceState(await dataSource.switchTenant(id))\n      ready.value = true",
    "      const state = await dataSource.switchTenant(id, activeDomains)\n      applySourceState(state, activeDomains, true)\n      ready.value = true",
    'enterprise.ts switchTenant',
  )
  next = next.replace("  if (!previewMode) void refresh().catch(() => undefined)", "  if (!previewMode) void refresh([]).catch(() => undefined)")
  next = replaceOnce(next, "    refresh,\n    switchTenant,", "    refresh,\n    ensureDomains,\n    switchTenant,", 'enterprise.ts return')
  return next
})

const pageSpecs = [
  ['src/features/enterprise/pages/MembersView.vue', "import { computed, ref, watch } from 'vue'", "import { computed, onMounted, ref, watch } from 'vue'", '<div class="members-view" data-ui-template="ListPage"', '<div class="members-view" data-enterprise-page="members" data-ui-template="ListPage"', "onMounted(() => void store.ensureDomains(['members', 'roles']).catch(() => undefined))\n"],
  ['src/features/enterprise/pages/RolesView.vue', "import { computed, ref, watch } from 'vue'", "import { computed, onMounted, ref, watch } from 'vue'", '<div class="page-stack" data-ui-template="ListPage">', '<div class="page-stack" data-enterprise-page="roles" data-ui-template="ListPage">', "onMounted(() => void store.ensureDomains(['roles', 'members']).catch(() => undefined))\n"],
  ['src/features/enterprise/pages/OrganizationView.vue', "import { computed, ref, watch } from 'vue'", "import { computed, onMounted, ref, watch } from 'vue'", '<div class="page-stack" data-ui-template="WorkbenchPage">', '<div class="page-stack" data-enterprise-page="organization" data-ui-template="WorkbenchPage">', "onMounted(() => void store.ensureDomains(['departments', 'members']).catch(() => undefined))\n"],
  ['src/features/enterprise/pages/CompanyView.vue', "import { computed, ref, watch } from 'vue'", "import { computed, onMounted, ref, watch } from 'vue'", '<div class="page-stack" data-ui-template="FormPage">', '<div class="page-stack" data-enterprise-page="company" data-ui-template="FormPage">', "onMounted(() => void store.ensureDomains(['company']).catch(() => undefined))\n"],
]
for (const [path, oldImport, newImport, oldRoot, newRoot, mounted] of pageSpecs) {
  migrate(path, (source) => {
    let next = replaceOnce(source, oldImport, newImport, `${path} vue import`)
    next = replaceOnce(next, oldRoot, newRoot, `${path} root`)
    next = replaceOnce(next, '</script>', `${mounted}</script>`, `${path} mounted`)
    return next
  })
}

migrate('src/features/enterprise/pages/PlansView.vue', (source) => replaceOnce(
  source,
  '<div class="page-stack" data-ui-template="WorkbenchPage">',
  '<div class="page-stack" data-enterprise-page="plan" data-ui-template="WorkbenchPage">',
  'PlansView root',
))

const testContracts = [
  ['e2e/enterprise-members-real.spec.ts', '[data-enterprise-member-source="server"]', '[data-enterprise-page="members"]'],
  ['e2e/enterprise-roles-real.spec.ts', '[data-enterprise-role-source="server"]', '[data-enterprise-page="roles"]'],
  ['e2e/enterprise-organization-real.spec.ts', '[data-enterprise-organization-source="server"]', '[data-enterprise-page="organization"]'],
  ['e2e/enterprise-company-real.spec.ts', '[data-enterprise-company-source="server"]', '[data-enterprise-page="company"]'],
  ['e2e/enterprise-plan-real.spec.ts', '[data-enterprise-plan-source="server"]', '[data-enterprise-page="plan"]'],
  ['e2e/enterprise-plan-change-real.spec.ts', '[data-enterprise-plan-source="server"]', '[data-enterprise-page="plan"]'],
]
for (const [path, oldSelector, canonicalSelector] of testContracts) {
  migrate(path, (source) => {
    const oldLine = `await expect(page.locator('${oldSelector}')).toBeVisible()`
    const newLine = `await expect(page.locator('${canonicalSelector}')).toBeVisible()\n  await expect(page.locator('[data-enterprise-source="api"]')).toBeVisible()`
    return source.replaceAll(oldLine, newLine)
  })
}

rmSync(resolve(root, 'scripts/apply-domain-scoped-enterprise.mjs'))
rmSync(resolve(root, '../.github/workflows/ui-route-domain-migration.yml'))
console.log('Applied domain-scoped canonical enterprise migration and removed one-shot files.')
