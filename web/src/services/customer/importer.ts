import type { CustomerSnapshot, FormValues } from '@/types/customer'
import { assertRule } from './policy'
import { operators } from './seed'
import { executeCustomerCommand } from './commands'
export interface ImportRow {
  line: number
  name: string
  category: string
  owner: string
  result: '通过' | '疑似重复' | '字段错误' | '已导入'
  reason: string
  existing: string
  selected: boolean
}
export function parseCustomerCsv(raw: string): string[][] {
  assertRule(raw.length <= 1024 * 1024, 'TOO_LARGE', 'CSV 文件不能超过 1 MB')
  const rows: string[][] = []
  let row: string[] = [],
    cell = '',
    quoted = false
  const text = raw.replace(/^\uFEFF/, '')
  for (let i = 0; i < text.length; i++) {
    const c = text[i]
    if (c === '"') {
      if (quoted && text[i + 1] === '"') {
        cell += '"'
        i++
      } else quoted = !quoted
    } else if (c === ',' && !quoted) {
      row.push(cell)
      cell = ''
    } else if ((c === '\r' || c === '\n') && !quoted) {
      if (c === '\r' && text[i + 1] === '\n') i++
      row.push(cell)
      if (row.some((x) => x.trim())) rows.push(row)
      row = []
      cell = ''
    } else cell += c
  }
  assertRule(!quoted, 'MALFORMED_CSV', 'CSV 引号未闭合，请修正后重试')
  row.push(cell)
  if (row.some((x) => x.trim())) rows.push(row)
  assertRule(rows.length > 1 && rows.length <= 501, 'ROW_LIMIT', '需包含表头和 1–500 条客户记录')
  return rows
}
export function validateImport(
  data: string[][],
  mapping: { name: number; category: number; owner: number },
  s: CustomerSnapshot,
): ImportRow[] {
  assertRule(new Set(Object.values(mapping)).size === 3, 'BAD_MAPPING', '三个字段必须映射到不同列')
  const names = new Set<string>()
  return data.slice(1).map((row, index) => {
    const name = String(row[mapping.name] || '').trim(),
      category = String(row[mapping.category] || '').trim(),
      owner = String(row[mapping.owner] || '').trim()
    const existing = s.customers.find((c) => c.name.trim() === name)
    let result: ImportRow['result'] = '通过',
      reason = '可以创建客户资料'
    if (!name || !category || !operators.includes(owner)) {
      result = '字段错误'
      reason = !name ? '客户名称不能为空' : !category ? '客户类型不能为空' : '负责人不在当前预览成员范围'
    } else if (existing || names.has(name)) {
      result = '疑似重复'
      reason = existing ? `已有 ${existing.id}，不覆盖已有客户` : '文件内同名客户重复'
    }
    names.add(name)
    return {
      line: index + 2,
      name,
      category,
      owner,
      result,
      reason,
      existing: existing?.id || '',
      selected: result === '通过',
    }
  })
}
export function applyCustomerImport(s: CustomerSnapshot, rows: ImportRow[], batch: string) {
  let next = s
  const results: ImportRow[] = []
  for (const row of rows) {
    if (!row.selected || row.result !== '通过') {
      results.push({ ...row })
      continue
    }
    try {
      const values: FormValues = {
        name: row.name,
        category: row.category,
        owner: row.owner,
        lifecycle: '潜在客户',
        mode: '设备租赁 + 服务',
        area: '华东',
      }
      const result = executeCustomerCommand(next, {
        action: 'create-customer',
        id: '',
        values,
        key: `import:${batch}:${row.line}:${row.name}`,
      })
      next = result.snapshot
      results.push({
        ...row,
        result: '已导入',
        existing: result.result.target,
        reason: `已创建 ${result.result.target}`,
        selected: false,
      })
    } catch (e) {
      results.push({ ...row, result: '字段错误', reason: (e as Error).message, selected: false })
    }
  }
  return { snapshot: next, results }
}
