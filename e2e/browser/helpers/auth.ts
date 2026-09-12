import { expect, type Page } from "@playwright/test";

import { ADMIN_PASSWORD } from "./constants";

/**
 * Signing in through the real sign-in screen, for the browser-driven
 * suites (`e2e/browser/*.spec.ts`). Every route but `GET /api/health` and
 * `POST /api/session` now answers 401 without a session
 * (docs/specs/auth.md, AC-A-102): `AuthGate` (`web/src/app/ui/AuthGate.tsx`)
 * shows `SignInPage` instead of the chat until one exists, so every spec
 * that used to start with a bare `page.goto("/")` has to sign in first
 * (docs/plans/auth.md, Task 6, Step 1).
 *
 * `e2e/src/helpers/auth.ts` is the process-level equivalent: it carries a
 * session cookie over bare `fetch` calls instead of driving a form, so the
 * two share no code - there is no HTTP request in common to factor out of
 * a `Page`-driven flow.
 */

/**
 * Navigates to "/", fills in the sign-in form and submits it, and waits
 * for the chat screen to appear - proof the session actually took, not
 * just that the button was clicked.
 */
export async function signIn(page: Page, name: string, password: string): Promise<void> {
  await page.goto("/");
  await page.waitForLoadState("networkidle");

  await page.getByLabel("名前").fill(name);
  await page.getByLabel("パスワード").fill(password);
  await page.getByRole("button", { name: "サインイン" }).click();

  await expect(page.getByRole("heading", { name: "チャット" })).toBeVisible();
}

/** signIn as the admin account e2e/playwright.config.ts seeds (ADMIN_PASSWORD). */
export async function signInAsAdmin(page: Page): Promise<void> {
  await signIn(page, "admin", ADMIN_PASSWORD);
}
