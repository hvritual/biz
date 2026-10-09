import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";

interface Fixture {
  access: Array<{
    tenant_a: string; tenant_b: string;
    admin_email: string; admin_password: string;
    member_id: string; member_email: string; member_password: string;
  }>;
  web_base_url: string;
  allowed_email: string;
  allowed_password: string;
}

interface SessionView {
  context_version?: number;
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

// Refs #293/#294, A1–A5. All business results below come from the live
// IdP/BFF/MySQL chain. The sole interception drops an actual committed reply;
// it never fulfills a fabricated subscription, role or entitlement response.
test("TestCE293AccessModuleActivationRoleUseAndRevocation", async ({ browser }, testInfo) => {
  const data = fixture();
  const sample = data.access[testInfo.retry];
  if (!sample) throw new Error("a fresh unactivated fixture is required for every attempt");
  const platform = await login(browser, data, data.allowed_email, data.allowed_password);
  const admin = await login(browser, data, sample.admin_email, sample.admin_password);
  const member = await login(browser, data, sample.member_email, sample.member_password);
  const sessions = [platform, admin, member];
  const errors: string[] = [];
  for (const actor of sessions) actor.page.on("pageerror", (error) => errors.push(error.message));
  const id = (kind: string) => `ce293-${kind}-${Date.now().toString(36)}-${Math.random().toString(16).slice(2, 8)}`;
  const read = (actor: typeof admin, path: string) => browserRequest(actor.page, data.web_base_url, path);
  const write = (actor: typeof admin, path: string, body: unknown, method = "POST", key = id("write")) =>
    browserRequest(actor.page, data.web_base_url, path, {
      method, body,
      headers: { "X-CSRF-Token": String(actor.session.csrf_token), "Idempotency-Key": key },
    });
  async function switchTenant(tenant: string) {
    const response = await write(admin, "/auth/session/tenant", { tenant_id: tenant });
    expect(response.status, response.text).toBe(200);
    admin.session = response.json as SessionView;
    expect(admin.session.active_tenant_id).toBe(tenant);
  }
  try {
    await switchTenant(sample.tenant_a);
    expect(member.session.active_tenant_id).toBe(sample.tenant_a);
    const subPath = (tenant: string) => `/v1/platform/tenants/${encodeURIComponent(tenant)}/subscription`;
    expect((await read(platform, subPath(sample.tenant_a))).status).toBe(404);
    expect((await read(platform, subPath(sample.tenant_b))).status).toBe(404);
    const deniedNames: string[] = [];
    async function deniedRole(actor: typeof admin) {
      const name = id("must-not-exist"); deniedNames.push(name);
      const response = await write(actor, "/v1/tenant/roles", { name });
      expect(response.status, response.text).toBe(403);
    }
    await deniedRole(member); // no IAM, no entitlement
    await deniedRole(admin); // IAM, no entitlement

    const planCode = id("access-plan");
    const createID = id("plan-create");
    const created = await write(platform, "/v1/platform/plans", {
      requestId: createID, planCode, name: "Access 模块真实验收套餐",
      terms: { modules: [{ moduleCode: "access-management", capabilityCodes: ["tenant.member.lifecycle", "tenant.role.permission"], quotas: [{ key: "tenant.members", value: "100", unlimited: false }], fields: [{ key: "member.profile", action: "read", mode: "masked" }] }], salesScope: ["default"], validityMode: "fixed_days", validityDays: 30, priceRef: "" },
      reason: "isolated Access journey; manual activation, not payment proof",
    }, "POST", createID);
    expect(created.status, created.text).toBe(200);
    const draft = created.json as { version: string; revision: string; state: string };
    expect(draft.state).toBe("DRAFT");
    const publishID = id("publish");
    const published = await write(platform, `/v1/platform/plans/${encodeURIComponent(planCode)}/versions/${draft.version}/publish`, {
      requestId: publishID, planCode, version: draft.version, expectedRevision: draft.revision, reason: "publish exact isolated Access version",
    }, "POST", publishID);
    expect(published.status, published.text).toBe(200);
    expect((published.json as { state: string }).state).toBe("PUBLISHED");

    // A1/A5: use the shipped INITIAL UI, then lose only the real confirm reply.
    await platform.page.goto(`${data.web_base_url}/#/platform/commercial/tenant-entitlements`);
    await platform.page.getByLabel("租户编号").fill(sample.tenant_a);
    await platform.page.getByRole("button", { name: "读取权益" }).click();
    const initial = platform.page.getByTestId("platform-initial-subscription");
    await expect(initial.getByRole("heading", { name: "首次开通套餐" })).toBeVisible();
    await initial.getByLabel("适用范围").fill("default");
    await initial.getByRole("button", { name: "读取套餐目录" }).click();
    await initial.getByLabel("套餐", { exact: true }).click();
    await platform.page.locator(`[data-slot="select-item"][data-ui-option-value="${planCode}"]`).click();
    await initial.getByRole("button", { name: "检查已发布版本" }).click();
    await expect(initial.getByText(`exact v${draft.version}`)).toBeVisible();
    await initial.getByLabel("首次开通原因").fill("Access 样板真实开通");
    await initial.getByRole("button", { name: "查看首次开通方案" }).click();
    await initial.getByText(/我已核对 exact 套餐版本/).click();
    await initial.getByLabel("确认原因").fill("Access 样板真实人工批准路径");
    let confirmations = 0, upstreamStatus = 0;
    let dropped = false;
    await platform.page.route("**/api/v1/platform/tenants/*/subscription/changes/*/confirm", async (route) => {
      confirmations++;
      const actual = await route.fetch(); upstreamStatus = actual.status();
      await route.abort("failed"); dropped = true;
    });
    await initial.getByRole("button", { name: "确认首次开通" }).click();
    await expect.poll(() => dropped).toBe(true);
    expect(upstreamStatus).toBe(200);
    await platform.page.reload();
    await expect(platform.page.locator(".subscription-card").getByText("Access 模块真实验收套餐", { exact: false })).toBeVisible();
    expect(confirmations).toBe(1);
    const subscription = await read(platform, subPath(sample.tenant_a));
    expect(subscription.status, subscription.text).toBe(200);
    expect(subscription.json).toMatchObject({ tenantId: sample.tenant_a, planCode, planVersion: String(draft.version), state: "ACTIVE" });
    await deniedRole(member); // entitlement now exists, IAM still absent

    // A3: real role editor reads the generated permission catalog and writes.
    await admin.page.goto(`${data.web_base_url}/#/enterprise/roles`);
    await admin.page.getByRole("button", { name: "新建角色", exact: true }).click();
    const dialog = admin.page.getByRole("dialog", { name: "新建角色" });
    const operatorName = id("operator");
    await dialog.getByLabel("角色名称").fill(operatorName);
    for (const permission of ["tenant.member.read", "tenant.role.read", "tenant.role.manage"]) {
      await dialog.locator(`[data-role-permission-leaf="${permission}"]`).getByRole("checkbox").check();
    }
    await dialog.getByLabel("数据范围").click();
    await admin.page.locator('[data-slot="select-item"][data-ui-option-value="all"]').click();
    await dialog.getByRole("button", { name: "保存角色" }).click();
    await expect(dialog).not.toBeVisible();
    await expect(admin.page.getByText(operatorName, { exact: true })).toBeVisible();
    const allRoles = await read(admin, "/v1/tenant/roles");
    expect(allRoles.status, allRoles.text).toBe(200);
    type RoleDTO = { id: string; name: string; version: string; permissions?: Array<{ permission: string }> };
    const roles = (allRoles.json as { roles: RoleDTO[] }).roles;
    const operator = roles.find((role) => role.name === operatorName)!;
    expect(operator).toBeTruthy();
    expect(operator.permissions).toEqual(expect.arrayContaining([
      expect.objectContaining({ permission: "tenant.member.read" }),
      expect.objectContaining({ permission: "tenant.role.manage" }),
    ]));
    for (const name of deniedNames) expect(roles.some((role) => role.name === name)).toBe(false);
    const assignment = await write(admin, `/v1/tenant/roles/${operator.id}/members`, { roleId: operator.id, userId: sample.member_id });
    expect(assignment.status, assignment.text).toBe(200);
    const memberRead = await read(member, "/v1/tenant/members");
    expect(memberRead.status, memberRead.text).toBe(200);
    await member.page.goto(`${data.web_base_url}/#/enterprise/roles`);
    await expect(member.page.getByRole("button", { name: "新建角色", exact: true })).toBeEnabled();
    const key = id("member-create");
    const body = { name: id("member-created") };
    const firstWrite = await write(member, "/v1/tenant/roles", body, "POST", key);
    const replay = await write(member, "/v1/tenant/roles", body, "POST", key);
    expect(firstWrite.status, firstWrite.text).toBe(200);
    expect(replay.status, replay.text).toBe(200);
    expect(replay.json).toEqual(firstWrite.json);
    const written = firstWrite.json as RoleDTO;
    const readback = await read(member, `/v1/tenant/roles/${written.id}`);
    expect(readback.status, readback.text).toBe(200);
    expect(readback.json).toEqual(firstWrite.json);
    await member.page.reload();
    await expect(member.page.getByText(body.name, { exact: true })).toBeVisible();
    const afterWrites = await read(admin, "/v1/tenant/roles");
    expect((afterWrites.json as { roles: RoleDTO[] }).roles.filter((role) => role.name === body.name)).toHaveLength(1);

    // A4: same identity switches tenant. Old context and foreign role cannot
    // authorize a write, even after the second tenant is legitimately entitled.
    const previewID = id("b-preview");
    const preview = await write(platform, subPath(sample.tenant_b) + "/changes/preview", { requestId: previewID, tenantId: sample.tenant_b, action: "INITIAL", targetPlanCode: planCode, targetPlanVersion: draft.version, salesScope: "default", reason: "second isolated tenant" }, "POST", previewID);
    expect(preview.status, preview.text).toBe(200);
    const previewDTO = preview.json as { changeId: string; previewHash: string };
    // Remove the reply-drop hook only after proving A did not reconfirm.
    expect(confirmations).toBe(1);
    await platform.page.unroute("**/api/v1/platform/tenants/*/subscription/changes/*/confirm");
    const confirmID = id("b-confirm");
    const confirmed = await write(platform, subPath(sample.tenant_b) + `/changes/${previewDTO.changeId}/confirm`, { requestId: confirmID, tenantId: sample.tenant_b, changeId: previewDTO.changeId, previewHash: previewDTO.previewHash, reason: "second isolated activation" }, "POST", confirmID);
    expect(confirmed.status, confirmed.text).toBe(200);
    const oldContext = JSON.stringify({ actor_kind: admin.session.actor_kind ?? "", platform_subject: "", user_id: admin.session.user_id ?? "", active_tenant_id: sample.tenant_a, context_version: admin.session.context_version ?? 0 });
    await switchTenant(sample.tenant_b);
    const foreignRead = await read(admin, `/v1/tenant/roles/${written.id}`);
    expect(foreignRead.status, foreignRead.text).toBe(404);
    const stale = await browserRequest(admin.page, data.web_base_url, "/v1/tenant/roles", { method: "POST", headers: { "X-CSRF-Token": String(admin.session.csrf_token), "Idempotency-Key": id("stale"), "X-Biz-Session-Context": oldContext }, body: { name: "stale-context-must-not-write" } });
    expect(stale.status, stale.text).toBe(409);
    const otherRoles = await read(admin, "/v1/tenant/roles");
    expect(otherRoles.status, otherRoles.text).toBe(200);
    expect((otherRoles.json as { roles: RoleDTO[] }).roles.some((role) => role.name === "stale-context-must-not-write")).toBe(false);
    await switchTenant(sample.tenant_a);
    const revoked = await write(admin, `/v1/tenant/roles/${operator.id}/members/${sample.member_id}/revoke`, { roleId: operator.id, userId: sample.member_id });
    expect(revoked.status, revoked.text).toBe(200);
    await deniedRole(member);
    const revokedRead = await read(member, "/v1/tenant/members");
    expect(revokedRead.status, revokedRead.text).toBe(403);
    await member.page.reload();
    await expect(member.page.getByRole("button", { name: "新建角色", exact: true })).toHaveCount(0);
    const finalRoles = await read(admin, "/v1/tenant/roles");
    for (const name of deniedNames) expect((finalRoles.json as { roles: RoleDTO[] }).roles.some((role) => role.name === name)).toBe(false);
    expect((await read(platform, subPath(sample.tenant_a))).json).toEqual(subscription.json);
    expect(errors).toEqual([]);
    await admin.page.screenshot({ path: testInfo.outputPath("ce293-access-roles-after-revocation.png"), fullPage: true });
  } finally {
    await Promise.all(sessions.map((actor) => actor.context.close()));
  }
});
