import type { Page } from "@playwright/test";

/**
 * The screens a11y.spec.ts and layout.spec.ts measure.
 *
 * Both gates used to measure one screen, at "/". That was right while every
 * screen was at "/" and wrong from the moment signing in came between a
 * visitor and the application: `ae4ba97` removed the sign-in helper both
 * gates used and left them measuring the sign-in screen alone, and every
 * screen built afterwards - the users screen, a workspace, the panel
 * builder, a chart - was never seen by either gate again. They stayed
 * green, which is what made it invisible: a guard reports what it failed
 * and never what it skipped.
 *
 * The helper that was deleted said, in its own doc comment, what should
 * have happened instead: "if a screen only a signed-in person reaches
 * still needs a gate, add it back there deliberately, named for what it
 * is." This is that list.
 *
 * What these gates measure is the screen, not what is on it: the platform
 * `playwright.config.ts` starts has no ORCHESTRA_SERVICES, so its catalogue
 * is empty and no screen here shows a real operation's data. Contrast,
 * target size, visible boundaries and sideways scroll are properties of the
 * controls, and the controls are all here. A screen whose layout only
 * breaks once real rows arrive is not covered by this, and would want a
 * gate of its own that seeded a service.
 */
export interface Screen {
  /** How the failure reads: "a workspace has no accessibility violations". */
  readonly name: string;
  /** Signs in where the screen needs it, navigates, and waits for it to settle. */
  readonly visit: (page: Page) => Promise<void>;
}

/**
 * Signs page in as the admin `playwright.config.ts` seeds through
 * ORCHESTRA_ADMIN_PASSWORD. Every screen but the sign-in screen needs
 * this: since `docs/specs/auth.md`, every route but GET /api/health and
 * /api/session answers 401 without a session, so a screen loaded without
 * one renders an error state nobody is meant to look at.
 */
async function signInAsAdmin(page: Page): Promise<void> {
  const response = await page.request.post("/api/session", {
    data: { name: "admin", password: "guard-admin-password" },
  });

  if (!response.ok()) {
    throw new Error(`browser guard: admin sign-in failed with status ${String(response.status())}`);
  }
}

/**
 * Creates a workspace through the loaded application's own `fetch`, and
 * returns its id, so the workspace screen has one to be. Over the API
 * rather than through the drawer's control: this module's job is to put a
 * screen in front of the gate, and driving the interface to get there would
 * make a layout failure anywhere on the way read as a failure of the screen
 * it was heading for.
 *
 * From inside the page rather than through `page.request`, and the reason
 * is measured: the session cookie is `SameSite=Lax`
 * (`docs/specs/auth.md`, A2), and Lax withholds a cookie from an unsafe
 * method that has no site context of its own. The page's `fetch` has one;
 * `page.request.post` does not, and answers 401 with a perfectly good
 * session sitting in the jar. The page is also the path a person's own
 * click takes.
 *
 * The caller has to have loaded the application first.
 */
async function createWorkspace(page: Page): Promise<string> {
  const id = await page.evaluate(async (): Promise<unknown> => {
    const response = await fetch("/api/workspaces", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ name: "ガードのワークスペース" }),
    });

    if (!response.ok) {
      throw new Error(`creating a workspace answered ${String(response.status)}`);
    }

    const created: unknown = await response.json();

    return typeof created === "object" && created !== null && "id" in created
      ? (created as Record<"id", unknown>).id
      : undefined;
  });

  if (typeof id !== "string" || id === "") {
    throw new Error("browser guard: creating a workspace returned no id");
  }

  return id;
}

/** Navigates to path and waits for the screen to stop moving. */
async function goto(page: Page, path: string): Promise<void> {
  await page.goto(path);
  await page.waitForLoadState("networkidle");
}

export const SCREENS: readonly Screen[] = [
  {
    name: "sign in",
    visit: async (page) => {
      await goto(page, "/");
    },
  },
  {
    name: "the chat",
    visit: async (page) => {
      await signInAsAdmin(page);
      await goto(page, "/");
    },
  },
  {
    name: "the admin's users screen",
    visit: async (page) => {
      await signInAsAdmin(page);
      await goto(page, "/#users");
    },
  },
  {
    name: "a workspace",
    visit: async (page) => {
      await signInAsAdmin(page);
      await goto(page, "/");
      await goto(page, `/#workspace-${await createWorkspace(page)}`);
    },
  },
  {
    // The panel builder is a form behind a control, so reaching it is a
    // click and not a path. It is where every control this product gained
    // most recently lives, which is exactly the set neither gate has ever
    // seen.
    name: "the panel builder",
    visit: async (page) => {
      await signInAsAdmin(page);
      await goto(page, "/");
      await goto(page, `/#workspace-${await createWorkspace(page)}`);
      await page.getByRole("button", { name: "パネルを追加" }).click();
      await page.waitForLoadState("networkidle");
    },
  },
];
