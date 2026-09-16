import { afterEach, describe, expect, it } from 'vitest'
import { currentUiLocale, formatCurrency, formatDateOnly, formatDateTime, formatNumber, formatPercent, formatRelativeTime, formatUnit, i18n, setUiLocale } from './index'

function flatten(value: unknown, prefix = '', out = new Map<string, string>()) {
  if (typeof value === 'string') out.set(prefix, value)
  else if (value && typeof value === 'object') for (const [key, child] of Object.entries(value)) flatten(child, prefix ? `${prefix}.${key}` : key, out)
  return out
}
function placeholders(value: string) { return [...value.matchAll(/\{([\w]+)\}/g)].map((match) => match[1]).sort() }

afterEach(() => setUiLocale('zh-CN'))
describe('CoffeeLink locale runtime', () => {
  it('keeps zh-CN and en-US message keys and interpolation parameters aligned', () => {
    const zh = flatten(i18n.global.getLocaleMessage('zh-CN'))
    const en = flatten(i18n.global.getLocaleMessage('en-US'))
    expect([...zh.keys()].sort()).toEqual([...en.keys()].sort())
    for (const [key, value] of zh) expect(placeholders(value)).toEqual(placeholders(en.get(key)!))
  })
  it('persists only a presentation preference and switches the document language', () => {
    setUiLocale('en-US')
    expect(currentUiLocale()).toBe('en-US')
    expect(document.documentElement.lang).toBe('en-US')
    expect(localStorage.getItem('coffeelink.locale')).toBe('en-US')
    expect(i18n.global.t('navigation.primary.enterprise')).toBe('Enterprise Center')
  })
  it('formats numbers, units, percent and currency without converting business values', () => {
    setUiLocale('en-US')
    expect(formatNumber(1234.5)).toContain('1,234.5')
    expect(formatPercent(0.125)).toContain('13%')
    expect(formatCurrency(12.5, 'EUR')).toMatch(/12\.50|12,50/)
    expect(formatUnit(45, 'celsius')).toMatch(/45/)
  })
  it('keeps date-only values stable and applies explicit timezone/DST to instants', () => {
    setUiLocale('en-US')
    expect(formatDateOnly('2026-03-29')).toMatch(/03\/29\/2026|3\/29\/2026/)
    const before = formatDateTime('2026-03-29T00:30:00Z', 'Europe/Berlin')
    const after = formatDateTime('2026-03-29T01:30:00Z', 'Europe/Berlin')
    expect(before).toMatch(/01:30|1:30/)
    expect(after).toMatch(/03:30|3:30/)
    expect(formatRelativeTime('2026-09-16T11:00:00Z', '2026-09-16T12:00:00Z')).toMatch(/hour|小时/)
  })
})
