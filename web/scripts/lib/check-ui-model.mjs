import { validatePatternBindings } from './pattern-contracts.mjs'
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { basename, relative, resolve } from 'node:path'
import { StaticSource, componentFile, readStrictJson } from './static-source.mjs'
import { validateUiContract } from './ui-contract-schema.mjs'
import { componentSources, readVue, verifyPageSource } from './vue-source.mjs'

const engineeringLanguagePatterns = [
  ['data mode badge', /实时数据|Live data/gi],
  ['runtime implementation status', /可信运行会话|Trusted runtime session|Runtime Workspace/gi],
  ['API mode', /API\s*模式|API mode/gi],
  ['server implementation', /服务端|from the server|by the server|loading server|server data|server identity|server records?|server rules?|server readback|server contract|server confirmed/gi],
  ['readback implementation', /回读|readback/gi],
  ['authority implementation', /权威|authoritative/gi],
  ['local implementation preview', /本地预览|本地模拟|local preview|local mock/gi],
  ['data source implementation', /数据源状态|data source status/gi],
  ['internal lifecycle terminology', /preview\s*\/\s*confirm\s*\/\s*receipt|\bmeter\b/gi],
  ['internal transport field', /请求\s*ID|幂等|会话引用|回执引用|请求摘要|request\s*ID|idempotency|session reference|receipt reference|request digest/gi],
]

function sourceFilesUnder(directory, files = []) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name)
    if (entry.isDirectory()) sourceFilesUnder(path, files)
    else files.push(path)
  }
  return files
}

function visibleSource(file) {
  if (file.endsWith('.vue')) return readVue(file).descriptor.template?.content ?? ''
  const source = readFileSync(file, 'utf8')
  return source.match(/'(?:\\.|[^'\\])*'|"(?:\\.|[^"\\])*"|`(?:\\.|[^`\\])*`/g)?.join('\n') ?? ''
}

function productLanguageFailures(root) {
  const featuresRoot = resolve(root, 'src/features')
  const i18nRoot = resolve(root, 'src/i18n')
  const files = [
    ...sourceFilesUnder(featuresRoot).filter((file) => {
      const normalized = file.replaceAll('\\', '/')
      return (file.endsWith('.vue') || file.endsWith('.ts'))
        && !normalized.includes('/features/runtime/')
        && !/\.(?:spec|test)\.ts$/.test(normalized)
    }),
    ...sourceFilesUnder(i18nRoot).filter((file) => file.endsWith('.ts')),
  ]
  const failures = []
  for (const file of files) {
    const source = visibleSource(file)
    for (const [label, pattern] of engineeringLanguagePatterns) {
      pattern.lastIndex = 0
      if (pattern.test(source)) failures.push(`${relative(root, file)}: product UI exposes engineering language (${label})`)
    }
  }
  const sourceBanner = resolve(root, 'src/features/enterprise/components/EnterpriseSourceBanner.vue')
  if (existsSync(sourceBanner)) failures.push('EnterpriseSourceBanner must not exist on product surfaces')
  return failures
}

export function checkUiModel(root) {
  const contract = validateUiContract(readStrictJson(resolve(root, 'ui-contracts.json')))
  validatePatternBindings(contract)
  const reader = new StaticSource(root)
  const routes = reader.router(resolve(root, 'src/router/index.ts'))
  const navigationFile = resolve(root, 'src/router/navigation.ts')
  const primary = reader.exported(navigationFile, 'primaryNavigation')
  const domains = reader.exported(resolve(root, 'src/router/customerNavigation.ts'), 'customerDomains')
  const failures = []
  const same = (actual, expected) => JSON.stringify(actual) === JSON.stringify(expected)
  if (!same(primary.map((item) => item.id), contract.navigation.primary_domain_ids)) failures.push('Primary navigation stable IDs differ from the contract')

  const activeDomains = Object.values(domains)
  const allLinks = activeDomains.flatMap((domain) => domain.links)
  const allActions = activeDomains.flatMap((domain) => domain.actions)
  const groupIds = new Set(allLinks.map((item) => item.groupId).filter(Boolean))
  const expectedGroups = contract.navigation.section_groups
  if (!expectedGroups || !Object.keys(expectedGroups).length) failures.push('Stable section group IDs must be declared')
  else {
    for (const id of Object.keys(expectedGroups)) if (!groupIds.has(id)) failures.push(`Missing navigation section group ${id}`)
    for (const item of allLinks) if (item.group && (!item.groupId || !Object.hasOwn(expectedGroups, item.groupId))) failures.push(`Unknown/missing section group ID for ${item.id}`)
  }
  const panel = readVue(resolve(root, 'src/features/app-shell/components/ModulePanel.vue'))
  // Vue bind directives are executable attributes too; comments and script text are excluded.
  let groupMarker = false
  const visit = (node) => {
    if (node.type === 1 && node.props?.some((property) => property.type === 6 && property.name === 'data-menu-group' || property.type === 7 && property.name === 'bind' && property.arg?.content === 'data-menu-group')) groupMarker = true
    for (const child of node.children ?? []) visit(child)
  }
  if (panel.descriptor.template?.ast) visit(panel.descriptor.template.ast)
  if (!groupMarker) failures.push('ModulePanel must render navigation groups')
  for (const item of [...allLinks, ...allActions]) {
    if (item.path && contract.navigation.forbidden_entity_path_tokens.some((token) => item.path.includes(token))) failures.push(`Global navigation binds a sample entity: ${item.path}`)
  }
  if (!domains['device-operations']?.links.some((item) => item.id === 'drinks')) failures.push('Drink configuration must remain in device-operations')
  if (domains['business-operations']?.links.some((item) => item.id === 'drinks')) failures.push('Business operations cannot own device drink configuration')
  for (const collection of contract.navigation.rental_work_collections) {
    const link = domains['rental-operations']?.links.find((item) => item.id === collection.id)
    if (link?.path !== collection.path) failures.push(`Rental navigation ${collection.id} must target ${collection.path}`)
    const route = routes.find((item) => item.path === collection.path && !item.branch)
    if (route?.meta.workKind !== collection.kind) failures.push(`${collection.path}: workKind must be ${collection.kind}`)
  }
  const rentalRoot = routes.find((route) => route.path === '/rental' && route.branch)
  if (componentFile(rentalRoot?.component) !== reader.file('@/features/customer/pages/CustomerAreaView.vue')) failures.push('Rental collection must preserve its CustomerArea action context')

  const pageContracts = new Map(contract.routes.map((page) => [page.path, page]))
  const leafPaths = new Set()
  for (const route of routes.filter((item) => !item.branch)) {
    if (leafPaths.has(route.path)) failures.push(`Duplicate concrete route: ${route.path}`)
    leafPaths.add(route.path)
    const corePrefix = route.path.startsWith('/platform/') || route.path.startsWith('/enterprise/') || route.path.startsWith('/system/')
    if (!route.redirect && (corePrefix || ['platform', 'tenant'].includes(route.meta.surface) || route.meta.pageTemplate)) {
      if (!pageContracts.has(route.path)) failures.push(`${route.path}: business route is missing its page contract`)
      if (!componentFile(route.component)) failures.push(`${route.path}: component is not a statically resolved Vue loader`)
    }
    if (!route.redirect && route.meta.surface !== 'runtime' && componentFile(route.component)) {
      const dependencies = componentSources(reader, componentFile(route.component))
      if (dependencies.some((source) => basename(source.file) === 'RuntimeConsoleView.vue')) failures.push(`${route.path}: non-runtime route cannot render RuntimeConsole`)
    }
  }
  // This is a regression invariant, not a replacement route registry.
  if (!pageContracts.has('/platform/tenants') || !leafPaths.has('/platform/tenants')) failures.push('The platform tenant management route cannot be removed')
  for (const page of contract.routes) {
    const route = routes.find((item) => item.path === page.path && !item.redirect)
    if (!route) { failures.push(`${page.path}: contracted route is missing`); continue }
    if (route.meta.surface !== page.surface) failures.push(`${page.path}: surface must be ${page.surface}`)
    if (route.meta.pageTemplate !== page.template) failures.push(`${page.path}: pageTemplate must be ${page.template}`)
    failures.push(...verifyPageSource(reader, page, componentFile(route.component)))
  }
  failures.push(...productLanguageFailures(root))
  return { failures: [...new Set(failures)], contract, routes, sourceCount: reader.modules.size }
}
