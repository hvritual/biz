import { createHash } from 'node:crypto'
import { existsSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs'
import { extname, join, relative, resolve } from 'node:path'

const root = resolve('.')
const src = resolve(root, 'src')
const self = resolve(root, 'scripts/migrate-ui-layers.mjs')
const supported = new Set(['.vue', '.ts', '.css', '.mjs', '.json', '.md'])
const pathMappings = [
  ['@/components/ui/', '@/ui/common/'],
  ['@/components/layout/', '@/features/app-shell/components/'],
  ['@/components/customer/', '@/features/customer/components/'],
  ['@/components/enterprise/', '@/features/enterprise/components/server/'],
  ['@/components/members/', '@/features/enterprise/components/members/'],
  ['@/components/organization/', '@/features/enterprise/components/organization/'],
  ['@/components/roles/', '@/features/enterprise/components/roles/'],
  ['@/components/platform/', '@/features/platform/components/'],
  ['@/components/settings/', '@/features/system/components/settings/'],
  ['@/components/siteRental/', '@/features/site-rental/components/'],
  ['@/views/customer/', '@/features/customer/pages/'],
  ['@/views/dashboard/', '@/features/dashboard/pages/'],
  ['@/views/enterprise/', '@/features/enterprise/pages/'],
  ['@/views/platform/', '@/features/platform/pages/'],
  ['@/views/runtime/', '@/features/runtime/pages/'],
  ['@/views/siteRental/', '@/features/site-rental/pages/'],
  ['@/views/system/', '@/features/system/pages/'],
  ['@/views/NotFoundView.vue', '@/features/system/pages/NotFoundView.vue'],
  ['./components/layout/', './features/app-shell/components/'],
  ['src/components/layout/', 'src/features/app-shell/components/'],
  ['src/components/ui/', 'src/ui/common/'],
  ['src/views/', 'src/features/'],
]

function walk(dir) {
  if (!existsSync(dir)) return []
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const file = join(dir, entry.name)
    if (entry.name === 'node_modules' || entry.name === 'dist' || entry.name === '.git') return []
    return entry.isDirectory() ? walk(file) : [file]
  })
}

function replacePaths(code) {
  return pathMappings.reduce((value, [from, to]) => value.split(from).join(to), code)
}

const primitiveTags = new Map([
  ['button', 'UiButton'],
  ['input', 'UiInput'],
  ['select', 'UiSelect'],
  ['option', 'UiOption'],
  ['textarea', 'UiTextarea'],
])

function migrateControls(code, file) {
  if (!file.endsWith('.vue') || file.includes('/src/ui/base/')) return code
  const used = []
  for (const [tag, component] of primitiveTags) {
    const open = new RegExp(`<${tag}\\b`, 'g')
    const close = new RegExp(`</${tag}>`, 'g')
    if (open.test(code)) {
      used.push(component)
      code = code.replace(open, `<${component}`).replace(close, `</${component}>`)
    }
  }
  if (!used.length) return code
  const names = [...new Set(used)].sort()
  const importLine = `import { ${names.join(', ')} } from '@/ui/base'\n`
  if (code.includes("from '@/ui/base'")) return code
  const setup = code.match(/<script setup[^>]*>/)
  if (setup) return code.replace(setup[0], `${setup[0]}\n${importLine}`)
  return `<script setup lang="ts">\n${importLine}</script>\n${code}`
}

const semanticColors = new Map([
  ['#fff', '--color-surface'], ['#ffffff', '--color-surface'], ['#2563eb', '--color-primary'], ['#087bff', '--color-primary'],
  ['#1d4ed8', '--color-primary-hover'], ['#0068e5', '--color-primary-hover'], ['#eff6ff', '--color-primary-soft'], ['#eaf3ff', '--color-primary-soft'],
  ['#1f2937', '--color-text'], ['#132044', '--color-text'], ['#475569', '--color-text-secondary'], ['#5d6d91', '--color-text-secondary'],
  ['#64748b', '--color-text-muted'], ['#7684a3', '--color-text-muted'], ['#f5f7fa', '--color-canvas'], ['#f0f7ff', '--color-canvas'],
  ['#f8fafc', '--color-surface-soft'], ['#f6f9fe', '--color-surface-soft'], ['#e5e7eb', '--color-border'], ['#e7edf6', '--color-border'],
  ['#cbd5e1', '--color-border-strong'], ['#cfd9e8', '--color-border-strong'], ['#dc2626', '--color-danger'], ['#ed4556', '--color-danger'],
  ['#d97706', '--color-warning'], ['#e99414', '--color-warning'], ['#7c3aed', '--color-violet'], ['#8e54ee', '--color-violet'],
])
const fixed = new Map()
const colorPattern = /#[0-9a-fA-F]{3,8}\b|rgba?\([^)]*\)|hsla?\([^)]*\)|oklch\([^)]*\)/g
function tokenFor(color) {
  const normalized = color.toLowerCase().replace(/\s+/g, ' ').trim()
  const semantic = semanticColors.get(normalized)
  if (semantic) return `var(${semantic})`
  if (!fixed.has(normalized)) fixed.set(normalized, `--color-fixed-${createHash('sha1').update(normalized).digest('hex').slice(0, 8)}`)
  return `var(${fixed.get(normalized)})`
}
function migrateColors(code, file) {
  if (file.endsWith('styles/tokens.css') || file.endsWith('styles/shadcn.css')) return code
  if (file.endsWith('.css')) return code.replace(colorPattern, tokenFor)
  if (!file.endsWith('.vue')) return code
  return code.replace(/<style\b[^>]*>[\s\S]*?<\/style>/g, (style) => style.replace(colorPattern, tokenFor))
}

const files = walk(root).filter((file) => supported.has(extname(file)) && file !== self)
for (const file of files) {
  let code = readFileSync(file, 'utf8')
  const next = migrateColors(migrateControls(replacePaths(code), file), file)
  if (next !== code) writeFileSync(file, next)
}

const tokensFile = resolve(src, 'styles/tokens.css')
let tokens = readFileSync(tokensFile, 'utf8')
const marker = '/* fixed colors centralized by three-layer migration */'
tokens = tokens.replace(new RegExp(`\\n?${marker}[\\s\\S]*$`), '').trimEnd()
if (fixed.size) {
  const lines = [...fixed.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([value, name]) => `  ${name}: ${value};`)
  tokens += `\n\n${marker}\n:root {\n${lines.join('\n')}\n}\n`
} else tokens += '\n'
writeFileSync(tokensFile, tokens)

const domainNames = { 'app-shell': '应用框架', customer: '客户经营', dashboard: '工作台', enterprise: '企业中心', platform: '平台管理', runtime: '运行工作区', 'site-rental': '租赁运营', system: '系统设置' }
const components = walk(resolve(src, 'features')).filter((file) => file.endsWith('.vue') && file.includes('/components/'))
const scopes = {}
for (const file of components.sort()) {
  const rel = relative(resolve(src, 'features'), file).replaceAll('\\', '/')
  const domain = rel.split('/')[0]
  const name = file.split(/[\\/]/).at(-1).replace(/\.vue$/, '')
  const kind = /Detail|Drawer/.test(name) ? '详情展示' : /Dialog/.test(name) ? '业务操作弹窗' : /Table|List/.test(name) ? '业务列表' : /Filter/.test(name) ? '业务筛选' : /Editor|Form/.test(name) ? '业务编辑' : /Tree/.test(name) ? '层级导航' : /Shell|Navigation|Panel|Header/.test(name) ? '业务布局与导航' : '领域展示'
  scopes[rel] = {
    scenario: `${domainNames[domain] ?? domain} · ${kind} · ${name}`,
    scope: `仅用于${domainNames[domain] ?? domain}业务域及其页面组合；需要跨业务复用时必须下沉到 ui/common，不得复制样式或绕过 ui/base。`,
  }
}
writeFileSync(resolve(src, 'features/component-scopes.json'), `${JSON.stringify(scopes, null, 2)}\n`)
console.log(`UI migration complete: ${files.length} text files scanned, ${components.length} business components documented, ${fixed.size} fixed colors centralized.`)
