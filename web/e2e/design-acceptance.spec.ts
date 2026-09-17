import { expect, test, type Page } from '@playwright/test'
import { mkdirSync } from 'node:fs'

const viewports = [
  { width: 1366, height: 768 },
  { width: 1440, height: 900 },
  { width: 1536, height: 1024 },
  { width: 390, height: 844 },
]

async function useZhLocale(page: Page) {
  await page.addInitScript(() => localStorage.setItem('coffeelink.locale', 'zh-CN'))
}

async function accessibilitySmoke(page: Page) {
  return page.evaluate(() => {
    const visible = (element: Element) => {
      const node = element as HTMLElement
      const style = getComputedStyle(node)
      const rect = node.getBoundingClientRect()
      return style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0
    }
    const issues: string[] = []
    const ids = [...document.querySelectorAll<HTMLElement>('[id]')].map((node) => node.id)
    for (const id of new Set(ids)) {
      if (ids.filter((value) => value === id).length > 1) issues.push(`duplicate-id:${id}`)
    }

    for (const element of document.querySelectorAll<HTMLElement>('[aria-labelledby],[aria-describedby]')) {
      for (const name of ['aria-labelledby', 'aria-describedby']) {
        for (const id of (element.getAttribute(name) ?? '').split(/\s+/).filter(Boolean)) {
          if (!document.getElementById(id)) issues.push(`broken-${name}:${id}`)
        }
      }
    }

    for (const element of document.querySelectorAll<HTMLElement>('[aria-controls]')) {
      const expanded = element.getAttribute('aria-expanded')
      for (const id of (element.getAttribute('aria-controls') ?? '').split(/\s+/).filter(Boolean)) {
        const target = document.getElementById(id)
        // aria-controls may reference a conditionally mounted controlled surface while collapsed.
        // Once the control claims that surface is expanded, the target must exist.
        if (!target && expanded === 'true') issues.push(`broken-aria-controls:${id}`)
        if (!target && expanded == null) issues.push(`broken-aria-controls:${id}`)
      }
    }

    for (const element of document.querySelectorAll<HTMLElement>('button,a[href],input,select,textarea')) {
      if (!visible(element) || element.closest('[inert]')) continue
      const control = element as HTMLInputElement
      const labels = 'labels' in control ? control.labels?.length ?? 0 : 0
      const name = element.getAttribute('aria-label')
        || element.getAttribute('aria-labelledby')
        || element.textContent?.trim()
        || element.getAttribute('title')
        || (labels ? 'labelled' : '')
      if (!name) issues.push(`unnamed:${element.tagName.toLowerCase()}.${element.className}`)
    }
    for (const image of document.querySelectorAll<HTMLImageElement>('img')) {
      if (visible(image) && !image.hasAttribute('alt')) issues.push(`missing-alt:${image.src}`)
    }
    if (document.querySelectorAll('h1').length !== 1) issues.push(`h1-count:${document.querySelectorAll('h1').length}`)
    return [...new Set(issues)]
  })
}

test('joined navigation is an overlay, closes by outside click/Escape and restores focus', async ({ page }) => {
  await useZhLocale(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/#/enterprise/members')
  const main = page.getByTestId('main-content')
  const before = await main.boundingBox()
  const enterprise = page.getByRole('button', { name: '企业中心', exact: true })
  await enterprise.click()
  const panel = page.locator('.module-panel')
  await expect(panel).toBeVisible()
  const panelBox = await panel.boundingBox()
  expect(Math.abs((panelBox?.width ?? 0) - 480)).toBeLessThanOrEqual(1)
  const open = await main.boundingBox()
  expect(Math.abs((open?.x ?? 0) - (before?.x ?? 0))).toBeLessThanOrEqual(1)
  expect(Math.abs((open?.width ?? 0) - (before?.width ?? 0))).toBeLessThanOrEqual(1)

  const scrim = page.getByRole('button', { name: '关闭悬浮菜单', exact: true })
  const scrimBox = await scrim.boundingBox()
  expect(scrimBox).not.toBeNull()
  expect(panelBox).not.toBeNull()
  // Click the exposed right edge of the scrim rather than a coordinate covered by the joined menu.
  const outsideX = scrimBox!.x + scrimBox!.width - 12
  const outsideY = scrimBox!.y + 24
  expect(outsideX).toBeGreaterThan(panelBox!.x + panelBox!.width)
  await page.mouse.click(outsideX, outsideY)
  await expect(panel).toHaveCount(0)
  await expect(enterprise).toBeFocused()

  await enterprise.click()
  await page.keyboard.press('Escape')
  await expect(panel).toHaveCount(0)
  await expect(enterprise).toBeFocused()

  await page.getByRole('button', { name: '收起一级菜单', exact: true }).click()
  const collapsed = await main.boundingBox()
  await enterprise.click()
  const collapsedOpen = await main.boundingBox()
  expect(Math.abs((collapsedOpen?.x ?? 0) - (collapsed?.x ?? 0))).toBeLessThanOrEqual(1)
  expect(Math.abs((collapsedOpen?.width ?? 0) - (collapsed?.width ?? 0))).toBeLessThanOrEqual(1)
})

test('member view is not selection and detail close preserves applied query and focus', async ({ page }) => {
  await useZhLocale(page)
  await page.goto('/#/enterprise/members')
  const main = page.getByTestId('main-content')
  const search = main.getByLabel('搜索成员', { exact: true })
  await search.fill('张')
  await main.getByRole('button', { name: '查询', exact: true }).click()
  const view = main.getByRole('button', { name: /^查看 / }).first()
  const row = view.locator('xpath=ancestor::tr')
  const checkbox = row.getByRole('checkbox')
  await view.scrollIntoViewIfNeeded()
  const scroll = await page.evaluate(() => window.scrollY)
  await view.click()
  await expect(checkbox).not.toBeChecked()
  await expect(page.getByRole('heading', { name: '成员详情', exact: true })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(view).toBeFocused()
  await expect(search).toHaveValue('张')
  expect(Math.abs((await page.evaluate(() => window.scrollY)) - scroll)).toBeLessThan(4)
  await expect(checkbox).not.toBeChecked()
})

test('no-results and permission/session states are explicit, not fake data', async ({ page }) => {
  await useZhLocale(page)
  await page.goto('/#/enterprise/members')
  const main = page.getByTestId('main-content')
  await main.getByLabel('搜索成员', { exact: true }).fill('definitely-no-such-member-9f9d')
  await main.getByRole('button', { name: '查询', exact: true }).click()
  await expect(main.locator('.member-data-panel .empty-state')).toBeVisible()
  await page.goto('/#/platform/tenants')
  await expect(page.getByText('尚未建立平台可信会话', { exact: true })).toBeVisible()
  await expect(page.getByText(/不展示示例租户数据/)).toBeVisible()
})

test('automated accessibility smoke has names, stable ARIA relationships and one page heading', async ({ page }) => {
  await useZhLocale(page)
  for (const path of ['/enterprise/members', '/platform/tenants']) {
    await page.goto(`/#${path}`)
    expect(await accessibilitySmoke(page)).toEqual([])
  }
})

for (const target of [
  { path: '/enterprise/members', slug: 'members' },
  { path: '/platform/tenants', slug: 'tenants' },
]) {
  for (const variant of [
    { locale: 'zh-CN', theme: 'blue' },
    { locale: 'en-US', theme: 'violet' },
  ]) {
    test(`acceptance matrix ${target.slug} ${variant.locale} ${variant.theme}`, async ({ page }) => {
      mkdirSync('screenshots/design-acceptance', { recursive: true })
      await page.clock.setFixedTime(new Date('2026-09-16T12:00:00Z'))
      await page.addInitScript(({ locale, theme }) => {
        localStorage.setItem('coffeelink.locale', locale)
        localStorage.setItem('coffeelink.ui-theme', JSON.stringify({ name: theme }))
      }, variant)
      for (const viewport of viewports) {
        await page.setViewportSize(viewport)
        await page.goto(`/#${target.path}`)
        await page.evaluate(() => document.fonts.ready)
        expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width)
        expect(await accessibilitySmoke(page)).toEqual([])
        await page.screenshot({
          path: `screenshots/design-acceptance/${target.slug}-${variant.locale}-${variant.theme}-${viewport.width}.png`,
          animations: 'disabled',
          fullPage: false,
        })
      }
    })
  }
}
