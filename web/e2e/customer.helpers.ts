import { expect, type Page, type Locator } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { createCustomerSeed } from '../src/services/customer/seed'
const dir = resolve('screenshots/customer-ui')
export async function ready(page: Page, path = '/customers') {
  await page.goto('/#' + path)
  await expect(page.locator('.customer-area h1').first()).toBeVisible()
  await page.evaluate(() => document.fonts.ready)
  await expect(page.locator('vite-error-overlay')).toHaveCount(0)
}
export async function action(page: Page, path: string, kind: string) {
  await ready(page, path + (path.includes('?') ? '&' : '?') + 'action=' + kind)
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  return dialog
}
export async function fill(dialog: Locator, values: Record<string, string | boolean>) {
  for (const [key, value] of Object.entries(values)) {
    const input = dialog.locator(`[data-field="${key}"]`).locator('input,select,textarea')
    if (typeof value === 'boolean') await input.setChecked(value)
    else if ((await input.evaluate((e) => e.tagName)) === 'SELECT') await input.selectOption(value)
    else await input.fill(value)
  }
}
export async function commit(dialog: Locator) {
  await dialog.locator('button[form="customer-action-form"]').click()
  await expect(dialog).toHaveCount(0)
}
export async function snap(page: Page, id: string) {
  mkdirSync(dir, { recursive: true })
  await page.evaluate(() => document.fonts.ready)
  const body = page.getByRole('dialog').locator('.dialog-body')
  if (await body.count()) {
    const metrics = await body.evaluate((el) => ({
      height: el.clientHeight,
      total: el.scrollHeight,
      top: el.scrollTop,
    }))
    await body.evaluate((el) => {
      el.scrollTop = 0
    })
    await page.screenshot({ path: `${dir}/${id}.png`, fullPage: false, animations: 'disabled' })
    if (metrics.total > metrics.height + 8) {
      for (
        let top = metrics.height - 100, part = 2;
        top < metrics.total;
        top += metrics.height - 100, part++
      ) {
        await body.evaluate((el, y) => {
          el.scrollTop = y
        }, top)
        await page.screenshot({
          path: `${dir}/${id}-detail-${part}.png`,
          fullPage: false,
          animations: 'disabled',
        })
        if (top + metrics.height >= metrics.total) break
      }
    }
    await body.evaluate((el, top) => {
      el.scrollTop = top
    }, metrics.top)
  } else await page.screenshot({ path: `${dir}/${id}.png`, fullPage: true, animations: 'disabled' })
}
export async function state(page: Page) {
  const raw = await page.evaluate(() => localStorage.getItem('coffeelink:customer-preview:v1:shanghai'))
  return raw ? JSON.parse(raw) : createCustomerSeed('shanghai')
}
export async function accept(page: Page, id: string, evidence: string[], transition = false) {
  if (transition) {
    const d = await action(page, '/customers/work/' + id, 'transition')
    await fill(d, { status: '待验收', nextAction: '核对业务来源并完成验收' })
    await commit(d)
  }
  const d = await action(page, '/customers/work/' + id, 'accept')
  for (const source of evidence) await d.getByRole('checkbox', { name: source, exact: true }).check()
  await fill(d, { reason: '已核对同一客户与事项范围内的有效业务来源，验收通过。', confirm: true })
  await commit(d)
  await expect(page.getByText('事项已结束 · 成功', { exact: true })).toBeVisible()
}
