import { expect, test } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { installUnauthenticatedSession } from './ui.helpers'

test.beforeEach(async ({ page }) => {
  await installUnauthenticatedSession(page)
})

const viewports = [
  { width: 1366, height: 768 },
  { width: 1440, height: 900 },
  { width: 1536, height: 1024 },
  { width: 390, height: 844 },
]

async function chooseLocale(page: import('@playwright/test').Page, option: '中文' | 'English') {
  await page.getByRole('combobox', { name: /语言|Language/ }).click()
  await page.getByRole('option', { name: option, exact: true }).click()
}

test('locale changes presentation but not route identity, member selection or product actions', async ({ page }) => {
  await page.goto('/#/enterprise/members')
  await chooseLocale(page, 'English')
  await expect(page).toHaveURL(/#\/enterprise\/members/)
  await expect(page.getByRole('heading', { name: 'Members', level: 1 })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Add member' })).toBeVisible()
  const firstView = page.getByRole('button', { name: /View .* profile/ }).first()
  const row = firstView.locator('xpath=ancestor::tr')
  const checkbox = row.getByRole('checkbox')
  await firstView.click()
  await expect(page.getByRole('heading', { name: 'Member details' })).toBeVisible()
  await expect(checkbox).not.toBeChecked()
  await page.getByRole('button', { name: 'Close member details' }).click()
  await expect(firstView).toBeFocused()
  await page.getByRole('button', { name: 'Add member' }).click()
  await expect(page.getByRole('dialog').getByRole('heading', { name: 'Add member' })).toBeVisible()
  await expect(page.getByLabel('Email', { exact: true })).toBeVisible()
  await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click()
  await chooseLocale(page, '中文')
  await expect(page).toHaveURL(/#\/enterprise\/members/)
  await expect(page.getByRole('heading', { name: '成员管理', level: 1 })).toBeVisible()
})

test('platform tenant route keeps trusted identity behavior while labels switch language', async ({ page }) => {
  await page.goto('/#/platform/tenants')
  await chooseLocale(page, 'English')
  await expect(page.getByRole('heading', { name: 'Tenant Management', level: 1 })).toBeVisible()
  await expect(page.getByText('Sign in with a platform account')).toBeVisible()
  await expect(page).toHaveURL(/#\/platform\/tenants/)
  await chooseLocale(page, '中文')
  await expect(page.getByRole('heading', { name: '租户管理', level: 1 })).toBeVisible()
})

for (const target of [
  { path: '/enterprise/members', heading: 'Members', slug: 'members' },
  { path: '/platform/tenants', heading: 'Tenant Management', slug: 'tenants' },
]) {
  test(`English pilot remains readable across CoffeeLink viewports: ${target.slug}`, async ({ page }) => {
    mkdirSync('screenshots/i18n', { recursive: true })
    await page.addInitScript(() => localStorage.setItem('coffeelink.locale', 'en-US'))
    for (const viewport of viewports) {
      await page.setViewportSize(viewport)
      await page.goto(`/#${target.path}`)
      await expect(page.getByRole('heading', { name: target.heading, level: 1 })).toBeVisible()
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width)
      await page.screenshot({ path: `screenshots/i18n/${target.slug}-en-${viewport.width}.png`, animations: 'disabled' })
    }
  })
}
