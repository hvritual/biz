import type { CustomerSnapshot, WorkItem, SourceRecord, FormValues } from '@/types/customer'

export class CustomerRuleError extends Error {
  constructor(
    public code: string,
    message: string,
  ) {
    super(message)
    this.name = 'CustomerRuleError'
  }
}
export function assertRule(condition: unknown, code: string, message: string): asserts condition {
  if (!condition) throw new CustomerRuleError(code, message)
}
export function requireText(values: FormValues, key: string, label: string): string {
  const value = String(values[key] ?? '').trim()
  assertRule(value.length > 0, 'REQUIRED', `请填写${label}`)
  return value
}
export function requireVersion(actual: number, expected?: number) {
  assertRule(
    expected === undefined || actual === expected,
    'VERSION_CONFLICT',
    '该记录已由其他成员更新。你的草稿已保留，请对比最新版本后重新提交。',
  )
}
export function workById(s: CustomerSnapshot, id: string) {
  const work = s.work.find((w) => w.id === id)
  assertRule(work, 'NOT_FOUND', '事项不存在或不属于当前租户')
  return work
}
export function editable(work: WorkItem, version?: number) {
  assertRule(work.writable, 'FORBIDDEN', '你没有此事项的编辑权限；原记录保持不变')
  requireVersion(work.version, version)
}
export function sourceById(s: CustomerSnapshot, work: WorkItem, id: string): SourceRecord {
  const source = s.sources.find((e) => e.id === id && e.customerId === work.customerId)
  assertRule(
    source?.verified,
    'UNVERIFIED_SOURCE',
    '请选择同一客户下已核验的业务来源；附件和草稿不能代替业务结果',
  )
  assertRule(
    !source.facts.workId || source.facts.workId === work.id,
    'EVIDENCE_SCOPE',
    '业务证据的事项范围不匹配，不能用于本次验收',
  )
  return source
}
export function closeRequirements(s: CustomerSnapshot, work: WorkItem, evidence: string[]): string[] {
  const sources = evidence.map((id) => sourceById(s, work, id))
  const has = (kind: SourceRecord['kind'], key: string, value: boolean | number = true) =>
    sources.some((e) => e.kind === kind && e.facts[key] === value)
  const missing: string[] = []
  if (work.kind === 'delivery') {
    const count = s.sites.filter(
      (site) => site.customerId === work.customerId && site.workId === work.id,
    ).length
    if (!count) missing.push('明确的逐点交付范围')
    if (
      !sources.some(
        (e) =>
          e.kind === 'acceptance' &&
          e.facts.allPassed === true &&
          e.facts.total === count &&
          e.facts.passed === count,
      )
    )
      missing.push('逐点复验全部通过')
    if (!sources.some((e) => e.kind === 'placement' && e.facts.active === true && e.facts.sites === count))
      missing.push('有效投放关系')
  }
  if (work.kind === 'service') {
    if (!has('service', 'workComplete')) missing.push('运维工单已完成')
    if (!has('recovery', 'recovered')) missing.push('设备恢复验证通过')
    if (!has('confirmation', 'confirmed')) missing.push('客户确认恢复')
  }
  if (
    work.kind === 'payment' &&
    !sources.some(
      (e) =>
        e.kind === 'receivable' &&
        e.facts.balance === 0 &&
        Number(e.facts.due) > 0 &&
        Number(e.facts.paid) >= Number(e.facts.due),
    )
  )
    missing.push('对应应收已全额核销（未核销余额为 0）')
  if (
    work.kind === 'renewal' &&
    !sources.some(
      (e) =>
        e.kind === 'contract' &&
        e.state === '已生效' &&
        e.facts.renewal === true &&
        work.evidenceIds.includes(String(e.facts.original)),
    )
  )
    missing.push('关联原合同的生效续约合同')
  if (work.kind === 'return') {
    if (!has('recovery', 'returned')) missing.push('实物全部回收')
    if (!has('settlement', 'settled')) missing.push('结算完成')
    if (!has('termination', 'terminated')) missing.push('投放关系已终止且有效期明确')
  }
  return missing
}
export function canArchive(s: CustomerSnapshot, id: string): string[] {
  const blockers: string[] = []
  if (s.work.some((w) => w.customerId === id && w.status !== '已结束')) blockers.push('仍有未结束事项')
  if (
    s.sources.some(
      (e) => e.customerId === id && e.kind === 'contract' && ['生效中', '已生效'].includes(e.state),
    )
  )
    blockers.push('仍有有效合同')
  if (s.sources.some((e) => e.customerId === id && e.kind === 'receivable' && Number(e.facts.balance) > 0))
    blockers.push('仍有未结应收')
  const customer = s.customers.find((c) => c.id === id)
  if (customer && customer.sites > 0) blockers.push('仍有有效投放点位')
  return blockers
}
export function assertNoCycle(s: CustomerSnapshot, id: string, dependency: string) {
  assertRule(id !== dependency, 'DEPENDENCY_CYCLE', '事项不能依赖自身')
  const visited = new Set<string>()
  function reaches(current: string): boolean {
    if (current === id) return true
    if (visited.has(current)) return false
    visited.add(current)
    return s.work.find((w) => w.id === current)?.dependencies.some(reaches) ?? false
  }
  assertRule(!reaches(dependency), 'DEPENDENCY_CYCLE', '此依赖会形成循环，请调整依赖关系')
}
export function clientProjection(s: CustomerSnapshot, grantId: string, now = new Date()) {
  const grant = s.grants.find((g) => g.id === grantId)
  assertRule(
    grant && !grant.revoked && new Date(`${grant.expires}T23:59:59`).getTime() >= now.getTime(),
    'SHARE_EXPIRED',
    '共享已撤销或已过期，请联系服务方重新授权',
  )
  const work = workById(s, grant.workId)
  assertRule(work.customerId === grant.customerId, 'SHARE_SCOPE', '授权客户与事项归属不匹配')
  const allowed = new Set(grant.fields)
  return {
    grantId: grant.id,
    customerName: s.customers.find((c) => c.id === grant.customerId)?.name,
    workId: work.id,
    title: allowed.has('事项标题') ? work.title : '已授权事项',
    status: allowed.has('处理进度') ? work.status : undefined,
    sites: allowed.has('交付范围')
      ? s.sites
          .filter((site) => site.customerId === work.customerId && site.workId === work.id)
          .map((site) => ({
            id: site.id,
            name: site.name,
            device: site.device,
            accepted: site.accepted,
          }))
      : [],
    attachments: s.sources
      .filter(
        (e) =>
          allowed.has('共享附件') &&
          grant.attachments.includes(e.id) &&
          e.customerId === work.customerId &&
          (!e.facts.workId || e.facts.workId === work.id),
      )
      .map((e) => ({ id: e.id, title: e.title, state: e.state })),
    activities: s.activities
      .filter((a) => a.target === work.id && a.visibility === 'shared')
      .map((a) => ({ action: a.action, detail: a.detail, time: a.time })),
    expires: grant.expires,
  }
}
