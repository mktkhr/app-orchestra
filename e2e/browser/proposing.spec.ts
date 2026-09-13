import { expect, test } from "@playwright/test";

import { signInAsAdmin } from "./helpers/auth";

/**
 * Browser-driven journey for "asking for a panel"
 * (`docs/plans/proposing.md`, Task 2; `docs/specs/proposing.md`, section 5):
 * ask a workspace's chat for a chart of something, accept it, reload, see
 * the panel.
 *
 * Drives the built product the same way `workspace.spec.ts` does, against
 * the propose-shaped fixture `playwright.config.ts`'s shared
 * `planFixtures` array carries for this spec ("在庫をステータス別に棒グラフで
 * 置いて") - the same query/response `WorkspacePage.test.tsx` already
 * exercises at the component level. No real LLM is ever called (the stub
 * planner answers over `ORCHESTRA_PLAN_FIXTURES`).
 *
 * Signs in as admin first (`helpers/auth.ts`), then makes a workspace the
 * same way `workspace.spec.ts` does - by asking a question on the chat
 * screen and saving its result - since there is no "new workspace" control
 * anywhere else in the product.
 */
test("asking a workspace's chat for a panel, accepting it, and seeing it after a reload (AC-N-101, AC-N-102, AC-N-103)", async ({
  page,
}) => {
  const workspaceName = `在庫ワークスペース-${Date.now()}`;

  await signInAsAdmin(page);

  // Ask on the chat screen and save the result to a new workspace, the
  // same way workspace.spec.ts makes one - there is no other route to a
  // workspace that exists yet.
  await page.getByRole("button", { name: "在庫の一覧を見せて" }).click();
  await expect(page.getByRole("table")).toBeVisible();

  await page.getByRole("button", { name: "ワークスペースに保存" }).click();

  const workspaceSelect = page.getByLabel("保存先のワークスペース");

  await expect(workspaceSelect).toBeVisible();
  await workspaceSelect.click();
  await page.getByRole("option", { name: "新しいワークスペースを作る" }).click();
  await page.getByRole("main").getByLabel("新しいワークスペース名").fill(workspaceName);
  await page.getByRole("button", { name: "保存" }).click();

  await expect(page.getByText(`「${workspaceName}」に保存しました`)).toBeVisible();

  // Open the workspace this created - only its own chat offers a proposal
  // (N4, AC-N-104; proved at the component level by ConversationPanel.test.tsx).
  await page.reload();
  await page.waitForLoadState("networkidle");
  await page.getByRole("link", { name: workspaceName }).click();
  await page.waitForLoadState("networkidle");

  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();

  // Ask for a panel: the stub planner answers this exact query with a
  // proposal for inventory/ListInventoryItems, drawn as the builder's own
  // form filled in (Task 1).
  await page.getByLabel("質問を入力").fill("在庫をステータス別に棒グラフで置いて");
  await page.getByRole("button", { name: "送信" }).click();

  // Accept it: AddPanelForm's own save control, relabelled "配置" for a
  // proposal (ProposalControl.tsx).
  await page.getByRole("button", { name: "配置" }).click();
  await expect(page.getByText("を配置しました")).toBeVisible();

  // AC-N-103's edited-before-placing case, and the form's own field-level
  // behaviour, are covered at the component level (ProposalControl.test.tsx);
  // this journey proves the accepted panel actually survives - reload, and
  // it draws again from the saved workspace, not from anything still held
  // in memory (the same check workspace.spec.ts makes for a hand-built one).
  await page.reload();
  await page.waitForLoadState("networkidle");

  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();
  await expect(page.getByText("ステータス別の在庫")).toBeVisible();
});
