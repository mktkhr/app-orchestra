import { expect, test, type Page } from "@playwright/test";

import { signInAsAdmin } from "./helpers/auth";
import {
  addPanel,
  COLUMNS,
  dragPanelAbove,
  dragResizeHandle,
  GRID_MARGIN_PX,
  readPanels,
  requireBoundingBox,
  requireDefined,
  requirePanel,
  ROW_HEIGHT_PX,
} from "./helpers/layout";

/**
 * Browser-driven layout journey (`docs/plans/layout.md` Task 3, Step 2):
 * sign in, open a workspace with three panels, resize two of them by their
 * own handle so they share a row, resize and drag the third to the front by
 * pointer, reload, and see the arrangement the same way - then narrow the
 * viewport to 375px and see it collapse to the single column
 * `docs/specs/layout.md` section 5 describes, with no panel wider than the
 * viewport it is in (AC-L-105).
 *
 * A panel the panel builder saves always starts full width and one row
 * tall - the builder has no width/height field of its own (Task 0's own
 * defaults are what a caller gets by leaving them out) - so the three
 * panels below start identical, and everything that makes one of them
 * "wide" and "first" happens through the pointer during the test itself:
 * パネルA and パネルB are narrowed by their own resize handle so they end
 * up sharing a row, パネルC is grown taller by the same handle (both halves
 * of AC-L-101, "width and height can be changed", across the three
 * panels together) and dragged to the front of the order (AC-L-102). What
 * the drag actually lands on is read back through `/api/workspaces/{id}`
 * afterwards rather than asserted as an exact column count - a rough
 * pointer drag's own pixel math is not the claim; that what it wrote
 * survives a reload is.
 *
 * `e2e/src/layout.test.ts` is this spec's process-level sibling: it drives
 * the same `PATCH` these interactions cause, directly, and asserts exact
 * geometry. AC-L-103's own keyboard claim already has its own test with no
 * pointer event in it at all
 * (`web/src/pages/workspace/ui/WorkspaceGrid.test.tsx`), so this spec
 * drives the pointer half only. `helpers/layout.ts` carries every pointer
 * and geometry helper this test uses.
 */

// Tall enough that the whole arranged workspace - three panels, one of them
// grown several rows tall - always fits without the page ever scrolling: a
// resize or a drag that scrolls the page mid-gesture moves the very panel a
// later step targets, which is a real behaviour a person would have to
// fight too, not something this journey is about.
const WIDE_VIEWPORT = { width: 1280, height: 3200 };
const NARROW_VIEWPORT = { width: 375, height: 812 };

/** Signs in, makes a workspace, and adds three panels to it - every one full width and one row (Task 0's own default), stacked in creation order. */
async function createWorkspaceWithThreePanels(
  page: Page,
): Promise<{ readonly workspaceId: string; readonly workspaceName: string }> {
  const workspaceName = `並べ替えワークスペース-${Date.now()}`;

  await signInAsAdmin(page);
  await page.getByLabel("新しいワークスペース名").fill(workspaceName);
  await page.getByRole("button", { name: "ワークスペースを作成" }).click();
  await page.getByRole("link", { name: workspaceName }).click();
  await page.waitForLoadState("networkidle");
  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();

  await addPanel(page, "パネルA");
  await addPanel(page, "パネルB");
  await addPanel(page, "パネルC");

  const workspaceId = requireDefined(
    page.url().split("/workspaces/")[1],
    "expected the address to name the workspace's id",
  );

  return { workspaceId, workspaceName };
}

test("resizing two panels to share a row, resizing and moving the third to the front, and reloading (AC-L-101, AC-L-102, AC-L-105)", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await page.setViewportSize(WIDE_VIEWPORT);

  const { workspaceId, workspaceName } = await createWorkspaceWithThreePanels(page);
  const initial = await readPanels(page, workspaceId);

  expect(requirePanel(initial, "パネルA").width).toBe(12);
  expect(requirePanel(initial, "パネルB").width).toBe(12);
  expect(requirePanel(initial, "パネルC").height).toBe(1);

  // `react-grid-layout`'s own column math: each column occupies `colWidth`
  // pixels with a `GRID_MARGIN_PX` gap between columns (11 gaps across 12
  // columns, not 12 - the two outer edges are the grid container's own
  // padding, not a column boundary), so dragging the handle left by six of
  // these - `colWidth` plus the gap it frees - reads back as six fewer
  // columns.
  const gridBox = await requireBoundingBox(page.locator(".react-grid-layout"), "no grid found");
  const colWidth = (gridBox.width - GRID_MARGIN_PX * (COLUMNS - 1)) / COLUMNS;
  const perColumnPx = colWidth + GRID_MARGIN_PX;

  await dragResizeHandle(page, "パネルA", -6 * perColumnPx, 0);
  await dragResizeHandle(page, "パネルB", -6 * perColumnPx, 0);
  await dragResizeHandle(page, "パネルC", 0, ROW_HEIGHT_PX);
  await page.waitForLoadState("networkidle");
  await dragPanelAbove(page, "パネルC", "パネルA");
  await page.waitForLoadState("networkidle");

  const arranged = await readPanels(page, workspaceId);
  const a = requirePanel(arranged, "パネルA");
  const b = requirePanel(arranged, "パネルB");
  const c = requirePanel(arranged, "パネルC");

  expect(c.position).toBe(0);
  expect(c.height).toBeGreaterThan(1);
  expect(a.width).toBeLessThan(12);
  expect(b.width).toBeLessThan(12);
  expect(c.width).toBeGreaterThan(a.width);

  // Reload - a fresh read of the workspace from the platform, not anything
  // held in the page's own memory - and see it drawn the same way: C
  // first, A and B after it (AC-L-101, AC-L-102).
  await page.reload();
  await page.waitForLoadState("networkidle");
  await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();

  const titlesInOrder = await page.locator(".MuiCardHeader-title").allTextContents();

  expect(titlesInOrder).toEqual(["パネルC", "パネルA", "パネルB"]);

  const reloaded = await readPanels(page, workspaceId);

  expect(requirePanel(reloaded, "パネルC")).toMatchObject({
    position: 0,
    width: c.width,
    height: c.height,
  });
  expect(requirePanel(reloaded, "パネルA").width).toBe(a.width);
  expect(requirePanel(reloaded, "パネルB").width).toBe(b.width);

  // At 375px the grid is static (`docs/specs/layout.md` section 5): every
  // panel spans the single column regardless of the width it was just
  // given, the same order survives, and no panel is wider than the
  // viewport it is in (AC-L-105).
  await page.setViewportSize(NARROW_VIEWPORT);
  await page.waitForLoadState("networkidle");

  const narrowTitlesInOrder = await page.locator(".MuiCardHeader-title").allTextContents();

  expect(narrowTitlesInOrder).toEqual(["パネルC", "パネルA", "パネルB"]);

  const viewport = requireDefined(page.viewportSize(), "expected a viewport size");
  const narrowItems = await page.locator(".react-grid-item").all();

  for (const item of narrowItems) {
    const box = await requireBoundingBox(item, "no bounding box for a narrow grid item");

    expect(box.x + box.width).toBeLessThanOrEqual(viewport.width + 1);
  }
});
