import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";

interface Fixture {
  base_url: string;
  web_base_url: string;
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
      const routedPath = path.startsWith("/v1/") ? "/api" + path : path;
      const response = await fetch(baseURL + routedPath, {
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
  await page.goto(data.web_base_url + "/auth/login?return_to=" + encodeURIComponent("/auth/session"));
  await expect(page).toHaveURL(/\/idp\/authorize/);
  await page.getByLabel("邮箱").fill(email);
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/auth\/session/);
  const result = await browserRequest(page, data.web_base_url, "/auth/session");
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
  const spoofed = await browserRequest(tenant.page, data.web_base_url, "/v1/platform/modules", {
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
  const deniedModules = await browserRequest(denied.page, data.web_base_url, "/v1/platform/modules");
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

  const modules = await browserRequest(allowed.page, data.web_base_url, "/v1/platform/modules");
  expect(modules.status, modules.text).toBe(200);
  const view = modules.json as { modules?: unknown[] };
  expect(view.modules).toEqual(expect.any(Array));
  expect((view.modules ?? []).length).toBeGreaterThan(0);

  // Unsafe platform operations still require the CE-12 CSRF boundary even
  // when the operation itself admits web-session authentication.
  const noCSRF = await browserRequest(allowed.page, data.web_base_url, "/v1/platform/modules", {
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

test("TestCE13PlatformCommercialLifecycleThroughTrustedWebSession", async ({ browser }) => {
  const data = fixture();
  const allowed = await login(browser, data, data.allowed_email, data.allowed_password);
  const csrf = allowed.session.csrf_token;
  expect(csrf).toBeTruthy();

  const requestID = (name: string) => `ce13-browser-${name}-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const write = (path: string, body: unknown, idempotencyKey: string, method = "POST") => browserRequest(
    allowed.page,
    data.web_base_url,
    path,
    {
      method,
      headers: { "X-CSRF-Token": String(csrf), "Idempotency-Key": idempotencyKey },
      body,
    },
  );

  // This creates an independent, immutable plan through the same cookie BFF
  // boundary the console uses. It deliberately carries no browser API key.
  const planCode = `ce13-browser-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
  const createID = requestID("plan-create");
  const created = await write("/v1/platform/plans", {
    requestId: createID,
    planCode,
    name: "CE-13 浏览器验收套餐",
    terms: {
      modules: [{ moduleCode: "device-operations", capabilityCodes: ["device.lifecycle"], quotas: [], fields: [] }],
      salesScope: ["default"],
      validityMode: "unlimited",
      validityDays: 0,
      priceRef: "",
    },
    reason: "CE-13 trusted web-session lifecycle acceptance",
  }, createID);
  expect(created.status, created.text).toBe(200);
  const draft = created.json as { planCode: string; version: number; revision: number; state: string };
  expect(draft.planCode).toBe(planCode);
  expect(draft.state).toBe("DRAFT");

  const staleRevision = draft.revision;
  const updateID = requestID("plan-update");
  const updated = await write(`/v1/platform/plans/${encodeURIComponent(planCode)}/versions/${draft.version}`, {
    requestId: updateID,
    planCode,
    version: draft.version,
    expectedRevision: staleRevision,
    name: "CE-13 浏览器验收套餐（已更新）",
    terms: {
      modules: [{ moduleCode: "device-operations", capabilityCodes: ["device.lifecycle"], quotas: [], fields: [] }],
      salesScope: ["default"], validityMode: "unlimited", validityDays: 0, priceRef: "",
    },
    reason: "exercise server revision CAS",
  }, updateID, "PATCH");
  expect(updated.status, updated.text).toBe(200);
  const currentDraft = updated.json as { revision: number };

  const conflictID = requestID("plan-conflict");
  const conflict = await write(`/v1/platform/plans/${encodeURIComponent(planCode)}/versions/${draft.version}`, {
    requestId: conflictID,
    planCode,
    version: draft.version,
    expectedRevision: staleRevision,
    name: "stale write must not win",
    terms: {
      modules: [{ moduleCode: "device-operations", capabilityCodes: ["device.lifecycle"], quotas: [], fields: [] }],
      salesScope: ["default"], validityMode: "unlimited", validityDays: 0, priceRef: "",
    },
    reason: "CE-13 stale version conflict acceptance",
  }, conflictID, "PATCH");
  expect(conflict.status, conflict.text).toBe(409);

  const publishID = requestID("plan-publish");
  const published = await write(`/v1/platform/plans/${encodeURIComponent(planCode)}/versions/${draft.version}/publish`, {
    requestId: publishID, planCode, version: draft.version, expectedRevision: currentDraft.revision,
    reason: "publish immutable browser acceptance version",
  }, publishID);
  expect(published.status, published.text).toBe(200);
  expect((published.json as { state: string }).state).toBe("PUBLISHED");

  // The tenant and base subscription are seeded through the API-key-only
  // bootstrap operation. Every CE-13 management action below is web-session.
  const subscription = await browserRequest(allowed.page, data.web_base_url, `/v1/platform/tenants/${data.tenant_id}/subscription`);
  expect(subscription.status, subscription.text).toBe(200);
  const source = await browserRequest(allowed.page, data.web_base_url, `/v1/platform/tenants/${data.tenant_id}/entitlement-overrides`);
  expect(source.status, source.text).toBe(200);
  const sourceVersion = (source.json as { sourceVersion: number }).sourceVersion;

  const overrideID = requestID("override-create");
  const override = await write(`/v1/platform/tenants/${data.tenant_id}/entitlement-overrides`, {
    requestId: overrideID, tenantId: data.tenant_id, expectedVersion: sourceVersion,
    moduleCode: "device-operations", target: "ENTITLEMENT_TARGET_CAPABILITY", key: "device.lifecycle",
    fieldAction: "", effect: "ENTITLEMENT_EFFECT_DENY", effectiveAt: "", expiresAt: "",
    reason: "CE-13 browser source and provenance acceptance",
  }, overrideID);
  expect(override.status, override.text).toBe(200);
  const overrideReceipt = override.json as { source: { id: string }; sourceVersion: number };
  expect(overrideReceipt.source.id).toBeTruthy();

  const explained = await write(`/v1/platform/tenants/${data.tenant_id}/entitlements`, {
    tenantId: data.tenant_id, capabilityCodes: ["device.lifecycle"],
  }, requestID("entitlement-explain"));
  expect(explained.status, explained.text).toBe(200);
  const decisions = (explained.json as { decisions?: Array<{ allowed: boolean; sources?: Array<{ sourceKind?: string }> }> }).decisions ?? [];
  expect(decisions.some((decision) => !decision.allowed && (decision.sources ?? []).some((item) => item.sourceKind === "override"))).toBe(true);

  const revokeID = requestID("override-revoke");
  const revoked = await write(`/v1/platform/tenants/${data.tenant_id}/entitlement-overrides/${encodeURIComponent(overrideReceipt.source.id)}/revoke`, {
    requestId: revokeID, tenantId: data.tenant_id, id: overrideReceipt.source.id,
    expectedVersion: overrideReceipt.sourceVersion, reason: "CE-13 browser revoke acceptance",
  }, revokeID);
  expect(revoked.status, revoked.text).toBe(200);

  const previewID = requestID("subscription-preview");
  const preview = await write(`/v1/platform/tenants/${data.tenant_id}/subscription/change-previews`, {
    requestId: previewID, tenantId: data.tenant_id, action: "SWITCH",
    targetPlanCode: planCode, targetPlanVersion: draft.version, effectiveAt: "",
    reason: "CE-13 manual change preview acceptance",
  }, previewID);
  expect(preview.status, preview.text).toBe(200);
  const previewDTO = preview.json as { changeId: string; previewHash: string };
  expect(previewDTO.changeId).toBeTruthy();
  expect(previewDTO.previewHash).toBeTruthy();

  const confirmID = requestID("subscription-confirm");
  const receipt = await write(`/v1/platform/tenants/${data.tenant_id}/subscription/changes/${encodeURIComponent(previewDTO.changeId)}/confirm`, {
    requestId: confirmID, tenantId: data.tenant_id, changeId: previewDTO.changeId,
    previewHash: previewDTO.previewHash, reason: "CE-13 platform manual approval",
  }, confirmID);
  expect(receipt.status, receipt.text).toBe(200);
  const receiptDTO = receipt.json as { changeId: string; status: string; pricingAuthority: string };
  expect(receiptDTO.changeId).toBe(previewDTO.changeId);
  expect(receiptDTO.status).toBeTruthy();
  expect(receiptDTO.pricingAuthority).toBe("PLATFORM_MANUAL_APPROVAL");

  const readback = await browserRequest(allowed.page, data.web_base_url, `/v1/platform/tenants/${data.tenant_id}/subscription/changes/${encodeURIComponent(previewDTO.changeId)}`);
  expect(readback.status, readback.text).toBe(200);
  expect((readback.json as { changeId: string }).changeId).toBe(previewDTO.changeId);
  await allowed.context.close();
});

test("TestCE13PlatformCommercialVisibleConsoleFlow", async ({ browser }, testInfo) => {
  const data = fixture();
  const context = await browser.newContext({ viewport: { width: 1366, height: 768 } });
  const page = await context.newPage();
  const code = `ce13-ui-${Date.now()}`;

  await page.goto(`${data.web_base_url}/auth/login?return_to=${encodeURIComponent('/#/platform/commercial/plans')}`);
  await expect(page).toHaveURL(/\/idp\/authorize/);
  await page.getByLabel("邮箱").fill(data.allowed_email);
  await page.getByLabel("密码").fill(data.allowed_password);
  await page.getByRole("button", { name: "登录" }).click();
  await page.goto(`${data.web_base_url}/#/platform/commercial/plans`);
  await expect(page).toHaveURL(/#\/platform\/commercial\/plans/);

  await page.getByRole("button", { name: "新建套餐" }).click();
  const editor = page.getByRole("dialog", { name: "新建套餐首稿" });
  await editor.getByLabel("套餐代码").fill(code);
  await editor.getByLabel("套餐名称").fill("CE-13 可见控制台套餐");
  await editor.getByRole("button", { name: "添加模块" }).click();
  await editor.getByLabel("模块").selectOption("device-operations");
  await editor.getByLabel("device.lifecycle").check();
  await editor.getByRole("button", { name: "添加范围" }).click();
  await editor.getByPlaceholder("default").fill("default");
  await editor.getByRole("button", { name: "提交到服务端" }).click();
  await expect(page.getByText("套餐草稿已创建。")).toBeVisible();
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByRole("button", { name: "发布", exact: true }).click();
  await expect(page.getByText("套餐版本已发布；后续修订必须创建新版本。")).toBeVisible();
  for (const [width, height] of [[1536, 1024], [1440, 900], [1366, 768], [390, 844]]) {
    await page.setViewportSize({ width, height });
    const dimensions = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, clientWidth: document.documentElement.clientWidth }));
    expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth);
    await page.screenshot({ path: testInfo.outputPath(`ce13-plan-published-${width}x${height}.png`), fullPage: true });
  }

  await page.setViewportSize({ width: 1366, height: 768 });
  const main = page.getByTestId("main-content");
  const before = await main.boundingBox();
  const platformTrigger = page.getByRole("button", { name: "平台商业", exact: true });
  await platformTrigger.focus();
  await page.screenshot({ path: testInfo.outputPath("ce13-platform-trigger-focus-1366x768.png"), fullPage: true });
  await page.keyboard.press("Enter");
  const overlay = page.getByRole("dialog", { name: "平台商业导航" });
  await expect(overlay).toBeVisible();
  const overlayBox = await overlay.boundingBox();
  const after = await main.boundingBox();
  expect(overlayBox?.width).toBeGreaterThanOrEqual(450);
  expect(overlayBox?.width).toBeLessThanOrEqual(481);
  expect(after?.x).toBe(before?.x);
  expect(after?.width).toBe(before?.width);
  await page.keyboard.press("Tab");
  await expect(overlay.getByRole("button").first()).toBeFocused();
  await page.screenshot({ path: testInfo.outputPath("ce13-platform-overlay-keyboard-1366x768.png"), fullPage: true });
  await page.keyboard.press("Escape");
  await expect(overlay).toBeHidden();

  await page.goto(`${data.web_base_url}/#/platform/commercial/tenant-entitlements`);
  await page.getByLabel("租户 ID").fill(data.tenant_id);
  await page.getByRole("button", { name: "读取权益" }).click();
  await expect(page.locator(".subscription-card")).toBeVisible();
  await page.getByRole("button", { name: "新增专项来源" }).click();
  const override = page.getByRole("dialog", { name: "新增专项权益来源" });
  await override.getByLabel("模块", { exact: true }).selectOption("device-operations");
  await override.getByLabel("目标类型", { exact: true }).selectOption("ENTITLEMENT_TARGET_CAPABILITY");
  await override.getByLabel("目标 key", { exact: true }).selectOption("device.lifecycle");
  await override.getByLabel("效果", { exact: true }).selectOption("ENTITLEMENT_EFFECT_DENY");
  await override.getByLabel("原因", { exact: true }).fill("CE-13 可见来源验证");
  await override.getByRole("button", { name: "创建专项来源" }).click();
  await expect(page.getByText("专项权益来源已创建，正在使用服务端新版本重新解析。")).toBeVisible();
  await expect(page.getByText("override").first()).toBeVisible();

  const change = page.getByTestId("ce13-subscription-change");
  await change.getByLabel("目标 plan_code").fill(code);
  await change.getByLabel("目标版本").fill("1");
  await change.getByLabel("预览原因").fill("CE-13 可见人工变更预览");
  await change.getByRole("button", { name: "生成不可变预览" }).click();
  await expect(change.getByText(/change /)).toBeVisible();
  await change.getByLabel(/PLATFORM_MANUAL_APPROVAL/).check();
  await change.getByLabel("确认原因").fill("CE-13 可见人工批准");
  await change.getByRole("button", { name: "确认此 preview_hash" }).click();
  await expect(change.getByText("不可变变更回执")).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("ce13-tenant-change-1366.png"), fullPage: true });

  await context.close();
});

test("TestCE13TenantSessionCannotUsePlatformConsole", async ({ browser }) => {
  const data = fixture();
  const tenant = await login(browser, data, data.tenant_email, data.tenant_password);
  await tenant.page.goto(`${data.web_base_url}/#/platform/commercial/modules`);
  await expect(tenant.page.getByRole("heading", { name: "平台商业管理" })).toBeVisible();
  await expect(tenant.page.getByText("当前可信平台会话无权")).toBeVisible();
  await expect(tenant.page.getByRole("button", { name: "查看详情" })).toHaveCount(0);
  await tenant.context.close();
});
