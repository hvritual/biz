import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { checkUiModel } from './lib/check-ui-model.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
try {
  const result = checkUiModel(root)
  if (result.failures.length) throw new Error(result.failures.join('\n'))
  console.log(`UI contracts passed: ${result.contract.routes.length} guarded routes; ${result.routes.length} resolved routes; static modules=${result.sourceCount}; comment/hidden-region and RuntimeConsole regressions guarded.`)
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error))
  process.exitCode = 1
}
