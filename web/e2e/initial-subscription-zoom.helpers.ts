import { chromium, expect, type BrowserContext, type ConsoleMessage, type Page, type TestInfo } from '@playwright/test'
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import type { CurrentAuthorizationResponse } from '../src/services/runtime/api'
import { expectInitialSelectContentFits, type InitialSubscriptionObservation } from './initial-subscription-visual.helpers'

type Manifest = {
  services: Array<{
    fullName: string
    domain: string
    application: { name: string }
    methods?: Array<{
      name: string
      http?: Array<{ method: string; path: string }>
      operation?: { id: string; useCase: string; tenantRequired?: boolean; permissions?: string[]; permissionMode?: string; authentication?: string[] }
    }> | null
  }>
}

/** Platform session DTO and generated actions; grants mirror runtimeWebAuth. API fixture only. */
export async function installInitialPlatformSession(page: Page) {
  const required = [
    'commercial.module.list', 'commercial.plan.discover', 'commercial.plan.list', 'commercial.plan.get', 'commercial.plan.eligibility',
    'commercial.subscription.get', 'commercial.entitlement.override.list', 'commercial.entitlement.explain',
    'commercial.subscription.change.preview', 'commercial.subscription.change.confirm', 'commercial.subscription.change.get',
    'commercial.provisioning.task.get', 'commercial.provisioning.task.retry',
  ]
  const manifest = JSON.parse(readFileSync(new URL('../../contracts/generated/manifest.json', import.meta.url), 'utf8')) as Manifest
  const actions: CurrentAuthorizationResponse['actions'] = manifest.services.flatMap((service) =>
    (service.methods ?? []).flatMap(({ operation, name, http }) => {
      if (!operation || !required.includes(operation.id)) return []
      expect(operation.authentication).toContain('web-session')
      expect(Boolean(operation.tenantRequired)).toBe(false)
      return [{
        code: operation.id, domain: service.domain, application: service.application.name,
        use_case: operation.useCase, tenant_required: Boolean(operation.tenantRequired),
        authentication: operation.authentication ?? [], permissions: operation.permissions ?? [],
        permission_mode: operation.permissionMode ?? 'all',
        rpc: `/${service.fullName}/${name}`, http,
      }]
    }),
  )
  expect(actions.map((action) => action.code).sort()).toEqual([...required].sort())
  const grants = [...new Set(actions.flatMap((action) => action.permissions))].sort().map((permission) => ({
    permission, role_id: 'direct:platform-admin', role_name: '', scope: '',
  }))
  const snapshot: CurrentAuthorizationResponse = {
    authenticated: true, actor_kind: 'platform', platform_subject: 'platform-admin',
    roles: [], grants, data_policies: [], site_ids: [], modules: [], actions, button_codes: [...required].sort(),
  }
  await page.route('**/api/auth/session', (route) => route.fulfill({ json: {
    authenticated: true, actor_kind: 'platform', platform_subject: 'platform-admin',
    context_version: 7, csrf_token: 'csrf-initial-native',
  } }))
  await page.route('**/api/auth/authorization', (route) => route.fulfill({ json: snapshot }))
}

type ZoomTabs = {
  query: (query: { url: string }) => Promise<Array<{ id?: number }>>
  getZoom: (id: number) => Promise<number>
  setZoom: (id: number, factor: number) => Promise<void>
}

async function metrics(page: Page) {
  return page.evaluate(() => ({
    width: window.innerWidth, height: window.innerHeight,
    outerWidth: window.outerWidth, outerHeight: window.outerHeight,
    dpr: window.devicePixelRatio, cssZoom: getComputedStyle(document.documentElement).zoom,
    horizontalOverflow: document.documentElement.scrollWidth > window.innerWidth + 1,
  }))
}

/** Full Chromium's native tab zoom, following commercial-state-zoom.helpers.ts. */
export async function qualifyInitialSubscriptionZoom(info: TestInfo, run: (
  page: Page, afterNavigation: () => Promise<void>, observe: InitialSubscriptionObservation,
) => Promise<void>) {
  const extension = info.outputPath('initial-zoom-control')
  const profile = info.outputPath('initial-browser-profile')
  mkdirSync(extension, { recursive: true })
  writeFileSync(join(extension, 'manifest.json'), JSON.stringify({
    manifest_version: 3, name: 'First subscription zoom qualification', version: '1.0.0',
    host_permissions: ['http://127.0.0.1/*'], background: { service_worker: 'background.js' },
  }))
  writeFileSync(join(extension, 'background.js'), 'chrome.runtime.onInstalled.addListener(() => {});\n')
  let context: BrowserContext | undefined
  try {
    context = await chromium.launchPersistentContext(profile, {
      channel: 'chromium', headless: true, viewport: null,
      deviceScaleFactor: undefined, isMobile: undefined,
      baseURL: 'http://127.0.0.1:4173', locale: 'zh-CN', timezoneId: 'Asia/Shanghai',
      args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`, '--window-size=1440,1000'],
    })
    const worker = context.serviceWorkers()[0] ?? await context.waitForEvent('serviceworker', { timeout: 15000 })
    const page = await context.newPage()
    const pageErrors: string[] = []
    const consoleMessages: Array<{ type: string; text: string; location: ReturnType<ConsoleMessage['location']> }> = []
    page.on('pageerror', (error) => pageErrors.push(error.message))
    page.on('console', (message) => {
      if (['error', 'warning'].includes(message.type())) {
        consoleMessages.push({ type: message.type(), text: message.text(), location: message.location() })
      }
    })
    let before: Awaited<ReturnType<typeof metrics>> | undefined
    let zoomChange: { initial: number; final: number } | undefined
    let navigations = 0
    const zoom = async (set = false) => worker.evaluate(async (shouldSet) => {
      const tabs = (globalThis as unknown as { chrome: { tabs: ZoomTabs } }).chrome.tabs
      const matches = await tabs.query({ url: 'http://127.0.0.1:4173/*' })
      if (matches.length !== 1 || matches[0]?.id === undefined) throw new Error('Expected exactly one qualification tab')
      const id = matches[0].id
      const initial = await tabs.getZoom(id)
      if (shouldSet) await tabs.setZoom(id, 2)
      return { initial, final: await tabs.getZoom(id) }
    }, set)
    const verifyZoom = async () => {
      const browserZoom = await zoom()
      expect(browserZoom).toEqual({ initial: 2, final: 2 })
      const rendered = await metrics(page)
      if (!before) throw new Error('Native zoom was not initialized')
      expect(rendered.outerWidth).toBe(before.outerWidth)
      expect(rendered.outerHeight).toBe(before.outerHeight)
      expect(Math.abs(rendered.width * 2 - before.width)).toBeLessThanOrEqual(2)
      expect(Math.abs(rendered.height * 2 - before.height)).toBeLessThanOrEqual(2)
      expect(rendered.dpr / before.dpr).toBeCloseTo(2, 2)
      expect(rendered.cssZoom).toBe(before.cssZoom)
      expect(rendered.horizontalOverflow).toBe(false)
      return { ...rendered, browserZoom: browserZoom.final }
    }
    const afterNavigation = async () => {
      navigations += 1
      if (!before) {
        before = await metrics(page)
        zoomChange = await zoom(true)
        expect(zoomChange).toEqual({ initial: 1, final: 2 })
        await expect.poll(async () => (await metrics(page)).dpr / before!.dpr).toBeCloseTo(2, 2)
      }
      await verifyZoom()
    }
    const stages: unknown[] = []
    const observe: InitialSubscriptionObservation = async (stage, anchor) => {
      await anchor.scrollIntoViewIfNeeded()
      await expect(anchor).toBeInViewport()
      await expect(page).toHaveTitle(/CoffeeLink/)
      await expect(page.locator('vite-error-overlay')).toHaveCount(0)
      // Read getZoom again at every state, including the shared scenario's reload.
      const rendered = await verifyZoom()
      await expectInitialSelectContentFits(page, stage)
      const clippedText = await page.locator('.initial-card, .subscription-card')
        .locator('h2, h3, p, strong, small, .processing-state span, .verification-state span')
        .evaluateAll((elements) => elements.filter((element) => element.clientWidth > 0 && element.scrollWidth > element.clientWidth + 1)
          .map((element) => element.textContent))
      expect(clippedText, `${stage}: required text must not be clipped`).toEqual([])
      const devtools = await page.context().newCDPSession(page)
      try {
        // CSS clips crop the physical surface at native zoom; capture it directly.
        const capture = await devtools.send('Page.captureScreenshot', { format: 'png', fromSurface: true, captureBeyondViewport: false })
        const png = Buffer.from(capture.data, 'base64')
        const screenshot = { width: png.readUInt32BE(16), height: png.readUInt32BE(20) }
        expect(Math.abs(screenshot.width - rendered.width * rendered.dpr)).toBeLessThanOrEqual(2)
        expect(Math.abs(screenshot.height - rendered.height * rendered.dpr)).toBeLessThanOrEqual(2)
        writeFileSync(info.outputPath(`ce340-native-200-percent-${stage}.png`), png)
        stages.push({ stage, ...rendered, screenshot })
      } finally { await devtools.detach() }
    }
    await run(page, afterNavigation, observe)
    expect(navigations).toBe(2)
    expect(stages).toEqual(expect.arrayContaining([expect.objectContaining({ stage: 'provisioning-restored', browserZoom: 2 })]))
    expect(pageErrors).toEqual([])
    await info.attach('ce340-native-browser-zoom-evidence', {
      body: Buffer.from(JSON.stringify({
        scope: 'API_FIXTURE_PRESENTATION_ONLY', method: 'chrome.tabs.setZoom/getZoom',
        browser: context.browser()?.version(), zoomChange, before, after: await verifyZoom(), stages, pageErrors,
        // The shared recovery fixtures intentionally return 404 and 503.
        consoleMessages,
      }, null, 2)), contentType: 'application/json',
    })
  } finally {
    try { await context?.close() } finally {
      rmSync(profile, { recursive: true, force: true })
      rmSync(extension, { recursive: true, force: true })
    }
  }
}
