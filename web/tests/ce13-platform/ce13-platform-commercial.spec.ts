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
  initial_tenant_id: string;
  allowed_api_key: string;
  allowed_subject: string;
  denied_subject: string;
  read_only_email: string;
  read_only_password: string;
  read_only_subject: string;
  manage_email: string;
  manage_password: string;
  manage_subject: string;
  technical_email: string;
  technical_password: string;
  technical_subject: string;
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

interface AuthorizationView {
  authenticated: boolean;
  actor_kind: string;
  platform_subject?: string;
  button_codes: string[];
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

async function acceptPrivacyConsentIfRequired(page: Page) {
  const heading = page.getByRole("heading", { name: "确认隐私与服务协议" });
  if (await heading.isVisible()) {
    await page.getByLabel(/我已阅读并同意/).check();
    await page.getByRole("button", { name: "同意并继续" }).click();
  }
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
  await page.getByLabel("账号 / 手机号 / 邮箱", { exact: true }).fill(email);
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "登录" }).click();
  await acceptPrivacyConsentIfRequired(page);
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
  const deniedProjection = await browserRequest(denied.page, data.web_base_url, "/auth/authorization");
  expect(deniedProjection.status, deniedProjection.text).toBe(200);
  expect((deniedProjection.json as AuthorizationView).platform_subject).toBe(data.denied_subject);
  expect((deniedProjection.json as AuthorizationView).button_codes).not.toContain("commercial.module.list");
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

  const allowedProjection = await browserRequest(allowed.page, data.web_base_url, "/auth/authorization");
  expect(allowedProjection.status, allowedProjection.text).toBe(200);
  const allowedAuthorization = allowedProjection.json as AuthorizationView;
  expect(allowedAuthorization.platform_subject).toBe(data.allowed_subject);
  expect(allowedAuthorization.button_codes).toEqual(expect.arrayContaining([
    "commercial.module.list",
    "commercial.module.create",
    "commercial.module.set_sales_status",
    "commercial.module.set_technical_status",
  ]));

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

test("TestCE340PlatformFirstSubscriptionThroughTrustedWebSession", async ({ browser }, testInfo) => {
  const data = fixture();
  const allowed = await login(browser, data, data.allowed_email, data.allowed_password);
  const csrf = allowed.session.csrf_token;
  expect(csrf).toBeTruthy();

  const requestID = (name: string) => `ce340-browser-${name}-${Date.now()}-${Math.random().toString(16).slice(2)}`;
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

  const before = await browserRequest(
    allowed.page,
    data.web_base_url,
    `/v1/platform/tenants/${encodeURIComponent(data.initial_tenant_id)}/subscription`,
  );
  expect(before.status).toBe(404);

  const planCode = `ce340-first-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
  const createID = requestID("plan-create");
  const created = await write("/v1/platform/plans", {
    requestId: createID,
    planCode,
    name: "CE-340 首次开通套餐",
    terms: {
      modules: [{
        moduleCode: "device-operations",
        capabilityCodes: ["device.lifecycle"],
        quotas: [{ key: "tenant.devices", value: "100", unlimited: false }],
        fields: [],
      }],
      salesScope: ["default"],
      validityMode: "fixed_days",
      validityDays: 365,
      priceRef: "",
    },
    reason: "CE-340 trusted first activation fixture",
  }, createID);
  expect(created.status, created.text).toBe(200);
  const draft = created.json as { version: number; revision: number; state: string };
  expect(draft.state).toBe("DRAFT");

  const publishID = requestID("plan-publish");
  const published = await write(`/v1/platform/plans/${encodeURIComponent(planCode)}/versions/${draft.version}/publish`, {
    requestId: publishID,
    planCode,
    version: draft.version,
    expectedRevision: draft.revision,
    reason: "publish CE-340 exact target",
  }, publishID);
  expect(published.status, published.text).toBe(200);
  expect((published.json as { state: string }).state).toBe("PUBLISHED");

  await allowed.page.goto(`${data.web_base_url}/#/platform/commercial/tenant-entitlements`);
  await allowed.page.getByLabel("租户编号").fill(data.initial_tenant_id);
  await allowed.page.getByRole("button", { name: "读取权益" }).click();
  const initial = allowed.page.getByTestId("platform-initial-subscription");
  await expect(initial.getByRole("heading", { name: "首次开通套餐" })).toBeVisible();
  await expect(initial.getByText("完成首次开通不会自动给成员分配角色或操作权限。")).toBeVisible();

  await initial.getByLabel("适用范围").fill("default");
  await initial.getByRole("button", { name: "读取套餐目录" }).click();
  await initial.getByLabel("套餐", { exact: true }).click();
  await allowed.page.locator(`[data-slot="select-item"][data-ui-option-value="${planCode}"]`).click();
  await initial.getByRole("button", { name: "检查已发布版本" }).click();
  await expect(initial.getByText(`exact v${draft.version}`)).toBeVisible();

  await initial.getByLabel("首次开通原因").fill("CE-340 平台首次开通");
  await initial.getByRole("button", { name: "查看首次开通方案" }).click();
  await initial.getByText(/我已核对 exact 套餐版本/).click();
  await initial.getByLabel("确认原因").fill("CE-340 真实平台会话批准");
  const browserErrors: string[] = [];
  allowed.page.on("pageerror", (error) => browserErrors.push(error.message));
  const viewports = [
    { width: 1366, height: 768 }, { width: 1440, height: 900 },
    { width: 1536, height: 1024 }, { width: 390, height: 844 },
  ];
  for (const viewport of viewports) {
    await allowed.page.setViewportSize(viewport);
    await expect(initial.getByRole("button", { name: "确认首次开通" })).toBeEnabled();
    await allowed.page.screenshot({ path: testInfo.outputPath(`ce340-confirm-${viewport.width}.png`), fullPage: true });
  }
  await allowed.page.setViewportSize(viewports[0]);
  // Execute the real command, then drop only its browser reply. No mocked
  // subscription, receipt, entitlement or business-success response is used.
  let confirmationCalls = 0;
  let commandStatus = 0;
  await allowed.page.route("**/api/v1/platform/tenants/*/subscription/changes/*/confirm", async (route) => {
    confirmationCalls += 1;
    const actual = await route.fetch();
    commandStatus = actual.status();
    await route.abort("failed");
  });
  await initial.getByRole("button", { name: "确认首次开通" }).click();
  await expect(initial.getByText("首次开通结果待确认", { exact: true })).toBeVisible();
  await expect(allowed.page).toHaveURL(/initialChange=/);
  expect(commandStatus).toBe(200);
  expect(confirmationCalls).toBe(1);
  await allowed.page.screenshot({ path: testInfo.outputPath("ce340-lost-confirmation.png"), fullPage: true });
  await allowed.page.reload();
  await expect(allowed.page.locator(".subscription-card").getByText(/CE-340 首次开通套餐/)).toBeVisible();
  expect(confirmationCalls).toBe(1);
  expect(browserErrors).toEqual([]);
  for (const viewport of viewports) {
    await allowed.page.setViewportSize(viewport);
    await expect(allowed.page.locator(".subscription-card")).toBeVisible();
    await allowed.page.screenshot({ path: testInfo.outputPath(`ce340-restored-${viewport.width}.png`), fullPage: true });
  }
  const finalSubscription = await browserRequest(
    allowed.page,
    data.web_base_url,
    `/v1/platform/tenants/${encodeURIComponent(data.initial_tenant_id)}/subscription`,
  );
  expect(finalSubscription.status, finalSubscription.text).toBe(200);
  expect(finalSubscription.json).toMatchObject({
    tenantId: data.initial_tenant_id,
    planCode,
    planVersion: String(draft.version),
    salesScope: "default",
  });

  const entitlements = await browserRequest(
    allowed.page,
    data.web_base_url,
    `/v1/platform/tenants/${encodeURIComponent(data.initial_tenant_id)}/entitlements`,
    {
      method: "POST",
      headers: { "X-CSRF-Token": String(csrf), "Idempotency-Key": requestID("entitlements") },
      body: { tenantId: data.initial_tenant_id, capabilityCodes: ["device.lifecycle"] },
    },
  );
  expect(entitlements.status, entitlements.text).toBe(200);
  const decisions = (entitlements.json as { decisions?: Array<{ kind: string; key: string; allowed: boolean }> }).decisions ?? [];
  expect(decisions).toEqual(expect.arrayContaining([
    expect.objectContaining({ kind: "capability", key: "device.lifecycle", allowed: true }),
    expect.objectContaining({ kind: "quota", key: "tenant.devices" }),
  ]));

  await allowed.context.close();
});

test("TestCE13PlatformCommercialVisibleConsoleFlow", async ({ browser }, testInfo) => {
  const data = fixture();
  const context = await browser.newContext({ viewport: { width: 1366, height: 768 } });
  const page = await context.newPage();
  const code = `ce13-ui-${Date.now()}`;

  await page.goto(`${data.web_base_url}/auth/login?return_to=${encodeURIComponent('/#/platform/commercial/plans')}`);
  await expect(page).toHaveURL(/\/idp\/authorize/);
  await page.getByLabel("账号 / 手机号 / 邮箱", { exact: true }).fill(data.allowed_email);
  await page.getByLabel("密码").fill(data.allowed_password);
  await page.getByRole("button", { name: "登录" }).click();
  await acceptPrivacyConsentIfRequired(page);
  await page.goto(`${data.web_base_url}/#/platform/commercial/plans`);
  await expect(page).toHaveURL(/#\/platform\/commercial\/plans/);

  await page.getByRole("button", { name: "新建套餐" }).click();
  const editor = page.getByRole("dialog", { name: "创建套餐草稿" });
  await editor.getByLabel("套餐代码").fill(code);
  await editor.getByLabel("套餐名称").fill("CE-13 可见控制台套餐");
  await editor.getByRole("button", { name: "添加模块" }).click();
  await editor.getByLabel("模块").click();
  await page.locator('[data-slot="select-item"][data-ui-option-value="device-operations"]').click();
  await editor.getByLabel("device.lifecycle").check();
  await editor.getByRole("button", { name: "添加范围" }).click();
  await editor.getByPlaceholder("default").fill("default");
  await editor.getByRole("button", { name: "创建草稿", exact: true }).click();
  await expect(page.getByText("套餐草稿已创建。")).toBeVisible();
  await page.getByRole("button", { name: "发布前检查", exact: true }).click();
  const preflight = page.getByRole("dialog", { name: "发布前检查" });
  await expect(preflight.getByText("此检查不是“发布成功”证明")).toBeVisible();
  await preflight.getByRole("button", { name: "继续发布", exact: true }).click();
  const publish = page.getByRole("dialog", { name: "确认发布套餐版本" });
  await expect(publish.getByText("发布后内容不可直接覆盖")).toBeVisible();
  await publish.getByRole("button", { name: "确认发布", exact: true }).click();
  await expect(page.getByText("套餐版本已发布；后续修订需要创建新版本。")).toBeVisible();
  for (const [width, height] of [[1536, 1024], [1440, 900], [1366, 768], [390, 844]]) {
    await page.setViewportSize({ width, height });
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())));
    const dimensions = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, clientWidth: document.documentElement.clientWidth }));
    expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth);
    await page.screenshot({ path: testInfo.outputPath(`ce13-plan-published-${width}x${height}.png`), fullPage: true });
  }

  await page.setViewportSize({ width: 1366, height: 768 });
  const main = page.getByTestId("main-content");
  const before = await main.boundingBox();
  const platformTrigger = page.getByRole("button", { name: "平台管理", exact: true });
  await platformTrigger.focus();
  await page.screenshot({ path: testInfo.outputPath("ce13-platform-trigger-focus-1366x768.png"), fullPage: true });
  await page.keyboard.press("Enter");
  const overlay = page.getByRole("dialog", { name: "平台管理导航" });
  await expect(overlay).toBeVisible();
  const overlayBox = await overlay.boundingBox();
  const after = await main.boundingBox();
  expect(overlayBox?.width).toBeGreaterThanOrEqual(450);
  expect(overlayBox?.width).toBeLessThanOrEqual(481);
  expect(after?.x).toBe(before?.x);
  expect(after?.width).toBe(before?.width);
  await expect(overlay.getByRole("button").first()).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(overlay.getByRole("button").nth(1)).toBeFocused();
  await page.screenshot({ path: testInfo.outputPath("ce13-platform-overlay-keyboard-1366x768.png"), fullPage: true });
  await page.keyboard.press("Escape");
  await expect(overlay).toBeHidden();

  await page.goto(`${data.web_base_url}/#/platform/commercial/tenant-entitlements`);
  await page.getByLabel("租户编号").fill(data.tenant_id);
  await page.getByRole("button", { name: "读取权益" }).click();
  await expect(page.locator(".subscription-card")).toBeVisible();
  await page.getByRole("button", { name: "新增专项权益" }).click();
  const override = page.getByRole("dialog", { name: "新增专项权益" });
  await override.getByLabel("模块", { exact: true }).click();
  await page.locator('[data-slot="select-item"][data-ui-option-value="device-operations"]').click();
  await override.getByLabel("授权范围", { exact: true }).click();
  await page.locator('[data-slot="select-item"][data-ui-option-value="ENTITLEMENT_TARGET_CAPABILITY"]').click();
  await override.getByLabel("具体项目", { exact: true }).click();
  await page.locator('[data-slot="select-item"][data-ui-option-value="device.lifecycle"]').click();
  await override.getByLabel("授权结果", { exact: true }).click();
  await page.locator('[data-slot="select-item"][data-ui-option-value="ENTITLEMENT_EFFECT_DENY"]').click();
  await override.getByLabel("原因", { exact: true }).fill("CE-13 可见来源验证");
  await override.getByRole("button", { name: "创建专项权益" }).click();
  await expect(page.getByText("CE-13 可见来源验证", { exact: true })).toBeVisible();
  await expect(page.getByText("拒绝", { exact: true }).first()).toBeVisible();

  const change = page.getByTestId("ce13-subscription-change");
  await change.getByLabel("目标套餐编号").fill(code);
  await change.getByLabel("目标版本").fill("1");
  await change.getByLabel("变更原因").fill("CE-13 可见人工变更预览");
  await change.getByRole("button", { name: "查看变更方案" }).click();
  await expect(change.getByText(/变更编号/)).toBeVisible();
  await change.getByLabel(/已核对本次套餐变更影响/).check();
  await change.getByLabel("确认原因").fill("CE-13 可见人工批准");
  await change.getByRole("button", { name: "确认变更", exact: true }).click();
  await expect(change.getByText("变更结果")).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("ce13-tenant-change-1366.png"), fullPage: true });

  await context.close();
});

test("TestCE13PlatformModuleAuthorizationProjectionAndOperationQuadrants", async ({ browser }) => {
  const data = fixture();

  async function moduleFixture(page: Page) {
    const response = await browserRequest(page, data.web_base_url, "/v1/platform/modules");
    expect(response.status, response.text).toBe(200);
    const modules = (response.json as { modules?: Array<{
      moduleCode: string;
      version: number | string;
      technicalStatus: string;
      salesStatus: string;
    }> }).modules ?? [];
    expect(modules.length).toBeGreaterThan(0);
    return modules[0]!;
  }

  async function openModuleDetail(page: Page) {
    await page.goto(`${data.web_base_url}/#/platform/commercial/modules`);
    await expect(page.getByTestId("ce13-module-catalog")).toBeVisible();
    await expect(page.getByRole("button", { name: "查看详情" }).first()).toBeVisible();
    await page.getByRole("button", { name: "查看详情" }).first().click();
    await expect(page.getByRole("dialog", { name: /模块详情/ })).toBeVisible();
  }

  const readOnly = await login(browser, data, data.read_only_email, data.read_only_password);
  const readProjection = await browserRequest(readOnly.page, data.web_base_url, "/auth/authorization");
  expect(readProjection.status, readProjection.text).toBe(200);
  expect((readProjection.json as AuthorizationView).button_codes).toEqual(expect.arrayContaining([
    "commercial.module.list",
    "commercial.module.get",
  ]));
  expect((readProjection.json as AuthorizationView).button_codes).not.toEqual(expect.arrayContaining([
    "commercial.module.create",
    "commercial.module.set_sales_status",
    "commercial.module.set_technical_status",
  ]));
  await openModuleDetail(readOnly.page);
  await expect(readOnly.page.getByRole("button", { name: "新增模块" })).toBeDisabled();
  await expect(readOnly.page.getByRole("button", { name: "编辑基础配置" })).toBeDisabled();
  await expect(readOnly.page.getByRole("button", { name: "调整技术状态" })).toBeDisabled();
  await expect(readOnly.page.getByRole("button", { name: /^(停售销售|恢复销售)$/ })).toBeDisabled();
  await readOnly.context.close();

  const manage = await login(browser, data, data.manage_email, data.manage_password);
  const manageProjection = await browserRequest(manage.page, data.web_base_url, "/auth/authorization");
  const manageAuthorization = manageProjection.json as AuthorizationView;
  expect(manageProjection.status, manageProjection.text).toBe(200);
  expect(manageAuthorization.button_codes).toEqual(expect.arrayContaining([
    "commercial.module.list",
    "commercial.module.create",
    "commercial.module.update",
    "commercial.module.set_sales_status",
  ]));
  expect(manageAuthorization.button_codes).not.toContain("commercial.module.set_technical_status");
  const manageModule = await moduleFixture(manage.page);
  await openModuleDetail(manage.page);
  await expect(manage.page.getByRole("button", { name: "新增模块" })).toBeEnabled();
  await expect(manage.page.getByRole("button", { name: "编辑基础配置" })).toBeEnabled();
  await expect(manage.page.getByRole("button", { name: "调整技术状态" })).toBeDisabled();
  await expect(manage.page.getByRole("button", { name: /^(停售销售|恢复销售)$/ })).toBeEnabled();
  const manageSession = await browserRequest(manage.page, data.web_base_url, "/auth/session");
  const manageCSRF = String((manageSession.json as SessionView).csrf_token ?? "");
  expect(manageCSRF).not.toBe("");
  const deniedTechnical = await browserRequest(
    manage.page,
    data.web_base_url,
    `/v1/platform/modules/${encodeURIComponent(manageModule.moduleCode)}/technical-status`,
    {
      method: "POST",
      headers: { "X-CSRF-Token": manageCSRF, "Idempotency-Key": `ce13-manage-no-tech-${Date.now()}` },
      body: {
        requestId: `ce13-manage-no-tech-${Date.now()}`,
        moduleCode: manageModule.moduleCode,
        technicalStatus: manageModule.technicalStatus === "MODULE_TECHNICAL_STATUS_READY"
          ? "MODULE_TECHNICAL_STATUS_NOT_READY"
          : "MODULE_TECHNICAL_STATUS_READY",
        version: String(manageModule.version),
        reason: "manage must not imply technical authority",
      },
    },
  );
  expect(deniedTechnical.status, deniedTechnical.text).toBe(403);
  await manage.context.close();

  const technical = await login(browser, data, data.technical_email, data.technical_password);
  const technicalProjection = await browserRequest(technical.page, data.web_base_url, "/auth/authorization");
  const technicalAuthorization = technicalProjection.json as AuthorizationView;
  expect(technicalProjection.status, technicalProjection.text).toBe(200);
  expect(technicalAuthorization.button_codes).toEqual(expect.arrayContaining([
    "commercial.module.list",
    "commercial.module.set_technical_status",
  ]));
  expect(technicalAuthorization.button_codes).not.toContain("commercial.module.create");
  expect(technicalAuthorization.button_codes).not.toContain("commercial.module.set_sales_status");
  const technicalModule = await moduleFixture(technical.page);
  await openModuleDetail(technical.page);
  await expect(technical.page.getByRole("button", { name: "新增模块" })).toBeDisabled();
  await expect(technical.page.getByRole("button", { name: "编辑基础配置" })).toBeDisabled();
  await expect(technical.page.getByRole("button", { name: "调整技术状态" })).toBeEnabled();
  await expect(technical.page.getByRole("button", { name: /^(停售销售|恢复销售)$/ })).toBeDisabled();
  const technicalSession = await browserRequest(technical.page, data.web_base_url, "/auth/session");
  const technicalCSRF = String((technicalSession.json as SessionView).csrf_token ?? "");
  expect(technicalCSRF).not.toBe("");
  const deniedSales = await browserRequest(
    technical.page,
    data.web_base_url,
    `/v1/platform/modules/${encodeURIComponent(technicalModule.moduleCode)}/sales-status`,
    {
      method: "POST",
      headers: { "X-CSRF-Token": technicalCSRF, "Idempotency-Key": `ce13-tech-no-manage-${Date.now()}` },
      body: {
        requestId: `ce13-tech-no-manage-${Date.now()}`,
        moduleCode: technicalModule.moduleCode,
        salesStatus: technicalModule.salesStatus === "MODULE_SALES_STATUS_SELLABLE"
          ? "MODULE_SALES_STATUS_RETIRED"
          : "MODULE_SALES_STATUS_SELLABLE",
        version: String(technicalModule.version),
        reason: "technical authority must not imply module management",
      },
    },
  );
  expect(deniedSales.status, deniedSales.text).toBe(403);
  await technical.context.close();
});

test("TestCE13TenantSessionCannotUsePlatformConsole", async ({ browser }) => {
  const data = fixture();
  const tenant = await login(browser, data, data.tenant_email, data.tenant_password);
  await tenant.page.goto(`${data.web_base_url}/#/platform/commercial/modules`);
  await expect(tenant.page).toHaveURL(/#\/authorization-state\?reason=forbidden/);
  await expect(tenant.page.getByRole("heading", { name: "模块目录", exact: true })).toHaveCount(0);
  await tenant.context.close();
});
