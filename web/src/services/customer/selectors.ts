import type { CustomerSnapshot, Plan, WorkItem } from '@/types/customer'

/** Scope every professional-business projection to the current customer and work. */
export function workSources(s: CustomerSnapshot, work: WorkItem, linked = false) {
  return s.sources.filter(
    (source) =>
      source.customerId === work.customerId &&
      (!source.facts.workId || source.facts.workId === work.id) &&
      (!linked || work.evidenceIds.includes(source.id)),
  )
}
export function workSites(s: CustomerSnapshot, work: WorkItem) {
  return s.sites.filter((site) => site.customerId === work.customerId && site.workId === work.id)
}
export function receivableFor(s: CustomerSnapshot, work: WorkItem) {
  const records = workSources(s, work, true).filter((source) => source.kind === 'receivable')
  const superseded = new Set(records.map((source) => String(source.facts.original || '')))
  return records.filter((source) => !superseded.has(source.id)).at(-1)
}
export function planResults(s: CustomerSnapshot, plan: Plan) {
  const work = s.work.filter(
    (item) =>
      item.customerId === plan.customerId &&
      (item.planId === plan.id || plan.milestones.some((m) => m.workIds.includes(item.id))),
  )
  return plan.targets.map((target) => ({
    ...target,
    actual:
      target.source === '逐点投放验收'
        ? s.sites.filter(
            (site) =>
              site.customerId === plan.customerId && site.accepted && work.some((w) => w.id === site.workId),
          ).length
        : target.source === '已生效续约合同'
          ? work.filter((w) => w.kind === 'renewal' && w.resolution === '成功').length
          : target.source === '运行验证与客户确认'
            ? work.filter((w) => w.kind === 'service' && w.resolution === '成功').length
            : target.actual,
  }))
}
export function outcomeMetrics(s: CustomerSnapshot, month = '2026-09') {
  const cohort = s.sources.filter(
    (r) => r.kind === 'contract' && String(r.facts.end).startsWith(month) && !r.facts.renewal,
  )
  const renewed = cohort.filter((contract) =>
    s.work.some(
      (w) =>
        w.customerId === contract.customerId &&
        w.kind === 'renewal' &&
        w.resolution === '成功' &&
        w.evidenceIds.includes(contract.id),
    ),
  ).length
  const receivables = new Map(
    s.work
      .filter((w) => w.kind === 'payment')
      .map((w) => receivableFor(s, w))
      .filter((r) => !!r)
      .map((r) => [`${r.customerId}:${r.facts.original || r.id}`, r]),
  )
  return {
    cohort: cohort.length,
    renewed,
    renewalRate: cohort.length ? `${Math.round((renewed / cohort.length) * 100)}%` : '—',
    paid: [...receivables.values()].reduce((sum, r) => sum + Number(r.facts.paid || 0), 0),
    due: [...receivables.values()].reduce((sum, r) => sum + Number(r.facts.due || 0), 0),
    deliveryTotal: s.sites.length,
    deliveryPassed: s.sites.filter((x) => x.accepted).length,
    success: s.work.filter((w) => w.resolution === '成功').length,
    closed: s.work.filter((w) => w.status === '已结束').length,
  }
}
