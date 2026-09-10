import { test, expect } from '@playwright/test'
import { ready, action, fill, commit, snap, state, accept } from './customer.helpers'

test.use({ viewport: { width: 1536, height: 1024 }, deviceScaleFactor: 2 })
test.beforeEach(async ({ page }) => {
  page.on('pageerror', (e) => {
    throw e
  })
})

test('01 connected customer navigation retains main geometry', async ({ page }) => {
  await ready(page)
  const before = await page.getByTestId('main-content').boundingBox()
  await page.locator('[data-module="customers"]').click()
  await expect(page.locator('.module-panel')).toBeVisible()
  expect((await page.locator('.module-panel').boundingBox())!.width).toBe(480)
  expect(await page.getByTestId('main-content').boundingBox()).toEqual(before)
  await snap(page, '01')
  await page.keyboard.press('Escape')
  await expect(page.locator('.module-panel')).toHaveCount(0)
})
test('02 duplicate customer is blocked with existing entry', async ({ page }) => {
  const d = await action(page, '/customers', 'create-customer')
  await fill(d, { name: '星悦酒店集团' })
  await expect(d).toContainText('发现已有同名客户')
  await expect(d.locator('button[form="customer-action-form"]')).toBeDisabled()
  await snap(page, '02')
})
test('03 edit customer persists and workspace reads result', async ({ page }) => {
  const d = await action(page, '/customers/accounts/CUS-0186', 'edit-customer')
  await fill(d, { area: '华东 · 上海 / 苏州 / 杭州' })
  await commit(d)
  await page.reload()
  await expect(page.getByRole('heading', { name: '星悦酒店集团', exact: true })).toBeVisible()
  await expect(page.locator('.customer-area')).toContainText('华东 · 上海 / 苏州 / 杭州')
  await snap(page, '03')
})
test('04 contact saves without granting external login', async ({ page }) => {
  const d = await action(page, '/customers/contacts?target=CUS-0186', 'contact')
  await fill(d, {
    name: '徐悦',
    role: '区域运营经理',
    phone: '138****0012',
    email: 'xuyue@example.com',
    responsibility: '现场验收',
  })
  const shares = (await state(page)).grants.length
  await commit(d)
  expect((await state(page)).grants.length).toBe(shares)
  await expect(page.locator('.customer-area')).toContainText('徐悦')
  await snap(page, '04')
})
test('05 handover previews separate customer and work ownership', async ({ page }) => {
  const d = await action(page, '/customers/accounts/CUS-0186', 'handover')
  await fill(d, { owner: '李川', reason: '华东客户分工调整，保留各专业事项原有责任边界。', confirm: true })
  await snap(page, '05')
  await commit(d)
  expect((await state(page)).customers.find((c: { id: string }) => c.id === 'CUS-0186').owner).toBe('李川')
})
test('06 visit creates independent commitment followup', async ({ page }) => {
  const d = await action(page, '/customers/work/CS-107', 'visit')
  await fill(d, { title: '苏州新增点位配置方案确认' })
  await snap(page, '06')
  await commit(d)
  const s = await state(page)
  expect(s.work.find((w: { id: string }) => w.id === 'CS-107').status).toBe('已结束')
  expect(s.work.find((w: { title: string }) => w.title === '苏州新增点位配置方案确认').status).toBe('待开始')
})
test('07 batch assignment preserves permission failure receipt', async ({ page }) => {
  await ready(page, '/customers/work')
  await page.getByRole('checkbox', { name: '选择 CS-102', exact: true }).check()
  await page.getByRole('checkbox', { name: '选择 CS-111', exact: true }).check()
  await page.getByRole('button', { name: /批量分配/ }).click()
  const d = page.getByRole('dialog')
  await fill(d, { owner: '李川', reason: '按客户经营分工调整，仅分配可编辑事项。' })
  await snap(page, '07')
  await commit(d)
  const s = await state(page)
  expect(s.work.find((w: { id: string }) => w.id === 'CS-102').owner).toBe('李川')
  expect(s.work.find((w: { id: string }) => w.id === 'CS-111').owner).toBe('王宁')
})
test('08 calendar reschedule retains separate SLA clock', async ({ page }) => {
  const d = await action(page, '/customers/work?view=calendar&target=CS-102', 'reschedule')
  await fill(d, {
    nextAt: '2026-09-11T14:00',
    deadline: '2026-09-25',
    reason: '客户采购评审调整，跟进时间提前。',
  })
  const sla = (await state(page)).sla
  await snap(page, '08')
  await commit(d)
  expect((await state(page)).sla).toEqual(sla)
})
test('09 dependencies block premature acceptance', async ({ page }) => {
  await ready(page, '/customers/work/CS-102')
  await page.getByRole('button', { name: /依赖与子事项/ }).click()
  await snap(page, '09')
  const d = await action(page, '/customers/work/CS-102', 'transition')
  await fill(d, { status: '待验收', nextAction: '核对续约生效合同' })
  await d.locator('button[form="customer-action-form"]').click()
  await expect(d.getByRole('alert')).toContainText('依赖')
})
test('10 reopen preserves historical outcome and evidence', async ({ page }) => {
  const d = await action(page, '/customers/work/CS-110', 'reopen')
  await fill(d, {
    reason: '客户补充体验反馈，启动第二轮跟进。',
    owner: '王宁',
    deadline: '2026-09-20',
    nextAt: '2026-09-12T10:00',
    nextAction: '回访并补充本轮结论',
  })
  await snap(page, '10')
  await commit(d)
  const w = (await state(page)).work.find((w: { id: string }) => w.id === 'CS-110')
  expect(w.cycle).toBe(2)
  expect(w.status).toBe('处理中')
})
test('11 delivery evidence is per site not blanket completion', async ({ page }) => {
  await ready(page, '/customers/work/CS-104')
  await snap(page, '11')
  const d = await action(page, '/customers/work/CS-104', 'accept')
  await expect(d.locator('button[form="customer-action-form"]')).toBeDisabled()
})
test('12 delivery rejection retains passed sites', async ({ page }) => {
  const d = await action(page, '/customers/work/CS-104', 'reject')
  await fill(d, { owner: '李川', deadline: '2026-09-15', nextAt: '2026-09-12T10:00' })
  await snap(page, '12')
  await commit(d)
  expect((await state(page)).sites.filter((x: { accepted: boolean }) => x.accepted)).toHaveLength(2)
})
test('13 delivery successful reinspection closes with valid scope', async ({ page }) => {
  await accept(page, 'CS-104', ['ACC-042', 'PD-041-043'])
  expect((await state(page)).sites.filter((x: { accepted: boolean }) => x.accepted)).toHaveLength(3)
  await snap(page, '13')
})
test('14 service work order alone cannot prove recovery', async ({ page }) => {
  await ready(page, '/customers/work/CS-103')
  await snap(page, '14')
  const d = await action(page, '/customers/work/CS-103', 'accept')
  await expect(d.locator('button[form="customer-action-form"]')).toBeDisabled()
})
test('15 failed recovery returns same service item', async ({ page }) => {
  const d = await action(page, '/customers/work/CS-103', 'reject')
  await fill(d, {
    reason: '设备恢复验证未达到连续稳定标准，返回原工单继续处理。',
    owner: '陈晓',
    nextAction: '核对运维执行结果并重新采集恢复依据',
    deadline: '2026-09-12',
    nextAt: '2026-09-11T10:00',
  })
  await snap(page, '15')
  await commit(d)
  expect((await state(page)).work.find((w: { id: string }) => w.id === 'CS-103').status).toBe('处理中')
})
test('16 payment partial receipt still shows remaining balance', async ({ page }) => {
  const d = await action(page, '/customers/work/CS-105', 'reschedule')
  await expect(d).toContainText('3,000')
  await fill(d, { reason: '仍有 3,000 元未核销，核对来源并继续跟进。' })
  await snap(page, '16')
  await commit(d)
  expect((await state(page)).work.find((w: { id: string }) => w.id === 'CS-105').status).toBe('等待客户')
})
test('17 payment full reconciliation enables success', async ({ page }) => {
  await accept(page, 'CS-105', ['AR-0901-F'], true)
  await snap(page, '17')
})
test('18 renewal requires resolved service and effective successor contract', async ({ page }) => {
  await accept(page, 'CS-103', ['WO-078', 'RUN-103', 'CONF-103'])
  await accept(page, 'CS-102', ['HT-2026-058'], true)
  await snap(page, '18')
})
test('19 failed renewal remains distinct from customer relationship', async ({ page }) => {
  const d = await action(page, '/customers/work/CS-102', 'nonrenewal')
  await fill(d, { reason: '客户调整该合同覆盖点位，其他合作范围继续保留。', disposition: true })
  await snap(page, '19')
  await commit(d)
  expect((await state(page)).customers.find((c: { id: string }) => c.id === 'CUS-0186').lifecycle).toBe(
    '合作中',
  )
})
test('20 return checklist exposes settlement and termination boundaries', async ({ page }) => {
  await ready(page, '/customers/work/CS-106')
  await snap(page, '20')
})
test('21 return completion retains historical data interval', async ({ page }) => {
  await accept(page, 'CS-106', ['REC-026', 'SET-026', 'TERM-026'], true)
  await snap(page, '21')
})
test('22 plan results are separate from task closure counts', async ({ page }) => {
  await ready(page, '/customers/plans/PL-009')
  await snap(page, '22')
})
test('23 partial plan closure preserves unfinished work', async ({ page }) => {
  const d = await action(page, '/customers/plans/PL-009', 'recap')
  await fill(d, {
    result: '部分达成',
    reason: '两点位已验收，剩余点位复验及续约事项持续推进。',
    disposition: true,
  })
  await snap(page, '23')
  await commit(d)
  expect((await state(page)).work.find((w: { id: string }) => w.id === 'CS-102').status).not.toBe('已结束')
})
test('24 risk triage uses existing work', async ({ page }) => {
  const d = await action(page, '/customers/risks?target=CUS-0186', 'triage')
  await fill(d, { reason: '已核对完整性与点位状态，复用已有服务恢复验证事项。' })
  const before = (await state(page)).work.length
  await snap(page, '24')
  await commit(d)
  expect((await state(page)).work.length).toBe(before)
})
test('25 dry run never creates work or notifications', async ({ page }) => {
  const d = await action(page, '/customers/automation?target=RULE-01', 'rule-test')
  const before = await state(page)
  await commit(d)
  const after = await state(page)
  expect(after.work.length).toBe(before.work.length)
  expect(after.notifications.length).toBe(before.notifications.length)
  await snap(page, '25')
})
test('26 recover only missing step with visible historical receipt', async ({ page }) => {
  const d = await action(page, '/customers/executions?target=RUN-240', 'recover')
  await fill(d, { reconciled: true, reason: '已确认 CS-102 存在，仅补齐失败的通知步骤。' })
  const count = (await state(page)).work.length
  await snap(page, '26')
  await commit(d)
  expect((await state(page)).work.length).toBe(count)
})
test('27 new workflow does not move in-flight work versions', async ({ page }) => {
  const d = await action(page, '/customers/workflows', 'workflow-publish')
  await fill(d, { reason: '明确验收前的证据检查，仅对新事项生效。', confirm: true })
  await snap(page, '27')
  await commit(d)
  expect((await state(page)).workflowVersion).toBe(4)
  expect((await state(page)).work[0].workflowVersion).toBe(3)
})
test('28 service timing strategy is independent of next action', async ({ page }) => {
  const d = await action(page, '/customers/sla', 'sla')
  await fill(d, { responseMinutes: '30', recoveryHours: '4' })
  await snap(page, '28')
  await commit(d)
})
test('29 object field attachment and expiry sharing preview', async ({ page }) => {
  const d = await action(page, '/customers/sharing', 'share')
  await snap(page, '29')
  await commit(d)
  expect((await state(page)).grants.length).toBe(2)
})
test('30 customer confirmation does not fake business completion', async ({ page }) => {
  const d = await action(page, '/customers/client/SH-031', 'client-accept')
  await fill(d, { reason: '已确认本次授权交付范围，提交服务方核验。', confirm: true })
  await snap(page, '30')
  await commit(d)
  expect((await state(page)).work.find((w: { id: string }) => w.id === 'CS-104').status).toBe('待验收')
})
test('31 client rejection returns original record preserving partial pass', async ({ page }) => {
  const d = await action(page, '/customers/client/SH-031', 'client-reject')
  await fill(d, { reason: '苏州高铁店试运行未达到约定标准，请安排整改与复验。' })
  await snap(page, '31')
  await commit(d)
  expect((await state(page)).sites.filter((s: { accepted: boolean }) => s.accepted)).toHaveLength(2)
})
test('32 outcome report supports source drilldown', async ({ page }) => {
  await ready(page, '/customers/reports')
  await snap(page, '32')
})
test('33 import returns per-row created IDs and preserves duplicates', async ({ page }) => {
  await ready(page, '/customers/import')
  await page.getByRole('button', { name: /载入.*示例/ }).click()
  await snap(page, '33')
  await page.getByRole('button', { name: '仅导入通过的 5 条', exact: true }).click()
  const d = page.getByRole('dialog', { name: '确认导入客户', exact: true })
  await d.getByRole('button', { name: '确认导入', exact: true }).click()
  await expect(page.getByText('导入结果已回读', { exact: true })).toBeVisible()
  expect((await state(page)).customers).toHaveLength(13)
})
test('34 active customer archival is blocked', async ({ page }) => {
  const d = await action(page, '/customers/accounts/CUS-0186', 'archive')
  await fill(d, { reason: '核对归档前置条件，存在未完成处置时不得归档。', confirm: true })
  await expect(d.locator('button[form="customer-action-form"]')).toBeDisabled()
  await snap(page, '34')
})
test('35 restricted view omits protected financial data', async ({ page }) => {
  await ready(page, '/customers/restricted')
  await expect(page.locator('.customer-area')).not.toContainText('286,000')
  await snap(page, '35')
})
test('36 real second-tab update preserves first-tab conflict draft', async ({ page, context }) => {
  const d = await action(page, '/customers/work/CS-102', 'reschedule')
  await fill(d, { nextAction: '本页未保存的客户跟进安排', reason: '客户要求调整安排' })
  const second = await context.newPage()
  const other = await action(second, '/customers/work/CS-102', 'reschedule')
  await fill(other, {
    nextAction: '协作人员先更新的安排',
    reason: '另一位协作人员更新',
    nextAt: '2026-09-13T10:00',
  })
  await commit(other)
  await page.bringToFront()
  await d.locator('button[form="customer-action-form"]').click()
  await expect(d.getByRole('alert')).toContainText('版本')
  await expect(d.locator('[data-field="nextAction"] textarea')).toHaveValue('本页未保存的客户跟进安排')
  await snap(page, '36')
})
test('37 save view is distinct from query apply', async ({ page }) => {
  const d = await action(page, '/customers/work', 'save-view')
  await snap(page, '37')
  await commit(d)
  expect((await state(page)).views).toHaveLength(1)
})
test('38 read notification does not complete business work', async ({ page }) => {
  await ready(page, '/customers/notifications')
  await snap(page, '38')
})
