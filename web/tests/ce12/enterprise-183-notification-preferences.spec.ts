import { expect, test, type BrowserContext, type Page, type Route } from '@playwright/test'
import { readFileSync, writeFileSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import type { NotificationPreferences } from '../../src/services/enterprise/notificationPreferencesRuntime'
import type { TrustedSession } from '../../src/services/runtime/api'

interface Fixture {
  base_url: string
  ui_base_url: string
  email: string
  password: string
  allowed_tenant: string
  iam_denied_tenant: string
}
const endpoint = '/auth/personal/notification-preferences'
const routePattern = '**/api/auth/personal/notification-preferences'
const savedText = '通知偏好已保存，且已从服务端重新读取确认。'
const panel = (page: Page) => page.locator('[data-ui-region="notification-preferences"]')
const dialog = (page: Page) => page.getByRole('dialog', { name: '确认修改通知偏好' })
const smsSwitch = (page: Page) => panel(page).getByRole('switch', { name: '短信通知', exact: true })

function fixture(): Fixture {
  const path = process.env.CE12_E2E_ENV_FILE
  if (!path) throw new Error('CE12_E2E_ENV_FILE is required')
  const data = JSON.parse(readFileSync(path, 'utf8')) as Fixture
  for (const url of [data.base_url, data.ui_base_url]) {
    expect(new URL(url).hostname).toBe('127.0.0.1')
  }
  return data
}
async function login(page: Page, data: Fixture) {
  await page.goto(data.base_url + '/auth/login?return_to=/auth/session')
  await page.getByLabel('账号 / 手机号 / 邮箱', { exact: true }).fill(data.email)
  await page.getByLabel('密码').fill(data.password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  if (await page.getByRole('heading', { name: '确认隐私与服务协议' }).isVisible()) {
    await page.getByLabel(/我已阅读并同意/).check()
    await page.getByRole('button', { name: '同意并继续' }).click()
  }
  await expect(page).toHaveURL(data.base_url + '/auth/session')
}
async function session(context: BrowserContext, data: Fixture): Promise<TrustedSession> {
  const response = await context.request.get(data.base_url + '/auth/session')
  expect(response.status()).toBe(200)
  const value = await response.json() as TrustedSession
  expect(value.authenticated).toBe(true)
  expect(value.actor_kind).toBe('user')
  return value
}
async function selectTenant(context: BrowserContext, data: Fixture, tenant: string) {
  const current = await session(context, data)
  const response = await context.request.post(data.base_url + '/auth/session/tenant', {
    headers: { 'X-CSRF-Token': current.csrf_token ?? '' }, data: { tenant_id: tenant },
  })
  expect(response.status()).toBe(200)
}
async function readPreferences(context: BrowserContext, data: Fixture): Promise<NotificationPreferences> {
  const current = await session(context, data)
  const response = await context.request.get(data.base_url + endpoint, { headers: {
    'X-Biz-Session-Context': JSON.stringify({ actor_kind: current.actor_kind, platform_subject: '',
      user_id: current.user_id, active_tenant_id: current.active_tenant_id, context_version: current.context_version }),
  } })
  expect(response.status()).toBe(200)
  const value = await response.json() as NotificationPreferences
  expect(value.tenant_id).toBe(current.active_tenant_id)
  expect(value.user_id).toBe(current.user_id)
  return value
}
// Read-only observation of the existing isolated CE12 database. No containers,
// schemas, rows or application authority are created/modified by this helper.
function persisted(value: NotificationPreferences) {
  const container = process.env.CE12_MYSQL_CONTAINER
  if (!container || !/^ce12-browser-\d+-\d+$/.test(container)) throw new Error('isolated CE12 MySQL container required')
  for (const id of [value.tenant_id, value.user_id]) if (!/^[\w:-]+$/.test(id)) throw new Error('unexpected fixture identifier')
  const sql = `SELECT channel,state,version FROM biz_notification_preferences WHERE tenant_id='${value.tenant_id}' AND user_id='${value.user_id}' ORDER BY channel;`
  const rows = execFileSync('docker', ['exec', container, 'mysql', '-uroot', '-proot', '--batch',
    '--skip-column-names', 'biz_ce12_browser', '-e', sql], { encoding: 'utf8', timeout: 10000, stdio: ['ignore', 'pipe', 'pipe'] })
    .trim().split('\n').filter(Boolean).map((line) => line.split('\t'))
  for (const channel of ['sms', 'email'] as const) {
    const row = rows.find((candidate) => candidate[0] === channel)
    if (value[channel].state === 'default') expect(row).toBeUndefined()
    else expect(row).toEqual([channel, value[channel].state, String(value[channel].version)])
  }
  return rows
}
async function openProfile(page: Page, data: Fixture) {
  await page.goto(data.ui_base_url + '/#/enterprise/personal-profile')
  await expect(page.locator('[data-enterprise-page="personal-profile"]')).toBeVisible()
  await expect(smsSwitch(page)).toBeEnabled()
}
async function confirmSMS(page: Page) {
  await smsSwitch(page).click()
  await expect(dialog(page)).toBeVisible()
  await dialog(page).getByRole('button', { name: '确认保存', exact: true }).click()
}

test('TestEnterprise183NotificationPreferencesLiveBrowserMySQLRecoveryAndIsolation', async ({ page, context, browser }) => {
  const data = fixture(), errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await login(page, data)
  await selectTenant(context, data, data.iam_denied_tenant)
  const tenantBefore = await readPreferences(context, data)
  persisted(tenantBefore)
  await selectTenant(context, data, data.allowed_tenant)
  const initial = await readPreferences(context, data)
  persisted(initial)
  await openProfile(page, data)
  await expect(smsSwitch(page)).toBeChecked({ checked: initial.sms.allowed })

  await test.step('cancel does not write; confirmation persists independently of email', async () => {
    await smsSwitch(page).click()
    await dialog(page).getByRole('button', { name: '取消修改', exact: true }).click()
    await expect(smsSwitch(page)).toBeChecked({ checked: initial.sms.allowed })
    expect(await readPreferences(context, data)).toEqual(initial)
    persisted(initial)
    await confirmSMS(page)
    await expect(panel(page).getByText(savedText, { exact: true })).toBeVisible()
  })
  const committed = await readPreferences(context, data)
  expect(committed.sms.allowed).toBe(!initial.sms.allowed)
  expect(committed.sms.version).toBe(initial.sms.version + 1)
  expect(committed.email).toEqual(initial.email)
  const firstRows = persisted(committed)
  await page.reload()
  await expect(smsSwitch(page)).toBeChecked({ checked: committed.sms.allowed })
  await expect(smsSwitch(page)).toBeEnabled()

  const failedWrites: Array<{ key: string; body: string | null }> = []
  await test.step('transport failure leaves MySQL unchanged and recovers with the original key', async () => {
    let fail = true
    const handler = async (route: Route) => {
      if (route.request().method() === 'POST') {
        failedWrites.push({ key: route.request().headers()['idempotency-key'] ?? '', body: route.request().postData() })
        if (fail) { fail = false; await route.abort('failed'); return }
      }
      await route.continue()
    }
    await page.route(routePattern, handler)
    await confirmSMS(page)
    await expect(dialog(page)).toContainText('提交结果尚不确定')
    expect(await readPreferences(context, data)).toEqual(committed)
    expect(persisted(committed)).toEqual(firstRows)
    await expect(smsSwitch(page)).toBeChecked({ checked: committed.sms.allowed })
    await dialog(page).getByRole('button', { name: '核对原请求结果' }).click()
    await expect(panel(page).getByText(savedText, { exact: true })).toBeVisible()
    expect(failedWrites).toHaveLength(2)
    expect(failedWrites[0]?.key).not.toBe('')
    expect(failedWrites[1]).toEqual(failedWrites[0])
    await page.unroute(routePattern, handler)
  })
  const recovered = await readPreferences(context, data)
  expect(recovered.sms.version).toBe(committed.sms.version + 1)
  expect(recovered.sms.allowed).toBe(initial.sms.allowed)
  persisted(recovered)

  await test.step('accepted write with failed readback recovers with GET only', async () => {
    await page.reload(); await expect(smsSwitch(page)).toBeEnabled()
    let failRead = false, writes = 0
    const handler = async (route: Route) => {
      if (route.request().method() === 'POST') { writes++; failRead = true }
      else if (failRead) { failRead = false; await route.abort('failed'); return }
      await route.continue()
    }
    await page.route(routePattern, handler)
    await confirmSMS(page)
    await expect(dialog(page)).toContainText('服务端已接收修改，但尚未读回确认')
    const accepted = await readPreferences(context, data)
    expect(accepted.sms.version).toBe(recovered.sms.version + 1)
    expect(accepted.sms.allowed).toBe(!recovered.sms.allowed)
    persisted(accepted)
    await expect(panel(page).getByText(savedText, { exact: true })).toHaveCount(0)
    await dialog(page).getByRole('button', { name: '核对原请求结果' }).click()
    await expect(panel(page).getByText(savedText, { exact: true })).toBeVisible()
    expect(writes).toBe(1)
    expect(await readPreferences(context, data)).toEqual(accepted)
    await page.unroute(routePattern, handler)
  })
  const final = await readPreferences(context, data)
  await selectTenant(context, data, data.iam_denied_tenant)
  expect(await readPreferences(context, data)).toEqual(tenantBefore)
  persisted(tenantBefore)
  await selectTenant(context, data, data.allowed_tenant)
  await openProfile(page, data)

  await test.step('four viewports preserve confirmation, keyboard focus and persisted values', async () => {
    for (const viewport of [{ width: 1366, height: 768 }, { width: 1440, height: 900 },
      { width: 1536, height: 1024 }, { width: 390, height: 844 }]) {
      await page.setViewportSize(viewport)
      await panel(page).scrollIntoViewIfNeeded()
      await expect(smsSwitch(page)).toBeChecked({ checked: final.sms.allowed })
      await page.screenshot({ path: `test-results/enterprise183-live-${viewport.width}.png`, fullPage: true })
      await smsSwitch(page).focus(); await smsSwitch(page).press('Space')
      await expect(dialog(page)).toBeVisible()
      await expect(dialog(page)).toContainText('当前企业')
      const bounds = await dialog(page).boundingBox()
      expect(bounds).not.toBeNull()
      expect(bounds!.x).toBeGreaterThanOrEqual(0)
      expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(viewport.width)
      await page.screenshot({ path: `test-results/enterprise183-live-confirm-${viewport.width}.png` })
      await page.keyboard.press('Escape')
      await expect(dialog(page)).toBeHidden()
      await expect(smsSwitch(page)).toBeFocused()
      await expect(smsSwitch(page)).toBeChecked({ checked: final.sms.allowed })
    }
    expect(await readPreferences(context, data)).toEqual(final)
    persisted(final)
  })

  const fresh = await browser.newContext()
  try {
    const freshPage = await fresh.newPage()
    await login(freshPage, data)
    await selectTenant(fresh, data, data.allowed_tenant)
    await openProfile(freshPage, data)
    await expect(smsSwitch(freshPage)).toBeChecked({ checked: final.sms.allowed })
    expect(await readPreferences(fresh, data)).toEqual(final)
  } finally { await fresh.close() }
  expect(errors).toEqual([])
  expect(await page.locator('vite-error-overlay').count()).toBe(0)
  writeFileSync('test-results/enterprise183-live-evidence.json', JSON.stringify({
    candidate_sha: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
    candidate_tree: execFileSync('git', ['rev-parse', 'HEAD^{tree}'], { encoding: 'utf8' }).trim(),
    test: test.info().title, real_idp_bff_mysql: true, mocked_success_responses: false,
    failure_injection: ['abort_before_backend', 'abort_readback_after_real_commit'],
    initial, committed, recovered, final, tenant_before: tenantBefore,
    persisted_rows: persisted(final), application_errors: errors,
  }, null, 2) + '\n')
})
