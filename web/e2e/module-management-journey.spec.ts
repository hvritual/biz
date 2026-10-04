import { expect, test, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { selectUiOption } from './ui.helpers'

// Test fixtures only. The production page never falls back to these records.
const initial = {
  moduleCode: 'access-management', name: '成员与权限', category: 'access', salesScope: ['default'],
  technicalStatus: 'MODULE_TECHNICAL_STATUS_READY', salesStatus: 'MODULE_SALES_STATUS_SELLABLE',
  capabilityCodes: ['tenant.lifecycle', 'tenant.member.lifecycle', 'tenant.role.permission'],
  quotaSchemaKeys: ['member.count'], fieldPolicySchemaKeys: [], dependencies: [], version: '9',
}
const definitions = [
  {
    moduleCode: 'access-management', capabilityCodes: initial.capabilityCodes, quotaSchemaKeys: initial.quotaSchemaKeys,
    fieldPolicySchemaKeys: initial.fieldPolicySchemaKeys, dependencies: [], implementationReady: true,
  },
  {
    moduleCode: 'device-operations', capabilityCodes: ['device.lifecycle', 'device.transfer'], quotaSchemaKeys: ['tenant.devices'],
    fieldPolicySchemaKeys: ['device.identity'], dependencies: [], implementationReady: true,
  },
]
const output = 'test-results/screenshots/module-journey'
type FixtureMode = 'normal' | 'denied' | 'conflict' | 'unknown' | 'stale-readback'
const allModuleActions = [
  'commercial.module.list',
  'commercial.module.get',
  'commercial.module.create',
  'commercial.module.update',
  'commercial.module.set_sales_status',
  'commercial.module.set_technical_status',
]
async function install(page: Page, mode: FixtureMode = 'normal', initialAuthorization = allModuleActions) {
  let current = { ...initial }
  let authorization = [...initialAuthorization]
  let created: typeof initial | null = null
  let writes = 0
  let readable = mode !== 'stale-readback'
  const headers: Record<string, string>[] = []
  await page.route('**/api/**', async (route) => {
    const req = route.request()
    const path = new URL(req.url()).pathname
    if (path === '/api/auth/session') return route.fulfill({ json: { authenticated: true, actor_kind: 'platform', platform_subject: 'module-test-admin', context_version: 1, csrf_token: 'module-test-csrf' } })
    if (path === '/api/auth/authorization' && req.method() === 'GET') return route.fulfill({ json: {
      authenticated: true,
      actor_kind: 'platform',
      platform_subject: 'module-test-admin',
      roles: [],
      grants: [],
      data_policies: [],
      site_ids: [],
      modules: [],
      actions: authorization.map((code) => ({
        code,
        domain: 'commercial',
        application: 'module_catalog',
        use_case: code,
        tenant_required: false,
        authentication: ['web-session'],
        permissions: [],
        permission_mode: 'all',
      })),
      button_codes: authorization,
    } })
    if (path === '/api/auth/action-catalog' && req.method() === 'GET') return route.fulfill({ json: { module_definitions: definitions } })
    if (path === '/api/v1/platform/modules' && req.method() === 'GET') return route.fulfill({ json: { modules: created ? [current, created] : [current] } })
    if (path === '/api/v1/platform/modules' && req.method() === 'POST') {
      writes += 1
      headers.push(req.headers())
      const body = req.postDataJSON()
      if (mode === 'denied') return route.fulfill({ status: 403, json: { message: 'forbidden' } })
      created = {
        ...initial, moduleCode: body.moduleCode, name: body.name, category: body.category, salesScope: body.salesScope,
        technicalStatus: 'MODULE_TECHNICAL_STATUS_READY', salesStatus: 'MODULE_SALES_STATUS_RETIRED',
        capabilityCodes: ['device.lifecycle', 'device.transfer'], quotaSchemaKeys: ['tenant.devices'], fieldPolicySchemaKeys: ['device.identity'], dependencies: [], version: '1',
      }
      if (mode === 'unknown') return route.abort('failed')
      return route.fulfill({ json: created })
    }
    if (path === '/api/v1/platform/modules/device-operations' && req.method() === 'GET') {
      if (!created) return route.fulfill({ status: 404, json: { message: 'not found' } })
      return route.fulfill({ json: created })
    }
    if (path === '/api/v1/platform/modules/access-management' && req.method() === 'GET') return route.fulfill({ json: readable ? current : initial })
    if (path.startsWith('/api/v1/platform/modules/access-management') && ['POST', 'PATCH'].includes(req.method())) {
      writes += 1
      headers.push(req.headers())
      const body = req.postDataJSON()
      if (mode === 'denied') return route.fulfill({ status: 403, json: { message: 'forbidden' } })
      if (mode === 'conflict' && writes === 1) {
        current = { ...current, version: '10', name: '其他管理员更新的模块名' }
        return route.fulfill({ status: 409, json: { message: 'conflict' } })
      }
      expect(body.version).toBe(current.version)
      if (path.endsWith('/sales-status')) {
        current = { ...current, salesStatus: body.salesStatus }
      } else if (path.endsWith('/technical-status')) {
        current = { ...current, technicalStatus: body.technicalStatus, salesStatus: 'MODULE_SALES_STATUS_RETIRED', version: String(BigInt(current.version) + 1n) }
      } else {
        current = { ...current, name: body.name, category: body.category, salesScope: body.salesScope, salesStatus: 'MODULE_SALES_STATUS_RETIRED', version: String(BigInt(current.version) + 1n) }
      }
      if (mode === 'unknown') return route.abort('failed')
      return route.fulfill({ json: current })
    }
    return route.fulfill({ status: 404, json: { message: 'No fixture for this endpoint' } })
  })
  return {
    writes: () => writes,
    headers,
    created: () => created,
    current: () => current,
    allowReadback: () => { readable = true },
    setAuthorization: (actions: string[]) => { authorization = [...actions] },
  }
}
async function open(page: Page) {
  await page.goto('/#/platform/commercial/modules')
  await expect(page.getByRole('heading', { name: '模块目录', level: 1 })).toBeVisible()
  await page.getByRole('button', { name: '查看详情', exact: true }).click()
  await expect(page.getByRole('dialog', { name: '模块详情 · 成员与权限' })).toBeVisible()
}
async function capture(page: Page, name: string) {
  await page.evaluate(() => document.fonts.ready)
  await page.screenshot({ path: `${output}/${name}.png`, fullPage: false })
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
}

test.beforeEach(() => mkdirSync(output, { recursive: true }))


test('registered module onboarding creates a retired catalog record and confirms it by GET', async ({ page }) => {
  const fixture = await install(page)
  await page.goto('/#/platform/commercial/modules')
  await page.getByRole('button', { name: '新增模块', exact: true }).click()
  const create = page.getByRole('dialog', { name: '新增模块' })
  await expect(create.getByText('从已注册定义创建模块目录记录', { exact: true })).toBeVisible()
  await selectUiOption(create.getByLabel('模块定义'), 'device-operations')
  await expect(create.getByText('设备生命周期')).toBeVisible()
  await create.getByLabel('模块名称').fill('设备运营')
  await create.getByLabel('分类').fill('device')
  await create.getByLabel('销售范围').fill('default, enterprise')
  await create.getByLabel('创建原因').fill('将已实现设备运营定义纳入平台模块目录')
  await capture(page, '00-create-module-form')
  await create.getByRole('button', { name: '下一步', exact: true }).click()
  const confirm = page.getByRole('dialog', { name: '确认新增模块' })
  await expect(confirm.getByText(/创建后初始销售状态为停售/)).toBeVisible()
  await capture(page, '00-create-module-confirm')
  await confirm.getByRole('button', { name: '确认创建', exact: true }).click()
  const result = page.getByRole('dialog', { name: '模块创建结果' })
  await expect(result.getByText('模块已创建并确认', { exact: true })).toBeVisible()
  await expect(result.getByText('已停售')).toBeVisible()
  await capture(page, '00-create-module-result')
  expect(fixture.writes()).toBe(1)
  expect(fixture.created()).toMatchObject({ moduleCode: 'device-operations', version: '1', salesStatus: 'MODULE_SALES_STATUS_RETIRED' })
  expect(fixture.headers[0]?.['idempotency-key']).toMatch(/^platform-module-create-/)
})

test('module journey captures individual operations in all four CoffeeLink viewports', async ({ page }) => {
  test.setTimeout(120_000)
  const browserErrors: string[] = []
  page.on('pageerror', (error) => browserErrors.push(error.message))
  const fixture = await install(page)
  for (const [width, height] of [[1366, 768], [1440, 900], [1536, 1024], [390, 844]]) {
    await page.setViewportSize({ width: width!, height: height! })
    await page.goto('/#/platform/commercial/modules')
    await expect(page.getByRole('button', { name: '查看详情', exact: true })).toBeVisible()
    await capture(page, `${width}-01-catalog`)
    await page.getByLabel('关键词', { exact: true }).fill('成员')
    await capture(page, `${width}-02-filter`)
    await page.getByRole('button', { name: '查看详情', exact: true }).click()
    await capture(page, `${width}-03-detail`)
    await page.getByRole('tab', { name: '能力与权限' }).click()
    await expect(page.getByText('platform.module.technical.manage', { exact: true })).toBeVisible()
    await capture(page, `${width}-04-permissions`)
    await page.getByRole('tab', { name: '关联页面' }).click()
    await page.getByRole('button', { name: '查看成员管理关联定义', exact: true }).click()
    await capture(page, `${width}-05-routes`)
    await page.getByRole('tab', { name: '可用性核验' }).click()
    await expect(page.getByText('成员级授权核验尚未接入', { exact: true })).toBeVisible()
    await capture(page, `${width}-06-unavailable-verification`)
    await page.getByRole('tab', { name: '概览', exact: true }).click()
    await page.getByRole('button', { name: '编辑基础配置' }).click()
    await page.getByLabel('模块名称').fill('成员与权限')
    await page.getByLabel('变更原因').fill('界面回归测试，保持真实契约边界')
    await capture(page, `${width}-07-edit`)
    await page.getByRole('button', { name: '保存基础配置', exact: true }).click()
    await expect(page.getByText('变更已确认生效', { exact: true })).toBeVisible()
    await capture(page, `${width}-08-metadata-result`)
    await page.getByRole('button', { name: '返回模块详情' }).click()
    await page.getByRole('button', { name: /^(停售销售|恢复销售)$/ }).click()
    await page.getByLabel('变更原因').fill('测试销售状态单独确认')
    await capture(page, `${width}-09-sales-confirm`)
    await page.getByRole('button', { name: /^(确认停售|确认恢复销售)$/ }).click()
    await expect(page.getByText('变更已确认生效', { exact: true })).toBeVisible()
    await capture(page, `${width}-10-sales-result`)
    await page.getByRole('button', { name: '返回模块详情' }).click()
    await page.getByRole('button', { name: '调整技术状态', exact: true }).click()
    const target = width === 1440 || width === 390 ? 'MODULE_TECHNICAL_STATUS_READY' : 'MODULE_TECHNICAL_STATUS_DISABLED'
    await selectUiOption(page.getByLabel('目标技术状态'), target)
    await page.getByLabel('变更原因').fill('测试技术状态独立确认，不代表运行验证')
    await capture(page, `${width}-11-technical-confirm`)
    await page.getByRole('button', { name: /^(确认技术停用|确认应用技术状态)$/ }).click()
    await expect(page.getByText('变更已确认生效', { exact: true })).toBeVisible()
    await capture(page, `${width}-12-technical-result`)
    await page.getByRole('button', { name: '关闭', exact: true }).click()
  }
  expect(fixture.writes()).toBe(12)
  for (const header of fixture.headers) {
    expect(header['x-csrf-token']).toBe('module-test-csrf')
    expect(header['idempotency-key']).toMatch(/^ce13-module-/)
    expect(header.authorization).toBeUndefined()
  }
  expect(browserErrors).toEqual([])
})

test('module version conflict preserves the draft and requires explicit reconfirmation', async ({ page }) => {
  const fixture = await install(page, 'conflict')
  await open(page)
  await page.getByRole('button', { name: '编辑基础配置' }).click()
  await page.getByLabel('模块名称').fill('我的原草稿')
  await page.getByLabel('变更原因').fill('保留变更原因')
  await page.getByRole('button', { name: '保存基础配置', exact: true }).click()
  const conflictDialog = page.getByRole('dialog', { name: '核对模块版本冲突' })
  await expect(conflictDialog).toBeVisible()
  await expect(conflictDialog.getByRole('cell', { name: '其他管理员更新的模块名', exact: true })).toBeVisible()
  await expect(conflictDialog.getByText(/我的原草稿/)).toBeVisible()
  await capture(page, '13-version-conflict')
  expect(fixture.writes()).toBe(1)
  await page.getByRole('button', { name: '使用最新版本继续编辑' }).click()
  await expect(page.getByLabel('模块名称')).toHaveValue('我的原草稿')
  await expect(page.getByLabel('变更原因')).toHaveValue('保留变更原因')
  expect(fixture.writes()).toBe(1)
  await page.getByRole('button', { name: '保存基础配置', exact: true }).click()
  await expect(page.getByText('变更已确认生效', { exact: true })).toBeVisible()
  expect(fixture.writes()).toBe(2)
})

test('a forbidden write is not success and read access does not imply manage access', async ({ page }) => {
  await install(page, 'denied')
  await open(page)
  await page.getByRole('button', { name: '停售销售' }).click()
  await page.getByLabel('变更原因').fill('无写权限回归')
  await page.getByRole('button', { name: '确认停售' }).click()
  await expect(page.getByText('未获操作授权', { exact: true })).toBeVisible()
  await expect(page.getByText('变更已确认生效', { exact: true })).toHaveCount(0)
  await capture(page, '14-write-forbidden')
})

test('uncertain writes survive closing and reopening without a second submission', async ({ page }) => {
  const fixture = await install(page, 'unknown')
  await open(page)
  await page.getByRole('button', { name: '停售销售' }).click()
  await page.getByLabel('变更原因').fill('结果未知，禁止盲目重提')
  await page.getByRole('button', { name: '确认停售' }).click()
  await expect(page.getByText('结果尚未确认', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '重新读取最新状态' }).click()
  await expect(page.getByText('结果尚未确认', { exact: true })).toBeVisible()
  await capture(page, '15-unknown-result')
  await page.getByRole('button', { name: '关闭', exact: true }).click()
  await page.getByRole('button', { name: '查看详情' }).click()
  await expect(page.getByText('结果尚未确认', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '返回模块详情' }).click()
  await expect(page.getByRole('button', { name: '恢复销售', exact: true })).toBeDisabled()
  expect(fixture.writes()).toBe(1)
})

test('a receipt alone is not completion; a later matching GET confirms without rewriting', async ({ page }) => {
  const fixture = await install(page, 'stale-readback')
  await open(page)
  await page.getByRole('button', { name: '停售销售' }).click()
  await page.getByLabel('变更原因').fill('读回滞后')
  await page.getByRole('button', { name: '确认停售' }).click()
  await expect(page.getByText('结果尚未确认', { exact: true })).toBeVisible()
  fixture.allowReadback()
  await page.getByRole('button', { name: '重新读取最新状态' }).click()
  await expect(page.getByText('变更已确认生效', { exact: true })).toBeVisible()
  expect(fixture.writes()).toBe(1)
})

test('keyboard tabs and cancelling an edit preserve context and never write', async ({ page }) => {
  const fixture = await install(page)
  await open(page)
  await page.getByRole('tab', { name: '概览', exact: true }).focus()
  await page.keyboard.press('End')
  await expect(page.getByRole('tab', { name: '可用性核验' })).toBeFocused()
  await page.keyboard.press('Home')
  await page.getByRole('button', { name: '编辑基础配置' }).click()
  await page.getByLabel('模块名称').fill('暂存的草稿')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog', { name: '放弃本次编辑？' })).toBeVisible()
  await capture(page, '16-discard-confirm')
  await page.getByRole('button', { name: '继续编辑' }).click()
  await expect(page.getByLabel('模块名称')).toHaveValue('暂存的草稿')
  await page.getByRole('button', { name: '取消', exact: true }).click()
  await page.getByRole('button', { name: '放弃编辑' }).click()
  await page.getByRole('button', { name: '关闭', exact: true }).click()
  await expect(page.getByRole('button', { name: '查看详情' })).toBeFocused()
  expect(fixture.writes()).toBe(0)
})


test('read-only platform authorization keeps module inspection but disables every module write', async ({ page }) => {
  const fixture = await install(page, 'normal', ['commercial.module.list', 'commercial.module.get'])
  await page.goto('/#/platform/commercial/modules')
  await expect(page.getByRole('button', { name: '新增模块', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: '查看详情', exact: true }).click()
  await expect(page.getByRole('button', { name: '编辑基础配置', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: /^(停售销售|恢复销售)$/ })).toBeDisabled()
  await expect(page.getByRole('button', { name: '调整技术状态', exact: true })).toBeDisabled()
  expect(fixture.writes()).toBe(0)
})

test('module manage authorization enables metadata and sales changes but not technical status', async ({ page }) => {
  const fixture = await install(page, 'normal', [
    'commercial.module.list',
    'commercial.module.get',
    'commercial.module.create',
    'commercial.module.update',
    'commercial.module.set_sales_status',
  ])
  await page.goto('/#/platform/commercial/modules')
  await expect(page.getByRole('button', { name: '新增模块', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: '查看详情', exact: true }).click()
  await expect(page.getByRole('button', { name: '编辑基础配置', exact: true })).toBeEnabled()
  await expect(page.getByRole('button', { name: /^(停售销售|恢复销售)$/ })).toBeEnabled()
  await expect(page.getByRole('button', { name: '调整技术状态', exact: true })).toBeDisabled()
  expect(fixture.writes()).toBe(0)
})

test('technical authorization enables technical status without granting general module management', async ({ page }) => {
  const fixture = await install(page, 'normal', [
    'commercial.module.list',
    'commercial.module.get',
    'commercial.module.set_technical_status',
  ])
  await page.goto('/#/platform/commercial/modules')
  await expect(page.getByRole('button', { name: '新增模块', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: '查看详情', exact: true }).click()
  await expect(page.getByRole('button', { name: '编辑基础配置', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: /^(停售销售|恢复销售)$/ })).toBeDisabled()
  await expect(page.getByRole('button', { name: '调整技术状态', exact: true })).toBeEnabled()
  expect(fixture.writes()).toBe(0)
})

test('revoked manage grant is re-read before submit and preserves the unsubmitted draft', async ({ page }) => {
  const fixture = await install(page)
  await open(page)
  await page.getByRole('button', { name: '编辑基础配置', exact: true }).click()
  await page.getByLabel('模块名称').fill('撤权后仍保留的草稿')
  await page.getByLabel('变更原因').fill('验证实时撤权')
  fixture.setAuthorization(['commercial.module.list', 'commercial.module.get'])
  await page.getByRole('button', { name: '保存基础配置', exact: true }).click()

  await expect(page.getByText(/当前平台授权已变化，已阻止变更提交/)).toBeVisible()
  await expect(page.getByLabel('模块名称')).toHaveValue('撤权后仍保留的草稿')
  expect(fixture.writes()).toBe(0)
})
