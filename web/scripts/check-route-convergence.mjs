import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { basename, join, relative, resolve } from 'node:path'

const root = resolve('.')
const repoRoot = resolve(root, '..')
const contractPath = resolve(repoRoot, 'docs/design/ui-route-convergence/ui-route-contract.json')
const failures = []

if (!existsSync(contractPath)) {
  console.error(`route convergence contract missing: ${contractPath}`)
  process.exit(1)
}

const contract = JSON.parse(readFileSync(contractPath, 'utf8'))
const routerPath = resolve(root, 'src/router/index.ts')
const router = readFileSync(routerPath, 'utf8')

function walk(dir) {
  const entries = []
  for (const item of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, item.name)
    if (item.isDirectory()) entries.push(...walk(path))
    else entries.push(path)
  }
  return entries
}

function routeBlock(path) {
  const marker = `path: '${path}'`
  const start = router.indexOf(marker)
  if (start < 0) return null
  const next = router.indexOf('\n    {', start + marker.length)
  return router.slice(start, next < 0 ? router.length : next)
}

const featureRoot = resolve(root, 'src/features')
const featureFiles = walk(featureRoot).filter((path) => path.endsWith('.vue'))
const pageFiles = featureFiles.filter((path) => path.includes('/pages/'))
const shellFiles = featureFiles.filter((path) => path.includes('/features/app-shell/'))

for (const file of pageFiles) {
  const name = basename(file)
  const rel = relative(root, file)
  for (const suffix of contract.forbidden_page_suffixes) {
    if (name.endsWith(suffix)) failures.push(`${rel}: forbidden parallel page suffix ${suffix}`)
  }
  const source = readFileSync(file, 'utf8')
  if (source.includes('VITE_DATA_MODE')) failures.push(`${rel}: page must not select UI using VITE_DATA_MODE`)
  if (source.includes('RuntimeConsoleView.vue') && !rel.includes('features/runtime/pages/')) {
    failures.push(`${rel}: business page must not embed RuntimeConsoleView.vue`)
  }
}

for (const file of shellFiles) {
  const source = readFileSync(file, 'utf8')
  if (source.includes('VITE_DATA_MODE')) {
    failures.push(`${relative(root, file)}: product shell must not branch product structure using VITE_DATA_MODE`)
  }
}

if (router.includes('VITE_DATA_MODE')) failures.push('src/router/index.ts: router must not select components using VITE_DATA_MODE')

for (const expected of contract.canonical_routes) {
  const block = routeBlock(expected.path)
  if (!block) {
    failures.push(`${expected.path}: canonical route missing`)
    continue
  }
  if (!block.includes(`import('${expected.component}')`)) {
    failures.push(`${expected.path}: route must render ${expected.component}`)
  }
  if (!block.includes(`surface: '${expected.surface}'`)) {
    failures.push(`${expected.path}: route must declare surface=${expected.surface}`)
  }
  if (!block.includes(`pageTemplate: '${expected.template}'`)) {
    failures.push(`${expected.path}: route must declare pageTemplate=${expected.template}`)
  }
  if (block.includes('EntryView.vue') || block.includes('RealView.vue') || block.includes('DemoView.vue')) {
    failures.push(`${expected.path}: route must not pass through entry/real/demo page variants`)
  }
}

const routeBlocks = router.split(/\n    \{/).slice(1)
for (const block of routeBlocks) {
  if (!block.includes('RuntimeConsoleView.vue')) continue
  const runtimeSurface = contract.runtime_console_allowed_surfaces.some((surface) => block.includes(`surface: '${surface}'`))
  if (!runtimeSurface) failures.push('RuntimeConsoleView.vue may only be mounted on an explicitly runtime surface')
}

const enterpriseStorePath = resolve(root, 'src/stores/enterprise.ts')
if (existsSync(enterpriseStorePath)) {
  const source = readFileSync(enterpriseStorePath, 'utf8')
  if (source.includes("from '@/services/demo/repository'")) {
    failures.push('enterprise store must not import the demo repository directly; use createEnterpriseDataSource')
  }
  if (!source.includes('createEnterpriseDataSource')) {
    failures.push('enterprise store must select demo/api through the enterprise data-source boundary')
  }
}

const dataSourcePath = resolve(root, 'src/services/enterprise/dataSource.ts')
if (!existsSync(dataSourcePath)) failures.push('enterprise data-source boundary is missing')
else if (!readFileSync(dataSourcePath, 'utf8').includes('VITE_DATA_MODE')) {
  failures.push('enterprise data-source boundary must own data-mode selection')
}

if (failures.length) {
  console.error(`Route convergence gate failed (${failures.length}):`)
  for (const failure of failures) console.error(`- ${failure}`)
  process.exit(1)
}

console.log(
  `Route convergence passed: ${contract.canonical_routes.length} canonical enterprise routes; product shell has no data-mode UI branch; no parallel page trees detected.`,
)
