const templates = new Set(['ListPage', 'WorkbenchPage', 'FormPage', 'MetricsPage'])
const surfaces = new Set(['platform', 'tenant', 'runtime'])
const flags = [
  'platform_surface_forbids_runtime_console', 'navigation_overlay_must_not_push_content',
  'screenshots_are_acceptance_evidence', 'navigation_section_groups_required',
  'navigation_forbids_entity_bound_paths', 'navigation_rental_work_collections_required',
  'enterprise_canonical_routes_required', 'product_shell_forbids_data_mode_ui_branch',
]

function object(value, path, keys) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path}: expected object`)
  for (const key of Object.keys(value)) if (!keys.includes(key)) throw new Error(`${path}: unknown field ${key}`)
}
function text(value, path) {
  if (typeof value !== 'string' || !value.trim()) throw new Error(`${path}: expected non-empty string`)
}
function strings(value, path, empty = false) {
  if (!Array.isArray(value) || (!empty && !value.length)) throw new Error(`${path}: expected ${empty ? '' : 'non-empty '}array`)
  const seen = new Set()
  value.forEach((item) => {
    text(item, path)
    if (seen.has(item)) throw new Error(`${path}: duplicate value ${item}`)
    seen.add(item)
  })
}

export function validateUiContract(contract) {
  object(contract, '$', ['schema_version', 'design_system', 'visual_viewports', 'rules', 'navigation', 'routes', 'patterns'])
  if (contract.schema_version !== 1) throw new Error('$.schema_version: unsupported version')
  text(contract.design_system, '$.design_system')
  if (!Array.isArray(contract.visual_viewports)) throw new Error('$.visual_viewports: expected array')
  const viewports = contract.visual_viewports.map((viewport) => {
    object(viewport, '$.visual_viewports[]', ['width', 'height'])
    if (!Number.isInteger(viewport.width) || !Number.isInteger(viewport.height)) throw new Error('Viewport dimensions must be integers')
    return `${viewport.width}x${viewport.height}`
  })
  if (viewports.join(',') !== '1366x768,1440x900,1536x1024,390x844') throw new Error('The four CoffeeLink acceptance viewports must be preserved')
  object(contract.rules, '$.rules', [...flags, 'navigation_drink_configuration_domain'])
  for (const flag of flags) if (contract.rules[flag] !== true) throw new Error(`$.rules.${flag}: required protection cannot be disabled`)
  if (contract.rules.navigation_drink_configuration_domain !== 'device-operations') throw new Error('Drink configuration must belong to device-operations')
  object(contract.navigation, '$.navigation', [
    'primary_domains', 'primary_domain_ids', 'required_section_groups', 'section_groups',
    'forbidden_entity_path_tokens', 'rental_work_collections',
  ])
  strings(contract.navigation.primary_domains, '$.navigation.primary_domains')
  strings(contract.navigation.primary_domain_ids, '$.navigation.primary_domain_ids')
  if (contract.navigation.primary_domain_ids.length !== contract.navigation.primary_domains.length) throw new Error('Primary navigation IDs and display labels must describe the same entries')
  strings(contract.navigation.required_section_groups, '$.navigation.required_section_groups')
  strings(contract.navigation.forbidden_entity_path_tokens, '$.navigation.forbidden_entity_path_tokens')
  if (!Array.isArray(contract.navigation.rental_work_collections) || !contract.navigation.rental_work_collections.length) throw new Error('Rental work collections must be declared')
  const collectionPaths = new Set()
  for (const collection of contract.navigation.rental_work_collections) {
    object(collection, '$.navigation.rental_work_collections[]', ['id', 'label', 'path', 'kind'])
    for (const key of ['label', 'path', 'kind']) text(collection[key], `rental_work_collections.${key}`)
    if (collection.id !== undefined) text(collection.id, 'rental_work_collections.id')
    if (collectionPaths.has(collection.path)) throw new Error(`Duplicate rental collection ${collection.path}`)
    collectionPaths.add(collection.path)
  }
  if (contract.navigation.section_groups !== undefined) {
    object(contract.navigation.section_groups, '$.navigation.section_groups', Object.keys(contract.navigation.section_groups))
    for (const [key, label] of Object.entries(contract.navigation.section_groups)) {
      if (!/^[a-z][a-z0-9-]*$/.test(key)) throw new Error(`Invalid section group ID ${key}`)
      text(label, `section_groups.${key}`)
    }
  }
  if (!Array.isArray(contract.routes) || !contract.routes.length) throw new Error('$.routes: expected non-empty array')
  const paths = new Set()
  for (const page of contract.routes) {
    object(page, '$.routes[]', ['path', 'component', 'surface', 'template', 'required_regions', 'states', 'purpose', 'exceptions'])
    for (const key of ['path', 'component', 'surface', 'template']) text(page[key], `route.${key}`)
    if (!page.path.startsWith('/') || !page.component.startsWith('@/')) throw new Error(`Invalid route/component reference: ${page.path}`)
    if (paths.has(page.path)) throw new Error(`Duplicate page contract: ${page.path}`)
    paths.add(page.path)
    if (!surfaces.has(page.surface)) throw new Error(`Unknown surface: ${page.surface}`)
    if (!templates.has(page.template)) throw new Error(`Unknown page template: ${page.template}`)
    strings(page.required_regions, `${page.path}.required_regions`, true)
    if (page.states !== undefined) strings(page.states, `${page.path}.states`)
    if (page.purpose !== undefined) text(page.purpose, `${page.path}.purpose`)
    if (page.exceptions !== undefined) {
      if (!Array.isArray(page.exceptions)) throw new Error(`${page.path}.exceptions must be an array`)
      for (const exception of page.exceptions) {
        object(exception, 'exception', ['rule', 'reason', 'evidence', 'review_condition'])
        for (const key of ['rule', 'reason', 'evidence', 'review_condition']) text(exception[key], `exception.${key}`)
      }
    }
  }
  if (contract.patterns !== undefined) {
    object(contract.patterns, '$.patterns', [...templates])
    for (const [name, pattern] of Object.entries(contract.patterns)) {
      object(pattern, `patterns.${name}`, ['status', 'use_when', 'avoid_when', 'required_regions', 'optional_regions', 'states', 'examples'])
      if (!['implemented', 'reserved'].includes(pattern.status)) throw new Error(`Invalid pattern status: ${name}`)
      for (const key of ['use_when', 'avoid_when']) text(pattern[key], `${name}.${key}`)
      for (const key of ['required_regions', 'optional_regions', 'states', 'examples']) strings(pattern[key], `${name}.${key}`, true)
      for (const example of pattern.examples) if (!paths.has(example)) throw new Error(`Pattern example has no page contract: ${example}`)
      if (pattern.status === 'implemented' && !pattern.examples.length) throw new Error(`Implemented pattern has no real consumer: ${name}`)
    }
  }
  return contract
}
