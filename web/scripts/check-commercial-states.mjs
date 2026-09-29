import { resolve } from 'node:path'
import { checkCommercialStates } from './lib/commercial-state-contract.mjs'
try {
  const result = checkCommercialStates(resolve(import.meta.dirname, '..'))
  if (result.failures.length) throw new Error(result.failures.join('\n'))
  console.log(`COMMERCIAL_STATE_TRANSLATIONS=PASS kinds=${result.kinds} values=${result.values} locales=2 sources=${result.sourceFiles}`)
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error))
  process.exitCode = 1
}
