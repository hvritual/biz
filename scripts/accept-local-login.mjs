#!/usr/bin/env node
// Runs only against already-started local services. It never starts processes
// or talks to the database directly; credentials stay in this Node process.
import { createRequire } from "node:module";
import { existsSync, mkdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const credentialsPath = process.env.YUNKA_BIZ_LOCAL_CREDENTIALS_FILE ?? join(root, ".local/biz-local-credentials.json");
const screenshots = join(root, ".local/login-acceptance");
const webBase = process.env.YUNKA_BIZ_ACCEPT_WEB_URL ?? "http://127.0.0.1:14183";
const chrome = process.env.PLAYWRIGHT_SYSTEM_CHROME ?? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";

function fail() {
  // Deliberately avoid rendering Error values: they can contain URL query data.
  process.stderr.write("LOCAL_LOGIN_ACCEPTANCE: FAIL\n");
  process.exitCode = 1;
}

function requireSession(value, actor) {
  if (!value || value.authenticated !== true || value.actor_kind !== actor) throw new Error("invalid trusted session");
  return value;
}

async function api(page, path, init = {}) {
  return page.evaluate(async ({ base, path, init }) => {
    const headers = new Headers(init.headers ?? {});
    if (init.body !== undefined) headers.set("Content-Type", "application/json");
    const routedPath = path.startsWith("/v1/") ? "/api" + path : path;
    const response = await fetch(base + routedPath, {
      method: init.method ?? "GET",
      headers,
      credentials: "include",
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
    });
    const text = await response.text();
    let json = null;
    try { json = text ? JSON.parse(text) : null; } catch { /* status is sufficient */ }
    return { status: response.status, json };
  }, { base: webBase, path, init });
}

async function login(browser, email, password) {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto(`${webBase}/auth/login?return_to=${encodeURIComponent("/auth/session")}`, { waitUntil: "domcontentloaded" });
  await page.getByLabel("邮箱").fill(email);
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "登录" }).click();
  await page.waitForURL(/\/auth\/session/);
  const session = (await api(page, "/auth/session")).json;
  return { context, page, session };
}

async function main() {
  if (!existsSync(credentialsPath) || !existsSync(chrome)) throw new Error("acceptance prerequisites unavailable");
  const credentials = JSON.parse(readFileSync(credentialsPath, "utf8"));
  if (!credentials.platform_email || !credentials.platform_password || !credentials.tenant_email || !credentials.tenant_password || !credentials.tenant_id) throw new Error("acceptance credentials incomplete");
  mkdirSync(screenshots, { recursive: true, mode: 0o700 });

  const webRequire = createRequire(join(root, "web", "package.json"));
  const { chromium } = webRequire("@playwright/test");
  const browser = await chromium.launch({ executablePath: chrome, headless: true });
  try {
    const platform = await login(browser, credentials.platform_email, credentials.platform_password);
    try {
      const session = requireSession(platform.session, "platform");
      if ((session.active_tenant_id ?? "") !== "" || !session.platform_subject) throw new Error("platform session is not tenantless");
      if ((await api(platform.page, "/v1/tenants")).status !== 200) throw new Error("platform API unavailable");
      await platform.page.goto(`${webBase}/#/platform/commercial/modules`, { waitUntil: "networkidle" });
      await platform.page.getByRole("heading", { name: "平台商业管理" }).waitFor();
      await platform.page.screenshot({ path: join(screenshots, "platform.png"), fullPage: true });
    } finally { await platform.context.close(); }

    const tenant = await login(browser, credentials.tenant_email, credentials.tenant_password);
    try {
      let session = requireSession(tenant.session, "user");
      if ((session.active_tenant_id ?? "") !== credentials.tenant_id) {
        const switched = await api(tenant.page, "/auth/session/tenant", { method: "POST", headers: { "X-CSRF-Token": session.csrf_token ?? "" }, body: { tenant_id: credentials.tenant_id } });
        if (switched.status !== 200) throw new Error("tenant selection failed");
        session = requireSession((await api(tenant.page, "/auth/session")).json, "user");
      }
      if (session.active_tenant_id !== credentials.tenant_id) throw new Error("tenant session did not bind requested tenant");
      if ((await api(tenant.page, "/v1/platform/modules")).status !== 403) throw new Error("tenant platform access was not forbidden");
      const members = await api(tenant.page, "/v1/tenant/members");
      if (members.status !== 200 || !members.json?.members?.some((member) => member.userId === "local-tenant-admin")) throw new Error("tenant member readback failed");
      await tenant.page.goto(`${webBase}/#/workspace/members`, { waitUntil: "networkidle" });
      await tenant.page.getByRole("heading", { name: "业务成员", level: 1, exact: true }).waitFor();
      await tenant.page.screenshot({ path: join(screenshots, "tenant.png"), fullPage: true });
    } finally { await tenant.context.close(); }
  } finally { await browser.close(); }
  process.stdout.write("LOCAL_LOGIN_ACCEPTANCE: PASS platform=tenantless tenant=user platform_api=403\n");
}

main().catch(fail);
