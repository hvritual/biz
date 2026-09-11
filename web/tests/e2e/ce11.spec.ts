import { test, expect } from "@playwright/test";

test("TestCE11 controlled Vue baseline keeps preview explicit and overlay non-displacing", async ({ page }) => {
  await page.goto("/#/enterprise/members");
  await expect(page.getByRole("heading", { name: "成员管理", exact: true })).toBeVisible();

  const main = page.getByTestId("main-content");
  const before = await main.boundingBox();
  await page.locator('[data-nav-id="enterprise"]').click();

  const drawer = page.getByTestId("module-drawer");
  await expect(drawer).toBeVisible();
  const box = await drawer.boundingBox();
  expect(box?.width).toBe(480);
  expect(await main.boundingBox()).toEqual(before);

  await expect(drawer.locator(".flyout-link .lucide-chevron-right")).toHaveCount(0);
  await expect(drawer.locator(".flyout-shortcuts a")).toHaveCount(6);
});
