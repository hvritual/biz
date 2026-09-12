import { expect, test, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'

const evidenceDir = 'test-results/screenshots'

async function ready(page: Page) {
  await page.goto('/#/enterprise/members')
  await expect(page.locator('h1')).toBeVisible()
  await page.evaluate(() => document.fonts.ready)
}

async function openEnterpriseMenu(page: Page, mobile: boolean) {
  if (mobile) {
    await page.getByRole('button', { name: '打开主导航', exact: true }).click()
  }
  await page.locator('[data-module="enterprise"]').click()
  await expect(page.locator('.module-panel')).toBeVisible()
}

async function capturePair(
  page: Page,
  width: number,
  height: number,
  normalName: string,
  menuName: string,
) {
  await page.setViewportSize({ width, height })
  await ready(page)
  await page.screenshot({ path: `${evidenceDir}/${normalName}` })
  await openEnterpriseMenu(page, width <= 390)
  await page.screenshot({ path: `${evidenceDir}/${menuName}` })
  await page.keyboard.press('Escape')
}

test('TestCE11 preserves exact required viewport evidence after customer and site integration', async ({
  page,
}) => {
  mkdirSync(evidenceDir, { recursive: true })
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(message.text())
  })

  await capturePair(page, 1366, 768, 'responsive-1366.png', 'responsive-menu-1366.png')
  await capturePair(page, 1440, 900, 'responsive-1440.png', 'responsive-menu-1440.png')
  await capturePair(page, 390, 844, 'responsive-390.png', 'responsive-menu-390.png')

  expect(errors).toEqual([])
})
