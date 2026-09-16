import { readFileSync } from 'node:fs'
import { basename, relative } from 'node:path'
import { parse } from 'vue/compiler-sfc'
import ts from 'typescript'

export function readVue(file) {
  const source = readFileSync(file, 'utf8')
  const { descriptor, errors } = parse(source, { filename: file, sourceMap: false })
  if (errors.length) throw new Error(`${file}: ${errors.map((error) => error.message ?? String(error)).join('; ')}`)
  return { source, descriptor }
}

function hidden(element) {
  return element.props?.some((property) =>
    property.type === 6 && (property.name === 'hidden' || (property.name === 'style' && /(?:display\s*:\s*none|visibility\s*:\s*hidden)/i.test(property.value?.content ?? ''))) ||
    property.type === 7 && ['if', 'show'].includes(property.name) && /^(?:false|0|null|undefined)$/.test(property.exp?.content.trim() ?? ''),
  )
}

export function templateMarkers(descriptor) {
  const regions = new Set()
  const templates = new Set()
  const attributes = new Set()
  const visit = (node) => {
    if (!node) return
    if (node.type === 1) {
      if (hidden(node)) return
      for (const property of node.props ?? []) {
        if (property.type !== 6) continue
        attributes.add(property.name)
        const value = property.value?.content
        if (property.name === 'data-ui-template' && value) templates.add(value)
        if (property.name === 'data-ui-region' && value) {
          // Components can provide content through props/slots. Empty native placeholders cannot.
          const isComponent = node.tagType === 1 || /^[A-Z]/.test(node.tag)
          const meaningful = node.children?.some((child) => child.type !== 3 && (child.type !== 2 || child.content.trim()))
          if (isComponent || meaningful) regions.add(value)
        }
      }
    }
    for (const child of node.children ?? []) visit(child)
  }
  visit(descriptor.template?.ast)
  return { regions, templates, attributes }
}

/** Follows component composition only; unrelated stores/services are not a UI subtree. */
export function componentSources(resolver, entry) {
  const visited = new Set()
  const output = []
  const usedNames = (descriptor) => {
    const used = new Set()
    const visit = (node) => {
      if (node.type === 1) {
        if (hidden(node)) return
        used.add(node.tag)
        for (const property of node.props ?? []) {
          if (property.type === 7 && property.name === 'bind' && property.arg?.content === 'is') {
            for (const name of property.exp?.content.match(/[A-Za-z_$][\w$]*/g) ?? []) used.add(name)
          }
        }
      }
      for (const child of node.children ?? []) visit(child)
    }
    if (descriptor.template?.ast) visit(descriptor.template.ast)
    return used
  }
  const used = (names, name) => names.has(name) || names.has(name.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase())
  const visit = (file, wanted = null) => {
    const key = `${file}#${wanted ? [...wanted].sort().join(',') : '*'}`
    if (visited.has(key)) return
    if (visited.size >= 512) throw new Error('Component import graph traversal limit exceeded')
    visited.add(key)
    const isVue = file.endsWith('.vue')
    let sources, names
    if (isVue) {
      const { descriptor, source } = readVue(file)
      output.push({ file, descriptor, source, ...templateMarkers(descriptor) })
      sources = [descriptor.script?.content, descriptor.scriptSetup?.content].filter(Boolean)
      names = usedNames(descriptor)
    } else if (/\.(?:ts|js|mjs)$/.test(file)) sources = [readFileSync(file, 'utf8')]
    else return
    for (const source of sources) {
      const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
      for (const statement of ast.statements) {
        const isImport = ts.isImportDeclaration(statement)
        const isExport = ts.isExportDeclaration(statement)
        if ((isVue && !isImport) || (!isVue && !isExport)) continue
        const specifier = statement.moduleSpecifier
        if (!specifier || !ts.isStringLiteralLike(specifier)) continue
        if (!(specifier.text.startsWith('.') || specifier.text.startsWith('@/'))) continue
        if (/\.(?:css|svg|png|jpe?g|json)$/.test(specifier.text)) continue
        const selected = new Set()
        if (isImport) {
          const clause = statement.importClause
          if (!clause || clause.isTypeOnly) continue
          if (clause.name && used(names, clause.name.text)) selected.add('default')
          if (clause.namedBindings && ts.isNamedImports(clause.namedBindings)) {
            for (const item of clause.namedBindings.elements) {
              if (!item.isTypeOnly && used(names, item.name.text)) selected.add(item.propertyName?.text ?? item.name.text)
            }
          }
          if (!selected.size) continue
        } else if (statement.exportClause && ts.isNamedExports(statement.exportClause)) {
          for (const item of statement.exportClause.elements) {
            if (!wanted || wanted.has(item.name.text)) selected.add(item.propertyName?.text ?? item.name.text)
          }
          if (!selected.size) continue
        } else if (wanted) for (const name of wanted) selected.add(name)
        const target = resolver.file(specifier.text, file)
        visit(target, selected.size ? selected : null)
      }
    }
  }
  visit(entry)
  return output
}

export function verifyPageSource(resolver, page, actualComponent) {
  const problems = []
  const expected = resolver.file(page.component)
  if (actualComponent !== expected) problems.push(`${page.path}: canonical component must be ${page.component}`)
  const sources = componentSources(resolver, expected)
  const entry = sources.find((source) => source.file === expected)
  if (!entry?.templates.has(page.template)) problems.push(`${page.path}: rendered template must declare data-ui-template=${page.template}`)
  const regions = new Set(sources.flatMap((source) => [...source.regions]))
  for (const region of page.required_regions) if (!regions.has(region)) problems.push(`${page.path}: missing executable region ${region}`)
  if (page.surface !== 'runtime') {
    for (const source of sources) if (basename(source.file) === 'RuntimeConsoleView.vue') problems.push(`${page.path}: business surface imports RuntimeConsole through ${relative(resolver.root, source.file)}`)
  }
  return problems
}
