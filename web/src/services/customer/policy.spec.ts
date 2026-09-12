import { describe, it, expect } from 'vitest'
import { createCustomerSeed } from './seed'
import { executeCustomerCommand, type ActionId, type Command } from './commands'
import { closeRequirements, canArchive, clientProjection, assertNoCycle } from './policy'
import { evaluateRule } from './automation'
import { parseCustomerCsv, validateImport, applyCustomerImport } from './importer'
import { outcomeMetrics } from './selectors'
import type { CustomerSnapshot, FormValues } from '@/types/customer'
const fresh = () => createCustomerSeed('shanghai')
const values: FormValues = {
  reason: '核对业务依据后处理',
  owner: '李川',
  deadline: '2026-09-25',
  nextAt: '2026-09-12T10:00',
  nextAction: '继续核对范围与客户反馈',
  confirm: true,
  title: '独立测试事项',
  criteria: '结果已验证',
  customerId: 'CUS-0186',
}
function run(
  s: CustomerSnapshot,
  action: ActionId,
  id: string,
  v: FormValues = {},
  extras: Partial<Command> = {},
) {
  return executeCustomerCommand(s, {
    action,
    id,
    values: { ...values, ...v },
    version:
      s.work.find((w) => w.id === id)?.version ??
      s.customers.find((c) => c.id === id)?.version ??
      s.plans.find((p) => p.id === id)?.version,
    key: crypto.randomUUID(),
    ...extras,
  })
}
function ready(s: CustomerSnapshot, id: string) {
  const w = s.work.find((w) => w.id === id)!
  w.status = '待验收'
  w.dependencies = []
  return s
}
describe('customer UI policy and complete preview loops', () => {
  it('keeps fresh tenant seeds independent', () => {
    const a = fresh(),
      b = fresh()
    a.customers[0]!.name = 'changed'
    expect(b.customers[0]!.name).toBe('星悦酒店集团')
    expect(createCustomerSeed('hangzhou').customers[0]!.name).toBe('杭州星澜酒店')
  })
  it('creates a customer without automatic client authorization', () => {
    const s = fresh()
    const r = run(s, 'create-customer', '', {
      name: '新客户',
      category: '连锁酒店',
      title: '',
      tenantLink: 'TN-NEW',
    })
    expect(r.snapshot.customers).toHaveLength(9)
    expect(r.snapshot.grants).toHaveLength(s.grants.length)
    expect(r.snapshot.customers[0]!.lifecycle).toBe('潜在客户')
  })
  it('rejects duplicate names without partial writes', () => {
    const s = fresh(),
      before = JSON.stringify(s)
    expect(() => run(s, 'create-customer', '', { name: '星悦酒店集团', category: '酒店' })).toThrow('同名')
    expect(JSON.stringify(s)).toBe(before)
  })
  it('rejects arbitrary lifecycle at customer creation', () => {
    expect(() =>
      run(fresh(), 'create-customer', '', { name: '新客户', category: '酒店', lifecycle: '合作中' }),
    ).toThrow('新建客户')
  })
  it('requires valid preview owner', () => {
    expect(() => run(fresh(), 'create-work', '', { owner: '不存在的成员' })).toThrow('负责人')
  })
  it('rejects stale customer version and preserves original', () => {
    const s = fresh()
    expect(() =>
      run(s, 'edit-customer', 'CUS-0186', { name: '覆盖资料', category: '酒店' }, { version: 0 }),
    ).toThrow('草稿已保留')
    expect(s.customers[0]!.name).toBe('星悦酒店集团')
  })
  it('creates contact without implicit authorization', () => {
    const r = run(fresh(), 'contact', 'CUS-0186', {
      name: '林经理',
      role: '运营',
      phone: '13800000000',
      email: 'lin@example.com',
    })
    expect(r.snapshot.contacts.at(-1)?.authorized).toBe(false)
  })
  it('rejects malformed contact email', () => {
    expect(() =>
      run(fresh(), 'contact', 'CUS-0186', { name: '林经理', email: 'bad', role: '运营', phone: '1' }),
    ).toThrow('邮箱')
  })
  it('handover only moves original customer owners eligible work', () => {
    const r = run(fresh(), 'handover', 'CUS-0186', { includeWork: true })
    expect(r.snapshot.customers[0]!.owner).toBe('李川')
    expect(r.snapshot.work.find((w) => w.id === 'CS-102')?.owner).toBe('李川')
    expect(r.snapshot.work.find((w) => w.id === 'CS-103')?.owner).toBe('陈晓')
  })
  it('visit close creates independent follow-up for promise', () => {
    const r = run(fresh(), 'visit', 'CS-107', {
      feedback: '需要扩点',
      promise: '提供方案',
      followup: true,
      closeVisit: true,
      actualAt: '2026-09-10T10:30',
    })
    expect(r.snapshot.work.find((w) => w.id === 'CS-107')?.status).toBe('已结束')
    expect(r.snapshot.work[0]).toMatchObject({ title: '独立测试事项', status: '待开始' })
    expect(r.snapshot.work).toHaveLength(13)
  })
  it('rejects customer promise with no follow-up', () => {
    expect(() =>
      run(fresh(), 'visit', 'CS-107', { feedback: '扩点', promise: '下周交方案', followup: false }),
    ).toThrow('后续事项')
  })
  it('partially assigns eligible records and returns skip reason', () => {
    const r = run(fresh(), 'assign', 'work', {}, { selected: ['CS-102', 'CS-111', 'CS-110'] })
    expect(r.result.detail).toContain('成功分配 1 项')
    expect(r.result.detail).toContain('跳过 2 项')
    expect(r.snapshot.work.find((w) => w.id === 'CS-111')?.owner).toBe('王宁')
  })
  it('rejects entirely ineligible batch', () => {
    expect(() => run(fresh(), 'assign', 'work', {}, { selected: ['CS-111'] })).toThrow('不可分配')
  })
  it('reschedule changes action date without touching SLA', () => {
    const s = fresh()
    const r = run(s, 'reschedule', 'CS-102', { nextAt: '2026-09-14T10:00' })
    expect(r.snapshot.sla).toEqual(s.sla)
    expect(r.snapshot.work[0]?.nextAt).toBe('2026-09-14T10:00')
  })
  it('rejects action date after deadline', () => {
    expect(() => run(fresh(), 'reschedule', 'CS-102', { nextAt: '2026-10-01T10:00' })).toThrow('截止时间')
  })
  it('rejects self dependency', () => {
    expect(() => assertNoCycle(fresh(), 'CS-102', 'CS-102')).toThrow('自身')
  })
  it('rejects indirect dependency cycle', () => {
    expect(() => run(fresh(), 'dependency', 'CS-103', { workId: 'CS-102' })).toThrow('循环')
  })
  it('rejects cross-customer dependency', () => {
    expect(() => run(fresh(), 'dependency', 'CS-103', { workId: 'CS-112' })).toThrow('同一客户')
  })
  it('does not allow ordinary transition directly to closed', () => {
    expect(() => run(fresh(), 'transition', 'CS-103', { status: '已结束' })).toThrow('验收')
  })
  it('blocks validation while a dependency remains open', () => {
    expect(() => run(fresh(), 'transition', 'CS-102', { status: '待验收' })).toThrow('依赖事项')
  })
  it('blocks partial delivery acceptance', () => {
    const s = fresh(),
      w = s.work.find((w) => w.id === 'CS-104')!
    expect(closeRequirements(s, w, ['ACC-041', 'INST-078'])).toContain('逐点复验全部通过')
  })
  it('accepts delivery only with full recheck and valid placement', () => {
    const r = run(fresh(), 'accept', 'CS-104', { evidence: 'ACC-042,PD-041-043' })
    expect(r.snapshot.work.find((w) => w.id === 'CS-104')?.resolution).toBe('成功')
    expect(r.snapshot.sites.every((s) => s.accepted)).toBe(true)
  })
  it('rejects acceptance retains already accepted sites', () => {
    const r = run(fresh(), 'reject', 'CS-104')
    expect(r.snapshot.work.find((w) => w.id === 'CS-104')?.status).toBe('处理中')
    expect(r.snapshot.sites.filter((s) => s.accepted)).toHaveLength(2)
  })
  it('closed work can reopen without deleting evidence', () => {
    let s = run(fresh(), 'accept', 'CS-104', { evidence: 'ACC-042,PD-041-043' }).snapshot
    const original = [...s.work.find((w) => w.id === 'CS-104')!.evidenceIds]
    s = run(s, 'reopen', 'CS-104').snapshot
    expect(s.work.find((w) => w.id === 'CS-104')).toMatchObject({
      status: '处理中',
      cycle: 2,
      resolution: '',
    })
    expect(s.work.find((w) => w.id === 'CS-104')?.evidenceIds).toEqual(original)
  })
  it('work order done alone cannot close service', () => {
    expect(() => run(fresh(), 'accept', 'CS-103', { evidence: 'WO-078' })).toThrow('设备恢复验证')
  })
  it('recovery and customer confirmation close service', () => {
    const r = run(fresh(), 'accept', 'CS-103', { evidence: 'WO-078,RUN-103,CONF-103' })
    expect(r.snapshot.work.find((w) => w.id === 'CS-103')?.resolution).toBe('成功')
  })
  it('evidence from a different work scope cannot be reused', () => {
    expect(() => run(fresh(), 'link-source', 'CS-107', { sourceId: 'RUN-103' })).toThrow('事项范围')
  })
  it('partial settlement does not close payment', () => {
    expect(() => run(ready(fresh(), 'CS-105'), 'accept', 'CS-105', { evidence: 'AR-0901' })).toThrow(
      '全额核销',
    )
  })
  it('valid full receivable writeoff closes payment', () => {
    const r = run(ready(fresh(), 'CS-105'), 'accept', 'CS-105', { evidence: 'AR-0901-F' })
    expect(r.snapshot.work.find((w) => w.id === 'CS-105')?.resolution).toBe('成功')
    expect(outcomeMetrics(r.snapshot).paid).toBe(12000)
  })
  it('draft contract cannot stand in for effective renewal', () => {
    expect(() => run(ready(fresh(), 'CS-102'), 'accept', 'CS-102', { evidence: 'HT-DRAFT-01' })).toThrow(
      '已核验',
    )
  })
  it('effective renewal contract with matching original closes renewal', () => {
    const r = run(ready(fresh(), 'CS-102'), 'accept', 'CS-102', { evidence: 'HT-2026-058' })
    expect(r.snapshot.work[0]?.resolution).toBe('成功')
    expect(outcomeMetrics(r.snapshot).renewalRate).toBe('100%')
  })
  it('nonrenewal leaves customer lifecycle unchanged and records failure', () => {
    const r = run(fresh(), 'nonrenewal', 'CS-102', { disposition: true, followup: true })
    expect(r.snapshot.customers[0]!.lifecycle).toBe('合作中')
    expect(r.snapshot.work.find((w) => w.id === 'CS-102')?.resolution).toBe('未达成')
    expect(outcomeMetrics(r.snapshot).renewalRate).toBe('0%')
  })
  it('physical return alone does not end placement', () => {
    expect(() => run(ready(fresh(), 'CS-106'), 'accept', 'CS-106', { evidence: 'REC-026' })).toThrow(
      '结算完成',
    )
  })
  it('settled and terminated return preserves history', () => {
    const r = run(ready(fresh(), 'CS-106'), 'accept', 'CS-106', { evidence: 'REC-026,SET-026,TERM-026' })
    expect(r.snapshot.sources.find((s) => s.id === 'TERM-026')?.facts.history).toBe(true)
    expect(r.snapshot.customers[0]!.lifecycle).toBe('合作中')
  })
  it('does not permit falsely claiming all plan targets achieved', () => {
    expect(() => run(fresh(), 'recap', 'PL-009', { result: '全部达成', disposition: true })).toThrow('未达成')
  })
  it('partial plan recap keeps unfinished work open', () => {
    const r = run(fresh(), 'recap', 'PL-009', { result: '部分达成', disposition: true, followup: true })
    expect(r.snapshot.plans[0]?.state).toBe('已结案')
    expect(r.snapshot.work.find((w) => w.id === 'CS-102')?.status).toBe('处理中')
  })
  it('dry run changes no business records and creates zero work', () => {
    const s = fresh(),
      r = run(s, 'rule-test', 'RULE-01')
    expect(r.snapshot.work).toEqual(s.work)
    expect(r.snapshot.notifications).toEqual(s.notifications)
    expect(r.snapshot.drafts['test:RULE-01']?.result).toContain('实际创建 0')
  })
  it('dry run recognizes business-cycle duplicate', () => {
    const s = fresh(),
      matches = evaluateRule(s, s.rules[0]!)
    expect(matches).toHaveLength(1)
    expect(matches[0]?.existing).toBe('CS-102')
  })
  it('uses specific rule condition instead of pretending all rules match contract', () => {
    const s = fresh()
    expect(evaluateRule(s, s.rules[2]!)).toHaveLength(0)
  })
  it('blocks publishing untested rule', () => {
    expect(() => run(fresh(), 'rule-publish', 'RULE-03')).toThrow('先测试')
  })
  it('recovers only failed notification step without duplicate work', () => {
    const s = fresh(),
      r = run(s, 'recover', 'RUN-240', { reconciled: true })
    expect(r.snapshot.work).toHaveLength(s.work.length)
    expect(r.snapshot.executions[0]?.state).toBe('已恢复')
    expect(r.snapshot.executions[0]?.history[2]).toContain('失败')
  })
  it('repeated recovery does not duplicate local notifications', () => {
    const first = run(fresh(), 'recover', 'RUN-240', { reconciled: true }).snapshot
    const second = run(first, 'recover', 'RUN-240', { reconciled: true }).snapshot
    expect(second.notifications).toHaveLength(first.notifications.length)
  })
  it('requires reconciliation before recovery', () => {
    expect(() => run(fresh(), 'recover', 'RUN-240', { reconciled: false })).toThrow('核对')
  })
  it('publishes template only for new work and pins in-flight version', () => {
    let s = run(fresh(), 'workflow-publish', 'workflow', { scope: '仅新建事项' }).snapshot
    expect(s.workflowVersion).toBe(4)
    expect(s.work[0]?.workflowVersion).toBe(3)
    s = run(s, 'create-work', '').snapshot
    expect(s.work[0]?.workflowVersion).toBe(4)
  })
  it('SLA changes do not reschedule work', () => {
    const s = fresh()
    const r = run(s, 'sla', 'sla', {
      responseMinutes: '45',
      recoveryHours: '6',
      calendar: '工作日 09:00–18:00',
      pause: '不暂停任何时钟',
    })
    expect(r.snapshot.work).toEqual(s.work)
    expect(r.snapshot.sla.responseMinutes).toBe(45)
  })
  it('shared projection omits internal fields and comments', () => {
    const s = run(fresh(), 'comment', 'CS-104', {
      comment: '内部利润与谈判策略不共享',
      shared: false,
    }).snapshot
    const p = clientProjection(s, 'SH-031', new Date('2026-09-10'))
    expect(JSON.stringify(p)).not.toContain('内部利润')
    expect(p).not.toHaveProperty('owner')
    expect(p).not.toHaveProperty('sources')
  })
  it('unshared attachment field means no attachment data', () => {
    const s = fresh()
    s.grants[0]!.fields = ['事项标题']
    const p = clientProjection(s, 'SH-031', new Date('2026-09-10'))
    expect(p.attachments).toHaveLength(0)
    expect(p.sites).toHaveLength(0)
  })
  it('revoked and expired client grants reject read', () => {
    const s = run(fresh(), 'revoke-share', 'SH-031').snapshot
    expect(() => clientProjection(s, 'SH-031')).toThrow('撤销')
    expect(() => clientProjection(fresh(), 'SH-031', new Date('2027-01-01'))).toThrow('过期')
  })
  it('sharing rejects internal field selection', () => {
    expect(() =>
      run(fresh(), 'share', '', {
        workId: 'CS-104',
        contactId: 'CT-02',
        expires: '2099-10-10',
        fields: '利润',
      }),
    ).toThrow('不能共享')
  })
  it('client confirmation does not auto-close provider work', () => {
    const r = run(fresh(), 'client-accept', 'SH-031')
    expect(r.snapshot.work.find((w) => w.id === 'CS-104')?.status).toBe('待验收')
  })
  it('client rejection returns original work and keeps passed scope', () => {
    const r = run(fresh(), 'client-reject', 'SH-031')
    expect(r.snapshot.work.find((w) => w.id === 'CS-104')?.status).toBe('处理中')
    expect(r.snapshot.sites.filter((s) => s.accepted)).toHaveLength(2)
  })
  it('active contracts and work block customer archive', () => {
    expect(canArchive(fresh(), 'CUS-0186').length).toBeGreaterThan(0)
    expect(() => run(fresh(), 'archive', 'CUS-0186')).toThrow('有效合同')
  })
  it('inactive cleared customer can archive and restore keeping history', () => {
    const first = run(fresh(), 'archive', 'CUS-0176').snapshot
    expect(first.customers.find((c) => c.id === 'CUS-0176')?.archived).toBe(true)
    const next = run(first, 'restore', 'CUS-0176').snapshot
    expect(next.customers.find((c) => c.id === 'CUS-0176')?.archived).toBe(false)
  })
  it('notification read never completes its linked work', () => {
    const s = fresh(),
      r = run(s, 'read-notice', 'NT-01')
    expect(r.snapshot.notifications[0]?.read).toBe(true)
    expect(r.snapshot.work).toEqual(s.work)
  })
  it('saved display view does not mutate query data', () => {
    const s = fresh(),
      r = run(s, 'save-view', 'work', { name: '我的续约', density: '紧凑', columns: '客户,负责人' })
    expect(r.snapshot.work).toEqual(s.work)
    expect(r.snapshot.views[0]?.density).toBe('紧凑')
  })
  it('same idempotency key replays result without duplicate create', () => {
    const command: Command = { action: 'create-work', id: '', values, key: 'same-operation' }
    const first = executeCustomerCommand(fresh(), command)
    const second = executeCustomerCommand(first.snapshot, command)
    expect(second.snapshot.work).toHaveLength(first.snapshot.work.length)
    expect(second.result.target).toBe(first.result.target)
  })
  it('CSV parser handles quoted commas and CRLF', () => {
    expect(parseCustomerCsv('客户,类型,负责人\r\n"A,B",酒店,张敏\r\n')[1]?.[0]).toBe('A,B')
  })
  it('CSV parser rejects incomplete quotes', () => {
    expect(() => parseCustomerCsv('name,type,owner\n"bad,hotel,name')).toThrow('引号')
  })
  it('import separates duplicate invalid and valid rows', () => {
    const rows = validateImport(
      [
        ['客户', '类型', '负责人'],
        ['星悦酒店集团', '酒店', '张敏'],
        ['新酒店', '酒店', '李川'],
        ['错误客户', '酒店', '未知'],
      ],
      { name: 0, category: 1, owner: 2 },
      fresh(),
    )
    expect(rows.map((r) => r.result)).toEqual(['疑似重复', '通过', '字段错误'])
  })
  it('import commits valid selected rows only and safely replays', () => {
    const s = fresh(),
      rows = validateImport(
        [
          ['客户', '类型', '负责人'],
          ['星悦酒店集团', '酒店', '张敏'],
          ['新酒店', '酒店', '李川'],
        ],
        { name: 0, category: 1, owner: 2 },
        s,
      )
    const first = applyCustomerImport(s, rows, 'batch')
    const next = applyCustomerImport(first.snapshot, rows, 'batch')
    expect(next.snapshot.customers).toHaveLength(9)
    expect(first.results[1]?.result).toBe('已导入')
  })
})

describe('customer scoped projections and evidence invariants', () => {
  it('partial receipts do not leak into another customer payment', async () => {
    const { receivableFor } = await import('./selectors')
    const s = fresh()
    expect(
      receivableFor(
        s,
        s.work.find((w) => w.id === 'CS-113')!,
      ),
    ).toBeUndefined()
    expect(
      receivableFor(
        s,
        s.work.find((w) => w.id === 'CS-105')!,
      )?.facts.balance,
    ).toBe(3000)
  })
  it('delivery closes only its own sites and matching verified count', () => {
    const s = fresh()
    s.sites.push({
      ...s.sites[0]!,
      id: 'OTHER-SITE',
      customerId: 'CUS-0185',
      workId: 'CS-999',
      accepted: false,
    })
    const r = run(s, 'accept', 'CS-104', { evidence: 'ACC-042,PD-041-043' })
    expect(r.snapshot.sites.find((site) => site.id === 'OTHER-SITE')?.accepted).toBe(false)
    const invalid = fresh()
    invalid.sources.find((x) => x.id === 'ACC-042')!.facts.total = 2
    expect(() => run(invalid, 'accept', 'CS-104', { evidence: 'ACC-042,PD-041-043' })).toThrow('逐点复验')
  })
  it('shared projection rejects customer mismatch and never includes other site scopes', () => {
    const s = fresh()
    s.sites.push({ ...s.sites[0]!, id: 'PRIVATE-SITE', customerId: 'CUS-0185', workId: 'CS-999' })
    expect(clientProjection(s, 'SH-031', new Date('2026-09-10T12:00:00Z')).sites).toHaveLength(3)
    s.grants[0]!.customerId = 'CUS-0185'
    expect(() => clientProjection(s, 'SH-031', new Date('2026-09-10T12:00:00Z'))).toThrow('归属不匹配')
  })
  it('visit preserves exact feedback and followup in traceable activity', () => {
    const r = run(fresh(), 'visit', 'CS-107', {
      method: '电话回访',
      contact: '周岚',
      actualAt: '2026-09-10T10:00',
      feedback: '客户要求九月底前确认新增点位范围',
      followup: false,
      promise: '',
    })
    expect(r.snapshot.activities.some((a) => a.detail.includes('客户要求九月底前确认新增点位范围'))).toBe(
      true,
    )
  })
})
