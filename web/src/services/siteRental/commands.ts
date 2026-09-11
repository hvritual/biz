import type { CustomerSnapshot } from '@/types/customer'
import type { RentalDraft, RentalSite, RentalStatement } from '@/types/siteRental'
import { rentalState, activeRule } from './model'
import { draftFingerprint, fingerprint, requireRental, validateRule } from './policy'
import { quoteRental } from './quote'
import { operators } from '@/services/customer/seed'
export type RentalAction =
  | { type: 'save-site'; site: RentalSite }
  | { type: 'save-draft'; draft: RentalDraft }
  | { type: 'publish-rule'; draft: RentalDraft }
  | { type: 'create-statement'; groupId: string; period: string }
  | {
      type: 'refresh-statement' | 'confirm-statement' | 'dispute' | 'resolve-dispute'
      statementId: string
      reason: string
    }
  | { type: 'operation'; siteId: string; operation: RentalSite['operation']; reason: string; nextAt: string }
export interface RentalCommand {
  tenant: string
  expectedRevision: number
  key: string
  action: RentalAction
}
export function executeRentalCommand(before: CustomerSnapshot, c: RentalCommand, now = new Date()) {
  requireRental(c.tenant === before.tenant, '租户已切换，请重新打开表单')
  requireRental(c.key.trim(), '缺少操作标识')
  const previous = rentalState(before).operations.find((x) => x.key === c.key)
  const payload = fingerprint(c.action)
  if (previous) {
    requireRental(previous.payload === payload, '操作标识已用于其他内容，请刷新')
    return { snapshot: before, result: { target: previous.target, detail: previous.detail } }
  }
  requireRental(
    c.expectedRevision === before.revision,
    '数据已被其他窗口更新。草稿仍保留，请重新读取后核对，未覆盖较新记录。',
  )
  const s: CustomerSnapshot = JSON.parse(JSON.stringify(before))
  s.rental = rentalState(s)
  const r = s.rental,
    a = c.action,
    time = now.toISOString(),
    id = () => crypto.randomUUID().slice(0, 8)
  let target = '',
    detail = ''
  if (a.type === 'save-site') {
    const site = a.site,
      existing = r.profiles.find((x) => x.id === site.id)
    requireRental(
      s.customers.some((x) => x.id === site.customerId && !x.archived),
      '请选择当前租户下未归档客户',
    )
    requireRental(
      site.name.trim() && site.address.trim() && site.parent.trim() && site.contact.trim(),
      '名称、位置层级、地址与现场联系人必填',
    )
    requireRental(operators.includes(site.owner), '请选择有效负责人')
    requireRental(['site', 'group'].includes(site.kind), '节点类型无效')
    requireRental(
      !r.profiles.some(
        (x) =>
          x.id !== site.id &&
          x.customerId === site.customerId &&
          x.parent === site.parent &&
          x.name.trim() === site.name.trim(),
      ),
      '同客户、同层级下已存在此点位，请复用已有档案',
    )
    if (existing) {
      requireRental(
        existing.customerId === site.customerId && existing.kind === site.kind,
        '已有点位不能直接改绑客户或节点类型',
      )
      // Lifecycle and service evidence cannot be overwritten by editing an address form.
      r.profiles[r.profiles.indexOf(existing)] = {
        ...site,
        phase: existing.phase,
        operation: existing.operation,
        service: existing.service,
        version: existing.version + 1,
      }
      target = existing.id
    } else {
      target = `SITE-${id()}`
      r.profiles.push({
        ...site,
        id: target,
        phase: '待勘察',
        operation: '正常运营',
        service: '待核实',
        version: 1,
      })
    }
    detail = '点位资料已保存；未自动投放设备、起租或授予客户数据权限'
  } else if (a.type === 'save-draft' || a.type === 'publish-rule') {
    const draft = JSON.parse(JSON.stringify(a.draft)) as RentalDraft
    validateRule(s, r, draft.rule, a.type === 'publish-rule', now)
    target = draft.rule.groupId
    if (a.type === 'save-draft') {
      r.drafts[target] = draft
      detail = '计费规则草稿已保存；未改变生效规则或任何账单'
    } else {
      requireRental(
        draft.testedFingerprint === draftFingerprint(draft),
        '请对当前配置重新试算；修改后旧试算失效',
      )
      quoteRental(s, r, draft.rule, draft.rule.effectiveFrom.slice(0, 7), draft.samples)
      draft.rule.id = `RV-${id()}`
      r.rules.push(draft.rule)
      delete r.drafts[target]
      detail = `版本 v${draft.rule.version} 已预约 ${draft.rule.effectiveFrom} 生效；历史版本及已确认对账单不变`
    }
  } else if (a.type === 'operation') {
    const site = r.profiles.find((x) => x.id === a.siteId)
    requireRental(site && site.kind === 'site' && site.phase !== '已撤场', '当前点位不能调整运营安排')
    requireRental(
      ['正常运营', '临时停用'].includes(a.operation) && a.reason.trim() && a.nextAt,
      '请填写运营安排、原因与下一次核实时间',
    )
    site.operation = a.operation
    site.nextAction = a.operation === '临时停用' ? '核对恢复安排及停用费用约定' : '核对恢复后的设备与服务'
    site.nextAt = a.nextAt
    site.version++
    target = site.id
    detail = '运营安排已更新；没有自动停租、改变额度、回收设备或撤销授权'
  } else if (a.type === 'create-statement') {
    const rule = activeRule(r, a.groupId, a.period, s.sources)
    requireRental(rule, '当前账期没有有效计费规则')
    const existing = r.statements.find((x) => x.groupId === a.groupId && x.period === a.period)
    if (existing) {
      target = existing.id
      detail = '该计费组本账期已有对账单，已返回原记录，未重复生成'
    } else {
      target = `ST-${id()}`
      const item: RentalStatement = {
        id: target,
        groupId: a.groupId,
        period: a.period,
        rule: JSON.parse(JSON.stringify(rule)),
        quote: quoteRental(s, r, rule, a.period),
        state: '草稿',
        history: [{ time, action: '创建草稿', reason: '依据本地示例合同和月末用量快照' }],
        confirmedAt: '',
      }
      r.statements.push(item)
      detail = '对账草稿已生成；不是应收账单，未记账或扣款'
    }
  } else {
    const statement = r.statements.find((x) => x.id === a.statementId)
    requireRental(statement, '对账单不存在或不在当前租户')
    requireRental(statement.state !== '已确认', '已确认快照不可覆盖；差异需通过后续调整流程处理')
    requireRental(a.reason.trim(), '请填写操作依据')
    target = statement.id
    if (a.type === 'refresh-statement' || a.type === 'confirm-statement') {
      const rule = activeRule(r, statement.groupId, statement.period, s.sources)
      requireRental(rule, '未找到该账期规则')
      const current = quoteRental(s, r, rule, statement.period)
      if (a.type === 'refresh-statement') {
        requireRental(statement.state === '草稿', '请先记录异议处理结论再刷新')
        statement.rule = JSON.parse(JSON.stringify(rule))
        statement.quote = current
        detail = '草稿已按最新来源重新核对；旧核对动作保留'
      } else {
        requireRental(statement.state === '草稿', '存在未结束异议，不能确认')
        requireRental(
          !current.blockers.length && current.totalCents !== null,
          current.blockers.join('；') || '金额尚未核实',
        )
        requireRental(
          current.sourceFingerprint === statement.quote.sourceFingerprint,
          '来源数据已变化，请先刷新草稿并重新核对',
        )
        statement.state = '已确认'
        statement.confirmedAt = time
        detail = '对账快照已确认并冻结；未生成正式应收、发票或扣款'
      }
    } else if (a.type === 'dispute') {
      requireRental(statement.state === '草稿', '该对账单已有处理中异议')
      statement.state = '异议处理中'
      detail = '已记录异议，确认操作已阻断'
    } else {
      requireRental(statement.state === '异议处理中', '当前没有待处理异议')
      statement.state = '草稿'
      detail = '异议结论已记录，需重新核对后确认'
    }
    statement.history.unshift({ time, action: a.type, reason: a.reason })
  }
  r.operations.push({ key: c.key, payload, target, detail })
  s.revision++
  s.activities.unshift({
    id: `EV-${id()}`,
    target,
    action: `点位租赁 / ${a.type}`,
    actor: '张敏（预览）',
    time,
    detail,
    visibility: 'internal',
  })
  return { snapshot: s, result: { target, detail } }
}
