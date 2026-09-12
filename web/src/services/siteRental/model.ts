import type { CustomerSnapshot } from '@/types/customer'
import type { RentalRule, RentalState, RentalMode } from '@/types/siteRental'
import { createRentalSeed } from './seed'
export const modeNames: Record<RentalMode, string> = {
  fixed: '固定月租',
  metered: '按杯计费',
  included: '保底与超量',
}
export const reviewPeriod = '2026-09'
export const money = (cents: number | null | undefined) =>
  cents == null
    ? '—'
    : new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY' }).format(cents / 100)
export const rentalState = (s: CustomerSnapshot): RentalState => {
  const state = s.rental ?? createRentalSeed(s)
  return {
    ...state,
    profiles: state.profiles.map((site) => {
      const delivery = s.sites.find((x) => x.id === site.id && x.customerId === site.customerId)
      return delivery?.accepted && site.phase === '安装验收中' ? { ...site, phase: '服务中' as const } : site
    }),
  }
}
export function activeRule(
  r: RentalState,
  groupId: string,
  period: string,
  sources: CustomerSnapshot['sources'],
) {
  const rule = r.rules
    .filter((x) => x.groupId === groupId && x.effectiveFrom <= `${period}-01`)
    .sort((a, b) => b.effectiveFrom.localeCompare(a.effectiveFrom))[0]
  if (!rule) return undefined
  const contract = sources.find(
    (x) => x.id === rule.contractId && x.customerId === rule.customerId && x.kind === 'contract',
  )
  // Expired contracts cannot shadow a successor group's terms. Missing sources still reach
  // quote validation, where they block confirmation rather than silently becoming zero fees.
  if (contract && String(contract.facts.end) < `${period}-01`) return undefined
  return rule
}
export function currentRules(r: RentalState, period: string, sources: CustomerSnapshot['sources']) {
  return [...new Set(r.rules.map((x) => x.groupId))]
    .map((id) => activeRule(r, id, period, sources))
    .filter((x): x is RentalRule => !!x)
}
export const ruleForSite = (
  r: RentalState,
  siteId: string,
  period: string,
  sources: CustomerSnapshot['sources'],
) => currentRules(r, period, sources).find((x) => x.siteIds.includes(siteId))
export const currentDeployments = (r: RentalState, siteId: string, date: string) =>
  r.deployments.filter((d) => d.siteId === siteId && d.from <= date && (!d.until || d.until > date))
export function summary(rule: RentalRule) {
  if (rule.mode === 'fixed')
    return `${money(rule.baseCents)} / ${rule.fixedUnit === 'device' ? '台' : '点位'} / 月`
  if (rule.mode === 'metered') return `${money(rule.unitCents)} / 可计费杯`
  return rule.minimumKind === 'minimum-spend'
    ? `最低消费 ${money(rule.baseCents)} / 月 · ${money(rule.unitCents)} / 杯`
    : `${money(rule.baseCents)} 含 ${rule.includedCups.toLocaleString()} 杯 · 超量 ${money(rule.unitCents)} / 杯`
}
export function periodEnd(period: string) {
  const [year, month] = period.split('-').map(Number)
  return new Date(Date.UTC(year!, month!, 0)).toISOString().slice(0, 10)
}
export function nextPeriod(date = new Date()) {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + 1, 1)).toISOString().slice(0, 10)
}
export function siteWorkIds(s: CustomerSnapshot, siteId: string) {
  return [
    ...new Set([
      ...s.sites.filter((x) => x.id === siteId).map((x) => x.workId),
      ...s.work.filter((x) => x.siteIds?.includes(siteId)).map((x) => x.id),
    ]),
  ]
}
