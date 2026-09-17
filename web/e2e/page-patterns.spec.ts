import { expect, test } from '@playwright/test'
import { mkdirSync } from 'node:fs'

const viewports = [
  { width: 1366, height: 768 }, { width: 1440, height: 900 },
  { width: 1536, height: 1024 }, { width: 390, height: 844 },
]
const examples = [
  { path: '/enterprise/members', pattern: 'ListPage', regions: ['page-heading', 'query', 'data', 'toolbar', 'pagination'] },
  { path: '/enterprise/roles', pattern: 'ListPage', regions: ['page-heading', 'query', 'data'] },
  { path: '/enterprise/organization', pattern: 'WorkbenchPage', regions: ['page-heading', 'workspace', 'organization-tree', 'department-detail'] },
  { path: '/enterprise/plan', pattern: 'WorkbenchPage', regions: ['page-heading', 'subscription', 'quota-summary', 'workspace'] },
  { path: '/enterprise/company', pattern: 'FormPage', regions: ['page-heading', 'form-workspace', 'form-actions', 'scope'] },
  { path: '/enterprise/branding', pattern: 'FormPage', regions: ['page-heading', 'form-workspace', 'form-actions', 'scope'] },
  { path: '/enterprise/logs', pattern: 'ListPage', regions: ['page-heading', 'query', 'data'] },
  { path: '/system/general', pattern: 'FormPage', regions: ['page-heading', 'form-workspace', 'scope'] },
]

for (const example of examples) {
  test(`page pattern has visible real regions across all viewports: ${example.path}`, async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', (error) => errors.push(error.message))
    page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()) })
    await page.clock.setFixedTime(new Date('2026-09-16T12:00:00Z'))
    mkdirSync('screenshots/page-patterns', { recursive: true })
    for (const viewport of viewports) {
      await page.setViewportSize(viewport)
      await page.goto(`/#${example.path}`)
      const surface = page.locator('.main-content [data-ui-template]').first()
      await expect(surface).toHaveAttribute('data-ui-template', example.pattern)
      await expect(surface.getByRole('heading', { level: 1 })).toBeVisible()
      for (const region of example.regions) {
        const element = surface.locator(`[data-ui-region="${region}"]`)
        await expect(element).toHaveCount(1)
        await expect(element).toBeVisible()
      }
      await expect(page.locator('vite-error-overlay')).toHaveCount(0)
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width)
      await page.evaluate(() => document.fonts.ready)
      await page.screenshot({
        path: `screenshots/page-patterns/${example.path.slice(1).replaceAll('/', '-')}-${viewport.width}.png`,
        fullPage: false, animations: 'disabled',
      })
    }
    expect(errors).toEqual([])
  })
}
