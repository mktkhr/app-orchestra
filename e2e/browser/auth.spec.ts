import { expect, test } from "@playwright/test";

import { signInAsAdmin } from "./helpers/auth";

/**
 * Browser-driven sign-in journey (docs/plans/auth.md, Task 6, Step 3;
 * docs/specs/auth.md, AC-A-107 end to end): sign in, see the chat, sign
 * out, and land back on the sign-in screen - a fresh reload afterwards
 * still shows it, rather than a chat a stale client-side state kept
 * drawing.
 */
test("signing in reaches the chat, and signing out returns to the sign-in screen (AC-A-107)", async ({
  page,
}) => {
  await signInAsAdmin(page);

  await expect(page.getByRole("heading", { name: "チャット" })).toBeVisible();
  await expect(page.getByText("admin")).toBeVisible();

  await page.getByRole("button", { name: "サインアウト" }).click();

  await expect(page.getByRole("heading", { name: "サインイン" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "チャット" })).toHaveCount(0);

  // A reload after signing out still shows the sign-in screen - the
  // session actually ended on the server, not only in the page's own
  // state (AC-A-107: "ends on sign-out").
  await page.reload();
  await page.waitForLoadState("networkidle");

  await expect(page.getByRole("heading", { name: "サインイン" })).toBeVisible();
});
