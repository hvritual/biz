import { test, expect, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'
const screenDir = 'screenshots/site-rental'
test.use({ viewport: { width: 1536, height: 1024 }, deviceScaleFactor: 2, locale: 'zh-CN' })
async function go(page: Page, path = '/sites') {
  await page.goto('/#' + path)
  await expect(page.locator('.rental-area h1').first()).toBeVisible()
  await page.evaluate(() => document.fonts.ready)
  await expect(page.locator('vite-error-overlay')).toHaveCount(0)
}
async function snap(page: Page, name: string) {
  mkdirSync(screenDir, { recursive: true })
  await page.evaluate(() => document.fonts.ready)
  const body = page.getByRole('dialog').locator('.dialog-body')
  if (await body.count()) {
    await body.evaluate((el) => (el.scrollTop = 0))
  }
  await page.screenshot({ path: `${screenDir}/${name}.png`, fullPage: false, animations: 'disabled' })
  if (await body.count()) {
    const size = await body.evaluate((el) => ({ height: el.clientHeight, total: el.scrollHeight }))
    if (size.total > size.height + 20) {
      for (
        let offset = size.height - 100, part = 2;
        offset < size.total;
        offset += size.height - 100, part++
      ) {
        await body.evaluate((el, top) => (el.scrollTop = top), offset)
        const suffix = part === 2 ? 'continued' : `detail-${part}`
        await page.screenshot({
          path: `${screenDir}/${name}-${suffix}.png`,
          fullPage: false,
          animations: 'disabled',
        })
        if (offset + size.height >= size.total) break
      }
    }
  } else if (!(await page.locator('.module-panel').count())) {
    const scrolls = await page.evaluate(() => document.documentElement.scrollHeight > innerHeight + 20)
    if (scrolls)
      await page.screenshot({ path: `${screenDir}/${name}-full.png`, fullPage: true, animations: 'disabled' })
  }
}
async function openRule(page: Page) {
  await go(page, '/sites/groups/BG-SZ-01')
  await page.getByRole('button', { name: '创建规则变更', exact: true }).click()
  const d = page.getByRole('dialog', { name: '变更计费规则', exact: true })
  await expect(d).toBeVisible()
  return d
}
async function createStatement(page: Page) {
  await go(page, '/sites/groups/BG-SZ-01')
  await page.getByRole('button', { name: '生成 / 查看对账草稿', exact: true }).click()
  const d = page.getByRole('dialog', { name: '点位租赁对账核对' })
  await expect(d).toBeVisible()
  return d
}
async function current(page: Page) {
  return page.evaluate(() => JSON.parse(localStorage.getItem('coffeelink:customer-preview:v1:shanghai')!))
}
test.beforeEach(async ({ page }) => {
  await page.clock.install({ time: new Date('2026-09-11T10:00:00Z') })
})
test('site list, actual linked records, and existing connected navigation', async ({ page }) => {
  await go(page)
  await expect(page.locator('.rental-site-table tbody tr')).toHaveCount(5)
  const metricBoxes = await page
    .locator('.rental-area .metric-grid > *')
    .evaluateAll((els) =>
      els.map((el) => ({ x: el.getBoundingClientRect().x, y: el.getBoundingClientRect().y })),
    )
  expect(metricBoxes).toHaveLength(4)
  expect(new Set(metricBoxes.map((box) => Math.round(box.y))).size).toBe(1)
  expect(metricBoxes[3]!.x).toBeGreaterThan(metricBoxes[0]!.x)
  await snap(page, 'S01-site-list')
  const before = await page.getByTestId('main-content').boundingBox()
  await page.locator('[data-module=sites]').hover()
  await expect(page.locator('.module-panel')).toBeVisible()
  const nav = (await page.locator('.primary-nav').boundingBox())!,
    panel = (await page.locator('.module-panel').boundingBox())!
  expect(panel.width).toBe(480)
  expect(panel.x).toBe(nav.x + nav.width)
  expect(await page.getByTestId('main-content').boundingBox()).toEqual(before)
  await expect(page.locator('.sub-link .lucide-chevron-right')).toHaveCount(0)
  await snap(page, 'S02-connected-navigation')
  await page.keyboard.press('Escape')
  await expect(page.locator('.module-panel')).toHaveCount(0)
  await page.getByRole('button', { name: '共享额度', exact: true }).click()
  await expect(page.locator('.rental-site-table tbody tr')).toHaveCount(2)
})
test('query draft is not applied until search, reset restores rows', async ({ page }) => {
  await go(page)
  await page.getByLabel('搜索点位', { exact: true }).fill('员工茶水间')
  await expect(page.locator('.rental-site-table tbody tr')).toHaveCount(5)
  await page.getByRole('button', { name: '查询', exact: true }).click()
  await expect(page.locator('.rental-site-table tbody tr')).toHaveCount(1)
  await page.getByRole('button', { name: '重置', exact: true }).click()
  await expect(page.locator('.rental-site-table tbody tr')).toHaveCount(5)
})
test('site details preserve deployments, terms and usage as distinct views', async ({ page }) => {
  for (const [path, name] of [
    ['/sites/SITE-041', 'S03-site-workspace'],
    ['/sites/SITE-041?tab=placements', 'S04-deployment-history'],
    ['/sites/SITE-044?tab=rental', 'S05-fixed-rent'],
    ['/sites/SITE-045?tab=rental', 'S06-metered-rent'],
    ['/sites/SITE-041?tab=usage', 'S07-usage-quality'],
    ['/sites/SITE-043?tab=service', 'S08-linked-work'],
    ['/sites/groups', 'S09-rule-list'],
    ['/sites/groups/BG-SZ-01', 'S10-shared-pool'],
  ]) {
    await go(page, path!)
    await snap(page, name!)
  }
  await expect(page.locator('.rental-stat-strip')).toContainText('1,900')
  await expect(page.locator('.rental-stat-strip')).toContainText('100')
  await expect(page.locator('.rental-contributions tbody tr')).toHaveCount(2)
  await expect(page.locator('.rental-calculation-line')).toHaveCount(1)
})
test('new site saves and reads back without automatically renting or provisioning', async ({ page }) => {
  await go(page)
  await page.getByRole('button', { name: '新建点位', exact: true }).click()
  const d = page.getByRole('dialog', { name: '新建点位档案' })
  await d.getByLabel('所属客户').selectOption('CUS-0186')
  await d.getByLabel('点位名称').fill('研发楼 · 5 楼茶水间')
  await d.getByLabel('上级位置 / 分组').fill('上海总部 / 研发楼')
  await d.getByLabel('详细地址').fill('上海 · 研发楼 5F')
  await d.getByLabel('客户现场联系人').fill('客户行政')
  await snap(page, 'S11-create-site')
  await d.getByRole('button', { name: '保存点位（预览）' }).click()
  await expect(d).toHaveCount(0)
  await expect(page.locator('.rental-identity')).toContainText('待勘察')
  await page.reload()
  await expect(page.locator('.rental-identity')).toContainText('研发楼 · 5 楼茶水间')
  const data = await current(page)
  expect(data.rental.profiles).toHaveLength(6)
  expect(data.rental.rules).toHaveLength(3)
  await snap(page, 'S12-created-readback')
})
test('duplicate point fails with draft preserved', async ({ page }) => {
  await go(page)
  await page.getByRole('button', { name: '新建点位', exact: true }).click()
  const d = page.getByRole('dialog')
  await d.getByLabel('所属客户').selectOption('CUS-0186')
  await d.getByLabel('点位名称').fill('苏州园区大堂')
  await d.getByLabel('上级位置 / 分组').fill('苏州运营片区')
  await d.getByLabel('详细地址').fill('苏州园区')
  await d.getByLabel('客户现场联系人').fill('林悦')
  await d.getByRole('button', { name: '保存点位（预览）' }).click()
  await expect(d.getByRole('alert')).toContainText('已存在')
  await expect(d.getByLabel('点位名称')).toHaveValue('苏州园区大堂')
  await snap(page, 'S13-duplicate-rejected')
})
test('shared rule configuration, side-effect free trial, draft, scheduled version and history', async ({
  page,
}) => {
  const d = await openRule(page)
  await d.getByLabel('基础费用（元）').fill('2600')
  await d.getByLabel('整组共享含杯额度（杯）').fill('2500')
  await d.getByLabel('SITE-041 试算杯数').fill('600')
  await d.getByLabel('SITE-042 试算杯数').fill('1300')
  await d.getByLabel('合同约定 / 变更依据').fill('客户确认 Q4 共享额度调整，旧账期不变')
  await snap(page, 'S14-shared-rule-editor')
  expect(
    await page.evaluate(() => localStorage.getItem('coffeelink:customer-preview:v1:shanghai')),
  ).toBeNull()
  await d.getByRole('button', { name: '试算并预览影响' }).click()
  await expect(d.locator('.rental-total')).toContainText('2,600')
  expect(
    await page.evaluate(() => localStorage.getItem('coffeelink:customer-preview:v1:shanghai')),
  ).toBeNull()
  await snap(page, 'S15-rule-change-preview')
  await d.getByRole('button', { name: '保存规则草稿' }).click()
  expect((await current(page)).rental.rules).toHaveLength(3)
  await page.getByRole('button', { name: '创建规则变更', exact: true }).click()
  await d.getByRole('button', { name: '试算并预览影响' }).click()
  await d.getByRole('button', { name: '确认预约发布' }).click()
  await expect(d).toHaveCount(0)
  expect((await current(page)).rental.rules).toHaveLength(4)
  await expect(page.locator('.rental-total')).toContainText('2,400')
  await page.getByRole('button', { name: '版本履历', exact: true }).click()
  await expect(page.locator('.rental-version')).toHaveCount(2)
  await snap(page, 'S16-version-history')
})
for (const [mode, name] of [
  ['fixed', 'S17-fixed-rule-editor'],
  ['metered', 'S18-metered-rule-editor'],
])
  test(`explicit independent ${mode} configuration`, async ({ page }) => {
    const d = await openRule(page)
    await d.getByLabel('计费模式', { exact: false }).selectOption(mode!)
    await d.getByLabel('计费范围').selectOption('independent')
    if (mode === 'fixed') await d.getByLabel('固定月租（元）').fill('1200')
    await d.getByLabel('SITE-041 试算杯数').fill('600')
    await d.getByLabel('SITE-042 试算杯数').fill('1300')
    await d.getByLabel('合同约定 / 变更依据').fill('仅试算模式，不发布或修改合同')
    await snap(page, name!)
    await d.getByRole('button', { name: '试算并预览影响' }).click()
    await expect(d.locator('.rental-calculation-line')).toHaveCount(2)
    await expect(d.locator('.rental-total')).toContainText(mode === 'fixed' ? '2,400' : '3,040')
  })
test('missing scope or mid-period date cannot be published', async ({ page }) => {
  await go(page, '/sites/groups?action=new-rule')
  const d = page.getByRole('dialog')
  await d.getByLabel('计费规则 / 组名称').fill('测试规则')
  await d.getByLabel('关联客户').selectOption('CUS-0186')
  await d.getByLabel('有效合同来源').selectOption('HT-2026-041')
  await d.getByLabel('固定月租（元）').fill('1200')
  await d.getByLabel('合同约定 / 变更依据').fill('测试校验')
  await d.getByRole('button', { name: '试算并预览影响' }).click()
  await expect(d.getByRole('alert')).toContainText('明确选择')
  await d.getByLabel('计费范围').selectOption('independent')
  await d.getByLabel('预约生效日期').fill('2026-10-15')
  await d.getByRole('button', { name: '试算并预览影响' }).click()
  await expect(d.getByRole('alert')).toContainText('月初')
  await snap(page, 'S19-rule-guard')
})
test('billing draft, dispute, resolution and frozen snapshot form a single-record loop', async ({ page }) => {
  const d = await createStatement(page)
  await snap(page, 'S20-statement-draft')
  await d.getByLabel('核对依据 / 异议处理说明').fill('核对测试制作是否按合同排除')
  await d.getByRole('button', { name: '登记异议', exact: true }).click()
  await expect(d).toContainText('异议未结束')
  await snap(page, 'S21-dispute-blocked')
  await d.getByLabel('核对依据 / 异议处理说明').fill('双方确认测试制作排除口径')
  await d.getByRole('button', { name: '记录解决结论' }).click()
  await d.getByLabel('核对依据 / 异议处理说明').fill('核对合同、成员范围、用量及排除项，确认金额')
  await d.getByRole('button', { name: '确认对账并冻结' }).click()
  await expect(d).toContainText('已确认快照锁定')
  await expect(d.locator('.rental-total')).toContainText('已确认对账金额')
  await snap(page, 'S22-frozen-statement')
  await d.getByRole('button', { name: '关闭', exact: true }).click()
  await page.getByRole('button', { name: '生成 / 查看对账草稿', exact: true }).click()
  expect((await current(page)).rental.statements).toHaveLength(1)
  await d.getByRole('button', { name: '关闭', exact: true }).click()
  await go(page, '/sites/statements')
  await snap(page, 'S23-statement-list')
})
test('missing usage is not zero and disables confirmation', async ({ page }) => {
  await go(page, '/sites/groups/BG-SZ-01')
  await page.getByLabel('计费组账期').fill('2026-10')
  await expect(page.locator('.rental-quote')).toContainText('不能按零计费')
  await snap(page, 'S24-missing-usage')
  await page.getByRole('button', { name: '生成 / 查看对账草稿', exact: true }).click()
  await expect(page.getByRole('button', { name: '确认对账并冻结' })).toBeDisabled()
})
test('pause keeps charges and deployments, tracks next verification', async ({ page }) => {
  await go(page, '/sites/SITE-041')
  await page.getByRole('button', { name: '调整运营安排', exact: true }).click()
  const d = page.getByRole('dialog')
  await d.getByLabel('调整原因').fill('客户场地临时装修')
  await d.getByLabel('下次核实时间').fill('2026-09-18T10:00')
  await snap(page, 'S25-operational-pause')
  await d.getByRole('button', { name: '确认临时停用' }).click()
  const data = await current(page)
  expect(data.rental.rules).toHaveLength(3)
  expect(data.rental.deployments).toHaveLength(6)
  await expect(page.locator('.rental-identity')).toContainText('临时停用')
})
test('point creates a work item in the same customer task store', async ({ page }) => {
  await go(page, '/sites/SITE-041')
  await page.getByRole('button', { name: '新建关联事项', exact: true }).click()
  const d = page.getByRole('dialog')
  await d.locator('[data-field=deadline] input').fill('2026-09-18')
  await d.locator('[data-field=nextAt] input').fill('2026-09-12T10:00')
  await d.locator('button[form=customer-action-form]').click()
  await expect(d).toHaveCount(0)
  const data = await current(page)
  expect(data.work[0].siteIds).toEqual(['SITE-041'])
  await page.goto(`/#/customers/work/${data.work[0].id}`)
  await expect(page.locator('.customer-area h1').first()).toBeVisible()
  await expect(page.getByRole('link', { name: 'SITE-041 · 打开点位工作区' })).toBeVisible()
})
test('tenant switching closes drafts and clears previous rental writes', async ({ page }) => {
  await createStatement(page)
  await page.getByRole('dialog').getByRole('button', { name: '关闭', exact: true }).click()
  await go(page)
  await page.getByRole('button', { name: '新建点位', exact: true }).click()
  await page.getByRole('dialog').getByLabel('点位名称').fill('未保存草稿')
  await page.getByRole('dialog').getByRole('button', { name: '取消', exact: true }).click()
  await page.getByRole('button', { name: '放弃修改', exact: true }).click()
  await page.getByLabel('切换企业', { exact: true }).selectOption('hangzhou')
  await go(page, '/sites/statements')
  await expect(page.getByText('本账期尚无对账记录')).toBeVisible()
})
test('concurrent writes preserve the first window draft instead of overwriting', async ({
  page,
  context,
}) => {
  await go(page, '/sites/SITE-041')
  await page.getByRole('button', { name: '编辑点位', exact: true }).click()
  const d = page.getByRole('dialog')
  await d.getByLabel('点位名称').fill('窗口一待保存名称')
  const second = await context.newPage()
  await go(second, '/sites/SITE-041')
  await second.getByRole('button', { name: '编辑点位', exact: true }).click()
  await second.getByRole('dialog').getByLabel('点位名称').fill('窗口二最新名称')
  await second.getByRole('dialog').getByRole('button', { name: '保存点位（预览）' }).click()
  await d.getByRole('button', { name: '保存点位（预览）' }).click()
  await expect(d.getByRole('alert')).toContainText('其他窗口更新')
  await expect(d.getByLabel('点位名称')).toHaveValue('窗口一待保存名称')
  await snap(page, 'S26-concurrent-conflict')
})
test('responsive views and dialogs are reachable without viewport overflow or console errors', async ({
  page,
}) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('console', (m) => {
    if (m.type() === 'error') errors.push(m.text())
  })
  for (const [width, height] of [
    [1366, 768],
    [1440, 900],
    [390, 844],
  ]) {
    await page.setViewportSize({ width: width!, height: height! })
    await go(page)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width!)
    await snap(page, `S27-sites-${width}`)
    await go(page, '/sites/groups/BG-SZ-01')
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width!)
    await snap(page, `S28-shared-${width}`)
    await page.getByRole('button', { name: '创建规则变更', exact: true }).click()
    const d = page.getByRole('dialog')
    await expect(d.getByRole('button', { name: '试算并预览影响' })).toBeVisible()
    await snap(page, `S29-editor-${width}`)
    await d.getByRole('button', { name: '取消', exact: true }).click()
  }
  expect(errors).toEqual([])
})

test('independent group totals are not duplicated into each site summary', async ({ page }) => {
  const d = await openRule(page)
  await d.getByLabel('计费模式').selectOption('fixed')
  await d.getByLabel('计费范围').selectOption('independent')
  await d.getByLabel('固定月租（元）').fill('1200')
  await d.getByLabel('合同约定 / 变更依据').fill('双方确认按点位独立月租，客户汇总不得成为每点金额')
  await d.getByRole('button', { name: '试算并预览影响' }).click()
  await d.getByRole('button', { name: '确认预约发布' }).click()
  await expect(d).toHaveCount(0)
  await go(page, '/sites/SITE-041')
  await page.getByLabel('点位账期').fill('2026-10')
  const fee = page.locator('.rental-stat-strip > div').filter({ hasText: '本点位预计费用' })
  await expect(fee).toContainText('1,200.00')
  await expect(fee).not.toContainText('2,400.00')
})

test('expired contract has no current quote while historical versions remain available', async ({ page }) => {
  await go(page, '/sites/groups/BG-SH-01')
  await page.getByLabel('计费组账期').fill('2026-10')
  await expect(page.getByText('该账期没有有效计费规则', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: '生成 / 查看对账草稿', exact: true })).toHaveCount(0)
  await snap(page, 'S30-expired-terms')
  await page.getByRole('button', { name: '版本履历', exact: true }).click()
  await expect(page.locator('.rental-version')).toHaveCount(1)
})
