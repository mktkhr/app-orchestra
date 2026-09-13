import { expect, test } from "@playwright/test";

import { signInAsAdmin } from "./helpers/auth";
import { ADMIN_PASSWORD } from "./helpers/constants";

/**
 * Browser-driven routing journey (`docs/plans/routing.md` Task 2, Step 4;
 * `docs/specs/routing.md` section 7).
 *
 * `e2e/src/routing.test.ts` proves the platform's own half of
 * AC-R-102/AC-R-103 (the fallback, over real TCP, with no browser); this
 * file proves the point the hash router existed to avoid reloading past:
 * a workspace opened by clicking keeps its own address, and reloading it
 * draws the same workspace again rather than falling back to the chat
 * (AC-R-101) - and, separately, that a person with no session reaches the
 * sign-in screen from any address and lands where they were going once
 * they sign in (AC-R-104).
 */
test("opening a workspace by its address and reloading still draws it (AC-R-101)", async ({
  page,
}) => {
  const workspaceName = `経路ワークスペース-${Date.now()}`;

  await signInAsAdmin(page);
  await page.getByLabel("新しいワークスペース名").fill(workspaceName);
  await page.getByRole("button", { name: "ワークスペースを作成" }).click();
  await page.getByRole("link", { name: workspaceName }).click();
  await page.waitForLoadState("networkidle");

  const workspaceUrl = page.url();

  expect(workspaceUrl).toMatch(/\/workspaces\/.+$/u);
  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();

  // Reload is the whole point (`docs/plans/routing.md` Task 2): a fresh
  // request for this exact address has to draw the same workspace, not
  // the chat `useHashRoute` would have fallen back to.
  await page.reload();
  await page.waitForLoadState("networkidle");

  expect(page.url()).toBe(workspaceUrl);
  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();

  // Same for /users (docs/specs/routing.md, AC-R-101's own wording): a
  // second address, reached this time through the drawer's own link
  // rather than a row `WorkspaceList` renders, survives the same way.
  await page.getByRole("link", { name: "ユーザー管理" }).click();
  await page.waitForLoadState("networkidle");
  expect(page.url()).toMatch(/\/users$/u);

  await page.reload();
  await page.waitForLoadState("networkidle");

  expect(page.url()).toMatch(/\/users$/u);
  await expect(page.getByRole("heading", { name: "ユーザー管理" })).toBeVisible();
});

test("a person with no session reaches sign-in from a workspace's own address, and lands there after signing in (AC-R-104)", async ({
  page,
}) => {
  const workspaceName = `セッション切れワークスペース-${Date.now()}`;

  await signInAsAdmin(page);
  await page.getByLabel("新しいワークスペース名").fill(workspaceName);
  await page.getByRole("button", { name: "ワークスペースを作成" }).click();
  await page.getByRole("link", { name: workspaceName }).click();
  await page.waitForLoadState("networkidle");

  const workspaceUrl = page.url();

  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();

  // The session ends on the server, not only in the page's own state
  // (auth.spec.ts's own AC-A-107 journey) - this is what leaves nobody
  // signed in for the direct navigation below to find.
  await page.getByRole("button", { name: "サインアウト" }).click();
  await expect(page.getByRole("heading", { name: "サインイン" })).toBeVisible();

  // Typed straight into a fresh browser, this workspace's own address
  // still reaches the sign-in screen - not a blank page, and not a silent
  // fallback to the chat at "/" (AC-R-104's first half). `page.goto` is a
  // full navigation, the same as pasting the address and pressing enter.
  await page.goto(workspaceUrl);
  await page.waitForLoadState("networkidle");
  await expect(page.getByRole("heading", { name: "サインイン" })).toBeVisible();

  // Signing in from here - without navigating anywhere else - lands back
  // on the workspace the address named (AC-R-104's second half).
  // `BrowserRouter` sits above `AuthGate` (`web/src/app/App.tsx`) and
  // reads `window.location.pathname`; signing in only changes which
  // component `AuthGate` renders under it (`SessionProvider.signIn` never
  // navigates), so the path never left `workspaceUrl` while the sign-in
  // screen was up.
  await page.getByLabel("名前").fill("admin");
  await page.getByLabel("パスワード").fill(ADMIN_PASSWORD);
  await page.getByRole("button", { name: "サインイン" }).click();

  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();
  expect(page.url()).toBe(workspaceUrl);
});
