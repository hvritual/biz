import { expect, test, type BrowserContext, type Page } from '@playwright/test'
import { readFileSync } from 'node:fs'

interface Fixture {
  base_url: string
  ui_base_url: string
  email: string
  password: string
  brand_tenant_a: string
  brand_tenant_b: string
  brand_tenant_a_name: string
  brand_tenant_b_name: string
}

interface SessionView {
  authenticated: boolean
  actor_kind?: string
  user_id?: string
  active_tenant_id?: string
  context_version?: number
  csrf_token?: string
  tenants?: Array<{ id: string; name: string }>
}

function fixture(): Fixture {
  const path = process.env.CE12_E2E_ENV_FILE
  if (!path) throw new Error('CE12_E2E_ENV_FILE is required')
  return JSON.parse(readFileSync(path, 'utf8')) as Fixture
}

async function acceptPrivacyConsentIfRequired(page: Page) {
  const heading = page.getByRole('heading', { name: '确认隐私与服务协议' })
  if (await heading.isVisible()) {
    await page.getByLabel(/我已阅读并同意/).check()
    await page.getByRole('button', { name: '同意并继续' }).click()
  }
}

async function login(page: Page, data: Fixture) {
  await page.goto(data.base_url + '/auth/login?return_to=/auth/session')
  await page.getByLabel('账号 / 手机号 / 邮箱').fill(data.email)
  await page.getByLabel('密码').fill(data.password)
  await page.getByRole('button', { name: '登录' }).click()
  await acceptPrivacyConsentIfRequired(page)
  await expect(page).toHaveURL(data.base_url + '/auth/session')
}

async function readSession(context: BrowserContext, data: Fixture) {
  const response = await context.request.get(data.base_url + '/auth/session')
  expect(response.status()).toBe(200)
  return (await response.json()) as SessionView
}

async function selectServerTenant(context: BrowserContext, data: Fixture, tenantId: string) {
  const session = await readSession(context, data)
  expect(session.authenticated).toBe(true)
  expect(session.csrf_token).toBeTruthy()
  const response = await context.request.post(data.base_url + '/auth/session/tenant', {
    headers: { 'X-CSRF-Token': session.csrf_token! },
    data: { tenant_id: tenantId },
  })
  expect(response.status(), await response.text()).toBe(200)
  return (await response.json()) as SessionView
}

async function chooseTenant(page: Page, name: string) {
  const select = page.getByRole('combobox', { name: '切换企业' })
  await expect(select).toBeEnabled()
  await select.click()
  await page.getByRole('option', { name, exact: true }).click()
}

function expectedContext(session: SessionView) {
  return JSON.stringify({
    actor_kind: session.actor_kind ?? '',
    platform_subject: '',
    user_id: session.user_id ?? '',
    active_tenant_id: session.active_tenant_id ?? '',
    context_version: session.context_version ?? 0,
  })
}

test('TestEnterprise171CrossTabTenantIsolationBackNavigationAndStaleContext', async ({ browser }) => {
  const data = fixture()
  const context = await browser.newContext({ viewport: { width: 1366, height: 768 } })
  try {
    const loginPage = await context.newPage()
    await login(loginPage, data)
    const tenantA = await selectServerTenant(context, data, data.brand_tenant_a)
    expect(tenantA.active_tenant_id).toBe(data.brand_tenant_a)
    expect(tenantA.context_version).toBeGreaterThan(1)
    await loginPage.close()

    const tabA = await context.newPage()
    const tabB = await context.newPage()

    await tabA.goto(data.ui_base_url + '/#/enterprise/branding')
    await tabB.goto(data.ui_base_url + '/#/enterprise/branding')

    await expect(tabA.getByRole('combobox', { name: '切换企业' })).toContainText(data.brand_tenant_a_name)
    await expect(tabB.getByRole('combobox', { name: '切换企业' })).toContainText(data.brand_tenant_a_name)
    await expect(tabA.locator('.branding-scope')).toContainText(data.brand_tenant_a)
    await expect(tabB.locator('.branding-scope')).toContainText(data.brand_tenant_a)

    const staleContext = expectedContext(await readSession(context, data))

    // Keep a browser-history entry created under tenant A. The context signal
    // must make that history entry re-read tenant B when the user returns.
    await tabB.goto(data.ui_base_url + '/#/enterprise/company')
    await expect(tabB.getByRole('combobox', { name: '切换企业' })).toContainText(data.brand_tenant_a_name)

    await chooseTenant(tabA, data.brand_tenant_b_name)
    await expect(tabA.getByRole('combobox', { name: '切换企业' })).toContainText(data.brand_tenant_b_name)
    await expect(tabA.locator('.branding-scope')).toContainText(data.brand_tenant_b)

    // Tab B receives only a wake-up signal. It must re-read /auth/session and
    // converge on the server-selected tenant without trusting a tenant id from
    // BroadcastChannel/localStorage.
    await expect(tabB.getByRole('combobox', { name: '切换企业' })).toContainText(data.brand_tenant_b_name)

    const current = await readSession(context, data)
    expect(current.active_tenant_id).toBe(data.brand_tenant_b)
    expect(current.context_version).toBeGreaterThan(tenantA.context_version ?? 0)

    const stale = await context.request.get(data.base_url + '/v1/tenant/members', {
      headers: { 'X-Biz-Session-Context': staleContext },
    })
    expect(stale.status(), await stale.text()).toBe(409)

    await tabB.goBack()
    await expect(tabB).toHaveURL(/#\/enterprise\/branding/)
    await expect(tabB.getByRole('combobox', { name: '切换企业' })).toContainText(data.brand_tenant_b_name)
    await expect(tabB.locator('.branding-scope')).toContainText(data.brand_tenant_b)
    await expect(tabB.locator('html')).toHaveAttribute('data-ui-theme', 'emerald')

    // A forged tenant header is still not authority after the cross-tab switch.
    const forged = await context.request.get(data.base_url + '/v1/devices', {
      headers: {
        'X-Tenant-ID': data.brand_tenant_a,
        'X-Platform': 'true',
        'X-Biz-Session-Context': expectedContext(current),
      },
    })
    expect(forged.status()).not.toBe(200)

    await tabA.screenshot({ path: 'test-results/enterprise-171-tab-a-tenant-b.png', fullPage: true })
    await tabB.screenshot({ path: 'test-results/enterprise-171-tab-b-back-tenant-b.png', fullPage: true })
  } finally {
    await context.close()
  }
})
