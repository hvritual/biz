import type { CustomerSnapshot } from '@/types/customer'
import type { RentalRule, RentalState, RentalQuote, QuoteLine } from '@/types/siteRental'
import { fingerprint, requireRental, validMonth } from './policy'
import { periodEnd, currentDeployments } from './model'
export function calculateLine(
  rule: RentalRule,
  cups: number | null,
  units: number,
  title: string,
  key: string,
): QuoteLine {
  if (cups !== null)
    requireRental(Number.isSafeInteger(cups) && cups >= 0 && cups <= 1000000000, '可计费杯数须为有效非负整数')
  let base = rule.baseCents,
    over = 0,
    extra: number | null = 0,
    total: number | null = base
  if (rule.mode === 'fixed') base = total = rule.baseCents * units
  else if (cups === null) {
    extra = total = null
  } else if (rule.mode === 'metered') {
    base = 0
    extra = total = cups * rule.unitCents
  } else if (rule.minimumKind === 'minimum-spend') {
    total = Math.max(base, cups * rule.unitCents)
    extra = total - base
  } else {
    over = Math.max(cups - rule.includedCups, 0)
    extra = over * rule.unitCents
    total = base + extra
  }
  requireRental(total === null || Number.isSafeInteger(total), '金额超出安全计算范围')
  return {
    key,
    title,
    cups,
    baseCents: base,
    includedCups: rule.mode === 'included' && rule.minimumKind === 'included' ? rule.includedCups : 0,
    overCups: cups === null ? null : over,
    extraCents: extra,
    totalCents: total,
  }
}
export function quoteRental(
  s: CustomerSnapshot,
  r: RentalState,
  rule: RentalRule,
  period: string,
  samples?: Record<string, number>,
): RentalQuote {
  requireRental(validMonth(period), '请选择有效账期')
  const blockers: string[] = [],
    start = `${period}-01`,
    end = periodEnd(period)
  const contract = s.sources.find(
    (x) => x.id === rule.contractId && x.customerId === rule.customerId && x.kind === 'contract',
  )
  if (!samples) {
    if (!contract?.verified || String(contract.facts.start) > start || String(contract.facts.end) < end)
      blockers.push('合同未覆盖完整账期；需核对起止与折算约定')
    if (rule.effectiveFrom > start) blockers.push('所选规则在该账期尚未生效')
  }
  const contributions = rule.siteIds.map((id) => {
    const site = r.profiles.find((x) => x.id === id)
    requireRental(
      site && site.customerId === rule.customerId && site.kind === 'site',
      '计费范围包含无权访问或不存在的点位',
    )
    const usage = r.usage.find((u) => u.siteId === id && u.period === period)
    if (samples) {
      const cups = samples[id]
      requireRental(
        cups !== undefined && Number.isSafeInteger(cups) && cups >= 0,
        '请填写每个点位的非负整数试算杯数',
      )
      return { siteId: id, name: site.name, raw: cups, excluded: 0, cups, quality: '试算输入' }
    }
    const cups = usage && usage.quality === '完整' ? usage.raw - usage.excluded : null
    if (usage)
      requireRental(
        Number.isSafeInteger(usage.raw) &&
          Number.isSafeInteger(usage.excluded) &&
          usage.raw >= usage.excluded &&
          usage.excluded >= 0,
        '用量口径异常，排除杯数不能超过原始杯数',
      )
    if (cups === null && rule.mode !== 'fixed')
      blockers.push(`${site.name}：${usage ? '数据待补传' : '用量未提供'}，不能按零计费`)
    if (site.phase === '待勘察' || site.phase === '安装验收中') blockers.push(`${site.name}：起租验收待核对`)
    if (site.operation === '临时停用') blockers.push(`${site.name}：停用费用约定需核对，不自动停租`)
    const placements = r.deployments.filter(
      (d) => d.siteId === id && d.from <= end && (!d.until || d.until > start),
    )
    const until = new Date(Date.parse(`${end}T00:00:00Z`) + 86400000).toISOString().slice(0, 10)
    const boundaries = [
      ...new Set([
        start,
        until,
        ...placements.flatMap((d) => [d.from, d.until].filter((x) => x > start && x < until)),
      ]),
    ].sort()
    const counts = boundaries
      .slice(0, -1)
      .map((date) => placements.filter((d) => d.from <= date && (!d.until || d.until > date)).length)
    if (counts.some((n) => n === 0)) blockers.push(`${site.name}：账期投放存在缺口，需先核对起租与折算约定`)
    if (rule.mode === 'fixed' && rule.fixedUnit === 'device' && new Set(counts).size > 1)
      blockers.push(`${site.name}：月内在租设备数量变化，折算约定尚待核对`)
    return {
      siteId: id,
      name: site.name,
      raw: usage?.raw ?? null,
      excluded: usage?.excluded ?? null,
      cups,
      quality: usage?.quality ?? '无数据',
    }
  })
  let lines: QuoteLine[]
  if (rule.scope === 'shared') {
    const known = contributions.every((c) => c.cups !== null)
    lines = [
      calculateLine(
        rule,
        known ? contributions.reduce((sum, c) => sum + c.cups!, 0) : null,
        1,
        rule.name,
        rule.groupId,
      ),
    ]
  } else
    lines = contributions.map((c) =>
      calculateLine(
        rule,
        c.cups,
        rule.fixedUnit === 'device' ? currentDeployments(r, c.siteId, end).length : 1,
        c.name,
        c.siteId,
      ),
    )
  const total = lines.every((l) => l.totalCents !== null)
    ? lines.reduce((sum, l) => sum + l.totalCents!, 0)
    : null
  requireRental(total === null || Number.isSafeInteger(total), '合计金额超出安全计算范围')
  return {
    period,
    ruleId: rule.id,
    ruleVersion: rule.version,
    mode: rule.mode,
    scope: rule.scope,
    contributions,
    lines,
    totalCents: total,
    blockers,
    sourceFingerprint: fingerprint({
      rule,
      period,
      contributions,
      placements: r.deployments.filter((x) => rule.siteIds.includes(x.siteId)),
      contract,
      blockers,
    }),
  }
}
