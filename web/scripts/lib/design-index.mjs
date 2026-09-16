import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readdirSync, readFileSync, renameSync, unlinkSync, writeFileSync } from 'node:fs'
import { basename, dirname, relative, resolve } from 'node:path'
import { StaticSource, readStrictJson } from './static-source.mjs'
import { componentSources, readVue } from './vue-source.mjs'
import { componentApi } from './component-api.mjs'
import { validateUiContract } from './ui-contract-schema.mjs'

const version = 1
const portable = (path) => path.replaceAll('\\', '/')
const hash = (data) => createHash('sha256').update(data).digest('hex')
const compare = (a, b) => a < b ? -1 : a > b ? 1 : 0
function walk(dir) {
  if (!existsSync(dir)) return []
  return readdirSync(dir, { withFileTypes: true }).sort((a, b) => compare(a.name, b.name)).flatMap((entry) => {
    if (entry.isSymbolicLink()) throw new Error(`Index sources cannot be symlinks: ${entry.name}`)
    const path = resolve(dir, entry.name)
    return entry.isDirectory() ? walk(path) : [path]
  })
}
export function indexSources(root) {
  const files = [...walk(resolve(root, 'src')), ...walk(resolve(root, 'scripts/lib')),
    resolve(root, 'ui-contracts.json'), resolve(root, 'package.json'), resolve(root, 'package-lock.json')]
    .filter((file) => /\.(?:vue|ts|mjs|css|json)$/.test(file) && !file.endsWith('.spec.ts'))
  return [...new Set(files)].sort(compare).map((file) => ({ path: portable(relative(root, file)), sha256: hash(readFileSync(file)) }))
}
function semantics(value, id, root) {
  if (!value) return { status: 'needs-review', aliases: [], useWhen: [], avoidWhen: [], examples: [] }
  const allowed = new Set(['scenario', 'scope', 'aliases', 'useWhen', 'avoidWhen', 'status', 'examples'])
  for (const key of Object.keys(value)) if (!allowed.has(key)) throw new Error(`${id}: metadata cannot define ${key}; API fields come from source`)
  for (const key of ['scenario', 'scope']) if (typeof value[key] !== 'string' || !value[key].trim()) throw new Error(`${id}: missing semantic ${key}`)
  const result = { ...value, status: value.status ?? 'needs-review' }
  if (!['implemented', 'preview', 'deprecated', 'needs-review'].includes(result.status)) throw new Error(`${id}: invalid status`)
  for (const key of ['aliases', 'useWhen', 'avoidWhen', 'examples']) {
    result[key] = value[key] ?? []
    if (!Array.isArray(result[key]) || result[key].some((item) => typeof item !== 'string' || !item.trim())) throw new Error(`${id}: invalid ${key}`)
    if (new Set(result[key]).size !== result[key].length) throw new Error(`${id}: duplicate ${key}`)
  }
  for (const example of result.examples) {
    if (!example.startsWith('src/') || example.includes('..') || !existsSync(resolve(root, example))) throw new Error(`${id}: missing/unsafe example ${example}`)
  }
  return result
}
function visibleLabels(descriptor) {
  const labels = new Set()
  const visit = (node) => {
    if (node.type === 1) for (const property of node.props ?? []) {
      if (property.type === 6 && ['title', 'label', 'aria-label', 'description'].includes(property.name) && property.value?.content) labels.add(property.value.content)
    }
    for (const child of node.children ?? []) visit(child)
  }
  if (descriptor.template?.ast) visit(descriptor.template.ast)
  return [...labels].sort(compare).slice(0, 24)
}

export function buildDesignIndex(root) {
  const sources = indexSources(root)
  const reader = new StaticSource(root)
  const contract = validateUiContract(readStrictJson(resolve(root, 'ui-contracts.json')))
  const scopes = readStrictJson(resolve(root, 'src/features/component-scopes.json'))
  const files = sources.filter((source) => source.path.endsWith('.vue'))
  const fileSet = new Set(files.map((source) => source.path))
  for (const scope of Object.keys(scopes)) if (!fileSet.has(`src/features/${scope}`)) throw new Error(`Stale component scope: ${scope}`)
  const entries = []
  for (const item of files) {
    const full = resolve(root, item.path)
    const { source, descriptor } = readVue(full)
    const isFeature = item.path.startsWith('src/features/')
    const domain = isFeature ? item.path.split('/')[2] : 'shared'
    const isPage = item.path.includes('/pages/')
    const semantic = semantics(scopes[item.path.replace(/^src\/features\//, '')], item.path, root)
    const api = componentApi(descriptor, full)
    const dependencies = componentSources(reader, full).filter((child) => child.file !== full).map((child) => portable(relative(root, child.file))).sort(compare)
    for (const dependency of dependencies) if (!fileSet.has(dependency)) throw new Error(`${item.path}: unresolved component dependency ${dependency}`)
    const layer = item.path.startsWith('src/ui/base/') ? 'base' : item.path.startsWith('src/ui/common/') ? 'common' : isPage ? 'page' : isFeature ? 'feature' : 'composition'
    if (layer === 'base' && dependencies.some((path) => path.startsWith('src/features/') || path.startsWith('src/ui/common/'))) throw new Error(`${item.path}: reverse base dependency`)
    if (layer === 'common' && dependencies.some((path) => path.startsWith('src/features/'))) throw new Error(`${item.path}: reverse common dependency`)
    entries.push({
      id: item.path.replace(/^src\//, '').replace(/\.vue$/, ''), name: basename(item.path, '.vue'),
      kind: isPage ? 'page' : 'component', layer, domain,
      importPath: `@/${item.path.slice(4)}`, ...semantic,
      api, dependencies, consumers: [], labels: visibleLabels(descriptor),
      routes: contract.routes.filter((page) => page.component === `@/${item.path.slice(4)}`).map((page) => page.path),
      source: { path: item.path, sha256: item.sha256, startLine: 1 },
      excerpt: source.split('\n').slice(0, 18).join('\n'),
      businessReadiness: 'not-inferred-from-ui-source',
    })
  }
  for (const entry of entries) entry.consumers = entries.filter((candidate) => candidate.dependencies.includes(entry.source.path)).map((candidate) => candidate.source.path).sort(compare)
  for (const name of ['ListPage', 'WorkbenchPage', 'FormPage', 'MetricsPage']) {
    const definition = contract.patterns?.[name]
    const pages = contract.routes.filter((page) => page.template === name)
    entries.push({ id: `pattern/${name}`, name, kind: 'pattern', layer: 'pattern', domain: 'shared',
      status: definition?.status ?? (pages.length ? 'implemented' : 'reserved'),
      useWhen: definition?.use_when ? [definition.use_when] : [], avoidWhen: definition?.avoid_when ? [definition.avoid_when] : [],
      routes: pages.map((page) => page.path), requiredRegions: definition?.required_regions ?? [],
      source: sources.find((source) => source.path === 'ui-contracts.json'),
      note: 'Pattern status describes UI composition, not availability of backend operations.',
    })
  }
  entries.sort((a, b) => compare(a.id, b.id))
  if (!files.length || !entries.length || new Set(entries.map((entry) => entry.id)).size !== entries.length) throw new Error('Index must contain unique, non-empty source entries')
  const fingerprint = hash(JSON.stringify(sources))
  return { schemaVersion: version, generatorVersion: version, sourceFingerprint: fingerprint, sources, entries,
    summary: { components: entries.filter((entry) => entry.kind === 'component').length, pages: entries.filter((entry) => entry.kind === 'page').length, patterns: 4, unresolvedDeclarations: entries.reduce((sum, entry) => sum + (entry.api?.unknown.length ?? 0), 0) } }
}

export function indexPath(root) { return resolve(root, '.cache/design-index.json') }
export function writeDesignIndex(root) {
  const index = buildDesignIndex(root)
  const file = indexPath(root)
  mkdirSync(dirname(file), { recursive: true })
  const temporary = `${file}.${process.pid}.tmp`
  try { writeFileSync(temporary, JSON.stringify(index, null, 2) + '\n'); renameSync(temporary, file) }
  finally { if (existsSync(temporary)) unlinkSync(temporary) }
  return index
}
export function readFreshIndex(root) {
  const file = indexPath(root)
  if (!existsSync(file)) throw new Error('Design index is missing; run npm run design:index')
  const index = readStrictJson(file)
  if (index.schemaVersion !== version || index.generatorVersion !== version || index.sourceFingerprint !== hash(JSON.stringify(indexSources(root)))) throw new Error('Design index is stale; regenerate with npm run design:index before querying')
  if (!Array.isArray(index.entries) || !index.entries.length) throw new Error('Design index is empty or invalid')
  return index
}
export function verifyIndex(root) {
  // An existing stale cache must fail before regeneration. A clean checkout can construct it.
  if (existsSync(indexPath(root))) readFreshIndex(root)
  else writeDesignIndex(root)
  const first = readFreshIndex(root)
  const rebuilt = buildDesignIndex(root)
  if (JSON.stringify(first) !== JSON.stringify(rebuilt)) throw new Error('Design index differs from deterministic source extraction')
  return first.summary
}

export function searchDesignIndex(index, options = {}) {
  const query = String(options.query ?? '').trim()
  if (query.length > 256) throw new Error('Design query exceeds 256 characters')
  const limit = options.limit ?? 8
  if (!Number.isInteger(limit) || limit < 1 || limit > 50) throw new Error('Limit must be 1..50')
  const segments = [...new Intl.Segmenter('zh-CN', { granularity: 'word' }).segment(query)].filter((item) => item.isWordLike).map((item) => item.segment.toLowerCase())
  const tokens = [...new Set([query.toLowerCase(), ...segments].filter(Boolean))]
  const results = []
  for (const entry of index.entries) {
    if (['kind', 'layer', 'domain', 'status'].some((key) => options[key] && entry[key] !== options[key])) continue
    const text = JSON.stringify([entry.name, entry.id, entry.scenario, entry.scope, entry.aliases, entry.useWhen, entry.labels, entry.routes, entry.api?.props.map((prop) => prop.name)]).toLowerCase()
    const matched = tokens.filter((token) => text.includes(token))
    if (tokens.length && !matched.length) continue
    const exactSemantic = [entry.name, entry.scenario, ...(entry.aliases ?? [])].some((value) => value?.toLowerCase() === query.toLowerCase())
    const score = (exactSemantic && query ? 30 : 0) + matched.reduce((sum, token) => sum + (entry.name.toLowerCase() === token ? 20 : entry.name.toLowerCase().includes(token) ? 8 : token === query.toLowerCase() ? 5 : 1), 0)
    results.push({ ...entry, score, matched, selectionReason: matched.length ? `Matched source/semantic terms: ${matched.join(', ')}` : 'Matched explicit filters' })
  }
  return results.sort((a, b) => b.score - a.score || compare(a.id, b.id)).slice(0, limit)
}
