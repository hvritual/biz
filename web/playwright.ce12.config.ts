import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/ce12",
  fullyParallel: false,
  workers: 1,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"], ["json", { outputFile: process.env.CE12_PLAYWRIGHT_JSON_OUTPUT_FILE ?? "test-results/ce12-results.json" }]],
  use: {
    baseURL: process.env.CE12_BIZ_BASE_URL ?? "http://127.0.0.1:18080",
    viewport: { width: 1366, height: 768 },
    locale: "zh-CN",
    timezoneId: "Asia/Shanghai",
    screenshot: "only-on-failure",
    // Passing CE12 journeys already persist explicit acceptance screenshots.
    // Record heavyweight trace/video only on the retry after an initial
    // failure, preserving diagnostics without taxing every successful test.
    trace: "on-first-retry",
    video: "on-first-retry",
    launchOptions: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH
      ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH }
      : {},
  },
});
