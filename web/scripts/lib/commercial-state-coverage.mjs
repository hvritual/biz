import { readFileSync, readdirSync, realpathSync } from 'node:fs'
import { isAbsolute, relative, resolve } from 'node:path'
import { StaticSource } from './static-source.mjs'

// A bounded source reader, not a second editable state catalog. Values always
// come from the existing Go constants/return sites and public Proto enums.
export function uncomment(source) {
  let output = ''
  for (let i = 0; i < source.length;) {
    const c = source[i]
    if (source.startsWith('//', i)) {
      const end = source.indexOf('\n', i)
      if (end < 0) break
      output += ' '.repeat(end - i)
      i = end
    } else if (source.startsWith('/*', i)) {
      const end = source.indexOf('*/', i + 2)
      if (end < 0) throw new Error('Unterminated source comment')
      output += source.slice(i, end + 2).replace(/[^\n]/g, ' ')
      i = end + 2
    } else if (c === '"' || c === "'" || c === '`') {
      let j = i + 1
      while (j < source.length && source[j] !== c) {
        if (source[j] === '\\' && c !== '`') j++
        j++
      }
      if (j >= source.length) throw new Error('Unterminated source literal')
      output += source.slice(i, j + 1)
      i = j + 1
    } else {
      output += c
      i++
    }
  }
  return output
}

function delimited(source, open, left, right) {
  let depth = 0
  for (let i = open; i < source.length; i++) {
    if (source[i] === '"' || source[i] === "'" || source[i] === '`') {
      const quote = source[i++]
      while (i < source.length && source[i] !== quote) {
        if (source[i] === '\\' && quote !== '`') i++
        i++
      }
    } else if (source[i] === left) depth++
    else if (source[i] === right && --depth === 0) return source.slice(open + 1, i)
  }
  throw new Error(`Unterminated ${left}${right} source block`)
}

function maskLiterals(source) {
  return source.replace(/"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|`[^`]*`/g,
    (literal) => literal.replace(/[^\n]/g, ' '))
}

export function goConstants(source, required = true) {
  const clean = uncomment(source)
  const records = []
  const add = (line) => {
    const entry = /^\s*(\w+)(?:\s+(\w+))?\s*=\s*("(?:\\.|[^"\\])*")\s*$/.exec(line)
    if (!entry) throw new Error(`Unsupported commercial constant declaration: ${line.trim()}`)
    if (records.some((item) => item.name === entry[1])) throw new Error(`Duplicate commercial constant ${entry[1]}`)
    records.push({ name: entry[1], type: entry[2] ?? '', value: JSON.parse(entry[3]) })
  }
  // Mask strings only while locating declarations. A raw string containing
  // apparent Go source must never manufacture an authoritative state.
  for (const match of maskLiterals(clean).matchAll(/\bconst\b/g)) {
    const offset = match.index + match[0].length
    const next = /\S/.exec(clean.slice(offset))
    if (!next) throw new Error('Incomplete commercial constant declaration')
    const open = offset + next.index
    if (clean[open] === '(') {
      const body = delimited(clean, open, '(', ')')
      for (const line of body.split(/\n|;/).filter((line) => line.trim())) add(line)
    } else {
      add(clean.slice(open).split(/\n|;/, 1)[0])
    }
  }
  if (required && !records.length) throw new Error('No commercial constant declaration found')
  return records
}

export function protoEnum(source, name) {
  const clean = uncomment(source)
  const match = new RegExp(`\\benum\\s+${name}\\s*\\{`).exec(clean)
  if (!match) throw new Error(`Missing Proto enum ${name}`)
  const body = delimited(clean, match.index + match[0].lastIndexOf('{'), '{', '}')
  const rows = body.split(';').map((row) => row.trim()).filter(Boolean)
  const values = rows.map((row) => {
    const entry = /^(\w+)\s*=\s*\d+$/.exec(row)
    if (!entry) throw new Error(`Unsupported enum ${name}: ${row}`)
    return entry[1]
  })
  if (!values.length || values.length !== new Set(values).size) throw new Error(`Empty or duplicate enum ${name}`)
  return values
}

function functionBody(source, name) {
  const clean = uncomment(source)
  const match = new RegExp(`^func\\s+(?:\\([^\\n]+?\\)\\s+)?${name}\\([^\\n]*\\)\\s+[^\\n{]+\\{`, 'm').exec(clean)
  if (!match) throw new Error(`Missing canonical function ${name}`)
  return delimited(clean, match.index + match[0].lastIndexOf('{'), '{', '}')
}

export function literalReturns(source, name) {
  let body = functionBody(source, name)
  // Nested helper functions return their own values, not this method's status.
  // Mask their bodies before inspecting every remaining return expression.
  let lexical = maskLiterals(body)
  for (const match of [...lexical.matchAll(/\bfunc\s*\(/g)].reverse()) {
    const open = lexical.indexOf('{', match.index)
    if (open < 0) throw new Error(`Unsupported nested function in ${name}`)
    const nested = delimited(body, open, '{', '}')
    const end = open + nested.length + 2
    body = body.slice(0, match.index) + body.slice(match.index, end).replace(/[^\n]/g, ' ') + body.slice(end)
  }
  lexical = maskLiterals(body)
  const values = [...lexical.matchAll(/\breturn\b/g)].map((match) => {
    const tail = body.slice(match.index + match[0].length)
    const expression = /^\s*("(?:\\.|[^"\\])*")[ \t]*(?=\n|;|\}|$)/.exec(tail)
    if (!expression) throw new Error(`Unsupported state return in ${name}; update the reader instead of skipping coverage`)
    return JSON.parse(expression[1])
  })
  if (!values.length) throw new Error(`No literal state returns in ${name}; update the source reader, do not skip coverage`)
  return values
}

export function serverStateGroups(repositoryRoot) {
  const root = realpathSync(repositoryRoot)
  function source(path) {
    const file = realpathSync(resolve(root, path))
    const rel = relative(root, file)
    if (rel.startsWith('..') || isAbsolute(rel)) throw new Error(`Source escapes repository: ${path}`)
    return readFileSync(file, 'utf8')
  }
  function packageConstants(directory) {
    const entries = readdirSync(resolve(root, directory)).filter((name) => name.endsWith('.go') && !name.endsWith('_test.go')).sort()
      .flatMap((name) => goConstants(source(`${directory}/${name}`), false))
    if (entries.length !== new Set(entries.map((entry) => entry.name)).size) throw new Error(`Duplicate package constant in ${directory}`)
    return entries
  }
  const domain = 'internal/commercial/domain/'
  const subscription = packageConstants(`${domain}subscription`)
  const plan = packageConstants(`${domain}plan`)
  const module = packageConstants('internal/commercial/modulecatalog')
  const entSource = source(`${domain}entitlement/model.go`)
  const ent = packageConstants(`${domain}entitlement`)
  const changeSource = source(`${domain}subscriptionchange/model.go`)
  const change = packageConstants(`${domain}subscriptionchange`)
  // These are source-symbol references, never copied wire values. An additional
  // constant is rejected until its presentation purpose is explicitly bound.
  const changeBindings = {
    changeAction: ['Switch', 'Renew', 'StopRenewal'],
    effectiveMode: ['Immediate', 'Scheduled'],
    receiptStatus: ['Applied', 'Scheduled', 'Provisioning', 'Failed'],
  }
  const bound = new Set(Object.values(changeBindings).flat())
  for (const entry of change) {
    if (!bound.has(entry.name)) throw new Error(`Unclassified subscription-change constant ${entry.name}`)
  }
  const values = (entries) => entries.map((entry) => entry.value)
  const fromType = (entries, type) => values(entries.filter((entry) => entry.type === type))
  const fromSymbols = (names) => names.map((name) => {
    const entry = change.find((item) => item.name === name)
    if (!entry) throw new Error(`Missing subscription-change symbol ${name}`)
    return entry.value
  })
  const entProto = source('contracts/proto/commercial/v1/entitlement.proto')
  const moduleProto = source('contracts/proto/commercial/v1/module.proto')
  const groups = {
    subscriptionState: values(subscription.filter((entry) => entry.name.startsWith('State'))),
    planState: values(plan),
    technicalStatus: [...fromType(module, 'TechnicalStatus'), ...protoEnum(moduleProto, 'ModuleTechnicalStatus')],
    salesStatus: [...fromType(module, 'SalesStatus'), ...protoEnum(moduleProto, 'ModuleSalesStatus')],
    entitlementTarget: [...fromType(ent, 'Kind'), ...protoEnum(entProto, 'EntitlementTarget')],
    entitlementEffect: [...fromType(ent, 'Effect'), ...protoEnum(entProto, 'EntitlementEffect')],
    sourceKind: fromType(ent, 'SourceKind'),
    sourceState: literalReturns(entSource, 'State'),
    decisionKind: fromType(ent, 'Kind'),
    changeAction: fromSymbols(changeBindings.changeAction),
    effectiveMode: fromSymbols(changeBindings.effectiveMode),
    receiptStatus: fromSymbols(changeBindings.receiptStatus),
    changeClassification: [...literalReturns(changeSource, 'Classify'), ...fromSymbols(['Renew', 'StopRenewal'])],
  }
  for (const [kind, codes] of Object.entries(groups)) {
    if (!codes.length) throw new Error(`Empty commercial state group ${kind}`)
    groups[kind] = [...new Set(codes)].sort()
  }
  return groups
}

export function coverageFailures(groups, catalog, messages) {
  const failures = []
  for (const [kind, codes] of Object.entries(groups)) {
    const terms = catalog[kind]
    if (!terms || typeof terms !== 'object') { failures.push(`${kind}: missing term group`); continue }
    for (const code of codes) {
      if (!Object.hasOwn(terms, code)) { failures.push(`${kind}.${code}: missing wire mapping`); continue }
      const key = terms[code]
      for (const locale of ['zh-CN', 'en-US']) {
        const translation = messages[locale]?.[kind]?.[key]
        if (typeof translation !== 'string' || !translation.trim()) failures.push(`${locale}:${kind}.${code}: missing translation ${key}`)
      }
    }
  }
  return failures
}

export function checkCommercialStateCoverage(webRoot) {
  const reader = new StaticSource(webRoot)
  const catalog = reader.exported(resolve(webRoot, 'src/i18n/backend-terms.ts'), 'backendTermCatalog')
  const messages = reader.exported(resolve(webRoot, 'src/i18n/backend-term-messages.ts'), 'backendTermMessages')
  const groups = serverStateGroups(resolve(webRoot, '..'))
  return { groups, failures: coverageFailures(groups, catalog, messages) }
}
