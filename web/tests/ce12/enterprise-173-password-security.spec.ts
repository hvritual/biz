import { expect, test, type Browser, type Page } from '@playwright/test'
import { existsSync, readFileSync } from 'node:fs'

interface Fixture {
  base_url: string
  ui_base_url: string
  email: string
  password: string
  security_change_email: string
  security_change_password: string
  security_recovery_email: string
  security_recovery_password: string
  security_admin_email: string
  security_admin_password: string
  security_viewer_email: string
  security_viewer_password: string
  shared_target_user_id: string
  cross_tenant_target_user_id: string
}

interface SessionView {
  authenticated: boolean
  csrf_token?: string
  active_tenant_id?: string
}

interface QualificationNotification {
  kind: string
  purpose: string
  destination: string
  secret: string
}

function fixture(): Fixture {
  const path = process.env.CE12_E2E_ENV_FILE
  if (!path) throw new Error('CE12_E2E_ENV_FILE is required')
  return JSON.parse(readFileSync(path, 'utf8')) as Fixture
}

function notifications() {
  const path = process.env.CE12_OTP_FILE
  if (!path || !existsSync(path)) return [] as QualificationNotification[]
  return readFileSync(path, 'utf8')
    .split('\n')
    .filter(Boolean)
    .map((line) => JSON.parse(line) as QualificationNotification)
}

async function acceptConsent(page: Page) {
  const heading = page.getByRole('heading', { name: '确认隐私与服务协议' })
  if (await heading.isVisible()) {
    await page.getByLabel(/我已阅读并同意/).check()
    await page.getByRole('button', { name: '同意并继续' }).click()
  }
}

async function login(browser: Browser, data: Fixture, identifier: string, password: string) {
  const context = await browser.newContext()
  const page = await context.newPage()
  await page.goto(data.base_url + '/auth/login?return_to=/auth/session')
  await page.getByLabel('账号 / 手机号 / 邮箱', { exact: true }).fill(identifier)
  await page.getByLabel('密码').fill(password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await acceptConsent(page)
  await expect(page).toHaveURL(data.base_url + '/auth/session')
  const response = await context.request.get(data.base_url + '/auth/session')
  const session = (await response.json()) as SessionView
  expect(session.authenticated).toBe(true)
  return { context, page, session }
}

async function expectPasswordRejected(browser: Browser, data: Fixture, identifier: string, password: string) {
  const context = await browser.newContext()
  try {
    const page = await context.newPage()
    await page.goto(data.base_url + '/auth/login?return_to=/auth/session')
    await page.getByLabel('账号 / 手机号 / 邮箱', { exact: true }).fill(identifier)
    await page.getByLabel('密码').fill(password)
    await page.getByRole('button', { name: '登录', exact: true }).click()
    await expect(page.getByRole('alert')).toContainText('账号或密码错误')
  } finally {
    await context.close()
  }
}

test('TestEnterprise173PasswordRecoveryRevokesSessionsAndChangesCredential', async ({ browser }) => {
  test.setTimeout(90_000)
  const data = fixture()
  const oldLogin = await login(browser, data, data.security_recovery_email, data.security_recovery_password)

  const recoveryContext = await browser.newContext()
  try {
    const page = await recoveryContext.newPage()
    await page.goto(data.base_url + '/auth/login?return_to=/auth/session')
    await page.getByRole('link', { name: '忘记密码' }).click()
    await expect(page.getByRole('heading', { name: '找回密码' })).toBeVisible()
    await page.getByLabel('找回密码账号 / 手机号 / 邮箱').fill(data.security_recovery_email)

    const before = notifications().length
    await page.getByRole('button', { name: '发送验证码' }).click()
    await expect(page.getByRole('alert')).toContainText('找回请求已受理')

    let code = ''
    await expect.poll(() => {
      const message = notifications()
        .slice(before)
        .find((candidate) =>
          candidate.kind === 'verification_code' &&
          candidate.purpose === 'password_recovery' &&
          candidate.destination === data.security_recovery_email)
      code = message?.secret ?? ''
      return code.length
    }).toBe(6)

    await page.getByLabel('找回密码验证码').fill(code)
    await page.getByLabel('找回密码新密码').fill('weakpass')
    await page.getByLabel('找回密码确认新密码').fill('weakpass')
    await page.getByRole('button', { name: '确认修改密码' }).click()
    await expect(page.getByRole('alert')).toContainText('8–16')

    await page.getByLabel('找回密码新密码').fill('RecoverNew9A')
    await page.getByLabel('找回密码确认新密码').fill('RecoverNew9A')
    await page.getByRole('button', { name: '确认修改密码' }).click()
    await expect(page.getByRole('alert')).toContainText('密码已更新')

    const oldSession = await oldLogin.context.request.get(data.base_url + '/auth/session')
    expect(((await oldSession.json()) as SessionView).authenticated).toBe(false)
  } finally {
    await recoveryContext.close()
    await oldLogin.context.close()
  }

  await expectPasswordRejected(browser, data, data.security_recovery_email, data.security_recovery_password)
  const relogin = await login(browser, data, data.security_recovery_email, 'RecoverNew9A')
  await relogin.context.close()
})

test('TestEnterprise173SelfPasswordChangeUIRevokesCurrentSession', async ({ browser }) => {
  const data = fixture()
  const active = await login(browser, data, data.security_change_email, data.security_change_password)
  try {
    await active.page.goto(data.ui_base_url + '/#/system/security')
    await expect(active.page.getByRole('heading', { name: '系统设置' }).or(active.page.getByText('修改我的密码'))).toBeVisible()
    await active.page.getByLabel('当前密码').fill(data.security_change_password)
    await active.page.getByLabel('新密码', { exact: true }).fill('ChangeNew9A')
    await active.page.getByLabel('确认新密码').fill('ChangeNew9A')
    await active.page.getByRole('button', { name: '修改密码' }).click()
    await expect(active.page.getByRole('status')).toContainText('旧会话已全部撤销')

    const sessionResponse = await active.context.request.get(data.base_url + '/auth/session')
    expect(((await sessionResponse.json()) as SessionView).authenticated).toBe(false)
  } finally {
    await active.context.close()
  }

  await expectPasswordRejected(browser, data, data.security_change_email, data.security_change_password)
  const relogin = await login(browser, data, data.security_change_email, 'ChangeNew9A')
  await relogin.context.close()
})

test('TestEnterprise173AdminRecoveryIsPermissionedScopedAndPolicyBlocked', async ({ browser }) => {
  const data = fixture()

  const viewer = await login(browser, data, data.security_viewer_email, data.security_viewer_password)
  try {
    const denied = await viewer.context.request.post(
      data.base_url + '/auth/tenant/members/' + encodeURIComponent(data.shared_target_user_id) + '/password-recovery',
      { headers: { 'X-CSRF-Token': viewer.session.csrf_token ?? '' }, maxRedirects: 0 },
    )
    expect(denied.status()).toBe(403)
  } finally {
    await viewer.context.close()
  }

  const admin = await login(browser, data, data.security_admin_email, data.security_admin_password)
  try {
    const pending = await admin.context.request.post(
      data.base_url + '/auth/tenant/members/' + encodeURIComponent(data.shared_target_user_id) + '/password-recovery',
      { headers: { 'X-CSRF-Token': admin.session.csrf_token ?? '' }, maxRedirects: 0 },
    )
    expect(pending.status()).toBe(409)
    const payload = await pending.json()
    expect(payload.status).toBe('POLICY_PENDING')
    expect(payload.policy).toBe('Q-007')
    expect(payload).not.toHaveProperty('temporary_password')
    expect(payload).not.toHaveProperty('new_password')
    expect(payload).not.toHaveProperty('credential')
    expect(payload).not.toHaveProperty('secret')

    const crossTenant = await admin.context.request.post(
      data.base_url + '/auth/tenant/members/' + encodeURIComponent(data.cross_tenant_target_user_id) + '/password-recovery',
      { headers: { 'X-CSRF-Token': admin.session.csrf_token ?? '' }, maxRedirects: 0 },
    )
    expect(crossTenant.status()).toBe(404)
  } finally {
    await admin.context.close()
  }

  const shared = await login(browser, data, data.email, data.password)
  await shared.context.close()
})
