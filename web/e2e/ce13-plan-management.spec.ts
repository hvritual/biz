import { expect, test, type Page, type Route } from '@playwright/test'

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
  const dialog = page.getByRole('dialog', { name: '新建套餐首稿' })
  await expect(dialog).toBeVisible()
  await dialog.getByRole('button', { name: '添加模块' }).click()
  await dialog.getByLabel('模块').selectOption('customer')
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
  const dialog = page.getByRole('dialog', { name: '新建套餐首稿' })
  await dialog.getByLabel('套餐代码').fill('office-new')
  await dialog.getByLabel('套餐名称').fill('新办公室套餐')
  await dialog.getByRole('button', { name: '添加范围' }).click()
  await dialog.getByPlaceholder('default').fill('default')
  await dialog.getByRole('button', { name: '提交到服务端' }).click()

  await expect(page.getByText('套餐草稿已创建。')).toBeVisible()
  await expect(page.getByText('新办公室套餐').first()).toBeVisible()
  expect(rereadCount).toBeGreaterThan(0)
  expect(createdBody).toMatchObject({ planCode: 'office-new', name: '新办公室套餐' })
  expect(createdHeaders?.['x-csrf-token']).toBe('csrf-ce13-plan')
  expect(createdHeaders?.authorization).toBeUndefined()
})
