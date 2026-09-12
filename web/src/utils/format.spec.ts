import { describe, it, expect } from 'vitest'
import { csvCell } from './format'
describe('safe CSV formatting', () => {
  it.each(['=SUM(A1:A2)', '+cmd', '-1+1', '@HYPERLINK', '\tformula'])(
    'neutralizes spreadsheet formula %s',
    (value) => expect(csvCell(value)).toContain("'" + value),
  )
  it('escapes double quotes', () => expect(csvCell('a"b')).toBe('"a""b"'))
})
