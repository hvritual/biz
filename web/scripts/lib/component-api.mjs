import ts from 'typescript'
import { propertyName, unwrap } from './static-source.mjs'

/** Extract declarations, not inferred business meaning. Unresolved shapes remain explicit. */
export function componentApi(descriptor, file) {
  const result = { props: [], emits: [], slots: [], unknown: [], inheritAttrs: true }
  const add = (kind, entry) => {
    if (!result[kind].some((current) => current.name === entry.name)) result[kind].push(entry)
  }
  for (const block of [descriptor.script, descriptor.scriptSetup].filter(Boolean)) {
    const ast = ts.createSourceFile(file, block.content, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
    if (ast.parseDiagnostics.length) throw new Error(`${file}: cannot parse component script`)
    const localTypes = new Map(ast.statements.filter((node) => ts.isInterfaceDeclaration(node) || ts.isTypeAliasDeclaration(node)).map((node) => [node.name.text, node]))
    const line = (node) => block.loc.start.line + ast.getLineAndCharacterOfPosition(node.getStart(ast)).line
    const source = (node) => node?.getText(ast) ?? 'unknown'
    const typeMembers = (node, seen = new Set()) => {
      if (!node) return null
      if (ts.isInterfaceDeclaration(node) && node.heritageClauses?.length) return null
      if (ts.isTypeLiteralNode(node) || ts.isInterfaceDeclaration(node)) return node.members
      if (ts.isTypeAliasDeclaration(node)) return typeMembers(node.type, seen)
      if (ts.isTypeReferenceNode(node) && ts.isIdentifier(node.typeName)) {
        const name = node.typeName.text
        if (seen.has(name)) return null
        seen.add(name)
        return typeMembers(localTypes.get(name), seen)
      }
      return null
    }
    const unknown = (kind, node) => result.unknown.push({ kind, declaration: source(node).slice(0, 300), line: line(node) })
    const visit = (node) => {
      if (ts.isCallExpression(node) && ts.isIdentifier(node.expression)) {
        const macro = node.expression.text
        const kind = { defineProps: 'props', defineEmits: 'emits', defineSlots: 'slots' }[macro]
        if (kind) {
          const type = node.typeArguments?.[0]
          const members = typeMembers(type)
          const argument = unwrap(node.arguments[0])
          if (members) {
            for (const member of members) {
              if (ts.isPropertySignature(member) || ts.isMethodSignature(member)) {
                const name = propertyName(member.name)
                if (name === null) { unknown(kind, member); continue }
                add(kind, { name, type: source(member.type), required: !member.questionToken, line: line(member), declaration: source(member) })
              } else if (kind === 'emits' && ts.isCallSignatureDeclaration(member)) {
                const eventType = member.parameters[0]?.type
                if (eventType && ts.isLiteralTypeNode(eventType) && ts.isStringLiteralLike(eventType.literal)) add(kind, { name: eventType.literal.text, type: member.parameters.slice(1).map(source).join(', '), line: line(member), declaration: source(member) })
                else unknown(kind, member)
              } else unknown(kind, member)
            }
          } else if (argument && ts.isArrayLiteralExpression(argument)) {
            for (const entry of argument.elements) {
              if (ts.isStringLiteralLike(entry)) add(kind, { name: entry.text, type: 'unknown', line: line(entry) })
              else unknown(kind, entry)
            }
          } else if (argument && ts.isObjectLiteralExpression(argument)) {
            for (const property of argument.properties) {
              const name = propertyName(property.name)
              if (name !== null && ts.isPropertyAssignment(property)) add(kind, { name, type: 'runtime-declaration', declaration: source(property.initializer), line: line(property) })
              else unknown(kind, property)
            }
          } else if (type || argument) unknown(kind, node)
        }
        if (macro === 'withDefaults' && node.arguments[1] && ts.isObjectLiteralExpression(node.arguments[1])) {
          for (const property of node.arguments[1].properties) {
            if (ts.isPropertyAssignment(property)) {
              // Applied after nested defineProps has been visited below.
              const name = propertyName(property.name)
              if (name !== null) defaults.push({ name, expression: source(property.initializer) })
            }
          }
        }
        if (macro === 'defineModel') {
          const name = node.arguments[0] && ts.isStringLiteralLike(node.arguments[0]) ? node.arguments[0].text : 'modelValue'
          add('props', { name, type: source(node.typeArguments?.[0]), line: line(node), model: true })
          add('emits', { name: `update:${name}`, type: source(node.typeArguments?.[0]), line: line(node), model: true })
        }
        if (macro === 'defineOptions' && node.arguments[0] && ts.isObjectLiteralExpression(node.arguments[0])) {
          const option = node.arguments[0].properties.find((item) => ts.isPropertyAssignment(item) && propertyName(item.name) === 'inheritAttrs')
          if (option?.initializer.kind === ts.SyntaxKind.FalseKeyword) result.inheritAttrs = false
        }
      }
      ts.forEachChild(node, visit)
    }
    const defaults = []
    visit(ast)
    for (const value of defaults) {
      const prop = result.props.find((item) => item.name === value.name)
      if (prop) { prop.defaultExpression = value.expression; prop.required = false }
    }
  }
  const slots = (node) => {
    if (node.type === 1 && node.tag === 'slot') {
      const dynamic = node.props?.find((property) => property.type === 7 && property.arg?.content === 'name')
      const name = node.props?.find((property) => property.type === 6 && property.name === 'name')?.value?.content ?? 'default'
      if (dynamic) result.unknown.push({ kind: 'slots', declaration: dynamic.exp?.content ?? 'dynamic slot', line: node.loc.start.line })
      else add('slots', { name, type: 'template-slot', line: node.loc.start.line })
    }
    for (const child of node.children ?? []) slots(child)
  }
  if (descriptor.template?.ast) slots(descriptor.template.ast)
  for (const kind of ['props', 'emits', 'slots']) result[kind].sort((a, b) => a.name.localeCompare(b.name, 'en'))
  return result
}
