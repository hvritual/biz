import { readdirSync, readFileSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { createHash } from 'node:crypto'
import ts from 'typescript'
import { StaticSource, propertyName, readStrictJson } from './static-source.mjs'

const forbiddenSelectors = new Set(['planName', 'planCode', 'plan_name', 'plan_code', 'planVersion', 'plan_version', 'tenantType', 'tenant_type'])

/** Scope is the router/navigation and canonical authorization projection,
 * not a general JavaScript data-flow proof. Never executes product imports. */
export function accessSelectorFindings(source, file = 'source.ts') {
  const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  if (ast.parseDiagnostics.length) return [`${file}: cannot parse access projection`]
  const findings = []
  const seen = new Set()
  const visit = (node) => {
    let name
    if (ts.isPropertyAccessExpression(node)) name = node.name.text
    else if (ts.isElementAccessExpression(node) && ts.isStringLiteralLike(node.argumentExpression)) name = node.argumentExpression.text
    else if (ts.isBindingElement(node)) name = propertyName(node.propertyName ?? node.name)
    else if (ts.isIdentifier(node) && forbiddenSelectors.has(node.text)) name = node.text
    if (forbiddenSelectors.has(name)) {
      const line = ast.getLineAndCharacterOfPosition(node.getStart(ast)).line + 1
      const finding = `${file}:${line}: plan/tenant-type selector ${name} cannot determine access; use trusted authorization actions`
      if (!seen.has(finding)) { seen.add(finding); findings.push(finding) }
    }
    ts.forEachChild(node, visit)
  }
  visit(ast)
  return findings
}

function sourceFiles(directory) {
  return readdirSync(directory, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name)).flatMap((entry) => {
    const file = join(directory, entry.name)
    if (entry.isSymbolicLink()) throw new Error(`Symlink access source forbidden: ${file}`)
    if (entry.isDirectory()) return sourceFiles(file)
    return /\.ts$/.test(entry.name) && !/\.(spec|test)\.ts$/.test(entry.name) ? [file] : []
  })
}

export function checkCommercialOnboardingUI(webRoot) {
  const root = resolve(webRoot)
  const repository = dirname(root)
  const findings = []
  const indexFile = join(repository, 'contracts/commercial/onboarding.v1.json')
  const planFile = join(repository, 'contracts/generated/operation-plans.json')
  const catalogFile = join(repository, 'contracts/commercial/generated/catalog.json')
  const index = readStrictJson(indexFile)
  const plan = readStrictJson(planFile)
  const catalog = readStrictJson(catalogFile)
  if (index.schema_version !== 1 || plan.schemaVersion !== 2 || catalog.schema_version !== 1) throw new Error('Unsupported onboarding input schema')
  const modules = new Map()
  const owners = new Map(catalog.capabilities.map((capability) => [capability.capability_code, capability.module_code]))
  const knownModules = new Set(owners.values())
  for (const entry of index.modules) {
    if (modules.has(entry.module_code)) throw new Error(`Duplicate module reference ${entry.module_code}`)
    if (!Array.isArray(entry.ui_routes) || new Set(entry.ui_routes).size !== entry.ui_routes.length) throw new Error(`Invalid/duplicate route references for ${entry.module_code}`)
    modules.set(entry.module_code, entry)
  }
  const operations = new Map()
  for (const op of plan.operations) {
    if (operations.has(op.operationId)) throw new Error(`Duplicate OperationPlan ${op.operationId}`)
    operations.set(op.operationId, op)
  }
  const mappings = new Map()
  for (const mapping of catalog.operations) {
    if (mappings.has(mapping.operation_id)) throw new Error(`Duplicate compiled operation ${mapping.operation_id}`)
    mappings.set(mapping.operation_id, mapping)
  }
  const inferSurface = (declared, object = {}) => {
    if (declared === 'tenant' || declared === 'platform') return declared
    if (typeof object.path === 'string' && object.path.startsWith('/platform/')) return 'platform'
    if (object.id === 'platform-commercial') return 'platform'
    return 'tenant'
  }
  const validateActions = (values, location, expectedModule, mode, surface = 'tenant') => {
    const requiredModules = new Set()
    if (!Array.isArray(values) || values.length === 0 || new Set(values).size !== values.length || values.some((value) => typeof value !== 'string' || !value)) {
      findings.push(`${location}: explicit nonempty unique authorizationActions required`)
      return requiredModules
    }
    if (mode !== undefined && mode !== 'all' && mode !== 'any') findings.push(`${location}: unknown authorizationMode`)
    if (expectedModule !== undefined && !knownModules.has(expectedModule)) findings.push(`${location}: unknown authorizationModule ${expectedModule}`)
    for (const action of values) {
      const op = operations.get(action)
      const mapping = mappings.get(action)
      if (!op || !mapping) { findings.push(`${location}: unknown authorization action ${action}`); continue }
      const authentication = Array.isArray(op.security?.authentication) ? op.security.authentication : []
      const webSession = authentication.includes('web-session')
      const bound = Boolean(op.bindings?.rpc || op.bindings?.http?.length)
      if (surface === 'platform') {
        if (op.security?.tenantRequired || !webSession || !bound || mapping.classification !== 'platform_management') {
          findings.push(`${location}: ${action} is not a public platform web action`)
        }
        continue
      }
      if (!op.security?.tenantRequired || !webSession || !bound) {
        findings.push(`${location}: ${action} is not a public tenant action`)
        continue
      }
      if (mapping.classification === 'tenant_business') {
        requiredModules.add(mapping.module_code)
        if (expectedModule && expectedModule !== mapping.module_code) findings.push(`${location}: module ${expectedModule} conflicts with ${action} owner ${mapping.module_code}`)
      }
    }
    return requiredModules
  }
  const reader = new StaticSource(root)
  const routes = reader.router(join(root, 'src/router/index.ts'))
  const routeMap = new Map()
  const consumers = []
  for (const route of routes.filter((entry) => !entry.branch)) {
    if (routeMap.has(route.path)) findings.push(`Duplicate concrete route ${route.path}`)
    routeMap.set(route.path, route)
    if (!route.meta?.authorizationActions) continue
    const needed = validateActions(
      route.meta.authorizationActions,
      route.path,
      route.meta.authorizationModule,
      route.meta.authorizationMode,
      inferSurface(route.meta.surface, route),
    )
    for (const module of needed) {
      if (!modules.get(module)?.ui_routes.includes(route.path)) findings.push(`${route.path}: missing onboarding consumer reference for ${module}`)
    }
    consumers.push({ route: route.path, actions: route.meta.authorizationActions, commercial_modules: [...needed].sort(), surface: route.meta.surface ?? 'unspecified' })
  }
  for (const [module, entry] of modules) {
    for (const path of entry.ui_routes) {
      const route = routeMap.get(path)
      if (!route) { findings.push(`${module}: indexed route ${path} does not exist`); continue }
      const actions = route.meta?.authorizationActions
      if (!Array.isArray(actions) || !actions.some((id) => mappings.get(id)?.classification === 'tenant_business' && mappings.get(id)?.module_code === module)) {
        findings.push(`${module}: indexed route ${path} lacks its commercial authorization action`)
      }
    }
  }
  const files = [...sourceFiles(join(root, 'src/router')), join(root, 'src/services/runtime/authorization.ts')]
  for (const file of files) {
    const context = reader.module(file)
    const local = relative(root, file)
    findings.push(...accessSelectorFindings(context.source, local))
    const visit = (node) => {
      if (ts.isObjectLiteralExpression(node)) {
        const props = new Map(node.properties.filter(ts.isPropertyAssignment).map((item) => [propertyName(item.name), item]))
        if (props.has('authorizationActions') || props.has('authorizationModule')) {
          const value = (key) => props.has(key) ? reader.value(context, props.get(key).initializer) : undefined
          const line = context.ast.getLineAndCharacterOfPosition(node.getStart(context.ast)).line + 1
          const object = {
            id: value('id'),
            path: value('path'),
          }
          validateActions(
            value('authorizationActions'),
            `${local}:${line}`,
            value('authorizationModule'),
            value('authorizationMode'),
            inferSurface(value('surface'), object),
          )
        }
      }
      ts.forEachChild(node, visit)
    }
    visit(context.ast)
  }
  const hashes = {}
  for (const file of new Set([indexFile, planFile, catalogFile, ...files, ...reader.modules.keys()])) {
    hashes[relative(repository, file)] = createHash('sha256').update(readFileSync(file)).digest('hex')
  }
  return {
    check: 'commercial_onboarding_ui_references', result: findings.length ? 'BLOCKED' : 'PASS',
    runtime_verification: 'NOT_PERFORMED', sales_admission: 'NOT_PERFORMED',
    consumers: consumers.sort((a, b) => a.route.localeCompare(b.route)),
    inputs_sha256: hashes, findings: [...new Set(findings)].sort(),
  }
}
