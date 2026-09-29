import { chromium, expect, type Page, type TestInfo } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

/** Qualification-only browser API. No content script or product permission is added. */
type ZoomTabs = {
  query: (query: { url: string }) => Promise<Array<{ id?: number }>>
  getZoom: (id: number) => Promise<number>
  setZoom: (id: number, factor: number) => Promise<void>
}

export async function qualifyCommercialStateZoom(info: TestInfo, setup: (page: Page) => Promise<unknown>) {
  const extension = info.outputPath('zoom-control')
  mkdirSync(extension, { recursive: true })
  writeFileSync(join(extension, 'manifest.json'), JSON.stringify({
    manifest_version: 3, name: 'Typography qualification zoom control', version: '1.0.0',
    host_permissions: ['http://127.0.0.1/*'],
    background: { service_worker: 'background.js' },
  }))
  writeFileSync(join(extension, 'background.js'), 'chrome.runtime.onInstalled.addListener(() => {});\n')

  // Re-evaluate registry selection against this exact candidate, not a saved
  // Props or token catalog. The derived index stays in its ignored cache.
  execFileSync(process.execPath, ['scripts/design-cli.mjs', 'build'], { timeout: 30000 })
  const registry = JSON.parse(execFileSync(process.execPath,
    ['scripts/design-cli.mjs', 'find', '--query', 'StatusBadge', '--json'],
    { encoding: 'utf8', timeout: 30000 })) as { sourceFingerprint: string; results: Array<{ id: string }> }
  expect(registry.results.some((entry) => entry.id === 'ui/common/StatusBadge')).toBe(true)

  // A persistent full Chromium context is required for the actual browser
  // tabs.setZoom API. This is not CSS zoom or deviceScaleFactor emulation.
  const context = await chromium.launchPersistentContext(info.outputPath('browser-profile'), {
    channel: 'chromium', headless: true, viewport: null,
    baseURL: 'http://127.0.0.1:4173', locale: 'zh-CN', timezoneId: 'Asia/Shanghai',
    args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`, '--window-size=1440,1000'],
  })
  const pageErrors: string[] = []
  const consoleErrors: string[] = []
  try {
    const worker = context.serviceWorkers()[0] ?? await context.waitForEvent('serviceworker', { timeout: 15000 })
    const page = await context.newPage()
    page.on('pageerror', (error) => pageErrors.push(error.message))
    page.on('console', (message) => { if (['error', 'warning'].includes(message.type())) consoleErrors.push(message.text()) })
    await setup(page)
    await page.goto('/#/enterprise/plan')
    await expect(page).toHaveTitle(/CoffeeLink/)
    const badge = page.locator('.current-plan .status-badge')
    await expect(badge).toHaveText('宽限期')
    await expect(badge).toHaveClass(/\bwarning\b/)
    const metrics = () => page.evaluate(() => ({
      width: window.innerWidth, outerWidth: window.outerWidth,
      dpr: window.devicePixelRatio, cssZoom: getComputedStyle(document.documentElement).zoom,
      horizontalOverflow: document.documentElement.scrollWidth > window.innerWidth + 1,
    }))
    const before = await metrics()
    const browserZoom = await worker.evaluate(async () => {
      const tabs = (globalThis as unknown as { chrome: { tabs: ZoomTabs } }).chrome.tabs
      const matches = await tabs.query({ url: 'http://127.0.0.1:4173/*' })
      if (matches.length !== 1 || matches[0]?.id === undefined) throw new Error('Expected exactly one qualification tab')
      const id = matches[0].id
      const initial = await tabs.getZoom(id)
      await tabs.setZoom(id, 2)
      return { initial, final: await tabs.getZoom(id) }
    })
    expect(browserZoom.initial).toBe(1)
    expect(browserZoom.final).toBe(2)
    await expect.poll(async () => (await metrics()).dpr / before.dpr).toBeCloseTo(2, 2)
    const after = await metrics()
    expect(after.outerWidth).toBe(before.outerWidth)
    expect(Math.abs(after.width * 2 - before.width)).toBeLessThanOrEqual(2)
    expect(after.cssZoom).toBe(before.cssZoom)
    expect(after.horizontalOverflow).toBe(false)
    mkdirSync('screenshots', { recursive: true })
    const samples = []
    for (const sample of [{ locale: 'zh-CN', label: '宽限期' }, { locale: 'en-US', label: 'Grace period' }]) {
      await page.evaluate((locale) => localStorage.setItem('coffeelink.locale', locale), sample.locale)
      await page.reload()
      await expect(badge).toHaveText(sample.label)
      await expect(badge).toHaveClass(/\bwarning\b/)
      const rendered = await badge.evaluate((element) => {
        const style = getComputedStyle(element)
        return { fontSize: parseFloat(style.fontSize), fits: element.scrollWidth <= element.clientWidth + 1 }
      })
      expect(rendered.fontSize).toBeGreaterThanOrEqual(12)
      expect(rendered.fits).toBe(true)
      expect((await metrics()).horizontalOverflow).toBe(false)
      await page.screenshot({ path: `screenshots/commercial-status-200-percent-${sample.locale}.png`, fullPage: false })
      await page.getByRole('button', { name: '使用额度', exact: true }).click()
      await expect(page.getByRole('row').filter({ hasText: sample.locale === 'zh-CN' ? '成员额度' : 'Member quota' })).toBeVisible()
      await page.getByRole('button', { name: '套餐概览', exact: true }).click()
      samples.push({ locale: sample.locale, ...rendered, ...await metrics() })
    }
    expect(pageErrors).toEqual([])
    expect(consoleErrors).toEqual([])
    await info.attach('commercial-typography-zoom-evidence', {
      body: Buffer.from(JSON.stringify({
        scope: 'API_FIXTURE_PRESENTATION_ONLY', browser: context.browser()?.version() ?? await page.evaluate(() => navigator.userAgent),
        method: 'chrome.tabs.setZoom/getZoom', browserZoom, before, after, samples, pageErrors, consoleErrors,
        registry: { sourceFingerprint: registry.sourceFingerprint, selected: 'ui/common/StatusBadge' },
      }, null, 2)), contentType: 'application/json',
    })
  } finally {
    await context.close()
    rmSync(info.outputPath('browser-profile'), { recursive: true, force: true })
    rmSync(extension, { recursive: true, force: true })
  }
}
