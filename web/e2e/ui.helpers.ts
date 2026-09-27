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
  // The global AppHeader always reads the #186 in-app unread count in API
  // mode. Keep that infrastructure request explicit in unrelated browser
  // fixtures while still failing every other unhandled auth/v1 request.
  await page.route('**/api/auth/personal/in-app-notifications', async (route) => {
    const request = route.request()
    if (request.method() !== 'GET') {
      throw new Error(`Unhandled notification inbox mutation: ${request.method()}`)
    }
    const rawContext = request.headers()['x-biz-session-context'] ?? ''
    let context: { active_tenant_id?: string; user_id?: string } = {}
    try {
      context = JSON.parse(rawContext) as typeof context
    } catch {
      throw new Error('Notification inbox request missing trusted session context')
    }
    if (!context.active_tenant_id || !context.user_id) {
      throw new Error('Notification inbox request has incomplete trusted session context')
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        tenant_id: context.active_tenant_id,
        user_id: context.user_id,
        unread_count: 0,
        messages: [],
        as_of: '2026-09-27T00:00:00Z',
      }),
    })
  })
  await page.route(/\/(?:api\/)?(?:auth|v1)\//, async (route) => {
    const request = route.request()
    throw new Error(`Unhandled API request: ${request.method()} ${new URL(request.url()).pathname}`)
  })
}

export async function installUnauthenticatedSession(page: Page) {
  await page.route('**/api/auth/session', async (route) => {
    await route.fulfill({ json: { authenticated: false } })
  })
}
