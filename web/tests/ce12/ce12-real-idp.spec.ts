import { expect, test, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";

interface Fixture {
  base_url: string;
  discovery_url: string;
  email: string;
  password: string;
  allowed_tenant: string;
  iam_denied_tenant: string;
  entitlement_denied_tenant: string;
}

interface SessionView {
  authenticated: boolean;
  actor_kind?: string;
  user_id?: string;
  active_tenant_id?: string;
  csrf_token?: string;
  tenants?: Array<{ id: string; name: string }>;
}

interface BrowserResult {
  status: number;
  text: string;
  json: unknown;
}

function fixture(): Fixture {
  const path = process.env.CE12_E2E_ENV_FILE;
  if (!path) throw new Error("CE12_E2E_ENV_FILE is required");
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

function sessionFrom(result: BrowserResult): SessionView {
  expect(result.status).toBe(200);
  return result.json as SessionView;
}

function entitlementDecision(result: BrowserResult, key: string): { allowed: boolean; reason: string } {
  expect(result.status, result.text).toBe(200);
  const view = result.json as {
    decisions?: Array<{ key?: string; allowed?: boolean; reason?: string }>;
  };
  const decision = view.decisions?.find((candidate) => candidate.key === key);
  expect(decision, `missing entitlement decision for ${key}: ${result.text}`).toBeTruthy();
  return { allowed: Boolean(decision?.allowed), reason: String(decision?.reason ?? "") };
}

test("TestCE12BrowserRealIdPToSessionTenantIAMEntitlement", async ({ page, request }) => {
  const data = fixture();

  const discovery = await request.get(data.discovery_url);
  expect(discovery.status()).toBe(200);
  const metadata = (await discovery.json()) as {
    issuer: string;
    authorization_endpoint: string;
    token_endpoint: string;
    jwks_uri: string;
    code_challenge_methods_supported: string[];
  };
  expect(metadata.issuer).toBe("http://127.0.0.1:18081/idp");
  expect(metadata.authorization_endpoint).toBe(metadata.issuer + "/authorize");
  expect(metadata.token_endpoint).toBe(metadata.issuer + "/token");
  expect(metadata.jwks_uri).toBe(metadata.issuer + "/jwks");
  expect(metadata.code_challenge_methods_supported).toContain("S256");

  await page.goto(data.base_url + "/auth/login?return_to=/auth/session");
  await expect(page).toHaveURL(/127\.0\.0\.1:18081\/idp\/authorize/);
  await expect(page.getByRole("heading", { name: "CoffeeLink 登录" })).toBeVisible();
  await page.screenshot({ path: "test-results/ce12-idp-login.png", fullPage: true });

  await page.getByLabel("邮箱").fill(data.email);
  await page.getByLabel("密码").fill(data.password);
  await page.getByRole("button", { name: "登录" }).click();

  await expect(page).toHaveURL(data.base_url + "/auth/session");
  const initial = sessionFrom(await browserRequest(page, data.base_url, "/auth/session"));
  expect(initial.authenticated).toBe(true);
  expect(initial.actor_kind).toBe("user");
  expect(initial.user_id).toBeTruthy();
  expect(initial.active_tenant_id ?? "").toBe("");
  expect(initial.csrf_token).toBeTruthy();
  expect(initial.tenants?.map((tenant) => tenant.id).sort()).toEqual(
    [data.allowed_tenant, data.iam_denied_tenant, data.entitlement_denied_tenant].sort(),
  );

  // Browser-supplied tenant headers are not authority. Until the server session
  // has a validated active tenant, /v1 remains unauthenticated for tenant work.
  const spoofed = await browserRequest(page, data.base_url, "/v1/devices", {
    headers: { "X-Tenant-ID": data.allowed_tenant, "X-Platform": "true" },
  });
  expect(spoofed.status, spoofed.text).toBe(401);

  const csrf = initial.csrf_token!;

  // Tenant selection succeeds because membership is valid, but IAM fails first:
  // this tenant deliberately does not grant device.read to the user's role.
  const iamSwitch = await browserRequest(page, data.base_url, "/auth/session/tenant", {
    method: "POST",
    headers: { "X-CSRF-Token": csrf },
    body: { tenant_id: data.iam_denied_tenant },
  });
  expect(iamSwitch.status, iamSwitch.text).toBe(200);
  const iamDenied = await browserRequest(page, data.base_url, "/v1/devices");
  expect(iamDenied.status, iamDenied.text).toBe(403);

  // Here IAM is present, while Commercial Entitlement explicitly denies
  // device.lifecycle. The entitlement view proves the second gate independently.
  const entitlementSwitch = await browserRequest(page, data.base_url, "/auth/session/tenant", {
    method: "POST",
    headers: { "X-CSRF-Token": csrf },
    body: { tenant_id: data.entitlement_denied_tenant },
  });
  expect(entitlementSwitch.status, entitlementSwitch.text).toBe(200);
  const deniedView = await browserRequest(page, data.base_url, "/v1/tenant/entitlements", {
    method: "POST",
    headers: { "X-CSRF-Token": csrf },
    body: { capability_codes: ["device.lifecycle"] },
  });
  const deniedDecision = entitlementDecision(deniedView, "device.lifecycle");
  expect(deniedDecision.allowed, deniedDecision.reason).toBe(false);
  const commerciallyDenied = await browserRequest(page, data.base_url, "/v1/devices");
  expect(commerciallyDenied.status, commerciallyDenied.text).toBe(403);
  expect(commerciallyDenied.text).toContain("device.list");

  // Positive closure: same authenticated member switches to the tenant where
  // both IAM device.read and device.lifecycle entitlement are effective.
  const allowedSwitch = await browserRequest(page, data.base_url, "/auth/session/tenant", {
    method: "POST",
    headers: { "X-CSRF-Token": csrf },
    body: { tenant_id: data.allowed_tenant },
  });
  expect(allowedSwitch.status, allowedSwitch.text).toBe(200);
  const allowedView = await browserRequest(page, data.base_url, "/v1/tenant/entitlements", {
    method: "POST",
    headers: { "X-CSRF-Token": csrf },
    body: { capability_codes: ["device.lifecycle"] },
  });
  const allowedDecision = entitlementDecision(allowedView, "device.lifecycle");
  expect(allowedDecision.allowed, allowedDecision.reason).toBe(true);

  const devices = await browserRequest(page, data.base_url, "/v1/devices");
  expect(devices.status, devices.text).toBe(200);
  expect(devices.json).toEqual(expect.any(Object));
  const deviceView = devices.json as { devices?: unknown[] };
  expect(deviceView.devices ?? []).toEqual(expect.any(Array));

  const finalSession = sessionFrom(await browserRequest(page, data.base_url, "/auth/session"));
  expect(finalSession.active_tenant_id).toBe(data.allowed_tenant);
  await page.screenshot({ path: "test-results/ce12-session-established.png", fullPage: true });

  const logout = await browserRequest(page, data.base_url, "/auth/logout", {
    method: "POST",
    headers: { "X-CSRF-Token": csrf },
  });
  expect(logout.status, logout.text).toBe(204);

  const afterLogout = sessionFrom(await browserRequest(page, data.base_url, "/auth/session"));
  expect(afterLogout.authenticated).toBe(false);
  const reused = await browserRequest(page, data.base_url, "/v1/devices");
  expect(reused.status, reused.text).toBe(401);
});
