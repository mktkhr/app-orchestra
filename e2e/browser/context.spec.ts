import { expect, test } from "@playwright/test";

import { signInAsAdmin } from "./helpers/auth";

/**
 * Browser-driven multi-turn context journey (docs/plans/context.md, Task 4,
 * Step 2; docs/specs/context.md, AC-M-101).
 *
 * Asks the example inventory question, then types a second question that
 * names no service at all, and expects the answer to still name the
 * inventory operation the first question did - proof that the turn the
 * first question produced (`toContextTurns`, `web/src/shared/api/client.ts`)
 * actually reaches `POST /api/plan` and comes back out the other side.
 * `e2e/playwright.config.ts`'s `ORCHESTRA_PLAN_FIXTURES` carries the
 * follow-up fixture, keyed on the turn (`stub.Key.Turns`,
 * docs/plans/context.md, Task 4) the same way `e2e/src/context.test.ts`
 * proves at the process level, without a browser.
 */
test("a follow-up naming no service stays on the service the first question used (AC-M-101)", async ({
  page,
}) => {
  await signInAsAdmin(page);

  await page.getByRole("button", { name: "在庫の一覧を見せて" }).click();

  await expect(page.getByText("在庫の一覧を見せて", { exact: true })).toBeVisible();
  await expect(page.getByText("inventory / ListInventoryItems").first()).toBeVisible();

  await page.getByLabel("質問を入力").fill("検品保留のものだけ見せて");
  await page.getByRole("button", { name: "送信" }).click();

  await expect(page.getByText("検品保留のものだけ見せて", { exact: true })).toBeVisible();
  await expect(page.getByText("inventory / ListInventoryItems").last()).toBeVisible();
});
