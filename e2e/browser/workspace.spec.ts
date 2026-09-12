import { expect, test } from "@playwright/test";

import { signInAsAdmin } from "./helpers/auth";

/**
 * Browser-driven workspace journey (AC-W-101, AC-W-102, AC-W-103,
 * docs/plans/workspaces.md Task 7 Step 3): ask, save, reopen, refresh.
 *
 * Drives the built product the same way `chat.spec.ts` does - the platform
 * serving `web/dist` against both dummy services, answering through the
 * stub planner fed by `e2e/playwright.config.ts`'s `ORCHESTRA_PLAN_FIXTURES`
 * (no real LLM is ever called by `make check`). `ORCHESTRA_DB_PATH` in that
 * same config is a file inside its own temporary directory, so this spec's
 * workspace does not collide with another suite's.
 *
 * Signs in as admin first (`helpers/auth.ts`) - see `chat.spec.ts`'s own
 * comment on why.
 */
test("asking, saving, reopening and refreshing a workspace panel (AC-W-101, AC-W-102, AC-W-103)", async ({
  page,
}) => {
  const workspaceName = `在庫ワークスペース-${Date.now()}`;

  await signInAsAdmin(page);

  // Ask, the same way chat.spec.ts does.
  const example = page.getByRole("button", { name: "在庫の一覧を見せて" });

  await expect(example).toBeVisible();
  await example.click();

  const table = page.getByRole("table");

  await expect(table).toBeVisible();
  await expect(table.getByText("itm-001")).toBeVisible();

  // Save the result to a new workspace.
  await page.getByRole("button", { name: "ワークスペースに保存" }).click();

  const workspaceSelect = page.getByLabel("保存先のワークスペース");

  await expect(workspaceSelect).toBeVisible();
  await workspaceSelect.click();
  await page.getByRole("option", { name: "新しいワークスペースを作る" }).click();

  // The drawer's own "make a workspace" form carries a field with this
  // same label, so scope to <main>, where the save control lives.
  await page.getByRole("main").getByLabel("新しいワークスペース名").fill(workspaceName);
  await page.getByRole("button", { name: "保存" }).click();

  await expect(page.getByText(`「${workspaceName}」に保存しました`)).toBeVisible();

  // Reopen it: the drawer's own workspace list is only loaded on mount, so
  // reload the page the way a person who came back later would.
  await page.reload();
  await page.waitForLoadState("networkidle");

  await page.getByRole("link", { name: workspaceName }).click();
  await page.waitForLoadState("networkidle");

  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();

  const panelTable = page.getByRole("table");

  await expect(panelTable).toBeVisible();
  await expect(panelTable.getByText("itm-001")).toBeVisible();

  // Refresh the panel: the control never disables itself (AC-W-103), it
  // only swaps its label while the call is in flight.
  const refresh = page.getByRole("button", { name: "更新" });

  await refresh.click();
  await expect(page.getByRole("button", { name: "更新" })).toBeVisible();
  await expect(panelTable.getByText("itm-001")).toBeVisible();
});
