import { createHash } from 'node:crypto'
import { readFileSync, realpathSync } from 'node:fs'
import { isAbsolute, relative, resolve } from 'node:path'
import { StaticSource } from './static-source.mjs'

/** Check translation completeness without importing or executing UI/application code. */
export function checkCommercialStates(webRoot) {
  const root = realpathSync(webRoot)
  const repo = realpathSync(resolve(root, '..'))
  const reader = new StaticSource(root)
  const file = resolve(root, 'src/services/commercial/state-vocabulary.generated.ts')
  const vocabulary = reader.exported(file, 'commercialStateVocabulary')
  const sources = reader.exported(file, 'commercialStateSources')
  const terms = reader.exported(resolve(root, 'src/i18n/commercial-state-terms.ts'), 'commercialStateTerms')
  const messages = reader.exported(resolve(root, 'src/i18n/backend-term-messages.ts'), 'backendTermMessages')
  const failures = []
  if (!Object.keys(vocabulary).length || !Object.keys(sources).length) failures.push('Commercial state source inventory is empty')
  for (const [path, expected] of Object.entries(sources)) {
    const actual = realpathSync(resolve(repo, path))
    const rel = relative(repo, actual)
    if (isAbsolute(path) || rel.startsWith('..') || isAbsolute(rel)) throw new Error('Commercial state source escapes repository')
    const hash = createHash('sha256').update(readFileSync(actual)).digest('hex')
    if (hash !== expected) failures.push(`${path}: backend state source changed; regenerate and review translations`)
  }
  for (const kind of Object.keys(terms)) if (!Object.hasOwn(vocabulary, kind)) failures.push(`${kind}: translation claims an undeclared state kind`)
  let count = 0
  for (const [kind, values] of Object.entries(vocabulary)) {
    if (!Array.isArray(values) || !values.length || new Set(values).size !== values.length) {
      failures.push(`${kind}: invalid derived vocabulary`)
      continue
    }
    const mapping = terms[kind] ?? {}
    for (const value of Object.keys(mapping)) if (!values.includes(value)) failures.push(`${kind}.${value}: unsupported wire alias`)
    for (const value of values) {
      count += 1
      const key = mapping[value]
      if (typeof key !== 'string' || !key) {
        failures.push(`${kind}.${value}: missing translation binding`)
        continue
      }
      for (const locale of ['zh-CN', 'en-US']) {
        const text = messages[locale]?.[kind]?.[key]
        if (typeof text !== 'string' || !text.trim() || text.includes('backendTerms.')) failures.push(`${locale}.${kind}.${key}: missing user copy`)
      }
    }
    for (const locale of ['zh-CN', 'en-US']) {
      const fallback = messages[locale]?.fallback?.[kind]
      if (typeof fallback !== 'string' || !fallback.trim()) failures.push(`${locale}.${kind}: safe fallback required`)
      // A missing state must not look like an actual success/active/pending result.
      if (kind.endsWith('State') || kind === 'receiptStatus' || kind.endsWith('Status')) {
        if (Object.values(messages[locale]?.[kind] ?? {}).includes(fallback)) failures.push(`${locale}.${kind}: unknown fallback impersonates a known state`)
      }
    }
  }
  return { failures, kinds: Object.keys(vocabulary).length, values: count, sourceFiles: Object.keys(sources).length }
}
