import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

// Archive only tracked source at the actual tested commit, never runner secrets or node_modules.
const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const repoRoot = resolve(webRoot, '..')
const output = resolve(webRoot, 'test-results/provenance')
const git = (...args) => execFileSync('git', args, { cwd: repoRoot, encoding: 'utf8' }).trim()
const testedCommit = git('rev-parse', 'HEAD')
if (!/^[a-f0-9]{40}$/.test(testedCommit)) throw new Error('Cannot identify the tested commit')
mkdirSync(output, { recursive: true })
const archive = resolve(output, 'tested-source.tar.gz')
execFileSync('git', ['archive', '--format=tar.gz', `--output=${archive}`, testedCommit], { cwd: repoRoot })
const sha256 = (data) => createHash('sha256').update(data).digest('hex')
writeFileSync(resolve(output, 'manifest.json'), JSON.stringify({
  schemaVersion: 1,
  testedCommit,
  testedTree: git('rev-parse', 'HEAD^{tree}'),
  parents: git('show', '-s', '--format=%P', 'HEAD').split(' ').filter(Boolean),
  node: process.version,
  lockfileSha256: sha256(readFileSync(resolve(webRoot, 'package-lock.json'))),
  archive: { file: 'tested-source.tar.gz', sha256: sha256(readFileSync(archive)) },
  evidenceOnly: true,
  note: 'The manifest identifies the checkout; it is not a test-pass or human-approval statement.',
}, null, 2) + '\n')
console.log(`UI evidence source: ${testedCommit}`)
