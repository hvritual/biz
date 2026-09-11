import { test, expect, type Page } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import path from "node:path";
const captureRoot = process.env.SCREENSHOT_DIR ?? "test-results/screenshots";
async function capture(page: Page, name: string) {
  await mkdir(captureRoot, { recursive: true });
  await page.screenshot({ path: path.join(captureRoot, name + ".png"), fullPage: false });
}
async function members(page: Page) {
  await page.goto("/#/enterprise/members");
  await expect(
    page.getByRole("heading", { name: "成员管理", exact: true }),
  ).toBeVisible();
}
test("480px joined overlay never displaces the main content", async ({ page }) => {
  await members(page);
  await expect(page).toHaveTitle(/CoffeeLink/);
  await capture(page, "01-members");
  const main = page.getByTestId("main-content");
  const before = await main.boundingBox();
  await page.locator('[data-nav-id="enterprise"]').click();
  const drawer = page.getByTestId("module-drawer");
  await expect(drawer).toBeVisible();
  const box = await drawer.boundingBox();
  expect(box?.width).toBe(480);
  expect(box?.x).toBe(208);
  expect(await main.boundingBox()).toEqual(before);
  const sublinks = drawer.locator(".flyout-link");
  await expect(sublinks).toHaveCount(6);
  await expect(drawer.locator(".flyout-shortcuts a")).toHaveCount(6);
  expect(
    await drawer
      .locator(".flyout-link.active")
      .evaluate((e) => e.getBoundingClientRect().width),
  ).toBeLessThan(190);
  await expect(sublinks.locator(".lucide-chevron-right")).toHaveCount(0);
  await capture(page, "02-floating-navigation");
  await page.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await expect(page.locator('[data-nav-id="enterprise"]')).toBeFocused();
  await page.locator('[data-nav-id="enterprise"]').click();
  await page
    .getByRole("button", { name: "关闭菜单浮层", exact: true })
    .click({ position: { x: 900, y: 20 } });
  await expect(drawer).toHaveCount(0);
});
test("collapse and reopen navigation without pushing content", async ({ page }) => {
  await members(page);
  await page.getByRole("button", { name: "收起主菜单" }).click();
  const main = page.getByTestId("main-content");
  const before = await main.boundingBox();
  await page.locator('[data-nav-id="enterprise"]').click();
  const box = await page.getByTestId("module-drawer").boundingBox();
  expect(box?.x).toBe(72);
  expect(box?.width).toBe(480);
  expect(await main.boundingBox()).toEqual(before);
  await capture(page, "03-collapsed-navigation");
});
test("member filter, pagination and empty state work", async ({ page }) => {
  await members(page);
  await page.getByRole("button", { name: "第2页", exact: true }).click();
  await expect(page.locator(".member-table tbody tr").first()).not.toContainText("张三");
  await page.getByRole("button", { name: "第1页", exact: true }).click();
  await expect(page.locator(".member-table tbody tr")).toHaveCount(8);
  await page.getByLabel("搜索成员", { exact: true }).fill("李四");
  await expect(page.locator(".member-table tbody tr")).toHaveCount(1);
  await page.getByLabel("搜索成员", { exact: true }).fill("不存在的成员");
  await expect(page.getByText("没有找到匹配的记录")).toBeVisible();
  await page.getByRole("button", { name: "清空筛选条件" }).click();
  await expect(page.locator(".member-table tbody tr")).toHaveCount(8);
});
test("create, edit, change role, reset, disable and enable member with audit receipts", async ({
  page,
}) => {
  await members(page);
  await page.getByRole("button", { name: "新增成员", exact: true }).click();
  let dialog = page.getByRole("dialog");
  await dialog.getByLabel("成员姓名").fill("界面测试员");
  await dialog.getByLabel("邮箱", { exact: false }).fill("lifecycle@example.com");
  await dialog.getByRole("button", { name: "新增成员", exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await page.getByRole("button", { name: "编辑界面测试员", exact: true }).click();
  dialog = page.getByRole("dialog");
  await dialog.getByLabel("成员姓名").fill("生命周期测试");
  await dialog.getByRole("button", { name: "保存修改" }).click();
  await expect(page.getByRole("button", { name: "编辑生命周期测试" })).toBeVisible();
  await page.getByRole("link").filter({ hasText: "生命周期测试" }).click();
  await expect(
    page.getByRole("heading", { name: "生命周期测试", exact: false }),
  ).toBeVisible();
  await capture(page, "11-member-detail");
  await page.getByRole("button", { name: "调整角色", exact: true }).click();
  dialog = page.getByRole("dialog");
  await dialog.getByLabel("选择新角色").selectOption({ label: "运营管理员" });
  await dialog.getByLabel("数据访问范围").selectOption({ label: "本人负责的数据" });
  await capture(page, "12-member-role-change");
  await dialog.getByRole("button", { name: "确认角色变更" }).click();
  await expect(dialog).not.toBeVisible();
  await page.getByRole("button", { name: "重置密码", exact: true }).click();
  dialog = page.getByRole("dialog");
  await expect(
    dialog.getByText("管理员无法查看原密码或新密码", { exact: false }),
  ).toBeVisible();
  await capture(page, "13-password-reset");
  await dialog.getByRole("button", { name: "生成演示重置回执" }).click();
  await page.getByRole("button", { name: "禁用账号", exact: true }).click();
  dialog = page.getByRole("dialog");
  await dialog.getByPlaceholder("说明本次操作的业务原因").fill("临时离岗");
  await dialog.getByRole("checkbox").check();
  await dialog.getByRole("button", { name: "确认禁用", exact: true }).click();
  await expect(page.getByRole("button", { name: "重新启用", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "重新启用", exact: true }).click();
  dialog = page.getByRole("dialog");
  await dialog.getByPlaceholder("说明本次操作的业务原因").fill("返回岗位");
  await dialog.getByRole("checkbox").check();
  await dialog.getByRole("button", { name: "确认重新启用" }).click();
  await expect(page.getByRole("button", { name: "禁用账号", exact: true })).toBeVisible();
  await page.goto("/#/enterprise/audit");
  await page.getByLabel("搜索操作日志").fill("生命周期测试");
  await expect(page.locator("tbody tr").first()).toContainText("生命周期测试");
});
test("tenant switching clears selection and isolates records", async ({ page }) => {
  await members(page);
  await page.getByLabel("选择当前页全部成员").check();
  await expect(page.getByText("已选择 8 位成员")).toBeVisible();
  await page.getByLabel("切换企业").selectOption("demo-hangzhou");
  await expect(page.getByText("已选择 8 位成员")).toHaveCount(0);
  await expect(page.locator(".metric-card").first()).toContainText("24");
  await expect(page.getByLabel("搜索成员", { exact: true })).toHaveValue("");
});
test("all delivered routes render without console or runtime errors", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (e) => {
    if (e.type() === "error") errors.push(e.text());
  });
  for (const [route, file, title] of [
    ["/enterprise/roles", "04-roles", "角色权限管理"],
    ["/enterprise/organization", "05-organization", "组织架构管理"],
    ["/enterprise/plan", "06-plan", "套餐信息管理"],
    ["/enterprise/profile", "07-company", "企业信息管理"],
    ["/enterprise/audit", "08-audit", "操作日志"],
    ["/settings/general", "09-settings", "系统设置 / 基础设置"],
    ["/workbench", "10-workbench", "你好，张三"],
  ]) {
    await page.goto("/#" + route);
    await expect(page.getByRole("heading", { name: title!, exact: true })).toBeVisible();
    await capture(page, file!);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
  }
  expect(errors).toEqual([]);
});
test("settings save, rollback and tab navigation work", async ({ page }) => {
  await page.goto("/#/settings/general");
  await page.getByLabel("平台名称", { exact: false }).fill("测试企业平台");
  await page.getByRole("button", { name: "保存更改" }).click();
  await expect(page.getByRole("status")).toContainText("已保存演示配置");
  await page.getByLabel("平台名称", { exact: false }).fill("未保存");
  await page.getByRole("button", { name: "撤销修改" }).click();
  await expect(page.getByLabel("平台名称", { exact: false })).toHaveValue("测试企业平台");
  await page.getByRole("link", { name: "安全设置", exact: true }).click();
  await expect(page.getByRole("switch", { name: "多因素认证" })).toBeVisible();
});
for (const size of [
  { width: 1366, height: 768 },
  { width: 1440, height: 900 },
  { width: 390, height: 844 },
])
  test(`responsive layout ${size.width}x${size.height}`, async ({ page }) => {
    await page.setViewportSize(size);
    await members(page);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await capture(page, `responsive-${size.width}`);
    await page.locator('[data-nav-id="enterprise"]').click();
    const box = await page.getByTestId("module-drawer").boundingBox();
    expect((box?.x ?? 0) + (box?.width ?? 0)).toBeLessThanOrEqual(size.width);
    await capture(page, `responsive-menu-${size.width}`);
  });
