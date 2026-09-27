import { expect, test } from '@playwright/test'
import { installUnauthenticatedSession } from './ui.helpers'

test.beforeEach(async ({ page }) => {
  await installUnauthenticatedSession(page)
})

async function useZh(page) {
  await page.addInitScript(() => localStorage.setItem('coffeelink.locale', 'zh-CN'))
}

async function geometry(page) {
  return page.locator('.main-content').boundingBox()
}

test('appearance toggles preserve member draft state, compact rows and teleported surfaces', async ({ page }) => {
  await useZh(page)
  await page.goto('/#/enterprise/members')
  const before = await geometry(page)
  const query = page.locator('.member-filters .search-field input')
  const firstRow = page.locator('.member-data-panel tbody tr').first()
  await expect(firstRow).toBeVisible()
  const rowBefore = await firstRow.boundingBox()
  await query.fill('preview-state')

  await page.getByRole('button', { name: '切换为深色模式' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-ui-mode', 'dark')
  await expect(query).toHaveValue('preview-state')
  const afterMode = await geometry(page)
  expect(Math.abs((afterMode?.x ?? 0) - (before?.x ?? 0))).toBeLessThanOrEqual(1)
  expect(Math.abs((afterMode?.width ?? 0) - (before?.width ?? 0))).toBeLessThanOrEqual(1)

  await page.getByRole('button', { name: '切换为紧凑密度' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-ui-density', 'compact')
  await expect(query).toHaveValue('preview-state')
  const rowAfter = await firstRow.boundingBox()
  expect(rowAfter?.height ?? 0).toBeLessThan(rowBefore?.height ?? 0)

  // Open the canonical member action dialog rather than a responsive header link.
  await page.locator('.member-tools .btn-primary').click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  expect(await dialog.evaluate((node) => getComputedStyle(node).colorScheme)).toContain('dark')
  await page.keyboard.press('Escape')

  const locale = page.getByRole('combobox', { name: '语言' })
  await locale.click()
  const listbox = page.getByRole('listbox')
  await expect(listbox).toBeVisible()
  expect(await listbox.evaluate((node) => getComputedStyle(node).colorScheme)).toContain('dark')
})

test('appearance preferences persist without becoming tenant brand authority', async ({ page }) => {
  await useZh(page)
  await page.goto('/#/enterprise/members')
  await page.getByRole('button', { name: '切换为深色模式' }).click()
  await page.getByRole('button', { name: '切换为紧凑密度' }).click()
  const brandBefore = await page.locator('html').getAttribute('data-ui-theme')
  await page.reload()
  await expect(page.locator('html')).toHaveAttribute('data-ui-mode', 'dark')
  await expect(page.locator('html')).toHaveAttribute('data-ui-density', 'compact')
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', brandBefore ?? 'blue')
  expect(await page.evaluate(() => localStorage.getItem('coffeelink.ui-theme'))).toBeNull()
})

test('platform tenant geometry keeps 200/68/480 shell dimensions across appearance changes', async ({ page }) => {
  await useZh(page)
  await page.setViewportSize({ width: 1366, height: 768 })
  await page.goto('/#/platform/tenants')
  const before = await geometry(page)
  await page.getByRole('button', { name: '切换为深色模式' }).click()
  await page.getByRole('button', { name: '切换为紧凑密度' }).click()
  const after = await geometry(page)
  expect(Math.abs((after?.x ?? 0) - (before?.x ?? 0))).toBeLessThanOrEqual(1)
  expect(Math.abs((after?.width ?? 0) - (before?.width ?? 0))).toBeLessThanOrEqual(1)

  const rail = await page.locator('.primary-nav').boundingBox()
  expect(Math.round(rail?.width ?? 0)).toBe(200)
  await page.locator('[data-module-id="platform-commercial"]').click()
  const panel = await page.locator('.module-panel').boundingBox()
  expect(Math.round(panel?.width ?? 0)).toBe(480)
})
