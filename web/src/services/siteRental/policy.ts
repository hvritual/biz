import type { CustomerSnapshot } from '@/types/customer'
import type { RentalRule, RentalState, RentalDraft } from '@/types/siteRental'
import { nextPeriod, periodEnd } from './model'
export function requireRental(ok: unknown, message: string): asserts ok {
  if (!ok) throw new Error(message)
}
export const fingerprint = (value: unknown) => JSON.stringify(value)
export const draftFingerprint = (draft: RentalDraft) =>
  fingerprint({ rule: draft.rule, samples: draft.samples })
export function validMonth(value: string) {
  return /^\d{4}-(0[1-9]|1[0-2])$/.test(value)
}
function nonnegative(value: number, name: string) {
  requireRental(
    Number.isSafeInteger(value) && value >= 0 && value <= 1000000000,
    `${name}须为非负整数，且不得超出预览上限`,
  )
}
export function validateRule(
  s: CustomerSnapshot,
  r: RentalState,
  rule: RentalRule,
  publishing = false,
  now = new Date(),
) {
  requireRental(
    rule.name.trim() && rule.reason.trim() && rule.billingOwner.trim(),
    '请填写规则名称、结算主体与约定依据',
  )
  requireRental(['fixed', 'metered', 'included'].includes(rule.mode), '请选择有效计费模式')
  requireRental(['independent', 'shared'].includes(rule.scope), '必须明确选择独立计费或共享额度')
  requireRental(
    rule.mode === 'included' || rule.scope === 'independent',
    '月租与按杯模式按点位独立核算；客户汇总不是共享额度',
  )
  requireRental(['included', 'minimum-spend'].includes(rule.minimumKind), '保底算法无效')
  requireRental(['site', 'device'].includes(rule.fixedUnit), '收费单位无效')
  requireRental(['客户汇总', '分别结算'].includes(rule.billPresentation), '账单汇总方式无效')
  requireRental(
    s.customers.some((c) => c.id === rule.customerId && !c.archived),
    '客户不存在、已归档或不在当前租户',
  )
  const contract = s.sources.find(
    (x) => x.id === rule.contractId && x.customerId === rule.customerId && x.kind === 'contract',
  )
  requireRental(contract?.verified, '须选择本客户已核验的有效合同来源')
  requireRental(
    /^\d{4}-(0[1-9]|1[0-2])-01$/.test(rule.effectiveFrom),
    '本轮仅支持月初生效；月中折算尚未接入，不能默认按天除算',
  )
  requireRental(
    String(contract.facts.start) <= rule.effectiveFrom &&
      String(contract.facts.end) >= periodEnd(rule.effectiveFrom.slice(0, 7)),
    '该合同未覆盖规则首个完整账期，请先关联续约或补充合同',
  )
  requireRental(
    rule.siteIds.length > 0 && new Set(rule.siteIds).size === rule.siteIds.length,
    '请选择不重复的实际服务点位',
  )
  requireRental(rule.scope !== 'shared' || rule.siteIds.length >= 2, '共享组至少选择两个点位')
  for (const id of rule.siteIds) {
    const site = r.profiles.find((x) => x.id === id)
    requireRental(
      site && site.customerId === rule.customerId && site.kind === 'site',
      '共享/独立计费范围只能包含同一客户的实际点位，不能包含分组节点',
    )
    requireRental(site.phase !== '已撤场', '已撤场点位不能开通新计费规则')
  }
  nonnegative(rule.baseCents, '基础费用分值')
  nonnegative(rule.unitCents, '单价分值')
  nonnegative(rule.includedCups, '含杯数')
  if (rule.mode === 'fixed') requireRental(rule.baseCents > 0, '月租金额须大于零')
  if (rule.mode !== 'fixed') requireRental(rule.unitCents > 0, '单价须大于零')
  if (rule.mode === 'included') requireRental(rule.baseCents > 0, '基础费用/最低消费须大于零')
  if (rule.mode === 'included' && rule.minimumKind === 'included')
    requireRental(rule.includedCups > 0, '含杯额度须大于零')
  const siblings = r.rules.filter((x) => x.groupId === rule.groupId)
  if (siblings.length) {
    requireRental(
      siblings.every((x) => x.customerId === rule.customerId && x.contractId === rule.contractId),
      '规则版本不能改绑客户或合同；续约应建立新计费组并处理生效边界',
    )
    const latest = siblings.sort((a, b) => b.version - a.version)[0]!
    requireRental(rule.version === latest.version + 1, '规则版本已更新，请重新读取并创建变更')
    requireRental(rule.effectiveFrom > latest.effectiveFrom, '新版本须晚于已有版本生效，不回溯覆盖历史')
  } else requireRental(rule.version === 1, '新计费组必须从版本 1 开始')
  // Compare effective intervals, not merely current memberships: future collisions must also be rejected.
  for (const other of r.rules.filter((x) => x.groupId !== rule.groupId)) {
    const successor = r.rules
      .filter((x) => x.groupId === other.groupId && x.effectiveFrom > other.effectiveFrom)
      .sort((a, b) => a.effectiveFrom.localeCompare(b.effectiveFrom))[0]
    const end = successor?.effectiveFrom || '9999-12-31'
    const otherContract = s.sources.find((x) => x.id === other.contractId)
    const contractEnd = String(otherContract?.facts.end || '9999-12-31')
    if (
      end > rule.effectiveFrom &&
      other.effectiveFrom <= String(contract.facts.end) &&
      contractEnd >= rule.effectiveFrom &&
      other.siteIds.some((id) => rule.siteIds.includes(id))
    )
      throw new Error(`点位已被 ${other.name} 覆盖，生效范围重叠；不得重复计费`)
  }
  if (publishing)
    requireRental(
      rule.effectiveFrom >= nextPeriod(now),
      '规则只能预约下一账期及以后生效，当前账期不可直接改写',
    )
}
