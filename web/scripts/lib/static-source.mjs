import { existsSync, readFileSync, realpathSync } from 'node:fs'
import { dirname, extname, isAbsolute, relative, resolve } from 'node:path'
import ts from 'typescript'

const componentTag = Symbol('static-component')
export const componentFile = (value) => value?.[componentTag] === true ? value.componentFile : undefined
const component = (file) => ({ [componentTag]: true, componentFile: file })

/** Reads a deliberately small TS data subset. It never imports or executes project code. */
export class StaticSource {
  constructor(root) {
    this.root = realpathSync(root)
    this.modules = new Map()
    this.active = new Set()
  }

  file(path, from = resolve(this.root, 'index.ts')) {
    let target
    if (path.startsWith('@/')) target = resolve(this.root, 'src', path.slice(2))
    else if (isAbsolute(path)) target = path
    else if (path.startsWith('.')) target = resolve(dirname(from), path)
    else throw new Error(`External module is not static project data: ${path}`)
    const candidate = [target, `${target}.ts`, `${target}.js`, `${target}.mjs`, resolve(target, 'index.ts')]
      .find((name) => extname(name) && existsSync(name))
    if (!candidate) throw new Error(`Missing source: ${path} from ${relative(this.root, from)}`)
    const actual = realpathSync(candidate)
    const rel = relative(this.root, actual)
    if (rel.startsWith('..') || isAbsolute(rel)) throw new Error(`Source escapes project: ${path}`)
    return actual
  }

  module(path) {
    const file = this.file(path)
    if (this.modules.has(file)) return this.modules.get(file)
    if (this.modules.size >= 512) throw new Error('Static module traversal limit exceeded')
    const source = readFileSync(file, 'utf8')
    const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
    if (ast.parseDiagnostics.length) throw new Error(`Cannot parse ${file}: ${ts.flattenDiagnosticMessageText(ast.parseDiagnostics[0].messageText, ' ')}`)
    const context = { file, source, ast, locals: new Map(), imports: new Map(), exports: new Map() }
    this.modules.set(file, context)
    for (const statement of ast.statements) {
      if (ts.isImportDeclaration(statement) && ts.isStringLiteral(statement.moduleSpecifier)) {
        const specifier = statement.moduleSpecifier.text
        const clause = statement.importClause
        if (!clause || clause.isTypeOnly) continue
        if (clause.name) context.imports.set(clause.name.text, { specifier, name: 'default' })
        if (clause.namedBindings && ts.isNamedImports(clause.namedBindings)) {
          for (const item of clause.namedBindings.elements) {
            if (!item.isTypeOnly) context.imports.set(item.name.text, { specifier, name: item.propertyName?.text ?? item.name.text })
          }
        }
      }
      if (ts.isVariableStatement(statement)) {
        for (const item of statement.declarationList.declarations) {
          if (!ts.isIdentifier(item.name) || !item.initializer) continue
          context.locals.set(item.name.text, item.initializer)
          if (statement.modifiers?.some((modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword)) {
            context.exports.set(item.name.text, item.name.text)
          }
        }
      }
      if (ts.isExportAssignment(statement)) context.exports.set('default', statement.expression)
      if (ts.isExportDeclaration(statement) && statement.exportClause && ts.isNamedExports(statement.exportClause)) {
        for (const item of statement.exportClause.elements) {
          const original = item.propertyName?.text ?? item.name.text
          if (statement.moduleSpecifier && ts.isStringLiteral(statement.moduleSpecifier)) {
            context.exports.set(item.name.text, { specifier: statement.moduleSpecifier.text, name: original })
          } else context.exports.set(item.name.text, original)
        }
      }
    }
    return context
  }

  fail(context, node, message) {
    const position = context.ast.getLineAndCharacterOfPosition(node.getStart(context.ast))
    throw new Error(`${relative(this.root, context.file)}:${position.line + 1}: ${message}`)
  }

  exported(path, name) {
    const context = this.module(path)
    const key = `${context.file}#${name}`
    if (this.active.has(key)) throw new Error(`Cyclic static export: ${key}`)
    this.active.add(key)
    try {
      const declaration = context.exports.get(name)
      if (!declaration) throw new Error(`Missing static export ${name} in ${context.file}`)
      if (typeof declaration === 'string') return this.identifier(context, declaration)
      if (declaration.specifier) return this.exported(this.file(declaration.specifier, context.file), declaration.name)
      return this.value(context, declaration)
    } finally {
      this.active.delete(key)
    }
  }

  identifier(context, name) {
    const local = context.locals.get(name)
    if (local) {
      const key = `${context.file}:local:${name}`
      if (this.active.has(key)) throw new Error(`Cyclic static declaration: ${key}`)
      this.active.add(key)
      try { return this.value(context, local) } finally { this.active.delete(key) }
    }
    const imported = context.imports.get(name)
    if (imported) {
      const file = this.file(imported.specifier, context.file)
      if (file.endsWith('.vue')) return component(file)
      return this.exported(file, imported.name)
    }
    throw new Error(`Unresolved static identifier ${name} in ${context.file}`)
  }

  value(context, raw, depth = 0) {
    if (depth > 64) this.fail(context, raw, 'Static expression nesting limit exceeded')
    const node = unwrap(raw)
    if (ts.isStringLiteralLike(node)) return node.text
    if (ts.isNumericLiteral(node)) return Number(node.text)
    if (node.kind === ts.SyntaxKind.TrueKeyword) return true
    if (node.kind === ts.SyntaxKind.FalseKeyword) return false
    if (node.kind === ts.SyntaxKind.NullKeyword) return null
    if (ts.isIdentifier(node)) return this.identifier(context, node.text)
    if (ts.isPropertyAccessExpression(node) || ts.isElementAccessExpression(node)) {
      const base = this.value(context, node.expression, depth + 1)
      const key = ts.isPropertyAccessExpression(node) ? node.name.text : this.value(context, node.argumentExpression, depth + 1)
      if (!base || typeof base !== 'object' || !Object.hasOwn(base, key)) this.fail(context, node, `Unknown static property ${key}`)
      return base[key]
    }
    if (ts.isArrayLiteralExpression(node)) {
      const output = []
      for (const element of node.elements) {
        if (ts.isSpreadElement(element)) {
          const values = this.value(context, element.expression, depth + 1)
          if (!Array.isArray(values)) this.fail(context, element, 'Array spread is not static array data')
          output.push(...values)
        } else output.push(this.value(context, element, depth + 1))
      }
      return output
    }
    if (ts.isObjectLiteralExpression(node)) {
      const output = Object.create(null)
      const literalKeys = new Set()
      for (const property of node.properties) {
        if (ts.isSpreadAssignment(property)) {
          const values = this.value(context, property.expression, depth + 1)
          if (!values || typeof values !== 'object' || Array.isArray(values)) this.fail(context, property, 'Object spread is not static object data')
          Object.assign(output, values)
          continue
        }
        if (!ts.isPropertyAssignment(property) && !ts.isShorthandPropertyAssignment(property)) {
          this.fail(context, property, 'Methods/accessors are not supported in static route/navigation data')
        }
        const name = propertyName(property.name)
        if (name === null) this.fail(context, property, 'Computed property cannot be resolved statically')
        if (literalKeys.has(name)) this.fail(context, property, `Duplicate property ${name}`)
        literalKeys.add(name)
        output[name] = ts.isShorthandPropertyAssignment(property)
          ? this.identifier(context, name)
          : this.value(context, property.initializer, depth + 1)
      }
      return output
    }
    if (ts.isArrowFunction(node)) {
      let body = unwrap(node.body)
      if (ts.isBlock(body)) {
        const statements = body.statements
        if (statements.length !== 1 || !ts.isReturnStatement(statements[0]) || !statements[0].expression) {
          this.fail(context, node, 'Component loader must directly return a literal import')
        }
        body = unwrap(statements[0].expression)
      }
      if (ts.isCallExpression(body) && body.expression.kind === ts.SyntaxKind.ImportKeyword && body.arguments.length === 1 && ts.isStringLiteralLike(body.arguments[0])) {
        return component(this.file(body.arguments[0].text, context.file))
      }
      this.fail(context, node, 'Unsupported dynamic loader in static route data')
    }
    this.fail(context, node, `Unsupported static expression: ${ts.SyntaxKind[node.kind]}`)
  }

  router(path) {
    const context = this.module(path)
    const names = new Set([...context.imports].filter(([, item]) => item.specifier === 'vue-router' && item.name === 'createRouter').map(([name]) => name))
    const candidates = []
    const visit = (node) => {
      if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && names.has(node.expression.text)) candidates.push(node)
      ts.forEachChild(node, visit)
    }
    visit(context.ast)
    if (candidates.length !== 1) throw new Error(`Expected exactly one createRouter in ${context.file}`)
    const options = unwrap(candidates[0].arguments[0])
    if (!options || !ts.isObjectLiteralExpression(options)) throw new Error('Router options must be an explicit object')
    const routes = options.properties.filter((item) => ts.isPropertyAssignment(item) && propertyName(item.name) === 'routes')
    if (routes.length !== 1) throw new Error('Router must declare exactly one routes array')
    return flattenRoutes(this.value(context, routes[0].initializer))
  }
}

export function unwrap(node) {
  while (node && (ts.isAsExpression(node) || ts.isSatisfiesExpression(node) || ts.isParenthesizedExpression(node) || ts.isNonNullExpression(node))) node = node.expression
  return node
}

export function propertyName(node) {
  return node && (ts.isIdentifier(node) || ts.isStringLiteralLike(node) || ts.isNumericLiteral(node)) ? node.text : null
}

export function flattenRoutes(routes, parent = '', inherited = Object.create(null)) {
  if (!Array.isArray(routes)) throw new Error('Routes must resolve to an array')
  return routes.flatMap((route) => {
    if (!route || typeof route.path !== 'string') throw new Error('Every route needs a static path')
    const path = route.path.startsWith('/') ? route.path : `${parent.replace(/\/$/, '')}/${route.path}`.replace(/\/$/, '') || '/'
    const meta = { ...inherited, ...route.meta }
    const entry = { ...route, path, meta, branch: Boolean(route.children?.length), children: undefined }
    return [entry, ...flattenRoutes(route.children ?? [], path, meta)]
  })
}

export function readStrictJson(file) {
  const text = readFileSync(file, 'utf8')
  const result = JSON.parse(text)
  const ast = ts.parseJsonText(file, text)
  const visit = (node, path = '$') => {
    if (ts.isObjectLiteralExpression(node)) {
      const seen = new Set()
      for (const property of node.properties) {
        const name = propertyName(property.name)
        if (seen.has(name)) throw new Error(`${file}: duplicate JSON property ${path}.${name}`)
        seen.add(name)
        if (ts.isPropertyAssignment(property)) visit(property.initializer, `${path}.${name}`)
      }
    } else if (ts.isArrayLiteralExpression(node)) node.elements.forEach((value, index) => visit(value, `${path}[${index}]`))
    else ts.forEachChild(node, (child) => visit(child, path))
  }
  visit(ast)
  return result
}
