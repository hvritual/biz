import { expect, test, type Browser, type BrowserContext, type Page } from '@playwright/test'
import { existsSync, readFileSync } from 'node:fs'

interface Fixture {
  base_url: string
  email: string
  password: string
  username: string
  phone: string
  single_email: string
  single_password: string
  single_username: string
  single_phone: string
  empty_email: string
  empty_password: string
  empty_username: string
  allowed_tenant: string
}

interface SessionView {
  authenticated: boolean
  user_id?: string
  active_tenant_id?: string
  csrf_token?: string
  tenants?: Array<{ id: string; name: string }>
}

interface QualificationNotification {
  event_id: string
  kind: string
  purpose: string
  channel: string
  destination: string
  secret: string
  attempt: number
}

function fixture(): Fixture {
  const path = process.env.CE12_E2E_ENV_FILE
  if (!path) throw new Error('CE12_E2E_ENV_FILE is required')
  return JSON.parse(readFileSync(path, 'utf8')) as Fixture
}

function otpFile() {
  const path = process.env.CE12_OTP_FILE
  if (!path) throw new Error('CE12_OTP_FILE is required')
  return path
}

function qualificationNotifications() {
  const path = otpFile()
  if (!existsSync(path)) return [] as QualificationNotification[]
  return readFileSync(path, 'utf8')
    .split('\n')
    .filter(Boolean)
    .map((line) => JSON.parse(line) as QualificationNotification)
}

async function acceptPrivacyConsentIfRequired(page: Page) {
  const heading = page.getByRole('heading', { name: '确认隐私与服务协议' })
  if (await heading.isVisible()) {
    await page.getByLabel(/我已阅读并同意/).check()
    await page.getByRole('button', { name: '同意并继续' }).click()
  }
}

async function readSession(context: BrowserContext, data: Fixture) {
  const response = await context.request.get(data.base_url + '/auth/session')
  expect(response.status()).toBe(200)
  return (await response.json()) as SessionView
}

async function startLogin(context: BrowserContext, data: Fixture) {
  const page = await context.newPage()
  await page.goto(data.base_url + '/auth/login?return_to=/auth/session')
  await expect(page.getByRole('heading', { name: 'CoffeeLink 登录' })).toBeVisible()
  return page
}

async function passwordLogin(
  browser: Browser,
  data: Fixture,
  identifier: string,
  password: string,
  remember = false,
) {
  const context = await browser.newContext()
  const page = await startLogin(context, data)
  await page.getByLabel('账号 / 手机号 / 邮箱', { exact: true }).fill(identifier)
  await page.getByLabel('密码').fill(password)
  if (remember) await page.getByLabel(/记住登录账号/).first().check()
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await acceptPrivacyConsentIfRequired(page)
  await expect(page).toHaveURL(data.base_url + '/auth/session')
  return { context, page, session: await readSession(context, data) }
}

async function otpLogin(
  browser: Browser,
  data: Fixture,
  identifier: string,
  expectedDestination: string,
  wrongFirst = false,
) {
  const context = await browser.newContext()
  const page = await startLogin(context, data)
  await page.getByRole('tab', { name: '验证码登录' }).click()
  await page.getByLabel('验证码账号 / 手机号 / 邮箱').fill(identifier)
  const before = qualificationNotifications().length
  await page.getByRole('button', { name: '发送验证码' }).click()
  await expect(page.getByRole('alert')).toContainText('验证码请求已受理')
  await expect(page.getByRole('button', { name: /秒后可重新发送/ })).toBeDisabled()

  let notification: QualificationNotification | undefined
  await expect.poll(() => {
    const messages = qualificationNotifications().slice(before)
    notification = messages.find(
      (candidate) =>
        candidate.kind === 'verification_code' &&
        candidate.purpose === 'login' &&
        candidate.destination === expectedDestination,
    )
    return Boolean(notification?.secret)
  }).toBe(true)

  if (!notification) throw new Error('qualification OTP evidence missing')
  if (wrongFirst) {
    const wrong = notification.secret === '000000' ? '111111' : '000000'
    await page.getByLabel('验证码').fill(wrong)
    await page.getByRole('button', { name: '验证码登录' }).click()
    await expect(page.getByRole('alert')).toContainText('账号或验证码错误')
  }
  await page.getByLabel('验证码').fill(notification.secret)
  await page.getByRole('button', { name: '验证码登录' }).click()
  await acceptPrivacyConsentIfRequired(page)
  await expect(page).toHaveURL(data.base_url + '/auth/session')
  return { context, page, session: await readSession(context, data) }
}

function expectMultiTenant(session: SessionView) {
  expect(session.authenticated).toBe(true)
  expect(session.active_tenant_id ?? '').toBe('')
  expect(session.tenants?.length ?? 0).toBeGreaterThan(1)
}

function expectSingleTenant(session: SessionView, tenantID: string) {
  expect(session.authenticated).toBe(true)
  expect(session.active_tenant_id).toBe(tenantID)
  expect(session.tenants?.map((tenant) => tenant.id)).toEqual([tenantID])
}

function expectNoTenant(session: SessionView) {
  expect(session.authenticated).toBe(true)
  expect(session.active_tenant_id ?? '').toBe('')
  expect(session.tenants ?? []).toHaveLength(0)
}

test('TestEnterprise172UsernamePhoneEmailPasswordOTPMatrix', async ({ browser }) => {
  test.setTimeout(120_000)
  const data = fixture()

  const passwordCases = [
    { identifier: data.username, password: data.password, mode: 'multi' },
    { identifier: data.phone, password: data.password, mode: 'multi' },
    { identifier: data.email, password: data.password, mode: 'multi' },
    { identifier: data.single_username, password: data.single_password, mode: 'single' },
    { identifier: data.single_phone, password: data.single_password, mode: 'single' },
    { identifier: data.single_email, password: data.single_password, mode: 'single' },
    { identifier: data.empty_username, password: data.empty_password, mode: 'empty' },
    { identifier: data.empty_email, password: data.empty_password, mode: 'empty' },
  ] as const

  for (const candidate of passwordCases) {
    const login = await passwordLogin(browser, data, candidate.identifier, candidate.password)
    try {
      if (candidate.mode === 'multi') expectMultiTenant(login.session)
      else if (candidate.mode === 'single') expectSingleTenant(login.session, data.allowed_tenant)
      else expectNoTenant(login.session)
    } finally {
      await login.context.close()
    }
  }

  // OTP representative matrix covers all three identifier types and all three
  // tenant cardinalities without weakening the server-side 60-second resend gate.
  const multiEmailOTP = await otpLogin(browser, data, data.email, data.email, true)
  try { expectMultiTenant(multiEmailOTP.session) } finally { await multiEmailOTP.context.close() }

  const multiPhoneOTP = await otpLogin(browser, data, data.phone, data.phone)
  try { expectMultiTenant(multiPhoneOTP.session) } finally { await multiPhoneOTP.context.close() }

  const singleUsernameOTP = await otpLogin(browser, data, data.single_username, data.single_email)
  try { expectSingleTenant(singleUsernameOTP.session, data.allowed_tenant) } finally { await singleUsernameOTP.context.close() }

  const emptyEmailOTP = await otpLogin(browser, data, data.empty_email, data.empty_email)
  try { expectNoTenant(emptyEmailOTP.session) } finally { await emptyEmailOTP.context.close() }
})

test('TestEnterprise172ClientValidationAntiEnumerationAndRememberIdentifier', async ({ browser }) => {
  const data = fixture()

  const validationContext = await browser.newContext()
  try {
    const page = await startLogin(validationContext, data)
    let loginPosts = 0
    page.on('request', (request) => {
      if (request.method() === 'POST' && request.url().endsWith('/idp/login')) loginPosts++
    })
    await page.getByLabel('账号 / 手机号 / 邮箱', { exact: true }).fill('12345')
    await page.getByLabel('密码').fill('not-sent')
    await page.getByRole('button', { name: '登录', exact: true }).click()
    await expect(page.getByRole('alert')).toContainText('请输入有效账号')
    expect(loginPosts).toBe(0)
  } finally {
    await validationContext.close()
  }

  const missingContext = await browser.newContext()
  try {
    const page = await startLogin(missingContext, data)
    await page.getByRole('tab', { name: '验证码登录' }).click()
    await page.getByLabel('验证码账号 / 手机号 / 邮箱').fill('ghostuser')
    await page.getByRole('button', { name: '发送验证码' }).click()
    await expect(page.getByRole('alert')).toHaveText('验证码请求已受理；若账号可用且渠道正常，将发送验证码。')
    await expect(page.locator('input[name="challenge_id"]')).toHaveValue(/vch-fake-/)
  } finally {
    await missingContext.close()
  }

  const remembered = await passwordLogin(browser, data, data.single_username, data.single_password, true)
  try {
    expectSingleTenant(remembered.session, data.allowed_tenant)
    const cookies = await remembered.context.cookies('http://127.0.0.1:18081/idp')
    const hint = cookies.find((cookie) => cookie.name === 'biz_login_hint')
    expect(hint).toBeTruthy()
    expect(hint?.value).not.toContain(data.single_password)

    const logout = await remembered.context.request.post(data.base_url + '/auth/logout', {
      headers: { 'X-CSRF-Token': remembered.session.csrf_token ?? '' },
      maxRedirects: 0,
    })
    expect(logout.status()).toBe(204)

    await remembered.page.goto(data.base_url + '/auth/login?return_to=/auth/session')
    await expect(remembered.page.getByLabel('账号 / 手机号 / 邮箱', { exact: true })).toHaveValue(data.single_username)
    const storage = await remembered.page.evaluate(() => ({ ...localStorage }))
    expect(JSON.stringify(storage)).not.toContain(data.single_password)
    expect(JSON.stringify(storage).toLowerCase()).not.toContain('token')
  } finally {
    await remembered.context.close()
  }
})
