import { expect, test } from "@playwright/test";
import { readFileSync } from "node:fs";

interface Fixture {
  base_url: string;
  privacy_email: string;
  privacy_password: string;
}

function fixture(): Fixture {
  const path = process.env.CE12_E2E_ENV_FILE;
  if (!path) throw new Error("CE12_E2E_ENV_FILE is required");
  return JSON.parse(readFileSync(path, "utf8")) as Fixture;
}

async function authenticateToConsent(page: import("@playwright/test").Page, data: Fixture) {
  await page.goto(data.base_url + "/auth/login?return_to=/auth/session");
  await expect(page.getByRole("heading", { name: "CoffeeLink 登录" })).toBeVisible();
  await expect(page.getByRole("note")).toContainText("Cookie 提示");
  await expect(page.getByRole("link", { name: "隐私政策" })).toHaveAttribute("href", "https://example.invalid/privacy");
  await expect(page.getByRole("link", { name: "服务条款" })).toHaveAttribute("href", "https://example.invalid/terms");
  await page.getByLabel("邮箱").fill(data.privacy_email);
  await page.getByLabel("密码").fill(data.privacy_password);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByRole("heading", { name: "确认隐私与服务协议" })).toBeVisible();
  await expect(page.getByText("协议版本 ce12-v1")).toBeVisible();
}

test("TestEnterprise169BrowserPrivacyConsentGate", async ({ page, browser }) => {
  const data = fixture();

  await authenticateToConsent(page, data);

  const requestId = await page.locator('input[name="request_id"]').first().inputValue();
  const csrf = await page.locator('input[name="csrf_token"]').first().inputValue();

  const missingCheckbox = await page.evaluate(async ({ requestId, csrf }) => {
    const body = new URLSearchParams({
      request_id: requestId,
      csrf_token: csrf,
      agreement_version: "ce12-v1",
      action: "accept",
    });
    const response = await fetch("/idp/consent", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      credentials: "include",
      body,
    });
    return { status: response.status, text: await response.text() };
  }, { requestId, csrf });
  expect(missingCheckbox.status).toBe(422);
  expect(missingCheckbox.text).toContain("请先阅读并勾选同意当前协议");

  const tamperedVersion = await page.evaluate(async ({ requestId, csrf }) => {
    const body = new URLSearchParams({
      request_id: requestId,
      csrf_token: csrf,
      agreement_version: "tampered-v0",
      agreement_accepted: "true",
      action: "accept",
    });
    const response = await fetch("/idp/consent", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      credentials: "include",
      body,
    });
    return { status: response.status, text: await response.text() };
  }, { requestId, csrf });
  expect(tamperedVersion.status).toBe(409);
  expect(tamperedVersion.text).toContain("协议版本已更新");

  await page.getByRole("button", { name: "拒绝并返回登录" }).click();
  await expect(page.getByRole("heading", { name: "CoffeeLink 登录" })).toBeVisible();
  await expect(page.getByRole("alert")).toContainText("已拒绝当前协议，登录未继续");
  const rejectedCookies = await page.context().cookies(data.base_url);
  expect(rejectedCookies.some((cookie) => cookie.name === "biz_session" || cookie.name === "__Host-biz-session")).toBe(false);

  await authenticateToConsent(page, data);

  for (const viewport of [
    { width: 1366, height: 768, name: "1366x768" },
    { width: 1440, height: 900, name: "1440x900" },
    { width: 1536, height: 1024, name: "1536x1024" },
    { width: 390, height: 844, name: "390x844" },
  ]) {
    await page.setViewportSize({ width: viewport.width, height: viewport.height });
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
    expect(overflow, "horizontal overflow at " + viewport.name).toBe(false);
    await page.screenshot({ path: "test-results/enterprise-169-consent-" + viewport.name + ".png", fullPage: true });
  }

  await page.getByLabel(/我已阅读并同意/).check();
  await page.getByRole("button", { name: "同意并继续" }).click();
  await expect(page).toHaveURL(data.base_url + "/auth/session");
  const session = await page.evaluate(async (baseURL) => {
    const response = await fetch(baseURL + "/auth/session", { credentials: "include" });
    return response.json();
  }, data.base_url);
  expect(session.authenticated).toBe(true);
  expect(session.user_id).toBeTruthy();

  const repeatContext = await browser.newContext({ viewport: { width: 1366, height: 768 } });
  try {
    const repeat = await repeatContext.newPage();
    await repeat.goto(data.base_url + "/auth/login?return_to=/auth/session");
    await repeat.getByLabel("邮箱").fill(data.privacy_email);
    await repeat.getByLabel("密码").fill(data.privacy_password);
    await repeat.getByRole("button", { name: "登录" }).click();
    await expect(repeat).toHaveURL(data.base_url + "/auth/session");
    await expect(repeat.getByRole("heading", { name: "确认隐私与服务协议" })).toHaveCount(0);
  } finally {
    await repeatContext.close();
  }
});
