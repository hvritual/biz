import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { relative, resolve } from 'node:path'
const root = resolve('src'), failures = []
function walk(dir) { return existsSync(dir) ? readdirSync(dir, { withFileTypes: true }).flatMap((entry) => entry.isDirectory() ? walk(resolve(dir, entry.name)) : [resolve(dir, entry.name)]) : [] }
const files = walk(root)
if (existsSync(resolve(root, 'components'))) failures.push('src/components is retired; use ui/base, ui/common or features/<domain>/components')
if (existsSync(resolve(root, 'views'))) failures.push('src/views is retired; pages belong to features/<domain>/pages')
if (existsSync(resolve(root, 'ui/theme'))) failures.push('src/ui/theme is retired; ui/base/theme.ts is the single runtime theme authority')
for (const file of files) {
  const path = relative(root, file).replaceAll('\\', '/')
  if (/\.(woff2?|ttf|otf)$/.test(path)) failures.push(`${path}: do not distribute font files`)
  if (!/\.(vue|ts|css)$/.test(file)) continue
  const code = readFileSync(file, 'utf8'), lines = code.split('\n').length
  if (path === 'App.vue' && lines > 60) failures.push('App.vue must remain a composition entry (<=60 lines)')
  if (path.endsWith('.vue') && lines > 500) failures.push(`${path}: component exceeds 500 formatted lines; split by responsibility`)
  if (/^(features|ui)\//.test(path) && /\b(fetch|XMLHttpRequest)\s*\(/.test(code)) failures.push(`${path}: network I/O belongs to services`)
  if (/^(services|stores|utils)\//.test(path) && /from ['"]@\/(features|ui)\//.test(code)) failures.push(`${path}: lower layers cannot import UI`)
  if (path.startsWith('ui/base/') && /from ['"]@\/(features|stores|services|router|ui\/common)\//.test(code)) failures.push(`${path}: base layer cannot depend on common/business/runtime layers`)
  if (path.startsWith('ui/common/') && /from ['"]@\/(features|stores|services)\//.test(code)) failures.push(`${path}: common UI cannot depend on business stores/services/features`)
  if (path.startsWith('features/') && /from ['"]@\/(components|views)\//.test(code)) failures.push(`${path}: legacy UI import is forbidden`)
  if (path.endsWith('.vue') && !path.startsWith('ui/base/') && /<(button|input|select|option|textarea|dialog|details|summary)\b/i.test(code)) failures.push(`${path}: raw browser interactive controls are forbidden outside ui/base`)
  if (path.endsWith('.vue')) {
    const styles = [...code.matchAll(/<style\b[^>]*>([\s\S]*?)<\/style>/g)].map((match) => match[1]).join('\n')
    if (/#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\(|oklch\(/.test(styles)) failures.push(`${path}: scoped styles must consume design tokens instead of raw colors`)
  }
  if (path.endsWith('.css') && path !== 'styles/tokens.css' && /#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\(|oklch\(/.test(code)) failures.push(`${path}: stylesheets must consume design tokens instead of raw colors`)
  if (/v-html\s*=/.test(code)) failures.push(`${path}: untrusted HTML rendering is not permitted`)
}
const tokens = readFileSync(resolve(root, 'styles/tokens.css'), 'utf8')
for (const [name, value] of [['--header-height', '56px'], ['--rail-width', '200px'], ['--rail-collapsed-width', '68px'], ['--module-width', '480px']]) if (!tokens.includes(`${name}: ${value}`)) failures.push(`${name} must be ${value}`)
if (!tokens.includes('--primary: var(--color-primary)')) failures.push('shadcn semantic primary token must resolve through CoffeeLink primary token')
const themeApiFile = resolve(root, 'ui/base/theme.ts')
if (!existsSync(themeApiFile)) failures.push('ui/base/theme.ts is required for application-level dynamic theming')
else {
  const themeApi = readFileSync(themeApiFile, 'utf8')
  for (const symbol of ['applyUiTheme', 'initializeUiTheme', 'resetUiTheme', 'setUiTheme', 'setUiThemePreset', 'uiThemePresets', 'useUiTheme']) if (!themeApi.includes(symbol)) failures.push(`ui/base/theme.ts must expose ${symbol}`)
}
const mainSource = readFileSync(resolve(root, 'main.ts'), 'utf8')
if (!mainSource.includes("import { initializeUiTheme } from './ui/base'")) failures.push('main.ts must initialize the application theme through ui/base')
if (!mainSource.includes('initializeUiTheme()')) failures.push('main.ts must initialize the persisted application theme before mounting')

const appShellSource = readFileSync(resolve(root, 'features/app-shell/components/AppShell.vue'), 'utf8')
const primaryNavigationSource = readFileSync(resolve(root, 'features/app-shell/components/PrimaryNavigation.vue'), 'utf8')
const modulePanelSource = readFileSync(resolve(root, 'features/app-shell/components/ModulePanel.vue'), 'utf8')
if (primaryNavigationSource.includes('width: 64px') || appShellSource.includes('--current-rail: 80px') || modulePanelSource.includes('100vw - 80px')) failures.push('app shell must consume rail design tokens instead of legacy 64/80px widths')
if (!appShellSource.includes('width: var(--current-rail)') || !appShellSource.includes('width: calc(var(--current-rail) + var(--module-width))')) failures.push('app shell rail/flyout geometry must be token-driven')
const desktopRailContract = /\.side-frame\s+:deep\(\.primary-nav\)\s*\{[^}]*width:\s*var\(--current-rail\);[^}]*flex:\s*0\s+0\s+var\(--current-rail\);[^}]*\}/s
if (!desktopRailContract.test(appShellSource)) failures.push('desktop primary navigation must pin width and flex-basis to --current-rail inside the joined flyout shell')
if (!modulePanelSource.includes('padding: 20px') || !modulePanelSource.includes('gap: 12px') || !modulePanelSource.includes('var(--radius-xl)')) failures.push('module panel must preserve CoffeeLink V1.1 20px padding, 12px columns and 18px outer radius')

const manifestFile = resolve(root, 'features/component-scopes.json')
if (!existsSync(manifestFile)) failures.push('features/component-scopes.json is required')
else {
  const manifest = JSON.parse(readFileSync(manifestFile, 'utf8'))
  const business = files.filter((file) => file.endsWith('.vue') && file.includes(`${resolve(root, 'features')}`) && file.includes(`${process.platform === 'win32' ? '\\' : '/'}components${process.platform === 'win32' ? '\\' : '/'}`)).map((file) => relative(resolve(root, 'features'), file).replaceAll('\\', '/')).sort()
  for (const path of business) {
    const item = manifest[path]
    if (!item?.scenario || !item?.scope) failures.push(`${path}: business component must declare scenario and scope`)
  }
  for (const path of Object.keys(manifest)) if (!business.includes(path)) failures.push(`${path}: stale component scope declaration`)
}
if (failures.length) { console.error(failures.join('\n')); process.exit(1) }
console.log(`Architecture checks passed: ${files.length} source files; three UI layers, single-source global theme API, feature pages, token ownership, native-control boundary, shell geometry and business scopes are guarded.`)
