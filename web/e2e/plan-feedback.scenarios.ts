import { expect, test, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'

type Options = { subscriptionStatus?: number; usageStatus?: number; memberUsed?: number }
type InstallPlanFixture = (page: Page, options?: Options) => Promise<unknown>
const readCodes = ['commercial.subscription.get_my', 'commercial.subscription.get_my_usage', 'commercial.entitlement.get_my']
const changeCode = 'commercial.subscription.change.targets_my'
const viewports = [{ width: 1366, height: 768 }, { width: 1440, height: 900 }, { width: 1536, height: 1024 }, { width: 390, height: 844 }]

async function denyChange(page: Page, kind: 'iam' | 'entitlement' | 'unknown') {
  await page.route('**/api/auth/authorization', (route) => route.fulfill({
    status: 200, contentType: 'application/json', body: JSON.stringify({
      authenticated: true, actor_kind: 'tenant', user_id: 'user-001', tenant_id: 'tenant-001',
      tenant_name: 'CoffeeLink 测试租户', timezone: 'Asia/Shanghai', roles: ['operator'],
      grants: kind === 'iam' ? [] : ['tenant.subscription.manage', 'commercial.catalog.read'].map((permission) => ({ permission, role_id: 'role-1', role_name: 'Operator', scope: 'all' })),
      data_policies: [], site_ids: [], permission_version: `plan-feedback-${kind}`,
      modules: [
        { code: 'access-management', allowed: true, reason: 'allowed', actions: readCodes },
        ...(kind === 'entitlement' ? [{ code: 'plan-change-fixture', allowed: false, reason: 'restricted', actions: [changeCode] }] : []),
      ],
      actions: readCodes.map((code) => ({ code, permissions: [], permission_mode: 'all' })),
      button_codes: readCodes,
    }),
  }))
}

async function capture(page: Page, name: string) {
  mkdirSync('screenshots/plan-feedback', { recursive: true })
  for (const viewport of viewports) {
    await page.setViewportSize(viewport)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({ path: `screenshots/plan-feedback/${name}-${viewport.width}.png`, fullPage: false })
  }
}

// Registered by the existing API-mode spec so the canonical harness actually
// executes these scenarios. These are controlled API fixtures, not live payment
// or production entitlement evidence. No workflow or authorization gate changes.
export function registerPlanFeedbackScenarios(install: InstallPlanFixture) {
  test('plan feedback 21: missing member permission is not an upsell and remains readable in four viewports', async ({ page }) => {
    await install(page)
    await denyChange(page, 'iam')
    await page.goto('/#/enterprise/plan')
    const feedback = page.locator('[data-plan-access-state][data-reason="iam"]')
    await expect(feedback).toContainText('当前成员没有管理套餐的权限')
    await expect(feedback).toContainText('购买套餐不会自动增加成员权限')
    await expect(page.getByRole('button', { name: '管理套餐变更', exact: true })).toBeDisabled()
    await expect(page.locator('[data-plan-context]')).toContainText('CoffeeLink 测试租户')
    await capture(page, '21-member-permission')
    await page.evaluate(() => localStorage.setItem('coffeelink.locale', 'en-US'))
    await page.reload()
    await expect(feedback).toContainText('buying a plan does not grant member permissions')
  })

  test('plan feedback 21: an explicit linked capability restriction is different from missing member permission', async ({ page }) => {
    await install(page)
    await denyChange(page, 'entitlement')
    await page.goto('/#/enterprise/plan')
    const feedback = page.locator('[data-plan-access-state][data-reason="entitlement"]')
    await expect(feedback).toContainText('修改成员角色不能解除套餐限制')
    await expect(page.getByRole('button', { name: '管理套餐变更', exact: true })).toBeDisabled()
    await feedback.getByRole('button', { name: '查看功能权益' }).click()
    await expect(page.getByRole('button', { name: '功能权益', exact: true })).toHaveClass(/active/)
    await capture(page, '21-capability-restriction')
  })

  test('plan feedback 21: an unexplained denied operation stays unknown', async ({ page }) => {
    await install(page)
    await denyChange(page, 'unknown')
    await page.goto('/#/enterprise/plan')
    await expect(page.locator('[data-plan-access-state][data-reason="unknown"]')).toContainText('暂时无法确定此操作的限制原因')
    await expect(page.getByRole('button', { name: '管理套餐变更', exact: true })).toBeDisabled()
    await expect(page.locator('[data-plan-access-state][data-reason="iam"]')).toHaveCount(0)
  })

  test('plan feedback 22: initial read failure retries the same company once and never issues a plan change', async ({ page }) => {
    const options: Options = { subscriptionStatus: 503 }
    await install(page, options)
    const commands: string[] = []
    page.on('request', (request) => {
      if (request.method() === 'POST' && /change-previews|\/confirm|\/orders|\/purchase/.test(request.url())) commands.push(request.url())
    })
    await page.goto('/#/enterprise/plan')
    const feedback = page.locator('[data-plan-read-feedback]')
    await expect(feedback).toContainText('套餐与权益暂不可用')
    await expect(page.locator('[data-plan-context]')).toContainText('CoffeeLink 测试租户')
    await expect(page.getByText('标准版', { exact: true })).toHaveCount(0)
    await capture(page, '22-read-failed')
    options.subscriptionStatus = 0
    let reads = 0
    let release!: () => void
    const wait = new Promise<void>((resolve) => { release = resolve })
    await page.route('**/api/v1/tenant/subscription', async (route) => {
      reads += 1
      await wait
      await route.fallback()
    })
    const retry = feedback.getByRole('button', { name: '重新读取', exact: true })
    await retry.click()
    await expect(feedback.getByRole('button', { name: '正在读取…', exact: true })).toBeDisabled()
    await feedback.getByRole('button').dispatchEvent('click')
    await expect.poll(() => reads).toBe(1)
    release()
    await expect(feedback).toHaveCount(0)
    await expect(page.locator('.current-plan h2')).toContainText('租赁成长版')
    expect(reads).toBe(1)
    expect(commands).toEqual([])
  })

  test('plan feedback 22: a failed refresh retains marked old data and tab context but blocks new changes', async ({ page }) => {
    const options: Options = {}
    await install(page, options)
    await page.goto('/#/enterprise/plan')
    await expect(page.getByRole('button', { name: '管理套餐变更', exact: true })).toBeEnabled()
    await page.getByRole('button', { name: '使用额度', exact: true }).click()
    options.subscriptionStatus = 503
    await page.locator('[data-plan-context]').getByRole('button', { name: '重新读取' }).click()
    const feedback = page.locator('[data-plan-read-feedback]')
    await expect(feedback).toContainText('下方保留上次读取的资料，仅供参考')
    await expect(page.locator('.current-plan h2')).toContainText('租赁成长版')
    await expect(page.getByRole('button', { name: '管理套餐变更', exact: true })).toBeDisabled()
    await expect(page.getByRole('button', { name: '使用额度', exact: true })).toHaveClass(/active/)
    await capture(page, '22-stale-read')
    options.subscriptionStatus = 0
    await feedback.getByRole('button', { name: '重新读取' }).click()
    await expect(feedback).toHaveCount(0)
    await expect(page.getByRole('button', { name: '管理套餐变更', exact: true })).toBeEnabled()
    await expect(page.getByRole('button', { name: '使用额度', exact: true })).toHaveClass(/active/)
  })

  test('plan feedback 22: usage-only failure retains limits and recovers unknown usage through a read', async ({ page }) => {
    const options: Options = { usageStatus: 503 }
    await install(page, options)
    await page.goto('/#/enterprise/plan')
    const feedback = page.locator('[data-plan-read-feedback][data-issue="usage"]')
    await expect(feedback).toContainText('当前用量未知')
    await page.getByRole('button', { name: '使用额度', exact: true }).click()
    const members = page.getByRole('row').filter({ hasText: '成员额度' })
    await expect(members).toContainText('未知')
    options.usageStatus = 0
    await feedback.getByRole('button', { name: '重新读取' }).click()
    await expect(feedback).toHaveCount(0)
    await expect(members).toContainText('100.0%')
  })
}
