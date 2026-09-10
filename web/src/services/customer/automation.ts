import type { AutomationRule, CustomerSnapshot } from '@/types/customer'
export interface RuleMatch {
  objectId: string
  businessKey: string
  existing: string
  action: string
}
export function evaluateRule(s: CustomerSnapshot, rule: AutomationRule, asOf = '2026-09-10'): RuleMatch[] {
  const day = (date: string) => Math.floor(new Date(date.slice(0, 10) + 'T00:00:00Z').getTime() / 86400000)
  if (rule.trigger === '合同到期')
    return s.sources
      .filter(
        (e) =>
          e.kind === 'contract' &&
          e.verified &&
          !e.facts.renewal &&
          day(String(e.facts.end)) >= day(asOf) &&
          day(String(e.facts.end)) - day(asOf) <= rule.days,
      )
      .map((e) => {
        const businessKey = `${s.tenant}:${e.id}:${String(e.facts.end).slice(0, 7)}`
        const existing = s.work.find((w) => w.businessKey === businessKey)
        return {
          objectId: e.id,
          businessKey,
          existing: existing?.id || '',
          action: existing ? '复用已有事项，不重复创建' : '预计创建续约事项',
        }
      })
  if (rule.trigger === '服务超时')
    return s.work
      .filter(
        (w) => w.kind === 'service' && w.status !== '已结束' && day(asOf) - day(w.deadline) >= rule.days,
      )
      .map((w) => ({
        objectId: w.id,
        businessKey: `${s.tenant}:${w.id}:sla`,
        existing: w.id,
        action: '预计升级提醒，不创建重复故障',
      }))
  if (rule.trigger === '长期未联系')
    return s.customers
      .filter(
        (c) =>
          c.lifecycle === '合作中' &&
          !c.archived &&
          Number.isFinite(day(c.lastContact)) &&
          day(asOf) - day(c.lastContact) >= rule.days,
      )
      .map((c) => ({
        objectId: c.id,
        businessKey: `${s.tenant}:${c.id}:contact:${asOf.slice(0, 7)}`,
        existing: '',
        action: '预计创建客户回访事项',
      }))
  return []
}
