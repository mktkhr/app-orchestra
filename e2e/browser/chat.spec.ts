import { expect, test } from "@playwright/test";

import { signInAsAdmin } from "./helpers/auth";

/**
 * Browser-driven end to end (AC-E-101).
 *
 * Drives the built product - the platform serving `web/dist`
 * (`ORCHESTRA_STATIC_DIR`, see `e2e/playwright.config.ts`'s webServer) -
 * against both dummy services, started from their own built binaries. The
 * platform answers through the stub planner (no real LLM is ever called by
 * `make check`): `e2e/playwright.config.ts` feeds it the one question this
 * spec asks through `ORCHESTRA_PLAN_FIXTURES`.
 *
 * Signs in as admin first (`helpers/auth.ts`) - every route but
 * `GET /api/health` and `POST /api/session` now answers 401 without a
 * session (docs/specs/auth.md, AC-A-102).
 */
test("clicking the list example question renders a table (AC-E-101)", async ({ page }) => {
  await signInAsAdmin(page);

  const example = page.getByRole("button", { name: "在庫の一覧を見せて" });

  await expect(example).toBeVisible();
  await example.click();

  // The question becomes a turn, followed by the platform's answer: a
  // table naming the inventory operation that produced it, with rows from
  // the running dummy service's own in-memory fixture data.
  await expect(page.getByText("在庫の一覧を見せて", { exact: true })).toBeVisible();
  await expect(page.getByText("inventory / ListInventoryItems")).toBeVisible();

  const table = page.getByRole("table");

  await expect(table).toBeVisible();
  await expect(table.getByText("itm-001")).toBeVisible();
});
