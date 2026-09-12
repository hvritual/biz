import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";

interface Fixture {
  base_url: string;
  allowed_email: string;
  allowed_password: string;
  denied_email: string;
  denied_password: string;
  tenant_email: string;
  tenant_password: string;
  tenant_id: string;
  allowed_api_key: string;
  allowed_subject: string;
  denied_subject: string;
}

interface SessionView {
  authenticated: boolean;
  actor_kind?: string;
  platform_subject?: string;
  active_tenant_id?: string;
}

interface CatalogEntry {
  planCode?: string;
  latestVersion?: string | number;
  latestRevision?: string | number;
  planRevision?: string | number;
  state?: string;
  name?: string;
}

interface CatalogPage {
  plans?: CatalogEntry[];
  nextAfterPlanCode?: string;
}

interface BrowserResult {
  status: number;
  text: string;
  json: unknown;
}

function fixture(): Fixture {
  const path = process.env.CE13_PLATFORM_E2E_ENV_FILE;
  if (!path) throw new Error("CE13_PLATFORM_E2E_ENV_FILE is required");
  return JSON.parse(readFileSync(path, "utf8")) as Fixture;
}

async function browserRequest(page: Page, baseURL: string, path: string): Promise<BrowserResult> {
  return page.evaluate(
    async ({ baseURL, path }) => {
      const response = await fetch(baseURL + path, { credentials: "include" });
      const text = await response.text();
      let json: unknown = null;
      try {
        json = text ? JSON.parse(text) : null;
      } catch {
        json = null;
      }
      return { status: response.status, text, json };
    },
    { baseURL, path },
  );
}

async function login(
  browser: Browser,
  data: Fixture,
  email: string,
  password: string,
): Promise<{ context: BrowserContext; page: Page; session: SessionView }> {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto(data.base_url + "/auth/login?return_to=/auth/session");
  await expect(page).toHaveURL(/\/idp\/authorize/);
  await page.getByLabel("邮箱").fill(email);
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(data.base_url + "/auth/session");
  const result = await browserRequest(page, data.base_url, "/auth/session");
  expect(result.status, result.text).toBe(200);
  return { context, page, session: result.json as SessionView };
}

test("TestCE13PlanCatalogTrustedPlatformDiscovery", async ({ browser, request }) => {
  const data = fixture();

  const apiKey = await request.get(data.base_url + "/v1/platform/plans?page_size=1", {
    headers: { Authorization: `Bearer ${data.allowed_api_key}` },
  });
  expect(apiKey.status(), await apiKey.text()).toBe(200);
  const apiKeyPage = (await apiKey.json()) as CatalogPage;
  expect(apiKeyPage.plans).toEqual(expect.any(Array));
  expect((apiKeyPage.plans ?? []).length).toBe(1);

  const tenant = await login(browser, data, data.tenant_email, data.tenant_password);
  expect(tenant.session.authenticated).toBe(true);
  expect(tenant.session.actor_kind).toBe("user");
  expect(tenant.session.active_tenant_id).toBe(data.tenant_id);
  const tenantCatalog = await browserRequest(tenant.page, data.base_url, "/v1/platform/plans?page_size=1");
  expect(tenantCatalog.status, tenantCatalog.text).toBe(403);
  await tenant.context.close();

  const denied = await login(browser, data, data.denied_email, data.denied_password);
  expect(denied.session.authenticated).toBe(true);
  expect(denied.session.actor_kind).toBe("platform");
  expect(denied.session.platform_subject).toBe(data.denied_subject);
  const deniedCatalog = await browserRequest(denied.page, data.base_url, "/v1/platform/plans?page_size=1");
  expect(deniedCatalog.status, deniedCatalog.text).toBe(403);
  await denied.context.close();

  const allowed = await login(browser, data, data.allowed_email, data.allowed_password);
  expect(allowed.session.authenticated).toBe(true);
  expect(allowed.session.actor_kind).toBe("platform");
  expect(allowed.session.platform_subject).toBe(data.allowed_subject);
  expect(allowed.session.active_tenant_id ?? "").toBe("");

  const firstResult = await browserRequest(allowed.page, data.base_url, "/v1/platform/plans?page_size=1");
  expect(firstResult.status, firstResult.text).toBe(200);
  const first = firstResult.json as CatalogPage;
  expect(first.plans).toEqual(expect.any(Array));
  expect((first.plans ?? []).length).toBe(1);
  const firstEntry = first.plans?.[0];
  expect(firstEntry?.planCode).toBeTruthy();
  expect(firstEntry?.name).toBeTruthy();
  expect(Number(firstEntry?.latestVersion ?? 0)).toBeGreaterThan(0);
  expect(Number(firstEntry?.latestRevision ?? 0)).toBeGreaterThan(0);
  expect(Number(firstEntry?.planRevision ?? 0)).toBeGreaterThan(0);
  expect(firstEntry?.state).toBeTruthy();

  if (first.nextAfterPlanCode) {
    expect(first.nextAfterPlanCode).toBe(firstEntry?.planCode);
    const secondResult = await browserRequest(
      allowed.page,
      data.base_url,
      `/v1/platform/plans?page_size=1&after_plan_code=${encodeURIComponent(first.nextAfterPlanCode)}`,
    );
    expect(secondResult.status, secondResult.text).toBe(200);
    const second = secondResult.json as CatalogPage;
    expect((second.plans ?? []).length).toBe(1);
    expect(second.plans?.[0]?.planCode).not.toBe(firstEntry?.planCode);
    expect((second.plans?.[0]?.planCode ?? "") > (firstEntry?.planCode ?? "")).toBe(true);
  }

  await allowed.context.close();
});
