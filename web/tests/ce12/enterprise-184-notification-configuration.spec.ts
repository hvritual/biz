import { expect, test, type BrowserContext, type Page, type Route } from '@playwright/test'
import { readFileSync, writeFileSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { selectUiOption } from '../../e2e/ui.helpers'
import type { TrustedSession } from '../../src/services/runtime/api'
import type { MessageConfiguration } from '../../src/services/enterprise/notificationConfigurationRuntime'
interface Fixture {
  base_url: string; ui_base_url: string
  notification: { email: string; password: string; reader_email: string; reader_password: string; tenant_a: string; tenant_b: string; site_id: string; site_b_id: string; owner_id: string; reader_id: string; contact_id: string }
}
function fixture(): Fixture {
  const path = process.env.CE12_E2E_ENV_FILE
  if (!path) throw new Error('CE12_E2E_ENV_FILE required')
  const data = JSON.parse(readFileSync(path, 'utf8')) as Fixture
  expect(data.notification).toBeTruthy()
  for (const url of [data.base_url, data.ui_base_url]) expect(new URL(url).hostname).toBe('127.0.0.1')
  return data
}
const panel = (page: Page) => page.locator('[data-enterprise-page="notification-settings"]')
const dialog = (page: Page) => page.getByRole('dialog', { name: /^(新建|编辑)企业消息配置$/ })
async function login(page: Page, data: Fixture, reader = false) {
  await page.goto(data.base_url + '/auth/login?return_to=/auth/session')
  await page.getByLabel('账号 / 手机号 / 邮箱', { exact: true }).fill(reader ? data.notification.reader_email : data.notification.email)
  await page.getByLabel('密码').fill(reader ? data.notification.reader_password : data.notification.password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  if (await page.getByRole('heading', { name: '确认隐私与服务协议' }).isVisible()) {
    await page.getByLabel(/我已阅读并同意/).check(); await page.getByRole('button', { name: '同意并继续' }).click()
  }
  await expect(page).toHaveURL(data.base_url + '/auth/session')
}
async function session(context: BrowserContext, data: Fixture): Promise<TrustedSession> {
  const response = await context.request.get(data.base_url + '/auth/session'); expect(response.status()).toBe(200)
  const result = await response.json() as TrustedSession; expect(result.authenticated).toBe(true); return result
}
async function selectTenant(context: BrowserContext, data: Fixture, tenant: string) {
  const current = await session(context, data)
  const response = await context.request.post(data.base_url + '/auth/session/tenant', { headers: { 'X-CSRF-Token': current.csrf_token ?? '' }, data: { tenant_id: tenant } })
  expect(response.status()).toBe(200)
}
async function open(page: Page, data: Fixture) {
  await page.goto(data.ui_base_url + '/#/system/notifications')
  await expect(panel(page)).toBeVisible()
  await expect(panel(page).getByRole('button', { name: '新建配置', exact: true })).toBeEnabled()
  await expect(page.locator('vite-error-overlay')).toHaveCount(0)
}
async function list(context: BrowserContext, data: Fixture) {
  const response = await context.request.get(data.base_url + '/v1/tenant/notification/configurations?page_size=100')
  expect(response.status()).toBe(200)
  const result = await response.json() as { items?: MessageConfiguration[]; tenantId: string; total?: string }
  return { items: result.items ?? [], tenantId: result.tenantId, total: Number(result.total ?? 0) }
}
// Read-only SQL evidence in the existing isolated CI database. No authority or
// business rows are created by the browser observer, and no production DSN is accepted.
function persisted(tenant: string) {
  const container = process.env.CE12_MYSQL_CONTAINER
  if (!container || !/^ce12-browser-\d+-\d+$/.test(container) || !/^[\w:-]+$/.test(tenant)) throw new Error('isolated CE12 database identity required')
  const sql = `SELECT id,level,version,notes FROM biz_notification_configurations WHERE tenant_id='${tenant}' ORDER BY level; SELECT COUNT(*) FROM biz_notification_configuration_recipients r LEFT JOIN biz_notification_configurations c ON c.id=r.configuration_id AND c.tenant_id=r.tenant_id WHERE r.tenant_id='${tenant}' AND c.id IS NULL;`
  return execFileSync('docker', ['exec', container, 'mysql', '-uroot', '-proot', '--batch', '--skip-column-names', 'biz_ce12_browser', '-e', sql], { encoding: 'utf8', timeout: 10000, stdio: ['ignore', 'pipe', 'pipe'] }).trim()
}
async function fillCreate(page: Page, data: Fixture) {
  await panel(page).getByRole('button', { name: '新建配置', exact: true }).click()
  const form = dialog(page), group = form.getByRole('group', { name: '业务点位', exact: true })
  await group.getByRole('textbox').fill('103'); await group.getByRole('button', { name: '查询', exact: true }).click()
  await expect(group.getByRole('combobox')).toBeEnabled()
  await selectUiOption(group.getByRole('combobox'), data.notification.site_id)
  await form.getByRole('checkbox', { name: '紧急', exact: true }).check()
  await form.getByRole('checkbox', { name: '一般', exact: true }).check()
  await expect(form.getByRole('checkbox', { name: '短信', exact: true })).toBeDisabled()
  await expect(form.getByRole('checkbox', { name: '邮件', exact: true })).toBeDisabled()
  await form.getByRole('checkbox', { name: '站内消息', exact: true }).check()
  await selectUiOption(form.getByRole('combobox', { name: '第一联系人', exact: true }), data.notification.owner_id)
  await selectUiOption(form.getByRole('combobox', { name: '第二联系人（可选）', exact: true }), data.notification.reader_id)
  await selectUiOption(form.getByRole('combobox', { name: '其他接收人', exact: true }), data.notification.contact_id)
  await form.getByRole('textbox', { name: '备注', exact: true }).fill('browser initial')
}
function rule(page: Page, priority = '紧急') { return panel(page).getByRole('region', { name: '点位通知规则' }).getByRole('row').filter({ hasText: priority }) }
async function edit(page: Page, notes: string) {
  await rule(page).getByRole('button', { name: '编辑', exact: true }).click()
  await expect(dialog(page)).toBeVisible(); await dialog(page).getByRole('textbox', { name: '备注', exact: true }).fill(notes)
}

// No successful API response is mocked. Only transport loss is injected; every
// accepted write is executed by the real BFF, generated Action and MySQL UoW.
test('TestEnterprise184US040To044LiveBrowserConfigurationRecoveryAndFourViewports', async ({ page, context, browser }) => {
  test.setTimeout(100000)
  const data = fixture(), n = data.notification, errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  await login(page, data); await selectTenant(context, data, n.tenant_a); await open(page, data)
  expect((await list(context, data)).total).toBe(0)
  const before = persisted(n.tenant_a)
  await test.step('US040 types use independent filters and full counts', async () => {
    const catalog = panel(page).getByRole('region', { name: '消息类型', exact: true })
    await selectUiOption(catalog.getByRole('combobox', { name: '消息等级', exact: true }), 'important')
    await catalog.getByRole('button', { name: '查询', exact: true }).click()
    await expect(catalog.getByText('筛选结果 2 条', { exact: true })).toBeVisible()
    await expect(catalog.getByRole('row')).toHaveCount(3)
    await catalog.getByRole('textbox', { name: '类型编码' }).fill('device.offline')
    await catalog.getByRole('button', { name: '查询', exact: true }).click()
    await expect(catalog.getByText('筛选结果 1 条', { exact: true })).toBeVisible()
  })
  await test.step('US042 cancel does not write; site beyond first 100 is selectable', async () => {
    await fillCreate(page, data); await dialog(page).getByRole('button', { name: '取消', exact: true }).click()
    expect(persisted(n.tenant_a)).toBe(before)
    await fillCreate(page, data); await dialog(page).getByRole('button', { name: '确认保存', exact: true }).click()
    await expect(panel(page).getByText('配置已保存，最新状态已确认。', { exact: true })).toBeVisible()
  })
  const created = await list(context, data)
  expect(created.total).toBe(2); expect(created.items.every(row => row.groupId === n.site_id && row.version === '1')).toBe(true)
  expect(created.items.every(row => row.secondaryUserId === n.reader_id && row.additionalUserIds?.[0] === n.contact_id)).toBe(true)
  expect(persisted(n.tenant_a)).toContain('browser initial')
  await page.reload(); await expect(rule(page).getByRole('button', { name: '编辑', exact: true })).toBeEnabled()

  const uncertain: Array<{ key: string; body: string | null }> = []
  await test.step('US043 lost response after real commit retries the original request once', async () => {
    let lose = true
    const intercept = async (route: Route) => {
      if (route.request().method() === 'PATCH') {
        uncertain.push({ key: route.request().headers()['idempotency-key'] ?? '', body: route.request().postData() })
        if (lose) { lose = false; const real = await route.fetch(); expect(real.status()).toBe(200); await route.abort('failed'); return }
      }
      await route.continue()
    }
    await page.route('**/api/v1/tenant/notification/configurations/*', intercept)
    await edit(page, 'response recovered'); await dialog(page).getByRole('button', { name: '确认保存', exact: true }).click()
    await expect(dialog(page)).toContainText('提交结果尚不确定')
    expect((await list(context, data)).items.find(row => row.level === 'urgent')?.version).toBe('2')
    await dialog(page).getByRole('button', { name: '核对本次操作', exact: true }).click()
    await expect(dialog(page)).toBeHidden()
    expect(uncertain).toHaveLength(2); expect(uncertain[0]?.key).not.toBe(''); expect(uncertain[1]).toEqual(uncertain[0])
    expect((await list(context, data)).items.find(row => row.level === 'urgent')?.version).toBe('2')
    await page.unroute('**/api/v1/tenant/notification/configurations/*', intercept)
  })
  await test.step('acknowledged write with failed readback recovers using GET only', async () => {
    let failRead = false, writes = 0
    const intercept = async (route: Route) => {
      if (route.request().method() === 'PATCH') { writes++; failRead = true }
      else if (route.request().method() === 'GET' && failRead) { failRead = false; await route.abort('failed'); return }
      await route.continue()
    }
    await page.route('**/api/v1/tenant/notification/configurations/*', intercept)
    await edit(page, 'readback recovered'); await dialog(page).getByRole('button', { name: '确认保存', exact: true }).click()
    await expect(dialog(page)).toContainText('配置修改已受理，但最新状态尚未确认')
    await expect(panel(page).getByText('配置已保存，最新状态已确认。', { exact: true })).toHaveCount(0)
    await dialog(page).getByRole('button', { name: '核对本次操作', exact: true }).click()
    await expect(dialog(page)).toBeHidden(); expect(writes).toBe(1)
    expect((await list(context, data)).items.find(row => row.level === 'urgent')?.version).toBe('3')
    await page.unroute('**/api/v1/tenant/notification/configurations/*', intercept)
  })
  await test.step('US041 recipient filters match actual API results', async () => {
    const rules = panel(page).getByRole('region', { name: '点位通知规则' })
    await selectUiOption(rules.getByRole('combobox', { name: '接收人筛选', exact: true }), n.contact_id)
    await rules.getByRole('button', { name: '查询', exact: true }).last().click()
    await expect(rules.getByText('筛选结果 2 条', { exact: true })).toBeVisible()
    await rules.getByRole('button', { name: '清空筛选', exact: true }).click()
  })
  await test.step('four viewports, modal boundaries, Escape and return focus', async () => {
    for (const viewport of [{ width: 1366, height: 768 }, { width: 1440, height: 900 }, { width: 1536, height: 1024 }, { width: 390, height: 844 }]) {
      await page.setViewportSize(viewport); await page.evaluate(() => window.scrollTo(0, 0))
      await expect(rule(page).getByRole('button', { name: '编辑', exact: true })).toBeEnabled()
      await expect(panel(page).locator('[aria-busy="true"]')).toHaveCount(0)
      await page.screenshot({ path: `test-results/enterprise184-live-${viewport.width}.png`, fullPage: true })
      const button = rule(page).getByRole('button', { name: '编辑', exact: true }); await button.click()
      await expect(dialog(page)).toBeVisible()
      // Capture the completed controlled-directory state, not a transient load.
      await expect(dialog(page).locator('[aria-busy="true"]')).toHaveCount(0)
      const bounds = await dialog(page).boundingBox()
      expect(bounds!.x).toBeGreaterThanOrEqual(0); expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(viewport.width)
      await page.screenshot({ path: `test-results/enterprise184-live-edit-${viewport.width}.png` })
      await page.keyboard.press('Escape'); await expect(dialog(page)).toBeHidden(); await expect(button).toBeFocused()
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    }
  })
  await page.setViewportSize({ width: 1366, height: 768 })
  await test.step('US044 deletion requires confirmation and leaves no orphan relations', async () => {
    await rule(page).getByRole('button', { name: '删除', exact: true }).click()
    const deletion = page.getByRole('dialog', { name: '删除企业消息配置' })
    await deletion.getByRole('button', { name: '取消', exact: true }).click(); expect((await list(context, data)).total).toBe(2)
    await rule(page).getByRole('button', { name: '删除', exact: true }).click()
    await deletion.getByRole('button', { name: '确认删除', exact: true }).click()
    await expect(panel(page).getByText('配置已删除，最新状态已确认。', { exact: true })).toBeVisible()
    expect((await list(context, data)).total).toBe(1); expect(persisted(n.tenant_a).split('\n').at(-1)).toBe('0')
  })
  await test.step('tenant switching drops old rows; fresh login preserves saved state', async () => {
    await selectUiOption(page.getByRole('combobox', { name: '切换企业' }), n.tenant_b)
    await expect(panel(page)).toContainText('消息验收企业 B')
    await expect(panel(page).getByRole('cell', { name: '消息点位 103' })).toHaveCount(0)
    expect((await list(context, data)).total).toBe(0)
    await selectUiOption(page.getByRole('combobox', { name: '切换企业' }), n.tenant_a)
    await expect(panel(page).getByRole('cell', { name: '消息点位 103' })).toBeVisible()
    const fresh = await browser.newContext({ locale: 'zh-CN', timezoneId: 'Asia/Shanghai' })
    try {
      const freshPage = await fresh.newPage(); await login(freshPage, data); await selectTenant(fresh, data, n.tenant_a); await open(freshPage, data)
      expect((await list(fresh, data)).total).toBe(1)
    } finally { await fresh.close() }
  })
  expect(errors).toEqual([]); await expect(page.locator('vite-error-overlay')).toHaveCount(0)
  writeFileSync('test-results/enterprise184-live-evidence.json', JSON.stringify({
    candidate_sha: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
    candidate_tree: execFileSync('git', ['rev-parse', 'HEAD^{tree}'], { encoding: 'utf8' }).trim(),
    real_idp_bff_mysql: true, mocked_success_responses: false, uncertain_retries: uncertain,
    created, final: await list(context, data), mysql: persisted(n.tenant_a), application_errors: errors,
  }, null, 2) + '\n')
})

test('TestEnterprise184ReadOnlyNavigationAndIndependentServerWriteDenial', async ({ page, context }) => {
  const data = fixture(), n = data.notification
  await login(page, data, true)
  await page.goto(data.ui_base_url + '/#/system/notifications')
  await expect(panel(page)).toBeVisible()
  await expect(panel(page).getByRole('button', { name: '新建配置', exact: true })).toHaveCount(0)
  await expect(panel(page).getByRole('button', { name: '编辑', exact: true })).toHaveCount(0)
  await expect(panel(page).getByRole('button', { name: '删除', exact: true })).toHaveCount(0)
  const current = await session(context, data)
  const denied = await context.request.post(data.base_url + '/v1/tenant/notification/configurations', { headers: { 'X-CSRF-Token': current.csrf_token ?? '', 'Idempotency-Key': 'n184-reader-must-not-write' }, data: { groupId: n.site_id, levels: ['important'], channels: ['in_app'], primaryUserId: n.reader_id } })
  expect(denied.status()).toBe(403)
  await page.goto(data.ui_base_url + '/#/system/general')
  await expect(page.getByRole('heading', { name: '没有访问权限', exact: true })).toBeVisible()
})
