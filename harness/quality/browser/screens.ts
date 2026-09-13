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
 * What these gates measure is the screen, not what is on it. `playwright.config.ts`
 * now runs the inventory dummy service alongside the platform (`docs/plans/layout.md`
 * Task 2 Step 4) and names it in `ORCHESTRA_SERVICES` - the smallest catalogue
 * that lets "a workspace" below save one real panel, since `AddPanel` refuses
 * any operation the catalogue does not expose (`internal/usecase/workspaces.go`),
 * whether or not that panel is ever invoked. This still asserts nothing about
 * that panel's own answer: contrast, target size, visible boundaries and
 * sideways scroll are properties of the controls around it - the header, its
 * buttons, the drag and resize affordances - and those are what a panel
 * finally puts in front of `guard-a11y` and `guard-layout` (AC-L-103). A
 * screen whose layout only breaks once real *rows* arrive is still not
 * covered by this, and would want a gate of its own that seeded more than
 * one operation.
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

/**
 * Saves one panel, over inventory's `ListInventoryItems` - the one
 * operation `ORCHESTRA_SERVICES` names for this config - onto workspaceId,
 * through the loaded application's own `fetch`, for the same reason
 * `createWorkspace` does: this module's job is to put a screen in front of
 * the gate, not to drive the interface to get there.
 *
 * The caller has to have loaded the application and created workspaceId
 * first.
 */
async function createPanel(page: Page, workspaceId: string): Promise<void> {
  const ok = await page.evaluate(async (id: string): Promise<boolean> => {
    const response = await fetch(`/api/workspaces/${id}/panels`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        service: "inventory",
        operationId: "ListInventoryItems",
        args: {},
        component: "table",
        title: "在庫一覧",
      }),
    });

    return response.ok;
  }, workspaceId);

  if (!ok) {
    throw new Error("browser guard: saving a panel failed");
  }
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
    name: "a workspace with a panel in it",
    visit: async (page) => {
      await signInAsAdmin(page);
      await goto(page, "/");
      const workspaceId = await createWorkspace(page);
      await createPanel(page, workspaceId);
      await goto(page, `/#workspace-${workspaceId}`);
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
