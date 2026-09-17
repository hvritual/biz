import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { parse } from 'vue/compiler-sfc'

const root = resolve(import.meta.dirname, '..')
const migrated = [
  'src/features/app-shell/components/AppHeader.vue',
  'src/features/app-shell/components/PrimaryNavigation.vue',
  'src/features/app-shell/components/ModulePanel.vue',
  'src/features/platform/pages/PlatformTenantsView.vue',
  'src/features/enterprise/pages/MembersView.vue',
  'src/features/enterprise/pages/BrandingView.vue',
  'src/features/enterprise/components/EnterpriseSourceBanner.vue',
  'src/features/enterprise/components/members/MemberFilters.vue',
  'src/features/enterprise/components/members/MemberOverview.vue',
  'src/features/enterprise/components/members/MemberTable.vue',
  'src/features/enterprise/components/members/MemberDetailDrawer.vue',
  'src/features/enterprise/components/members/MemberDetailOverview.vue',
  'src/features/enterprise/components/members/MemberActionDialog.vue',
  'src/features/enterprise/components/members/MemberBulkDialog.vue',
  'src/ui/common/AppPagination.vue',
]
const han = /[\u3400-\u9fff]/u
const failures = []
for (const path of migrated) {
  const file = resolve(root, path)
  const source = readFileSync(file, 'utf8')
  const { descriptor, errors } = parse(source, { filename: file })
  if (errors.length) failures.push(`${path}: SFC parse failed`)
  for (const block of [descriptor.script, descriptor.scriptSetup].filter(Boolean)) {
    const lines = block.content.split('\n')
    lines.forEach((line, index) => {
      if (han.test(line) && !/^\s*\/\//.test(line)) failures.push(`${path}:${block.loc.start.line + index}: hard-coded Han text remains in migrated script`)
    })
  }
  const visit = (node) => {
    if (node.type === 2 && han.test(node.content)) failures.push(`${path}:${node.loc.start.line}: hard-coded visible Han text remains in migrated template`)
    if (node.type === 1) {
      for (const prop of node.props ?? []) {
        if (prop.type === 6 && prop.value && ['aria-label', 'title', 'placeholder', 'alt', 'label'].includes(prop.name) && han.test(prop.value.content)) {
          failures.push(`${path}:${prop.loc.start.line}: hard-coded visible attribute ${prop.name}`)
        }
      }
    }
    for (const child of node.children ?? []) visit(child)
  }
  if (descriptor.template?.ast) visit(descriptor.template.ast)
}
if (failures.length) {
  console.error(failures.join('\n'))
  process.exit(1)
}
console.log(`i18n migration guard passed: ${migrated.length} scoped SFCs; zh-CN/en-US pilot has no hard-coded Han UI text.`)
