import { expect, test, type Page } from '@playwright/test'
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
  active_tenant_id?: string
  csrf_token?: string
}

function fixture(): Fixture {
  const path = process.env.CE12_E2E_ENV_FILE
  if (!path) throw new Error('CE12_E2E_ENV_FILE is required')
  return JSON.parse(readFileSync(path, 'utf8')) as Fixture
}

async function browserRequest(
  page: Page,
  baseURL: string,
  path: string,
  init?: { method?: string; headers?: Record<string, string>; body?: unknown },
) {
  return page.evaluate(
    async ({ baseURL, path, init }) => {
      const headers = new Headers(init?.headers ?? {})
      if (init?.body !== undefined) headers.set('Content-Type', 'application/json')
      const response = await fetch(baseURL + path, {
        method: init?.method ?? 'GET',
        headers,
        credentials: 'include',
        body: init?.body === undefined ? undefined : JSON.stringify(init.body),
      })
      const text = await response.text()
      let json: unknown = null
      try { json = text ? JSON.parse(text) : null } catch { json = null }
      return { status: response.status, text, json }
    },
    { baseURL, path, init },
  )
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
  await expect(page.getByRole('heading', { name: 'CoffeeLink 登录' })).toBeVisible()
  await page.getByLabel('账号 / 手机号 / 邮箱', { exact: true }).fill(data.email)
  await page.getByLabel('密码').fill(data.password)
  await page.getByRole('button', { name: '登录' }).click()
  await acceptPrivacyConsentIfRequired(page)
  await expect(page).toHaveURL(data.base_url + '/auth/session')
}

async function selectServerTenant(page: Page, data: Fixture, tenantId: string) {
  const sessionResult = await browserRequest(page, data.base_url, '/auth/session')
  expect(sessionResult.status, sessionResult.text).toBe(200)
  const session = sessionResult.json as SessionView
  expect(session.authenticated).toBe(true)
  expect(session.csrf_token).toBeTruthy()
  const switched = await browserRequest(page, data.base_url, '/auth/session/tenant', {
    method: 'POST',
    headers: { 'X-CSRF-Token': session.csrf_token! },
    body: { tenant_id: tenantId },
  })
  expect(switched.status, switched.text).toBe(200)
}

async function chooseTenant(page: Page, name: string) {
  const select = page.getByRole('combobox', { name: '切换企业' })
  await expect(select).toBeEnabled()
  await select.click()
  await page.getByRole('option', { name, exact: true }).click()
}

test('TestCE12BrowserRealTenantBrandAppearanceAndStaleReadback', async ({ page }) => {
  const data = fixture()
  await login(page, data)
  await selectServerTenant(page, data, data.brand_tenant_a)

  await page.goto(data.ui_base_url + '/#/enterprise/branding')
  const root = page.locator('html')
  await expect(root).toHaveAttribute('data-ui-theme', 'violet')
  await expect(page.locator('.branding-scope')).toContainText(data.brand_tenant_a)

  await page.getByRole('button', { name: '切换为深色模式' }).click()
  await page.getByRole('button', { name: '切换为紧凑密度' }).click()
  await expect(root).toHaveAttribute('data-ui-mode', 'dark')
  await expect(root).toHaveAttribute('data-ui-density', 'compact')

  await chooseTenant(page, data.brand_tenant_b_name)
  await expect(root).toHaveAttribute('data-ui-theme', 'emerald')
  await expect(root).toHaveAttribute('data-ui-mode', 'dark')
  await expect(root).toHaveAttribute('data-ui-density', 'compact')
  await expect(page.locator('.branding-scope')).toContainText(data.brand_tenant_b)

  await chooseTenant(page, data.brand_tenant_a_name)
  await expect(root).toHaveAttribute('data-ui-theme', 'violet')
  await expect(page.locator('.branding-scope')).toContainText(data.brand_tenant_a)

  let releaseReadback: (() => void) | undefined
  let markReadbackStarted: (() => void) | undefined
  const readbackStarted = new Promise<void>((resolve) => { markReadbackStarted = resolve })
  let delayNextAReadback = true

  await page.route('**/api/v1/tenant/branding', async (route) => {
    if (route.request().method() !== 'GET' || !delayNextAReadback) {
      await route.continue()
      return
    }
    const response = await route.fetch()
    const body = await response.body()
    let tenantId = ''
    try { tenantId = String((JSON.parse(body.toString()) as { tenantId?: string }).tenantId ?? '') } catch { tenantId = '' }
    if (tenantId !== data.brand_tenant_a) {
      await route.fulfill({ response })
      return
    }
    delayNextAReadback = false
    markReadbackStarted?.()
    await new Promise<void>((resolve) => { releaseReadback = resolve })
    await route.fulfill({ status: response.status(), headers: response.headers(), body })
  })

  await page.getByRole('button', { name: 'Amber' }).click()
  await page.getByRole('button', { name: '保存主题' }).click()
  await readbackStarted

  // The server accepted A's write, but its authoritative GET readback is held.
  // Tenant switching remains a separate lifecycle action and must make B authoritative.
  await chooseTenant(page, data.brand_tenant_b_name)
  await expect(root).toHaveAttribute('data-ui-theme', 'emerald')
  await expect(root).toHaveAttribute('data-ui-mode', 'dark')
  await expect(root).toHaveAttribute('data-ui-density', 'compact')

  releaseReadback?.()
  await page.waitForTimeout(250)
  await expect(root).toHaveAttribute('data-ui-theme', 'emerald')
  await expect(page.locator('.branding-scope')).toContainText(data.brand_tenant_b)

  // Switching back proves the real A write exists on the server while the stale
  // A response never contaminated B's active theme.
  await chooseTenant(page, data.brand_tenant_a_name)
  await expect(root).toHaveAttribute('data-ui-theme', 'amber')
  await expect(root).toHaveAttribute('data-ui-mode', 'dark')
  await expect(root).toHaveAttribute('data-ui-density', 'compact')
  await page.reload()
  await expect(root).toHaveAttribute('data-ui-theme', 'amber')
  await expect(root).toHaveAttribute('data-ui-mode', 'dark')
  await expect(root).toHaveAttribute('data-ui-density', 'compact')
  expect(await page.evaluate(() => localStorage.getItem('coffeelink.ui-theme'))).toBeNull()

  await page.screenshot({ path: 'test-results/ce12-theme-appearance.png', fullPage: true })
})
