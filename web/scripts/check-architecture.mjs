import { readdirSync, readFileSync } from 'node:fs'
import { resolve, relative } from 'node:path'
const root = resolve('src'),
  failures = []
function walk(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? walk(resolve(dir, e.name)) : [resolve(dir, e.name)],
  )
}
const files = walk(root)
for (const file of files) {
  const path = relative(root, file).replaceAll('\\', '/')
  if (/\.(woff2?|ttf|otf)$/.test(path)) failures.push(`${path}: do not distribute font files`)
  if (!/\.(vue|ts|css)$/.test(file)) continue
  const code = readFileSync(file, 'utf8'),
    lines = code.split('\n').length
  if (path === 'App.vue' && lines > 60) failures.push('App.vue must remain a composition entry (<=60 lines)')
  if (path.endsWith('.vue') && lines > 500)
    failures.push(`${path}: component exceeds 500 formatted lines; split by responsibility`)
  if (/^(views|components)\//.test(path) && /\b(fetch|XMLHttpRequest)\s*\(/.test(code))
    failures.push(`${path}: network I/O belongs to services`)
  if (/^(services|stores|utils)\//.test(path) && /from ['"]@\/(views|components)\//.test(code))
    failures.push(`${path}: lower layers cannot import UI`)
  if (path.startsWith('components/ui/') && /from ['"]@\/(stores|services)\//.test(code))
    failures.push(`${path}: UI primitives cannot depend on business stores`)
  if (/v-html\s*=/.test(code)) failures.push(`${path}: untrusted HTML rendering is not permitted`)
}
const tokens = readFileSync(resolve(root, 'styles/tokens.css'), 'utf8')
if (!tokens.includes('--module-width: 480px')) failures.push('Desktop flyout width must be 480 CSS pixels')
if (failures.length) {
  console.error(failures.join('\n'))
  process.exit(1)
}
console.log(
  `Architecture checks passed: ${files.length} source files; layer direction, primitive ownership, component size, no font binaries, no raw HTML.`,
)
