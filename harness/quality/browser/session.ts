import type { Page } from "@playwright/test";

/**
 * Signs a11y.spec.ts and layout.spec.ts's own Playwright page in as the
 * admin seeded by playwright.config.ts's own ORCHESTRA_ADMIN_PASSWORD
 * ("guard-admin-password"), before either gate measures a screen.
 *
 * Temporary, for docs/plans/auth.md's Task 2 only: every route but
 * GET /api/health and /api/session now answers 401 without a session
 * (docs/specs/auth.md, section 6), so the screen these gates load at "/" -
 * still the chat screen; Task 4 has not built a sign-in screen yet - would
 * otherwise render against a platform that refuses everything it asks.
 * Signing in first keeps these gates measuring the same real screen they
 * always have, rather than an error state Task 2 alone would introduce and
 * Task 6 never asked anyone to look at.
 *
 * Task 4 lands a sign-in screen at "/" for a visitor with no session -
 * PUBLIC_PAGES in a11y.spec.ts already names it that, forward-looking. At
 * that point this call is no longer signing in before measuring the public
 * page; it is measuring a screen nobody sees. Whoever lands Task 4 should
 * delete this call from both PUBLIC_PAGES's own target and layout.spec.ts's
 * page.goto("/") gates, and, if a screen only a signed-in person reaches
 * still needs a gate, add it back there deliberately, named for what it
 * is.
 */
export async function signInAsAdmin(page: Page): Promise<void> {
  const response = await page.request.post("/api/session", {
    data: { name: "admin", password: "guard-admin-password" },
  });

  if (!response.ok()) {
    throw new Error(`browser guard: admin sign-in failed with status ${response.status()}`);
  }
}
