import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";

interface Fixture {
  base_url: string;
  discovery_url: string;
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
  platform_oidc_issuer: string;
}

interface SessionView {
  authenticated: boolean;
  actor_kind?: string;
  user_id?: string;
  platform_subject?: string;
  active_tenant_id?: string;
  csrf_token?: string;
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

async function browserRequest(
  page: Page,
  baseURL: string,
  path: string,
  init?: { method?: string; headers?: Record<string, string>; body?: unknown },
): Promise<BrowserResult> {
  return page.evaluate(
    async ({ baseURL, path, init }) => {
      const headers = new Headers(init?.headers ?? {});
      if (init?.body !== undefined) headers.set("Content-Type", "application/json");
      const response = await fetch(baseURL + path, {
        method: init?.method ?? "GET",
        headers,
        credentials: "include",
        body: init?.body === undefined ? undefined : JSON.stringify(init.body),
      });
      const text = await response.text();
      let json: unknown = null;
      try {
        json = text ? JSON.parse(text) : null;
      } catch {
        json = null;
      }
      return { status: response.status, text, json };
    },
    { baseURL, path, init },
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
  await expect(page).toHaveURL(/127\.0\.0\.1:18081\/idp\/authorize/);
  await page.getByLabel("邮箱").fill(email);
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(data.base_url + "/auth/session");
  const result = await browserRequest(page, data.base_url, "/auth/session");
  expect(result.status, result.text).toBe(200);
  return { context, page, session: result.json as SessionView };
}

test("TestCE13PlatformCommercialTrustedWebSession", async ({ browser, request }) => {
  const data = fixture();

  const discovery = await request.get(data.discovery_url);
  expect(discovery.status()).toBe(200);
  const metadata = (await discovery.json()) as { issuer: string };
  expect(metadata.issuer).toBe(data.platform_oidc_issuer);

  // Existing controlled API-key callers remain valid after browser support is added.
  const apiKeyResponse = await request.get(data.base_url + "/v1/platform/modules", {
    headers: { Authorization: `Bearer ${data.allowed_api_key}` },
  });
  expect(apiKeyResponse.status(), await apiKeyResponse.text()).toBe(200);

  // A tenant browser identity cannot self-assert platform authority through headers.
  const tenant = await login(browser, data, data.tenant_email, data.tenant_password);
  expect(tenant.session.authenticated).toBe(true);
  expect(tenant.session.actor_kind).toBe("user");
  expect(tenant.session.active_tenant_id).toBe(data.tenant_id);
  const spoofed = await browserRequest(tenant.page, data.base_url, "/v1/platform/modules", {
    headers: {
      "X-Tenant-ID": data.tenant_id,
      "X-Platform": "true",
      "X-Principal": data.allowed_subject,
    },
  });
  expect(spoofed.status, spoofed.text).toBe(403);
  await tenant.context.close();

  // A real platform OIDC session without platform.module.read remains denied.
  const denied = await login(browser, data, data.denied_email, data.denied_password);
  expect(denied.session.authenticated).toBe(true);
  expect(denied.session.actor_kind).toBe("platform");
  expect(denied.session.platform_subject).toBe(data.denied_subject);
  expect(denied.session.active_tenant_id ?? "").toBe("");
  const deniedModules = await browserRequest(denied.page, data.base_url, "/v1/platform/modules");
  expect(deniedModules.status, deniedModules.text).toBe(403);
  await denied.context.close();

  // Positive closure: the trusted platform session is tenantless, keeps its
  // web-session provenance and reaches the real module catalog through the
  // server-side platform permission resolver.
  const allowed = await login(browser, data, data.allowed_email, data.allowed_password);
  expect(allowed.session.authenticated).toBe(true);
  expect(allowed.session.actor_kind).toBe("platform");
  expect(allowed.session.platform_subject).toBe(data.allowed_subject);
  expect(allowed.session.active_tenant_id ?? "").toBe("");
  expect(allowed.session.csrf_token).toBeTruthy();

  const modules = await browserRequest(allowed.page, data.base_url, "/v1/platform/modules");
  expect(modules.status, modules.text).toBe(200);
  const view = modules.json as { modules?: unknown[] };
  expect(view.modules).toEqual(expect.any(Array));
  expect((view.modules ?? []).length).toBeGreaterThan(0);

  // Unsafe platform operations still require the CE-12 CSRF boundary even
  // when the operation itself admits web-session authentication.
  const noCSRF = await browserRequest(allowed.page, data.base_url, "/v1/platform/modules", {
    method: "POST",
    body: {
      request_id: "ce13-no-csrf",
      module_code: "ce13-no-csrf",
      name: "CE13 no CSRF",
      category: "test",
      sales_scope: ["default"],
      reason: "must be rejected before mutation",
    },
  });
  expect(noCSRF.status, noCSRF.text).toBe(401);
  await allowed.context.close();
});
