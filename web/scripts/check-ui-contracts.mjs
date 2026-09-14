import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve('.')
const contract = JSON.parse(readFileSync(resolve(root, 'ui-contracts.json'), 'utf8'))
const router = readFileSync(resolve(root, 'src/router/index.ts'), 'utf8')
const navigation = readFileSync(resolve(root, 'src/router/navigation.ts'), 'utf8')
const customerNavigation = readFileSync(resolve(root, 'src/router/customerNavigation.ts'), 'utf8')
const rentalWorkRoutes = readFileSync(resolve(root, 'src/router/rentalWorkRoutes.ts'), 'utf8')
const modulePanel = readFileSync(resolve(root, 'src/features/app-shell/components/ModulePanel.vue'), 'utf8')
const failures = []

function routeBlock(path) {
  const marker = `path: '${path}'`
  const start = router.indexOf(marker)
  if (start < 0) return null
  const next = router.indexOf('\n    {', start + marker.length)
  return router.slice(start, next < 0 ? router.length : next)
}

function componentFile(importPath) {
  return resolve(root, importPath.replace(/^@\//, 'src/'))
}

function escaped(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

const expectedViewports = ['1366x768', '1440x900', '1536x1024', '390x844']
const actualViewports = contract.visual_viewports.map(({ width, height }) => `${width}x${height}`)
if (JSON.stringify(actualViewports) !== JSON.stringify(expectedViewports)) {
  failures.push(`visual viewports must be exactly ${expectedViewports.join(', ')}`)
}

const primaryStart = navigation.indexOf('export const primaryNavigation')
const primaryEnd = navigation.indexOf('export const enterpriseNavigation')
const primaryBlock = navigation.slice(primaryStart, primaryEnd)
const primaryLabels = [...primaryBlock.matchAll(/label: '([^']+)'/g)].map((match) => match[1])
if (JSON.stringify(primaryLabels) !== JSON.stringify(contract.navigation.primary_domains)) {
  failures.push(`primary navigation must be exactly ${contract.navigation.primary_domains.join(' / ')}`)
}

if (contract.rules.navigation_section_groups_required) {
  if (!navigation.includes('group?: string')) {
    failures.push('NavigationItem must expose group metadata for second-level information architecture')
  }
  if (!modulePanel.includes('data-menu-group')) {
    failures.push('ModulePanel must render navigation groups as explicit sections')
  }
  for (const group of contract.navigation.required_section_groups) {
    if (!customerNavigation.includes(`group: '${group}'`)) failures.push(`navigation section group is missing: ${group}`)
  }
}

if (contract.rules.navigation_forbids_entity_bound_paths) {
  for (const token of contract.navigation.forbidden_entity_path_tokens) {
    if (new RegExp(`path:\\s*['\"][^'\"]*${escaped(token)}`).test(customerNavigation)) {
      failures.push(`global navigation cannot bind to sample entity token ${token}`)
    }
  }
}

if (contract.rules.navigation_drink_configuration_domain === 'device-operations') {
  const deviceStart = customerNavigation.indexOf("'device-operations':")
  const businessStart = customerNavigation.indexOf("'business-operations':")
  const deviceBlock = customerNavigation.slice(deviceStart, businessStart)
  const businessBlock = customerNavigation.slice(businessStart)
  if (!deviceBlock.includes("label: '饮品配置'")) failures.push('饮品配置 must belong to device-operations')
  if (businessBlock.includes('饮品配置') || businessBlock.includes('饮品管理')) {
    failures.push('经营管理 must not own device drink configuration')
  }
}

if (contract.rules.navigation_rental_work_collections_required) {
  if (!router.includes("import { rentalWorkRoutes } from './rentalWorkRoutes'")) {
    failures.push('router must import rentalWorkRoutes')
  }
  if (!router.includes('...rentalWorkRoutes')) failures.push('router must mount rentalWorkRoutes')
  if (!rentalWorkRoutes.includes("component: () => import('@/features/customer/pages/CustomerAreaView.vue')")) {
    failures.push('rental work collections must preserve CustomerAreaView action context')
  }
  for (const entry of contract.navigation.rental_work_collections) {
    const navPattern = new RegExp(
      `label:\\s*'${escaped(entry.label)}'[\\s\\S]{0,220}?path:\\s*'${escaped(entry.path)}'`,
    )
    if (!navPattern.test(customerNavigation)) {
      failures.push(`${entry.label}: navigation must target ${entry.path}`)
    }
    const child = entry.path.split('/').filter(Boolean).at(-1)
    const routePattern = new RegExp(
      `path:\\s*'${escaped(child)}'[\\s\\S]{0,260}?title:\\s*'${escaped(entry.label)}'[\\s\\S]{0,120}?workKind:\\s*'${escaped(entry.kind)}'`,
    )
    if (!routePattern.test(rentalWorkRoutes)) {
      failures.push(`${entry.path}: route must map ${entry.label} to workKind=${entry.kind}`)
    }
  }
}

for (const page of contract.routes) {
  const block = routeBlock(page.path)
  if (!block) {
    failures.push(`${page.path}: route is not declared`)
    continue
  }
  if (!block.includes(`import('${page.component}')`)) failures.push(`${page.path}: route must render ${page.component}`)
  if (!block.includes(`pageTemplate: '${page.template}'`)) failures.push(`${page.path}: route must declare pageTemplate ${page.template}`)
  if (!block.includes(`surface: '${page.surface}'`)) failures.push(`${page.path}: route must declare ${page.surface} surface`)

  const file = componentFile(page.component)
  if (!existsSync(file)) {
    failures.push(`${page.path}: component file does not exist: ${page.component}`)
    continue
  }
  const source = readFileSync(file, 'utf8')
  if (!source.includes(`data-ui-template="${page.template}"`)) failures.push(`${page.path}: component must expose data-ui-template=${page.template}`)
  for (const region of page.required_regions) {
    if (!source.includes(`data-ui-region="${region}"`)) failures.push(`${page.path}: missing required UI region ${region}`)
  }
}

if (contract.rules.platform_surface_forbids_runtime_console) {
  const blocks = router.split(/\n    \{/).slice(1)
  for (const block of blocks) {
    if (block.includes("surface: 'platform'") && block.includes('RuntimeConsoleView.vue')) {
      failures.push('platform surface cannot render RuntimeConsoleView.vue')
    }
  }
}

const tenantBlock = routeBlock('/platform/tenants') ?? ''
if (tenantBlock.includes('RuntimeConsoleView.vue')) failures.push('/platform/tenants must never regress to RuntimeConsoleView.vue')

if (failures.length) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log(
  `UI contracts passed: ${contract.routes.length} guarded routes; viewports=${actualViewports.join(',')}; navigation IA and rental collections guarded; platform RuntimeConsole reuse blocked.`,
)
