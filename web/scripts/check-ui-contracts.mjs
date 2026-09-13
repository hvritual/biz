import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve('.')
const contract = JSON.parse(readFileSync(resolve(root, 'ui-contracts.json'), 'utf8'))
const router = readFileSync(resolve(root, 'src/router/index.ts'), 'utf8')
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

const expectedViewports = ['1366x768', '1440x900', '1536x1024', '390x844']
const actualViewports = contract.visual_viewports.map(({ width, height }) => `${width}x${height}`)
if (JSON.stringify(actualViewports) !== JSON.stringify(expectedViewports)) {
  failures.push(`visual viewports must be exactly ${expectedViewports.join(', ')}`)
}

for (const page of contract.routes) {
  const block = routeBlock(page.path)
  if (!block) {
    failures.push(`${page.path}: route is not declared`)
    continue
  }
  if (!block.includes(`import('${page.component}')`)) {
    failures.push(`${page.path}: route must render ${page.component}`)
  }
  if (!block.includes(`pageTemplate: '${page.template}'`)) {
    failures.push(`${page.path}: route must declare pageTemplate ${page.template}`)
  }
  if (!block.includes(`surface: '${page.surface}'`)) {
    failures.push(`${page.path}: route must declare ${page.surface} surface`)
  }

  const file = componentFile(page.component)
  if (!existsSync(file)) {
    failures.push(`${page.path}: component file does not exist: ${page.component}`)
    continue
  }
  const source = readFileSync(file, 'utf8')
  if (!source.includes(`data-ui-template="${page.template}"`)) {
    failures.push(`${page.path}: component must expose data-ui-template=${page.template}`)
  }
  for (const region of page.required_regions) {
    if (!source.includes(`data-ui-region="${region}"`)) {
      failures.push(`${page.path}: missing required UI region ${region}`)
    }
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
if (tenantBlock.includes('RuntimeConsoleView.vue')) {
  failures.push('/platform/tenants must never regress to RuntimeConsoleView.vue')
}

if (failures.length) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log(
  `UI contracts passed: ${contract.routes.length} guarded routes; viewports=${actualViewports.join(',')}; platform RuntimeConsole reuse blocked.`,
)
