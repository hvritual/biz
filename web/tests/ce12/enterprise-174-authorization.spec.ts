import { expect, test, type Browser, type Page } from '@playwright/test'
import { readFileSync } from 'node:fs'

interface Fixture {
  base_url: string
  security_viewer_email: string
  security_viewer_password: string
  allowed_tenant: string
  iam_denied_tenant: string
}

interface SessionView {
  authenticated: boolean
  csrf_token?: string
  active_tenant_id?: string
}

interface ActionView {
  code: string
  permissions: string[]
  permission_mode: string
}

interface AuthorizationView {
  authenticated: boolean
  user_id?: string
  tenant_id?: string
  tenant_name?: string
  timezone?: string
  permission_version?: string
  grants: Array<{ permission: string; scope: string }>
  modules: Array<{ code: string; allowed: boolean; reason: string; actions: string[] }>
  actions: ActionView[]
  button_codes: string[]
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

async function login(browser: Browser, data: Fixture) {
  const context = await browser.newContext()
  const page = await context.newPage()
  await page.goto(data.base_url + '/auth/login?return_to=/auth/session')
  await page.getByLabel('账号 / 手机号 / 邮箱', { exact: true }).fill(data.security_viewer_email)
  await page.getByLabel('密码').fill(data.security_viewer_password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await acceptConsent(page)
  await expect(page).toHaveURL(data.base_url + '/auth/session')
  const sessionResponse = await context.request.get(data.base_url + '/auth/session')
  const session = (await sessionResponse.json()) as SessionView
  expect(session.authenticated).toBe(true)
  expect(session.active_tenant_id).toBe(data.allowed_tenant)
  return { context, page, session }
}

test('TestEnterprise174CurrentAuthorizationAndActionCatalogAreSameSource', async ({ browser }) => {
  const data = fixture()
  const active = await login(browser, data)
  try {
    const catalogResponse = await active.context.request.get(data.base_url + '/auth/action-catalog')
    expect(catalogResponse.status()).toBe(200)
    const catalog = (await catalogResponse.json()) as {
      schema_version: string
      permissions: Array<{ permission: string; groups: string[]; actions: string[] }>
      actions: ActionView[]
    }
    expect(catalog.schema_version).toBe('v1')
    expect(catalog.permissions.some((item) => item.permission === 'tenant.member.read')).toBe(true)
    expect(catalog.actions.some((item) => item.code === 'tenant.member.list')).toBe(true)

    const response = await active.context.request.get(data.base_url + '/auth/authorization', {
      headers: {
        'X-Tenant-ID': data.iam_denied_tenant,
      },
      params: {
        tenant_id: data.iam_denied_tenant,
      },
    })
    expect(response.status()).toBe(200)
    const authorization = (await response.json()) as AuthorizationView
    expect(authorization.authenticated).toBe(true)
    expect(authorization.tenant_id).toBe(data.allowed_tenant)
    expect(authorization.tenant_id).not.toBe(data.iam_denied_tenant)
    expect(authorization.timezone).toBeTruthy()
    expect(authorization.permission_version).toMatch(/^sha256:/)

    const grants = new Set(authorization.grants.map((grant) => grant.permission))
    expect(grants.has('tenant.member.read')).toBe(true)
    expect(grants.has('tenant.member.manage')).toBe(false)
    expect(grants.has('tenant.branding.read')).toBe(true)

    for (const action of authorization.actions) {
      if (action.permissions.length === 0) continue
      if (action.permission_mode === 'any') {
        expect(action.permissions.some((permission) => grants.has(permission))).toBe(true)
      } else {
        expect(action.permissions.every((permission) => grants.has(permission))).toBe(true)
      }
    }
    expect([...authorization.button_codes].sort()).toEqual(authorization.actions.map((action) => action.code).sort())
    const accessModule = authorization.modules.find((module) => module.code === 'access-management')
    expect(accessModule?.allowed).toBe(true)
    expect(accessModule?.actions).toContain('tenant.member.list')

    const denied = await active.context.request.post(data.base_url + '/v1/tenant/roles', {
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': active.session.csrf_token ?? '',
        'Idempotency-Key': 'enterprise-174-denied-role-create',
      },
      data: { name: 'must-not-create' },
      maxRedirects: 0,
    })
    expect(denied.status()).toBe(403)
  } finally {
    await active.context.close()
  }

  const anonymous = await browser.newContext()
  try {
    const response = await anonymous.request.get(data.base_url + '/v1/tenant/members')
    expect(response.status()).toBe(401)
  } finally {
    await anonymous.close()
  }
})
