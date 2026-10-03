import { expect, test, type Page, type Route } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { selectUiOption } from './ui.helpers'

const modules = {
  modules: [
    {
      moduleCode: 'customer',
      name: '客户管理',
      category: 'business',
      salesScope: ['default'],
      technicalStatus: 'MODULE_TECHNICAL_STATUS_READY',
      salesStatus: 'MODULE_SALES_STATUS_SELLABLE',
      capabilityCodes: ['customer.read', 'customer.manage'],
      quotaSchemaKeys: ['customer.count'],
      fieldPolicySchemaKeys: ['customer.phone'],
      dependencies: [],
      version: '1',
    },
  ],
}

const published = {
  planCode: 'office-pro',
  version: '2',
  revision: '3',
  planRevision: '5',
  state: 'PUBLISHED',
  name: '办公室专业版',
  terms: {
    modules: [
      {
        moduleCode: 'customer',
        capabilityCodes: ['customer.read'],
        quotas: [{ key: 'customer.count', unlimited: false, value: '50' }],
        fields: [{ key: 'customer.phone', action: 'read', mode: 'masked' }],
      },
    ],
    salesScope: ['default'],
    validityMode: 'unlimited',
    validityDays: 0,
    priceRef: 'price:office-pro',
  },
  contentSha256: '0123456789abcdef',
  createdAt: '2026-09-12T00:00:00Z',
  publishedAt: '2026-09-12T00:05:00Z',
  retiredAt: '',
  actorId: 'platform-admin',
  reason: 'publish for CE-13 browser proof',
}

const draft = {
  ...published,
  version: '3',
  revision: '1',
  planRevision: '6',
  state: 'DRAFT',
  contentSha256: '',
  publishedAt: '',
  retiredAt: '',
  reason: 'prepare office pro v3',
}

const retired = {
  ...published,
  revision: '4',
  state: 'RETIRED',
  retiredAt: '2026-09-20T08:00:00Z',
  reason: 'retire office pro v2',
}

async function mockModules(page: Page) {
  await page.route('**/api/v1/platform/modules', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(modules),
  }))
}

async function fulfillJson(route: Route, body: unknown, status = 200) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

test('TestCE13PlanWorkspaceReadsGeneratedPlanContract', async ({ page }) => {
  await mockModules(page)
  let requestedURL = ''
  await page.route('**/api/v1/platform/plans/office-pro/versions*', async (route) => {
    requestedURL = route.request().url()
    await fulfillJson(route, { versions: [published], nextAfterVersion: '' })
  })

  await page.goto('/#/platform/commercial/plans')
  await page.getByLabel('套餐代码').fill('office-pro')
  await page.getByRole('button', { name: '读取版本' }).click()

  await expect(page.getByText('办公室专业版').first()).toBeVisible()
  await expect(page.getByText('已发布').first()).toBeVisible()
  await expect(page.getByText('customer.read')).toBeVisible()
  await expect(page.getByText(/customer\.count/)).toBeVisible()
  await expect(page.getByText(/customer\.phone/)).toBeVisible()
  expect(requestedURL).toContain('/api/v1/platform/plans/office-pro/versions?pageSize=50')

  await page.getByRole('button', { name: '新建套餐' }).click()
  const dialog = page.getByRole('dialog', { name: '创建套餐草稿' })
  await expect(dialog).toBeVisible()
  await dialog.getByRole('button', { name: '添加模块' }).click()
  await selectUiOption(dialog.getByLabel('模块'), 'customer')
  await expect(dialog.getByText('customer.read')).toBeVisible()
  await expect(dialog.getByRole('button', { name: '添加额度' })).toBeEnabled()
  await expect(dialog.getByRole('button', { name: '添加字段' })).toBeEnabled()
})

test('TestCE13PlanCreateUsesTrustedSessionCsrfAndRereadsServerFact', async ({ page }) => {
  await mockModules(page)

  await page.route('**/api/auth/session', (route) => fulfillJson(route, {
    authenticated: true,
    actor_kind: 'platform',
    csrf_token: 'csrf-ce13-plan',
  }))

  let createdBody: Record<string, unknown> | undefined
  let createdHeaders: Record<string, string> | undefined
  await page.route('**/api/v1/platform/plans', async (route) => {
    if (route.request().method() !== 'POST') return route.continue()
    createdBody = route.request().postDataJSON() as Record<string, unknown>
    createdHeaders = route.request().headers()
    await fulfillJson(route, {
      planCode: 'office-new',
      version: '1',
      revision: '1',
      planRevision: '1',
      state: 'DRAFT',
      name: '新办公室套餐',
      terms: {
        modules: [],
        salesScope: ['default'],
        validityMode: 'unlimited',
        validityDays: 0,
        priceRef: '',
      },
      contentSha256: '',
      createdAt: '2026-09-12T00:00:00Z',
      publishedAt: '',
      retiredAt: '',
      actorId: 'platform-admin',
      reason: 'platform console create',
    })
  })

  let rereadCount = 0
  await page.route('**/api/v1/platform/plans/office-new/versions*', async (route) => {
    rereadCount += 1
    await fulfillJson(route, {
      versions: [{
        planCode: 'office-new',
        version: '1',
        revision: '1',
        planRevision: '1',
        state: 'DRAFT',
        name: '新办公室套餐',
        terms: {
          modules: [],
          salesScope: ['default'],
          validityMode: 'unlimited',
          validityDays: 0,
          priceRef: '',
        },
        contentSha256: '',
        createdAt: '2026-09-12T00:00:00Z',
        publishedAt: '',
        retiredAt: '',
        actorId: 'platform-admin',
        reason: 'platform console create',
      }],
      nextAfterVersion: '',
    })
  })

  await page.goto('/#/platform/commercial/plans')
  await page.getByRole('button', { name: '新建套餐' }).click()
  const dialog = page.getByRole('dialog', { name: '创建套餐草稿' })
  await dialog.getByLabel('套餐代码').fill('office-new')
  await dialog.getByLabel('套餐名称').fill('新办公室套餐')
  await dialog.getByRole('button', { name: '添加范围' }).click()
  await dialog.getByPlaceholder('default').fill('default')
  await dialog.getByRole('button', { name: '创建草稿', exact: true }).click()

  await expect(page.getByText('套餐草稿已创建。')).toBeVisible()
  await expect(page.getByText('新办公室套餐').first()).toBeVisible()
  expect(rereadCount).toBeGreaterThan(0)
  expect(createdBody).toMatchObject({ planCode: 'office-new', name: '新办公室套餐' })
  expect(createdHeaders?.['x-csrf-token']).toBe('csrf-ce13-plan')
  expect(createdHeaders?.authorization).toBeUndefined()
})


test('TestCE13PlanPublishUsesPreflightThenAuthoritativeReread', async ({ page }) => {
  await mockModules(page)
  mkdirSync('screenshots/platform-plan-journey', { recursive: true })

  await page.route('**/api/auth/session', (route) => fulfillJson(route, {
    authenticated: true,
    actor_kind: 'platform',
    csrf_token: 'csrf-plan-publish',
  }))

  let currentVersions = [draft, published]
  await page.route('**/api/v1/platform/plans/office-pro/versions?**', (route) =>
    fulfillJson(route, { versions: currentVersions, nextAfterVersion: '' }))

  let publishBody: Record<string, unknown> | undefined
  let publishHeaders: Record<string, string> | undefined
  await page.route('**/api/v1/platform/plans/office-pro/versions/3/publish', async (route) => {
    publishBody = route.request().postDataJSON() as Record<string, unknown>
    publishHeaders = route.request().headers()
    const applied = {
      ...draft,
      revision: '2',
      state: 'PUBLISHED',
      contentSha256: 'published-v3-content-sha',
      publishedAt: '2026-10-03T01:00:00Z',
      reason: String(publishBody.reason ?? ''),
    }
    currentVersions = [applied, published]
    await fulfillJson(route, applied)
  })

  await page.goto('/#/platform/commercial/plans')
  await page.getByLabel('套餐代码').fill('office-pro')
  await page.getByRole('button', { name: '读取版本' }).click()
  await expect(page.getByText('办公室专业版').first()).toBeVisible()

  await page.getByRole('button', { name: '发布前检查' }).click()
  const preflight = page.getByRole('dialog', { name: '发布前检查' })
  await expect(preflight).toBeVisible()
  await expect(preflight.getByText('此检查不是“发布成功”证明')).toBeVisible()

  for (const viewport of [
    { width: 1366, height: 768 },
    { width: 1440, height: 900 },
    { width: 1536, height: 1024 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await expect(preflight).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewport.width)
    await page.screenshot({
      path: `screenshots/platform-plan-journey/04-preflight-${viewport.width}.png`,
      fullPage: false,
    })
  }

  await page.setViewportSize({ width: 1536, height: 1024 })
  await preflight.getByRole('button', { name: '继续发布' }).click()
  const confirm = page.getByRole('dialog', { name: '确认发布套餐版本' })
  await expect(confirm).toBeVisible()
  await expect(confirm.getByText('发布后内容不可直接覆盖')).toBeVisible()
  await page.screenshot({ path: 'screenshots/platform-plan-journey/05-publish-1536.png', fullPage: false })
  await confirm.getByRole('button', { name: '确认发布' }).click()

  await expect(page.getByText('套餐版本已发布；后续修订需要创建新版本。')).toBeVisible()
  await expect(page.getByText('已发布').first()).toBeVisible()
  expect(publishBody).toMatchObject({
    planCode: 'office-pro',
    version: '3',
    expectedRevision: '1',
    reason: '发布当前套餐版本',
  })
  expect(publishHeaders?.['x-csrf-token']).toBe('csrf-plan-publish')
})

test('TestCE13PlanCloneCreatesNewDraftWithoutOverwritingSource', async ({ page }) => {
  await mockModules(page)
  mkdirSync('screenshots/platform-plan-journey', { recursive: true })
  await page.setViewportSize({ width: 1536, height: 1024 })

  await page.route('**/api/auth/session', (route) => fulfillJson(route, {
    authenticated: true,
    actor_kind: 'platform',
    csrf_token: 'csrf-plan-clone',
  }))

  const clonedDraft = {
    ...published,
    version: '3',
    revision: '1',
    planRevision: '6',
    state: 'DRAFT',
    contentSha256: '',
    publishedAt: '',
    reason: '基于当前版本创建新草稿',
  }
  let currentVersions = [published]
  await page.route('**/api/v1/platform/plans/office-pro/versions?**', (route) =>
    fulfillJson(route, { versions: currentVersions, nextAfterVersion: '' }))

  let cloneBody: Record<string, unknown> | undefined
  await page.route('**/api/v1/platform/plans/office-pro/versions', async (route) => {
    if (route.request().method() !== 'POST') return route.continue()
    cloneBody = route.request().postDataJSON() as Record<string, unknown>
    currentVersions = [clonedDraft, published]
    await fulfillJson(route, clonedDraft)
  })

  await page.goto('/#/platform/commercial/plans')
  await page.getByLabel('套餐代码').fill('office-pro')
  await page.getByRole('button', { name: '读取版本' }).click()
  await page.getByRole('button', { name: '创建新版本' }).click()

  const dialog = page.getByRole('dialog', { name: '基于当前版本创建新版本' })
  await expect(dialog).toContainText('原版本保持不变')
  await expect(dialog).toContainText('版本号由系统分配')
  await page.screenshot({ path: 'screenshots/platform-plan-journey/06-clone-1536.png', fullPage: false })
  await dialog.getByRole('button', { name: '创建新版本' }).click()

  await expect(page.getByText('新版本草稿已创建；来源版本保持不变。')).toBeVisible()
  await expect(page.getByText('草稿').first()).toBeVisible()
  expect(cloneBody).toMatchObject({
    planCode: 'office-pro',
    fromVersion: '2',
    expectedPlanRevision: '5',
    reason: '基于当前版本创建新草稿',
  })
})

test('TestCE13PlanRetireExplainsNonDestructiveEffectAndRereads', async ({ page }) => {
  await mockModules(page)
  mkdirSync('screenshots/platform-plan-journey', { recursive: true })
  await page.setViewportSize({ width: 1536, height: 1024 })

  await page.route('**/api/auth/session', (route) => fulfillJson(route, {
    authenticated: true,
    actor_kind: 'platform',
    csrf_token: 'csrf-plan-retire',
  }))

  let currentVersions = [published]
  await page.route('**/api/v1/platform/plans/office-pro/versions?**', (route) =>
    fulfillJson(route, { versions: currentVersions, nextAfterVersion: '' }))

  let retireBody: Record<string, unknown> | undefined
  await page.route('**/api/v1/platform/plans/office-pro/versions/2/retire', async (route) => {
    retireBody = route.request().postDataJSON() as Record<string, unknown>
    currentVersions = [retired]
    await fulfillJson(route, retired)
  })

  await page.goto('/#/platform/commercial/plans')
  await page.getByLabel('套餐代码').fill('office-pro')
  await page.getByRole('button', { name: '读取版本' }).click()
  await page.getByRole('button', { name: '停售', exact: true }).click()

  const dialog = page.getByRole('dialog', { name: '确认停售套餐版本' })
  await expect(dialog).toContainText('停售不是删除')
  await expect(dialog).toContainText('已有引用继续保留')
  await page.screenshot({ path: 'screenshots/platform-plan-journey/07-retire-1536.png', fullPage: false })
  await dialog.getByRole('button', { name: '确认停售' }).click()

  await expect(page.getByText('套餐版本已停售；历史内容与已有引用继续保留。')).toBeVisible()
  await expect(page.getByText('已停售').first()).toBeVisible()
  expect(retireBody).toMatchObject({
    planCode: 'office-pro',
    version: '2',
    expectedRevision: '3',
    reason: '停止当前版本的新销售',
  })
})
