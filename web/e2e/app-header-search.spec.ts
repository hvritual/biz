import { expect, test, type Page, type TestInfo } from '@playwright/test'

async function ready(page: Page) {
  await page.goto('/#/enterprise/members')
  await expect(page.locator('.member-table')).toBeVisible()
}

test('global search keeps its icon and shortcut inside UiInput while preserving submit and focus behavior', async ({ page }, testInfo: TestInfo) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await ready(page)

  const search = page.getByRole('textbox', { name: '全局搜索成员', exact: true })
  const frame = page.locator('.global-search [data-slot="input-wrapper"]')
  await expect(frame.locator('[data-slot="input-prefix"]')).toBeVisible()
  await expect(frame.locator('[data-slot="input-prefix"] svg')).toBeVisible()
  await expect(frame.locator('[data-slot="input-suffix"]')).toHaveText('⌘ K')

  await page.keyboard.press('Meta+k')
  await expect(search).toBeFocused()
  await expect
    .poll(() => frame.evaluate((element) => getComputedStyle(element).outlineStyle))
    .toBe('solid')
  await expect
    .poll(() => search.evaluate((element) => getComputedStyle(element).outlineStyle))
    .toBe('none')
  await page.screenshot({ path: testInfo.outputPath('global-search-focused.png') })

  await search.fill('alice')
  await search.press('Enter')
  await expect(page).toHaveURL(/#\/enterprise\/members\?q=alice$/)
})
