import { expect, type Locator, type Page } from '@playwright/test'

export async function selectUiOption(control: Locator, value: string | number) {
  await control.click()
  const options = control.page().locator('[data-slot="select-item"]')
  const expected = String(value)
  const index = await options.evaluateAll(
    (elements, wanted) => elements.findIndex((element) => element.getAttribute('data-ui-option-value') === wanted),
    expected,
  )
  expect(index, `UI option value not found: ${expected}`).toBeGreaterThanOrEqual(0)
  await options.nth(index).click()
}

export async function installApiFailFast(page: Page) {
  await page.route(/\/(?:api\/)?(?:auth|v1)\//, async (route) => {
    const request = route.request()
    throw new Error(`Unhandled API request: ${request.method()} ${new URL(request.url()).pathname}`)
  })
}
