import { expect, test, type BrowserContext, type Page } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { selectUiOption } from '../../e2e/ui.helpers'

interface SessionView {
  authenticated: boolean
  actor_kind?: string
  user_id?: string
  active_tenant_id?: string
  context_version?: number
  csrf_token?: string
}
interface NotificationFixture {
  email: string
  password: string
  tenant_a: string
  tenant_b: string
  site_id: string
  owner_id: string
}
interface Fixture {
  base_url: string
  ui_base_url: string
  email: string
  password: string
  security_viewer_email: string
  security_viewer_password: string
  allowed_tenant: string
  notification: NotificationFixture
}
interface RoleView {
  id: string
  name: string
  version: number | string
  permissions?: Array<{ permission: string; scope: string }>
}
interface ConfigurationView {
  id: string
  tenantId: string
  groupId: string
  level: string
  channels: string[]
  primaryUserId: string
}
interface InboxView {
  tenant_id: string
  user_id: string
  unread_count: number
  messages: Array<{ type_code: string; reference_id: string }>
}

function fixture(): Fixture {
  const path = process.env.CE12_E2E_ENV_FILE
  if (!path) throw new Error('CE12_E2E_ENV_FILE is required')
  const data = JSON.parse(readFileSync(path, 'utf8')) as Fixture
  if (!data.notification) throw new Error('notification fixture is required')
  for (const url of [data.base_url, data.ui_base_url]) {
    expect(new URL(url).hostname).toBe('127.0.0.1')
  }
  return data
}

async function acceptConsent(page: Page) {
  const heading = page.getByRole('heading', { name: '确认隐私与服务协议' })
  if (await heading.isVisible()) {
    await page.getByLabel(/我已阅读并同意/).check()
    await page.getByRole('button', { name: '同意并继续' }).click()
  }
}

async function login(page: Page, data: Fixture, identifier: string, password: string) {
  await page.goto(data.base_url + '/auth/login?return_to=/auth/session')
  await page.getByLabel('账号 / 手机号 / 邮箱', { exact: true }).fill(identifier)
  await page.getByLabel('密码').fill(password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await acceptConsent(page)
  await expect(page).toHaveURL(data.base_url + '/auth/session')
}

async function readSession(context: BrowserContext, data: Fixture): Promise<SessionView> {
  const response = await context.request.get(data.base_url + '/auth/session')
  expect(response.status()).toBe(200)
  return await response.json() as SessionView
}

function trustedContext(session: SessionView) {
  return JSON.stringify({
    actor_kind: session.actor_kind ?? '',
    platform_subject: '',
    user_id: session.user_id ?? '',
    active_tenant_id: session.active_tenant_id ?? '',
    context_version: session.context_version ?? 0,
  })
}

async function selectTenant(context: BrowserContext, data: Fixture, tenant: string) {
  const current = await readSession(context, data)
  expect(current.authenticated).toBe(true)
  const response = await context.request.post(data.base_url + '/auth/session/tenant', {
    headers: { 'X-CSRF-Token': current.csrf_token ?? '' },
    data: { tenant_id: tenant },
  })
  expect(response.status(), await response.text()).toBe(200)
}

async function listRoles(context: BrowserContext, data: Fixture): Promise<RoleView[]> {
  const current = await readSession(context, data)
  const response = await context.request.get(data.base_url + '/v1/tenant/roles', {
    headers: { 'X-Biz-Session-Context': trustedContext(current) },
  })
  expect(response.status(), await response.text()).toBe(200)
  const body = await response.json() as { roles?: RoleView[] }
  return body.roles ?? []
}

async function ensureViewerReadPermission(context: BrowserContext, data: Fixture) {
  const current = await readSession(context, data)
  if (!current.authenticated || current.active_tenant_id !== data.allowed_tenant) return
  const role = (await listRoles(context, data)).find((candidate) => candidate.name === 'Security Viewer')
  if (!role) throw new Error('Security Viewer role is missing')
  const permissions = role.permissions ?? []
  if (permissions.some((grant) => grant.permission === 'tenant.member.read')) return
  const response = await context.request.put(
    data.base_url + '/v1/tenant/roles/' + encodeURIComponent(role.id) + '/permissions',
    {
      headers: {
        'X-CSRF-Token': current.csrf_token ?? '',
        'X-Biz-Session-Context': trustedContext(current),
        'Idempotency-Key': 'enterprise189-restore-viewer-' + Date.now(),
      },
      data: {
        roleId: role.id,
        version: role.version,
        permissions: [
          ...permissions,
          { permission: 'tenant.member.read', scope: 'DATA_SCOPE_ALL' },
        ],
      },
    },
  )
  expect(response.status(), await response.text()).toBe(200)
}

async function listConfigurations(context: BrowserContext, data: Fixture): Promise<ConfigurationView[]> {
  const current = await readSession(context, data)
  const response = await context.request.get(
    data.base_url + '/v1/tenant/notification/configurations?page=1&page_size=100',
    { headers: { 'X-Biz-Session-Context': trustedContext(current) } },
  )
  expect(response.status(), await response.text()).toBe(200)
  const body = await response.json() as { items?: ConfigurationView[] }
  return body.items ?? []
}

async function ensureGeneralInAppConfiguration(page: Page, context: BrowserContext, data: Fixture) {
  const n = data.notification
  const existing = (await listConfigurations(context, data)).find((row) =>
    row.groupId === n.site_id &&
    row.level === 'general' &&
    row.channels.includes('in_app') &&
    row.primaryUserId === n.owner_id,
  )
  await page.goto(data.ui_base_url + '/#/system/notifications')
  const panel = page.locator('[data-enterprise-page="notification-settings"]')
  await expect(panel).toBeVisible()
  if (existing) return existing

  await panel.getByRole('button', { name: '新建配置', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '新建企业消息配置' })
  const group = dialog.getByRole('group', { name: '业务点位', exact: true })
  await group.getByRole('textbox').fill('103')
  await group.getByRole('button', { name: '查询', exact: true }).click()
  await expect(group.getByRole('combobox')).toBeEnabled()
  await selectUiOption(group.getByRole('combobox'), n.site_id)
  await dialog.getByRole('checkbox', { name: '一般', exact: true }).check()
  await dialog.getByRole('checkbox', { name: '站内消息', exact: true }).check()
  await selectUiOption(dialog.getByRole('combobox', { name: '第一联系人', exact: true }), n.owner_id)
  await dialog.getByRole('textbox', { name: '备注', exact: true }).fill('#189 full runtime controlled event')
  await dialog.getByRole('button', { name: '确认保存', exact: true }).click()
  await expect(panel.getByText('配置已保存，最新状态已确认。', { exact: true })).toBeVisible()

  const created = (await listConfigurations(context, data)).find((row) =>
    row.groupId === n.site_id &&
    row.level === 'general' &&
    row.channels.includes('in_app') &&
    row.primaryUserId === n.owner_id,
  )
  expect(created).toBeTruthy()
  return created!
}

async function readInbox(context: BrowserContext, data: Fixture): Promise<InboxView> {
  const current = await readSession(context, data)
  const response = await context.request.get(data.base_url + '/auth/personal/in-app-notifications', {
    headers: { 'X-Biz-Session-Context': trustedContext(current) },
  })
  expect(response.status(), await response.text()).toBe(200)
  return await response.json() as InboxView
}

function injectControlledEvent(data: Fixture, eventID: string) {
  execFileSync(
    'go',
    ['test', '-count=1', '-tags=integration', './integration', '-run', '^TestEnterprise189AppendControlledBusinessEvent$'],
    {
      cwd: resolve(process.cwd(), '..'),
      encoding: 'utf8',
      timeout: 45_000,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: {
        ...process.env,
        ENTERPRISE189_EVENT_TENANT: data.notification.tenant_a,
        ENTERPRISE189_EVENT_GROUP: data.notification.site_id,
        ENTERPRISE189_EVENT_ID: eventID,
        ENTERPRISE189_EVENT_REFERENCE: data.notification.site_id,
      },
    },
  )
}

function readRoutingEvidence(eventID: string) {
  if (!/^[A-Za-z0-9_.:-]+$/.test(eventID)) throw new Error('unsafe event id')
  const container = process.env.CE12_MYSQL_CONTAINER
  if (!container || !/^ce12-browser-\d+-\d+$/.test(container)) throw new Error('isolated CE12 MySQL container required')
  const sql = [
    "SELECT state,attempts,COALESCE(failure_code,'') FROM biz_notification_events WHERE event_id='" + eventID + "';",
    "SELECT COUNT(*),SUM(read_at IS NOT NULL) FROM biz_notification_in_app WHERE event_id='" + eventID + "';",
    "SELECT outcome,COUNT(*) FROM biz_notification_route_outcomes WHERE event_id='" + eventID + "' GROUP BY outcome ORDER BY outcome;",
  ].join(' ')
  return execFileSync(
    'docker',
    ['exec', container, 'mysql', '-uroot', '-proot', '--batch', '--skip-column-names', 'biz_ce12_browser', '-e', sql],
    { encoding: 'utf8', timeout: 10_000, stdio: ['ignore', 'pipe', 'pipe'] },
  ).trim()
}

async function logoutFromUI(page: Page, context: BrowserContext, data: Fixture) {
  await page.getByRole('button', { name: '当前账号', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '当前账号' })
  await dialog.getByRole('button', { name: '退出登录', exact: true }).click()
  await expect.poll(async () => (await readSession(context, data)).authenticated).toBe(false)
}

test('TestEnterprise189RealIdentityRoleRevocationPersonalAndLogout', async ({ browser }) => {
  test.setTimeout(120_000)
  const data = fixture()
  const ownerContext = await browser.newContext({ locale: 'zh-CN', timezoneId: 'Asia/Shanghai' })
  const viewerContext = await browser.newContext({ locale: 'zh-CN', timezoneId: 'Asia/Shanghai' })
  const ownerPage = await ownerContext.newPage()
  const viewerPage = await viewerContext.newPage()
  const pageErrors: string[] = []
  ownerPage.on('pageerror', (error) => pageErrors.push(error.message))

  let ownerLoggedOut = false
  try {
    await login(viewerPage, data, data.security_viewer_email, data.security_viewer_password)
    await viewerPage.goto(data.ui_base_url + '/#/enterprise/members')
    await expect(viewerPage.locator('[data-enterprise-page="members"]')).toBeVisible()

    await login(ownerPage, data, data.email, data.password)
    await selectTenant(ownerContext, data, data.allowed_tenant)

    for (const viewport of [
      { width: 1366, height: 768 },
      { width: 1440, height: 900 },
      { width: 1536, height: 1024 },
      { width: 390, height: 844 },
    ]) {
      await ownerPage.setViewportSize(viewport)
      await ownerPage.goto(data.ui_base_url + '/#/enterprise/members')
      await expect(ownerPage.locator('[data-enterprise-page="members"]')).toBeVisible()
      expect(await ownerPage.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
      await ownerPage.screenshot({
        path: 'test-results/enterprise189-members-' + viewport.width + '.png',
        fullPage: true,
      })
    }

    await ownerPage.setViewportSize({ width: 1366, height: 768 })
    await ownerPage.goto(data.ui_base_url + '/#/enterprise/roles')
    await expect(ownerPage.locator('[data-enterprise-page="roles"]')).toBeVisible()
    const roleRow = ownerPage.locator('tbody tr').filter({ hasText: 'Security Viewer' })
    await expect(roleRow).toBeVisible()
    await roleRow.getByRole('button', { name: '编辑', exact: true }).click()
    const roleDialog = ownerPage.getByRole('dialog', { name: '编辑角色权限' })
    const memberRead = roleDialog.locator('[data-role-permission-leaf="tenant.member.read"] input')
    await expect(memberRead).toBeChecked()
    await memberRead.uncheck()
    await roleDialog.getByRole('button', { name: '保存角色' }).click()
    await expect(ownerPage.getByRole('status')).toContainText('角色配置已保存并更新。')

    await viewerPage.goto(data.ui_base_url + '/#/enterprise/members')
    await expect(viewerPage.locator('[data-authorization-state]')).toBeVisible()
    await expect(viewerPage.getByRole('heading', { name: '没有访问权限', exact: true })).toBeVisible()
    await expect(viewerPage.locator('[data-enterprise-page="members"]')).toHaveCount(0)

    await ensureViewerReadPermission(ownerContext, data)
    await viewerPage.goto(data.ui_base_url + '/#/enterprise/members')
    await expect(viewerPage.locator('[data-enterprise-page="members"]')).toBeVisible()

    await ownerPage.goto(data.ui_base_url + '/#/enterprise/personal-profile')
    await expect(ownerPage.locator('[data-enterprise-page="personal-profile"]')).toBeVisible()
    const preferences = ownerPage.locator('[data-ui-region="notification-preferences"]')
    await expect(preferences).toBeVisible()
    const sms = preferences.getByRole('switch', { name: '短信通知', exact: true })
    await expect(sms).toBeEnabled()
    await sms.click()
    const preferenceDialog = ownerPage.getByRole('dialog', { name: '确认修改通知偏好' })
    await expect(preferenceDialog).toBeVisible()
    await preferenceDialog.getByRole('button', { name: '取消修改', exact: true }).click()
    await expect(sms).toBeFocused()

    await logoutFromUI(ownerPage, ownerContext, data)
    ownerLoggedOut = true
    expect(pageErrors).toEqual([])
  } finally {
    if (!ownerLoggedOut) {
      try { await ensureViewerReadPermission(ownerContext, data) } catch { /* retain original assertion failure */ }
    }
    await ownerContext.close()
    await viewerContext.close()
  }
})

test('TestEnterprise189ControlledEventRoutesToUnreadAndMarkAllRead', async ({ browser }) => {
  test.setTimeout(120_000)
  const data = fixture()
  const context = await browser.newContext({ locale: 'zh-CN', timezoneId: 'Asia/Shanghai' })
  const page = await context.newPage()
  const pageErrors: string[] = []
  page.on('pageerror', (error) => pageErrors.push(error.message))

  try {
    await login(page, data, data.notification.email, data.notification.password)
    await selectTenant(context, data, data.notification.tenant_a)
    const configuration = await ensureGeneralInAppConfiguration(page, context, data)

    const eventID = 'enterprise189-' + Date.now().toString(36)
    const before = await readInbox(context, data)
    injectControlledEvent(data, eventID)

    await expect.poll(async () => (await readInbox(context, data)).unread_count, { timeout: 20_000 })
      .toBeGreaterThan(before.unread_count)

    const afterRoute = await readInbox(context, data)
    expect(afterRoute.messages.some((message) =>
      message.type_code === 'system.announcement' &&
      message.reference_id === data.notification.site_id,
    )).toBe(true)

    await page.goto(data.ui_base_url + '/#/system/notifications')
    for (const viewport of [
      { width: 1366, height: 768 },
      { width: 1440, height: 900 },
      { width: 1536, height: 1024 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport)
      const bell = page.locator('button.notification')
      await expect(bell).toBeVisible()
      await bell.click()
      const inbox = page.locator('[data-notification-inbox]')
      await expect(inbox).toBeVisible()
      await expect(inbox).toContainText('企业系统通知')
      await expect(inbox).toContainText(data.notification.site_id)
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
      await page.screenshot({
        path: 'test-results/enterprise189-inbox-' + viewport.width + '.png',
        fullPage: viewport.width >= 768,
      })
      await page.keyboard.press('Escape')
      await expect(inbox).toBeHidden()
      await expect(bell).toBeFocused()
    }

    await page.setViewportSize({ width: 1366, height: 768 })
    const bell = page.locator('button.notification')
    await bell.click()
    const inbox = page.locator('[data-notification-inbox]')
    await inbox.getByRole('button', { name: '全部已读', exact: true }).click()
    await expect(inbox.getByText('暂无未读消息', { exact: true })).toBeVisible()
    await expect.poll(async () => (await readInbox(context, data)).unread_count).toBe(0)

    const routingEvidence = readRoutingEvidence(eventID)
    expect(routingEvidence).toContain('ROUTED')
    expect(routingEvidence).toContain('IN_APP_CREATED')

    await page.keyboard.press('Escape')
    await selectUiOption(page.getByRole('combobox', { name: '语言' }), 'en-US')
    await bell.click()
    await expect(page.locator('[data-notification-inbox]')).toContainText('No unread notifications')
    await page.keyboard.press('Escape')
    await selectUiOption(page.getByRole('combobox', { name: 'Language' }), 'zh-CN')

    writeFileSync('test-results/enterprise189-full-runtime-evidence.json', JSON.stringify({
      candidate_sha: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
      candidate_tree: execFileSync('git', ['rev-parse', 'HEAD^{tree}'], { encoding: 'utf8' }).trim(),
      source_kind: 'controlled_fixture_to_real_runtime',
      source_boundary: 'qualification-only BusinessEventPublisher fixture; no production event fabrication endpoint',
      event_id: eventID,
      configuration,
      unread_before: before.unread_count,
      unread_after_route: afterRoute.unread_count,
      unread_after_mark_all: (await readInbox(context, data)).unread_count,
      mysql_routing_evidence: routingEvidence,
      application_errors: pageErrors,
    }, null, 2) + '\n')

    expect(pageErrors).toEqual([])
    await logoutFromUI(page, context, data)
  } finally {
    await context.close()
  }
})
