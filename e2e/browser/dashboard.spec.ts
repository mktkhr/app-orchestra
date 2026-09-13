import { expect, test } from "@playwright/test";

import { signInAsAdmin } from "./helpers/auth";

/**
 * Browser-driven dashboard journey (`docs/plans/dashboard.md` Task 7, Step
 * 2): sign in, make a workspace, build a panel over a list operation with
 * no question asked - a group-by and a bar chart, picked entirely from
 * `GET /api/catalog` (`features/panels`, Task 6) - see it draw, reload the
 * page, and see it draw again from the saved `view` (AC-P-102, AC-P-103,
 * AC-P-104).
 *
 * `ListInventoryItems` is a real, unmodified endpoint of the inventory
 * dummy service (`docs/specs/dashboard.md` sections 1/7 keep both dummy
 * services untouched by this whole subproject) - its own `status` field is
 * what gets grouped, and the platform never learns to compute anything: the
 * transform and the chart both run in the browser (P2/P3).
 *
 * No question is ever asked here - the planner is never involved (P8) - so
 * unlike `chat.spec.ts`/`workspace.spec.ts`, this spec needs nothing from
 * `e2e/playwright.config.ts`'s `ORCHESTRA_PLAN_FIXTURES`.
 *
 * Signs in as admin first (`helpers/auth.ts`), the same way every spec in
 * this directory does since `docs/plans/auth.md` Task 6.
 */
test("building a panel over a list operation, with a group-by and a bar chart, and drawing it again after a reload (AC-P-102, AC-P-103, AC-P-104)", async ({
  page,
}) => {
  const workspaceName = `在庫ダッシュボード-${Date.now()}`;

  await signInAsAdmin(page);

  // Make a workspace directly from the drawer - no question asked, no
  // result to save from (P8).
  await page.getByLabel("新しいワークスペース名").fill(workspaceName);
  await page.getByRole("button", { name: "ワークスペースを作成" }).click();

  await page.getByRole("link", { name: workspaceName }).click();
  await page.waitForLoadState("networkidle");

  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();

  // Step 1: open the builder and pick the operation - every value from
  // here on comes from the catalogue or from what is typed, never from a
  // question (AC-P-102).
  await page.getByRole("button", { name: "パネルを追加" }).click();

  await page.getByRole("combobox", { name: "操作" }).click();
  await page.getByText("在庫一覧").click();

  // Step 2: draw it as a chart.
  await page.getByRole("combobox", { name: "表示方法" }).click();
  await page.getByRole("option", { name: "グラフ" }).click();

  // Step 3: group by status, counted - the transform half (AC-P-104).
  await page.getByRole("switch", { name: "集計してから描画する" }).click();
  await page.getByRole("combobox", { name: "グループ化する項目" }).click();
  await page.getByRole("option", { name: "ステータス", exact: true }).click();

  // Step 4: the chart's own axes - the grouped rows' own two keys
  // (`transform.ts`'s own output shape), a bar chart by default.
  await page.getByRole("combobox", { name: "分類の軸" }).click();
  await page.getByRole("option", { name: "ステータス", exact: true }).click();
  await page.getByRole("combobox", { name: "値の軸" }).click();
  await page.getByRole("option", { name: "件数", exact: true }).click();

  await page.getByRole("button", { name: "追加" }).click();

  // The panel draws: a bar per status this service's own seed data holds
  // (four - see services/inventory/internal/adapter/repository/memory.go).
  const chart = page.locator(".MuiBarChart-root");

  await expect(chart).toBeVisible();
  await expect(page.locator(".MuiBarChart-element")).toHaveCount(4);

  // Reload - a fresh process reading the panel back from the platform,
  // not anything still held in the page's own memory - and see it draw
  // again from the saved view (AC-P-104, AC-P-106). `/workspaces/{id}`
  // already names this workspace (docs/plans/routing.md), so a bare
  // reload lands back on it without following the link again.
  await page.reload();
  await page.waitForLoadState("networkidle");

  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();
  await expect(page.locator(".MuiBarChart-root")).toBeVisible();
  await expect(page.locator(".MuiBarChart-element")).toHaveCount(4);
});

/**
 * Browser-driven edit journey (`docs/specs/dashboard.md` section 6a, P11,
 * P12; AC-P-110): build a plain table panel, open the builder's own form
 * over it, rename it and turn it into a bar chart, save through `PATCH`,
 * see the change draw immediately, reload, and see it draw as edited - not
 * as it was before.
 */
test("editing a panel's title and turning it into a chart draws the change immediately and after a reload (AC-P-108, AC-P-110)", async ({
  page,
}) => {
  const workspaceName = `編集ダッシュボード-${Date.now()}`;

  await signInAsAdmin(page);

  await page.getByLabel("新しいワークスペース名").fill(workspaceName);
  await page.getByRole("button", { name: "ワークスペースを作成" }).click();

  await page.getByRole("link", { name: workspaceName }).click();
  await page.waitForLoadState("networkidle");

  // Build a plain table panel first - nothing here is what this test is
  // about, only a panel worth editing.
  await page.getByRole("button", { name: "パネルを追加" }).click();
  await page.getByRole("combobox", { name: "操作" }).click();
  await page.getByText("在庫一覧").click();
  await page.getByRole("button", { name: "追加" }).click();

  await expect(page.getByText("在庫一覧", { exact: true })).toBeVisible();

  // Open the edit form over the saved panel - the same builder, over that
  // panel (P12) - and confirm its operation is stated, not offered (P13).
  await page.getByRole("button", { name: "編集" }).click();

  await expect(page.getByText(/在庫一覧/u).last()).toBeVisible();
  await expect(page.getByRole("combobox", { name: "操作" })).toHaveCount(0);

  const titleField = page.getByRole("textbox", { name: "パネル名" });

  await titleField.fill("在庫のステータス別件数");

  await page.getByRole("combobox", { name: "表示方法" }).click();
  await page.getByRole("option", { name: "グラフ" }).click();

  await page.getByRole("combobox", { name: "分類の軸" }).click();
  await page.getByRole("option", { name: "ステータス", exact: true }).click();
  await page.getByRole("combobox", { name: "値の軸" }).click();
  await page.getByRole("option", { name: "数量", exact: true }).click();

  await page.getByRole("button", { name: "保存" }).click();

  // Draws as edited immediately - the same tab, no reload yet (AC-P-108).
  // A panel's own title is a card header, not a heading - only the
  // workspace name above it is one - so this reads it back by text.
  await expect(page.getByText("在庫のステータス別件数").first()).toBeVisible();
  await expect(page.locator(".MuiBarChart-root")).toBeVisible();

  // Reload: a fresh read of the workspace from the platform draws the
  // edited panel, not the one it replaced (AC-P-110).
  await page.reload();
  await page.waitForLoadState("networkidle");

  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();
  await expect(page.getByText("在庫のステータス別件数").first()).toBeVisible();
  await expect(page.locator(".MuiBarChart-root")).toBeVisible();
});
