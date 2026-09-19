import { expect, test, type Browser, type Page } from '@playwright/test'
import { readFileSync } from 'node:fs'

interface Fixture {
  base_url: string
  ui_base_url: string
  security_viewer_email: string
  security_viewer_password: string
  allowed_tenant: string
}

function fixture(): Fixture {
  const path = process.env.CE12_E2E_ENV_FILE
  if (!path) throw new Error('CE12_E2E_ENV_FILE is required')
  return JSON.parse(readFileSync(path, 'utf8')) as Fixture
}

async function acceptConsent(page: Page) {
  const heading = page.getByRole('heading', { name: '确认隐私与服务协议' })
  if (await heading.isVisible()) {
    await page.getByLabel(/我已阅读并同意/).check()
    await page.getByRole('button', { name: '同意并继续' }).click()
  }
}

async function loginViewer(browser: Browser, data: Fixture) {
  const context = await browser.newContext()
  const page = await context.newPage()
  await page.goto(data.base_url + '/auth/login?return_to=/auth/session')
  await page.getByLabel('账号 / 手机号 / 邮箱', { exact: true }).fill(data.security_viewer_email)
  await page.getByLabel('密码').fill(data.security_viewer_password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await acceptConsent(page)
  await expect(page).toHaveURL(data.base_url + '/auth/session')
  return { context, page }
}

test('TestEnterprise175AuthorizedNavigationButtonsAndDeepLinksFailClosed', async ({ browser }) => {
  const data = fixture()
  const active = await loginViewer(browser, data)
  const roleRequests: string[] = []
  active.page.on('request', (request) => {
    if (new URL(request.url()).pathname.startsWith('/v1/tenant/roles')) roleRequests.push(request.url())
  })

  try {
    await active.page.goto(data.ui_base_url + '/#/enterprise/members?action=create')
    await expect(active.page.locator('[data-enterprise-page="members"]')).toBeVisible()
    await expect(active.page.getByRole('dialog')).toHaveCount(0)

    await active.page.locator('[data-module-id="enterprise"]').click()
    const drawer = active.page.locator('#module-drawer')
    await expect(drawer).toBeVisible()
    await expect(drawer.getByText('成员管理', { exact: true })).toBeVisible()
    await expect(drawer.getByText('角色权限', { exact: true })).toHaveCount(0)
    await expect(drawer.getByText('新建角色', { exact: true })).toHaveCount(0)

    await active.page.goto(data.ui_base_url + '/#/enterprise/roles')
    await expect(active.page.locator('[data-authorization-state]')).toBeVisible()
    await expect(active.page.getByRole('heading', { name: '没有访问权限' })).toBeVisible()
    await expect.poll(() => roleRequests.length).toBe(0)

    const denied = await active.context.request.post(data.base_url + '/v1/tenant/roles', {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': 'enterprise-175-direct-role-create',
      },
      data: { name: 'must-not-create' },
      maxRedirects: 0,
    })
    expect(denied.status()).toBe(403)
  } finally {
    await active.context.close()
  }
})
