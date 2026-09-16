import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { parseArgs } from 'node:util'
import { readFreshIndex, searchDesignIndex, verifyIndex, writeDesignIndex } from './lib/design-index.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
try {
  const [command, ...args] = process.argv.slice(2)
  if (command === 'build' || command === 'check') {
    if (args.length) throw new Error('Build/check do not accept extra arguments')
    const summary = command === 'build' ? writeDesignIndex(root).summary : verifyIndex(root)
    console.log(JSON.stringify({ command, ...summary }))
  } else if (command === 'find') {
    const { values } = parseArgs({ args, options: {
      query: { type: 'string' }, kind: { type: 'string' }, layer: { type: 'string' },
      domain: { type: 'string' }, status: { type: 'string' }, limit: { type: 'string' }, json: { type: 'boolean' },
    }, strict: true })
    const index = readFreshIndex(root)
    const results = searchDesignIndex(index, { ...values, limit: values.limit === undefined ? 8 : Number(values.limit) })
    if (values.json) console.log(JSON.stringify({ sourceFingerprint: index.sourceFingerprint, results, gap: results.length ? null : 'No matching existing UI; inspect the requirements before proposing a component.' }, null, 2))
    else if (!results.length) console.log('No matching existing UI. Record a bounded gap; do not invent an API.')
    else for (const entry of results) {
      console.log(`\n${entry.id} [${entry.kind}/${entry.layer}/${entry.status}]`)
      console.log(`${entry.source.path}\n${entry.selectionReason}`)
      if (entry.importPath) console.log(`import: ${entry.importPath}`)
      if (entry.scope) console.log(`scope: ${entry.scope}`)
      if (entry.api) console.log(`props: ${entry.api.props.map((prop) => prop.name).join(', ')}; unresolved declarations: ${entry.api.unknown.length}`)
      if (entry.avoidWhen?.length) console.log(`avoid: ${entry.avoidWhen.join('; ')}`)
      if (entry.excerpt) console.log(entry.excerpt)
    }
  } else throw new Error('Expected build, check or find')
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error))
  process.exitCode = 1
}
