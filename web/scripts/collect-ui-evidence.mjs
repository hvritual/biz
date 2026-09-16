import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { existsSync, lstatSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs'
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

// Small offline parser toolchain for reproducing contract/index tests; no application data.
const lock = JSON.parse(readFileSync(resolve(webRoot, 'package-lock.json'), 'utf8'))
const packages = lock.packages ?? {}
const selected = new Set()
const dependencyPath = (name, owner) => {
  let parent = owner
  while (parent) {
    const candidate = `${parent}/node_modules/${name}`
    if (packages[candidate]) return candidate
    const end = parent.lastIndexOf('/node_modules/')
    parent = end < 0 ? '' : parent.slice(0, end)
  }
  return `node_modules/${name}`
}
const include = (name) => {
  if (selected.has(name) || !packages[name] || !existsSync(resolve(webRoot, name))) return
  selected.add(name)
  for (const dependency of Object.keys({ ...packages[name].dependencies, ...packages[name].peerDependencies })) include(dependencyPath(dependency, name))
}
for (const name of ['vue', 'typescript', 'vue-i18n']) include(`node_modules/${name}`)
const files = []
const collect = (path) => {
  const full = resolve(webRoot, path)
  const stat = lstatSync(full)
  if (stat.isDirectory()) {
    for (const entry of readdirSync(full).sort()) if (entry !== 'node_modules') collect(`${path}/${entry}`)
  } else if (stat.isFile() && !/\.(?:woff2?|ttf|otf|eot)$/i.test(path)) files.push(path)
}
for (const name of [...selected].sort()) collect(name)
let toolchain = null
if (files.length) {
  const path = resolve(output, 'parser-toolchain.tar.gz')
  execFileSync('tar', ['--null', '--no-recursion', '-T', '-', '-czf', path], { cwd: webRoot, input: Buffer.from(files.join('\0') + '\0') })
  toolchain = { file: 'parser-toolchain.tar.gz', sha256: sha256(readFileSync(path)), packages: [...selected].sort(), excludesFonts: true }
}

writeFileSync(resolve(output, 'manifest.json'), JSON.stringify({
  schemaVersion: 1,
  testedCommit,
  testedTree: git('rev-parse', 'HEAD^{tree}'),
  parents: git('cat-file', '-p', 'HEAD').split('\n').filter((line) => line.startsWith('parent ')).map((line) => line.slice(7)),
  node: process.version,
  lockfileSha256: sha256(readFileSync(resolve(webRoot, 'package-lock.json'))),
  archive: { file: 'tested-source.tar.gz', sha256: sha256(readFileSync(archive)) },
  parserToolchain: toolchain,
  evidenceOnly: true,
  note: 'The manifest identifies the checkout; it is not a test-pass or human-approval statement.',
}, null, 2) + '\n')
console.log(`UI evidence source: ${testedCommit}`)
