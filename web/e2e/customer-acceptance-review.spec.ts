import { test, expect } from '@playwright/test'
import { action, fill, snap, commit, ready, accept, state } from './customer.helpers'

test.use({ viewport: { width: 1536, height: 1024 }, deviceScaleFactor: 2 })

for (const scenario of [
  { id: 'CS-104', image: '13-acceptance-dialog', evidence: ['ACC-042', 'PD-041-043'] },
  { id: 'CS-103', image: '14-service-acceptance-dialog', evidence: ['RUN-103', 'CONF-103'] },
  { id: 'CS-105', image: '17-payment-acceptance-dialog', evidence: ['AR-0901-F'] },
  { id: 'CS-102', image: '18-renewal-acceptance-dialog', evidence: ['HT-2026-058'] },
  { id: 'CS-106', image: '21-return-acceptance-dialog', evidence: ['REC-026', 'SET-026', 'TERM-026'] },
]) {
  test(`positive acceptance evidence and confirmation surface: ${scenario.id}`, async ({ page }) => {
    if (scenario.id === 'CS-102') {
      await ready(page, '/customers/work/CS-103')
      await accept(page, 'CS-103', ['RUN-103', 'CONF-103'])
    }
    await ready(page, `/customers/work/${scenario.id}`)
    const work = (await state(page)).work.find((w: { id: string }) => w.id === scenario.id)
    if (work.status !== '待验收') {
      const transition = await action(page, `/customers/work/${scenario.id}`, 'transition')
      await fill(transition, { status: '待验收', nextAction: '核对已提交的业务证据并验收' })
      await commit(transition)
    }
    const d = await action(page, `/customers/work/${scenario.id}`, 'accept')
    for (const id of scenario.evidence) await d.getByRole('checkbox', { name: new RegExp(id) }).check()
    await fill(d, {
      reason: '逐项核对本客户、本次事项的有效业务来源，确认范围一致且验收条件全部满足。',
      confirm: true,
    })
    await expect(d.getByRole('button', { name: '验收并成功关闭', exact: true })).toBeEnabled()
    await snap(page, scenario.image)
    await commit(d)
    expect((await state(page)).work.find((w: { id: string }) => w.id === scenario.id).resolution).toBe('成功')
  })
}

test('native create work drawer includes customer, responsibility, deadline and acceptance criteria', async ({
  page,
}) => {
  const d = await action(page, '/customers/work', 'create-work')
  await fill(d, {
    kind: 'visit',
    customerId: 'CUS-0186',
    title: '确认十月续租点位与回访安排',
    owner: '张敏',
    description: '联系采购负责人核对续租点位范围，并记录需要售后配合的条件。',
    criteria: '有明确回访结论和客户确认的下一次行动，未完成承诺另建事项。',
    nextAction: '联系客户采购负责人确认回访时间',
    nextAt: '2026-09-12T10:00',
    deadline: '2026-09-15',
  })
  await snap(page, '49-new-work-drawer')
  await commit(d)
  expect(
    (await state(page)).work.some((w: { title: string }) => w.title === '确认十月续租点位与回访安排'),
  ).toBe(true)
})

test('native transition drawer preserves source state until confirmation', async ({ page }) => {
  const d = await action(page, '/customers/work/CS-108', 'transition')
  const before = await state(page)
  await fill(d, {
    status: '处理中',
    nextAction: '完成新增点位现场条件核对',
    nextAt: '2026-09-12T10:00',
    reason: '负责人已接收事项，开始推进；本次不改变合同或设备状态。',
  })
  await snap(page, '50-transition-drawer')
  expect((await state(page)).revision).toBe(before.revision)
  await commit(d)
  expect((await state(page)).work.find((w: { id: string }) => w.id === 'CS-108').status).toBe('处理中')
})
