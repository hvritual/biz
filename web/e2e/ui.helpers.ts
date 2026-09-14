import { expect, type Locator } from '@playwright/test'

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
