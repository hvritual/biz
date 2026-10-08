import { expect, type Locator, type Page, type TestInfo } from '@playwright/test'

export type InitialSubscriptionObservation = (stage: string, anchor: Locator) => Promise<void>

/** Use actual Tab traversal: locator.focus()/click() would hide keyboard traps. */
export async function tabToInitialControl(page: Page, control: Locator) {
  await expect(control).toBeEnabled()
  for (let step = 0; step < 70; step++) {
    if (await control.evaluate((element) => element === document.activeElement)) {
      await expect(control).toBeFocused()
      await expect(control).toBeInViewport()
      return
    }
    await page.keyboard.press('Tab')
  }
  throw new Error(`Initial subscription control was not reachable by Tab: ${await control.getAttribute('id') ?? await control.textContent()}`)
}

export function observeInitialSubscription(page: Page, info: TestInfo, prefix: string): InitialSubscriptionObservation {
  return async (stage, anchor) => {
    await anchor.scrollIntoViewIfNeeded()
    await expect(anchor).toBeInViewport()
    const rendered = await page.evaluate(() => ({
      width: window.innerWidth, height: window.innerHeight,
      dpr: window.devicePixelRatio,
      cssZoom: getComputedStyle(document.documentElement).zoom,
      scrollWidth: document.documentElement.scrollWidth,
      horizontalOverflow: document.documentElement.scrollWidth > window.innerWidth + 1,
    }))
    expect(rendered.horizontalOverflow, `${stage}: page must not require horizontal scrolling`).toBe(false)
    const workspace = page.getByTestId('platform-initial-subscription')
    const inspected = page.locator('.initial-card, .subscription-card')
    if (await inspected.count()) {
      const clippedText = await inspected.locator('h2, h3, p, strong, small, .processing-state span, .verification-state span').evaluateAll((elements) => elements
        .filter((element) => element.clientWidth > 0 && element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.textContent))
      expect(clippedText, `${stage}: required text must wrap instead of being clipped`).toEqual([])
      if (rendered.width <= 680 && await workspace.count()) {
        const inputSizes = await workspace.locator('input:not([type="radio"]):not([type="checkbox"]), textarea').evaluateAll((elements) => elements.map((element) => parseFloat(getComputedStyle(element).fontSize)))
        expect(inputSizes.every((size) => size >= 16), `${stage}: mobile form text`).toBe(true)
      }
    }
    await page.screenshot({ path: info.outputPath(`ce340-${prefix}-${stage}.png`), fullPage: true })
    await info.attach(`ce340-${prefix}-${stage}`, {
      body: Buffer.from(JSON.stringify({ scope: 'API_FIXTURE_PRESENTATION_ONLY', stage, ...rendered })),
      contentType: 'application/json',
    })
  }
}
