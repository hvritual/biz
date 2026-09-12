import { describe, it, expect } from 'vitest'
import { createCustomerSeed } from '@/services/customer/seed'
import { loadCustomerSnapshot } from '@/services/customer/repository'
import { executeCustomerCommand } from '@/services/customer/commands'
import { createRentalSeed } from './seed'
import { activeRule, rentalState, ruleForSite } from './model'
import { calculateLine, quoteRental } from './quote'
import { validateRule, draftFingerprint } from './policy'
import { executeRentalCommand, type RentalAction } from './commands'
import type { RentalDraft, RentalRule } from '@/types/siteRental'
const now = new Date('2026-09-11T10:00:00Z')
function seed() {
  const s = createCustomerSeed('shanghai')
  s.rental = createRentalSeed(s)
  return s
}
function draft(s = seed()): RentalDraft {
  return {
    rule: {
      ...structuredClone(s.rental!.rules[0]!),
      id: '',
      version: 2,
      effectiveFrom: '2026-10-01',
      reason: '双方确认新的服务和计费约定',
    },
    samples: { 'SITE-041': 600, 'SITE-042': 1300 },
    testedFingerprint: '',
  }
}
function run(
  s: ReturnType<typeof seed>,
  action: RentalAction,
  key = crypto.randomUUID(),
  revision = s.revision,
) {
  return executeRentalCommand(s, { tenant: s.tenant, expectedRevision: revision, key, action }, now)
}
describe('三种计费模式与共享范围', () => {
  it.each([
    ['fixed', 120000],
    ['metered', 240000],
    ['included', 200000],
  ] as const)('%s formula', (mode, expected) => {
    const rule = { ...draft().rule, mode, baseCents: 120000, includedCups: 1000, unitCents: 160 }
    expect(calculateLine(rule, 1500, 1, 'sample', 'id').totalCents).toBe(expected)
  })
  it('minimum spend differs from included-cups pricing', () => {
    const rule = {
      ...draft().rule,
      baseCents: 120000,
      includedCups: 1000,
      minimumKind: 'minimum-spend' as const,
    }
    expect(calculateLine(rule, 1500, 1, 'x', 'x').totalCents).toBe(240000)
  })
  it('600 + 1300 uses one 2000 cup pool and one base fee', () => {
    const s = seed(),
      q = quoteRental(s, s.rental!, s.rental!.rules[0]!, '2026-09')
    expect(q.lines).toHaveLength(1)
    expect(q.lines[0]!.cups).toBe(1900)
    expect(q.lines[0]!.overCups).toBe(0)
    expect(q.totalCents).toBe(240000)
    expect(q.blockers).toEqual([])
  })
  it('independent quota is explicitly per site, not divided from a shared pool', () => {
    const s = seed(),
      rule = { ...s.rental!.rules[0]!, scope: 'independent' as const, baseCents: 120000, includedCups: 1000 }
    const q = quoteRental(s, s.rental!, rule, '2026-09')
    expect(q.lines).toHaveLength(2)
    expect(q.lines.map((x) => x.overCups)).toEqual([0, 300])
    expect(q.totalCents).toBe(288000)
  })
  it('bill presentation does not change fee calculations', () => {
    const s = seed(),
      r = s.rental!.rules[0]!
    expect(quoteRental(s, s.rental!, { ...r, billPresentation: '分别结算' }, '2026-09').totalCents).toBe(
      quoteRental(s, s.rental!, r, '2026-09').totalCents,
    )
  })
  it('missing usage is null, not zero, and blocks variable billing', () => {
    const s = seed()
    s.rental!.usage = []
    const q = quoteRental(s, s.rental!, s.rental!.rules[0]!, '2026-09')
    expect(q.totalCents).toBeNull()
    expect(q.blockers.length).toBe(2)
  })
  it('pending backfill blocks variable billing', () => {
    const s = seed()
    s.rental!.usage[0]!.quality = '待补传'
    const q = quoteRental(s, s.rental!, s.rental!.rules[0]!, '2026-09')
    expect(q.totalCents).toBeNull()
  })
  it('a true zero can be billed with the included base', () => {
    const s = seed()
    s.rental!.usage.forEach((x) => {
      x.raw = 0
      x.excluded = 0
    })
    const q = quoteRental(s, s.rental!, s.rental!.rules[0]!, '2026-09')
    expect(q.totalCents).toBe(240000)
    expect(q.lines[0]!.cups).toBe(0)
  })
  it('fixed rent does not depend on cup telemetry', () => {
    const s = seed()
    s.rental!.usage = []
    const q = quoteRental(s, s.rental!, s.rental!.rules[1]!, '2026-09')
    expect(q.totalCents).toBe(120000)
    expect(q.blockers).toHaveLength(0)
  })
  it('invalid exclusion totals fail instead of producing negative cups', () => {
    const s = seed()
    s.rental!.usage[0]!.excluded = 900
    expect(() => quoteRental(s, s.rental!, s.rental!.rules[0]!, '2026-09')).toThrow('口径异常')
  })
  it.each([-1, 0.5, NaN, Infinity])('reject invalid trial cups %s', (cups) => {
    expect(() => calculateLine(draft().rule, cups, 1, 'x', 'x')).toThrow()
  })
  it('uses integer cents', () => {
    const rule = { ...draft().rule, mode: 'metered' as const, unitCents: 1 }
    expect(calculateLine(rule, 333, 1, 'x', 'x').totalCents).toBe(333)
  })
  it('reject unsafe monetary totals', () => {
    expect(() =>
      calculateLine({ ...draft().rule, mode: 'metered', unitCents: 1000000000 }, 1000000000, 1, 'x', 'x'),
    ).toThrow('安全')
  })
  it('actual mid-month deployment requires an explicit proration policy', () => {
    const s = seed()
    s.rental!.deployments.find((x) => x.siteId === 'SITE-044')!.from = '2026-09-15'
    expect(quoteRental(s, s.rental!, s.rental!.rules[1]!, '2026-09').blockers.join()).toContain('缺口')
  })
  it('contiguous replacement does not double count a rented device', () => {
    const s = seed(),
      r = s.rental!,
      old = r.deployments.find((x) => x.siteId === 'SITE-044')!
    old.until = '2026-09-16'
    r.deployments.push({ ...old, id: 'NEW', device: 'NEW', from: '2026-09-16', until: '' })
    const q = quoteRental(s, r, { ...r.rules[1]!, fixedUnit: 'device' }, '2026-09')
    expect(q.totalCents).toBe(120000)
    expect(q.blockers).toEqual([])
  })
  it('change in actual device count blocks unsupported per-device proration', () => {
    const s = seed(),
      r = s.rental!
    r.deployments.push({
      ...r.deployments.find((x) => x.siteId === 'SITE-044')!,
      id: 'NEW',
      device: 'NEW',
      from: '2026-09-16',
    })
    expect(quoteRental(s, r, { ...r.rules[1]!, fixedUnit: 'device' }, '2026-09').blockers.join()).toContain(
      '数量变化',
    )
  })
})
describe('规则版本与范围门禁', () => {
  it('requires an explicit scope', () => {
    const s = seed(),
      d = draft(s)
    d.rule.scope = '' as RentalRule['scope']
    expect(() => validateRule(s, s.rental!, d.rule)).toThrow('明确')
  })
  it('requires at least two sites for sharing', () => {
    const s = seed(),
      d = draft(s)
    d.rule.siteIds = ['SITE-041']
    expect(() => validateRule(s, s.rental!, d.rule)).toThrow('两个')
  })
  it('rejects duplicate members', () => {
    const s = seed(),
      d = draft(s)
    d.rule.siteIds = ['SITE-041', 'SITE-041']
    expect(() => validateRule(s, s.rental!, d.rule)).toThrow('不重复')
  })
  it('rejects grouping nodes as billable sites', () => {
    const s = seed(),
      d = draft(s)
    s.rental!.profiles[0]!.kind = 'group'
    expect(() => validateRule(s, s.rental!, d.rule)).toThrow('分组节点')
  })
  it('rejects cross-customer members', () => {
    const s = seed(),
      d = draft(s)
    s.rental!.profiles[0]!.customerId = 'CUS-0185'
    expect(() => validateRule(s, s.rental!, d.rule)).toThrow('同一客户')
  })
  it('rejects foreign and unverified contracts', () => {
    const s = seed(),
      d = draft(s)
    d.rule.contractId = 'HT-DRAFT-01'
    expect(() => validateRule(s, s.rental!, d.rule)).toThrow('合同来源')
  })
  it('rejects contracts not covering the first complete period', () => {
    const s = seed(),
      d = draft(s)
    d.rule.contractId = 'HT-2026-018'
    expect(() => validateRule(s, s.rental!, d.rule)).toThrow('完整账期')
  })
  it('rejects mid-month and retroactive activation', () => {
    const s = seed(),
      d = draft(s)
    d.rule.effectiveFrom = '2026-10-15'
    expect(() => validateRule(s, s.rental!, d.rule, true, now)).toThrow('月初')
    d.rule.effectiveFrom = '2026-09-01'
    expect(() => validateRule(s, s.rental!, d.rule, true, now)).toThrow()
  })
  it('rejects overlapping groups for the same site', () => {
    const s = seed(),
      d = draft(s)
    d.rule.groupId = 'NEW'
    d.rule.version = 1
    expect(() => validateRule(s, s.rental!, d.rule)).toThrow('重复计费')
  })
  it.each([-1, 0.1, NaN, Infinity])('reject invalid price cents %s', (amount) => {
    const s = seed(),
      d = draft(s)
    d.rule.baseCents = amount
    expect(() => validateRule(s, s.rental!, d.rule)).toThrow('整数')
  })
  it('preview does not mutate either aggregate', () => {
    const s = seed(),
      d = draft(s),
      before = JSON.stringify(s)
    quoteRental(s, s.rental!, d.rule, '2026-10', d.samples)
    expect(JSON.stringify(s)).toBe(before)
  })
  it('publishes future version while historical effective rule remains', () => {
    const s = seed(),
      d = draft(s)
    d.testedFingerprint = draftFingerprint(d)
    const next = run(s, { type: 'publish-rule', draft: d }).snapshot
    expect(activeRule(next.rental!, 'BG-SZ-01', '2026-09', next.sources)!.version).toBe(1)
    expect(activeRule(next.rental!, 'BG-SZ-01', '2026-10', next.sources)!.version).toBe(2)
  })
  it('changes after preview invalidate the test proof', () => {
    const s = seed(),
      d = draft(s)
    d.testedFingerprint = draftFingerprint(d)
    d.rule.unitCents = 200
    expect(() => run(s, { type: 'publish-rule', draft: d })).toThrow('重新试算')
  })
  it('saved draft does not participate in fees', () => {
    const s = seed(),
      d = draft(s)
    d.rule.baseCents = 100
    const next = run(s, { type: 'save-draft', draft: d }).snapshot
    expect(ruleForSite(next.rental!, 'SITE-041', '2026-10', next.sources)!.baseCents).toBe(240000)
  })
})
describe('对账、租户与写入闭环', () => {
  it('creates one statement per group and period', () => {
    const s = seed(),
      a = { type: 'create-statement' as const, groupId: 'BG-SZ-01', period: '2026-09' }
    const one = run(s, a).snapshot
    const two = run(one, a).snapshot
    expect(two.rental!.statements).toHaveLength(1)
  })
  it('replays same operation but rejects payload reuse', () => {
    const s = seed(),
      a = { type: 'create-statement' as const, groupId: 'BG-SZ-01', period: '2026-09' }
    const one = run(s, a, 'same')
    expect(run(one.snapshot, a, 'same', 1).snapshot.revision).toBe(one.snapshot.revision)
    expect(() => run(one.snapshot, { ...a, period: '2026-10' }, 'same')).toThrow('标识已用于')
  })
  it('confirmed statement is immutable', () => {
    const s = seed(),
      one = run(s, { type: 'create-statement', groupId: 'BG-SZ-01', period: '2026-09' })
    const two = run(one.snapshot, {
      type: 'confirm-statement',
      statementId: one.result.target,
      reason: '已核对',
    }).snapshot
    expect(two.rental!.statements[0]!.state).toBe('已确认')
    expect(() =>
      run(two, { type: 'refresh-statement', statementId: one.result.target, reason: '新数据' }),
    ).toThrow('不可覆盖')
  })
  it('pending dispute blocks confirmation and requires resolution', () => {
    const one = run(seed(), { type: 'create-statement', groupId: 'BG-SZ-01', period: '2026-09' })
    const two = run(one.snapshot, {
      type: 'dispute',
      statementId: one.result.target,
      reason: '口径待核对',
    }).snapshot
    expect(() =>
      run(two, { type: 'confirm-statement', statementId: one.result.target, reason: '直接确认' }),
    ).toThrow('异议')
    const three = run(two, {
      type: 'resolve-dispute',
      statementId: one.result.target,
      reason: '已确认排除口径',
    }).snapshot
    expect(three.rental!.statements[0]!.state).toBe('草稿')
    expect(three.rental!.statements[0]!.history).toHaveLength(3)
  })
  it('changed source blocks confirmation until refreshed', () => {
    const one = run(seed(), { type: 'create-statement', groupId: 'BG-SZ-01', period: '2026-09' })
    one.snapshot.rental!.usage[0]!.raw++
    expect(() =>
      run(one.snapshot, { type: 'confirm-statement', statementId: one.result.target, reason: '确认' }),
    ).toThrow('来源数据已变化')
  })
  it('new rule does not rewrite a confirmed old-period snapshot', () => {
    const one = run(seed(), { type: 'create-statement', groupId: 'BG-SZ-01', period: '2026-09' })
    const two = run(one.snapshot, {
      type: 'confirm-statement',
      statementId: one.result.target,
      reason: '确认',
    }).snapshot
    const old = JSON.stringify(two.rental!.statements)
    const d = draft(two)
    d.testedFingerprint = draftFingerprint(d)
    const three = run(two, { type: 'publish-rule', draft: d }).snapshot
    expect(JSON.stringify(three.rental!.statements)).toBe(old)
  })
  it('missing data cannot be confirmed', () => {
    const s = seed()
    s.rental!.usage = []
    const one = run(s, { type: 'create-statement', groupId: 'BG-SZ-01', period: '2026-09' })
    expect(() =>
      run(one.snapshot, { type: 'confirm-statement', statementId: one.result.target, reason: '按0处理' }),
    ).toThrow('不能按零')
  })
  it('rejects stale revision without mutating original', () => {
    const s = seed(),
      before = JSON.stringify(s)
    expect(() =>
      run(s, { type: 'create-statement', groupId: 'BG-SZ-01', period: '2026-09' }, 'x', 0),
    ).toThrow('其他窗口')
    expect(JSON.stringify(s)).toBe(before)
  })
  it('rejects tenant mismatch', () => {
    const s = seed()
    expect(() =>
      executeRentalCommand(s, {
        tenant: 'hangzhou',
        expectedRevision: 1,
        key: 'x',
        action: { type: 'create-statement', groupId: 'BG-SZ-01', period: '2026-09' },
      }),
    ).toThrow('租户')
  })
  it('operational pause does not change prices or deployment relations', () => {
    const s = seed()
    const next = run(s, {
      type: 'operation',
      siteId: 'SITE-041',
      operation: '临时停用',
      reason: '客户放假',
      nextAt: '2026-09-18T10:00',
    }).snapshot
    expect(next.rental!.rules).toEqual(s.rental!.rules)
    expect(next.rental!.deployments).toEqual(s.rental!.deployments)
    expect(quoteRental(next, next.rental!, next.rental!.rules[0]!, '2026-09').blockers.join()).toContain(
      '不自动停租',
    )
  })
  it('duplicate master site is rejected', () => {
    const s = seed(),
      site = { ...s.rental!.profiles[0]!, id: '' }
    expect(() => run(s, { type: 'save-site', site })).toThrow('已存在')
  })
  it('new point remains survey-pending, no device or rule auto-created', () => {
    const s = seed(),
      site = { ...s.rental!.profiles[0]!, id: '', name: '新建测试点位' }
    const next = run(s, { type: 'save-site', site }).snapshot
    expect(next.rental!.profiles.at(-1)!.phase).toBe('待勘察')
    expect(next.rental!.rules).toEqual(s.rental!.rules)
    expect(next.rental!.deployments).toEqual(s.rental!.deployments)
  })
  it('site work uses existing customer aggregate, not a second task table', () => {
    const s = seed()
    const result = executeCustomerCommand(s, {
      action: 'create-work',
      id: 'CUS-0186',
      key: 'site-work',
      values: {
        customerId: 'CUS-0186',
        siteId: 'SITE-041',
        kind: 'service',
        title: '点位服务核对',
        owner: '张敏',
        deadline: '2026-09-18',
        nextAction: '联系客户',
        nextAt: '2026-09-12T10:00',
      },
    })
    expect(result.snapshot.work[0]!.siteIds).toEqual(['SITE-041'])
  })
  it('delivery acceptance readback updates canonical site phase', () => {
    const s = seed()
    s.sites[2]!.accepted = true
    expect(rentalState(s).profiles.find((x) => x.id === 'SITE-043')!.phase).toBe('服务中')
  })
  it('malformed saved rental data fails closed', () => {
    const s = seed()
    const raw = JSON.stringify({ ...s, rental: { schema: 1 } })
    expect(() => loadCustomerSnapshot('shanghai', { getItem: () => raw })).toThrow('读取')
  })
})

describe('合同到期、后续规则与有效时间范围', () => {
  it('expired terms no longer participate in a later billing period', () => {
    const s = seed()
    expect(activeRule(s.rental!, 'BG-SH-01', '2026-09', s.sources)).toBeDefined()
    expect(activeRule(s.rental!, 'BG-SH-01', '2026-10', s.sources)).toBeUndefined()
    expect(ruleForSite(s.rental!, 'SITE-044', '2026-10', s.sources)).toBeUndefined()
    expect(() => run(s, { type: 'create-statement', groupId: 'BG-SH-01', period: '2026-10' })).toThrow(
      '没有有效计费规则',
    )
  })
  it('a renewal group is selected after old contract expiry without rewriting September', () => {
    const s = seed(),
      old = s.rental!.rules[1]!
    const original = s.sources.find((x) => x.id === old.contractId)!
    s.sources.push({
      ...original,
      id: 'HT-RENEW',
      facts: { ...original.facts, start: '2026-10-01', end: '2027-09-30' },
    })
    const next: RentalRule = {
      ...old,
      id: 'RV-RENEW',
      groupId: 'BG-RENEW',
      contractId: 'HT-RENEW',
      version: 1,
      effectiveFrom: '2026-10-01',
      baseCents: 135000,
    }
    validateRule(s, s.rental!, next, true, now)
    s.rental!.rules.push(next)
    expect(ruleForSite(s.rental!, 'SITE-044', '2026-09', s.sources)!.id).toBe(old.id)
    expect(ruleForSite(s.rental!, 'SITE-044', '2026-10', s.sources)!.id).toBe(next.id)
  })
  it('future non-overlapping contract intervals do not block earlier terms', () => {
    const s = seed(),
      old = s.rental!.rules[1]!
    const original = s.sources.find((x) => x.id === old.contractId)!
    s.sources.push(
      { ...original, id: 'HT-NEAR', facts: { ...original.facts, start: '2026-10-01', end: '2026-12-31' } },
      { ...original, id: 'HT-FUTURE', facts: { ...original.facts, start: '2027-01-01', end: '2027-12-31' } },
    )
    s.rental!.rules.push({
      ...old,
      id: 'RV-FUTURE',
      groupId: 'BG-FUTURE',
      contractId: 'HT-FUTURE',
      version: 1,
      effectiveFrom: '2027-01-01',
    })
    expect(() =>
      validateRule(
        s,
        s.rental!,
        {
          ...old,
          id: '',
          groupId: 'BG-NEAR',
          contractId: 'HT-NEAR',
          version: 1,
          effectiveFrom: '2026-10-01',
        },
        true,
        now,
      ),
    ).not.toThrow()
  })
  it('a missing contract source remains visible as unverified, not as zero fees', () => {
    const s = seed()
    s.sources = s.sources.filter((x) => x.id !== s.rental!.rules[0]!.contractId)
    const active = activeRule(s.rental!, 'BG-SZ-01', '2026-09', s.sources)!
    expect(active).toBeDefined()
    expect(quoteRental(s, s.rental!, active, '2026-09').blockers.join()).toContain('合同')
  })
  it('new groups cannot pretend to be later published versions', () => {
    const s = seed(),
      rule = {
        ...draft(s).rule,
        groupId: 'BG-NEW',
        version: 8,
        scope: 'independent' as const,
        siteIds: ['SITE-043'],
      }
    expect(() => validateRule(s, s.rental!, rule)).toThrow('版本 1')
  })
})
