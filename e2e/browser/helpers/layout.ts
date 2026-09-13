import { expect, type Locator, type Page } from "@playwright/test";

/**
 * Pointer geometry helpers for `layout.spec.ts` - dragging a
 * `react-grid-layout` panel's own resize handle and its own header through
 * real mouse events, and reading the workspace's geometry back over the
 * page's own session. Kept out of the spec file itself only for
 * `eslint`'s own `max-lines` (`harness/quality/file-length.txt`); nothing
 * here is shared with another spec.
 */

/** `WorkspaceGrid.tsx`'s own constants: 12 columns, a 16px margin around and between them, a 360px row plus its own margin. */
export const COLUMNS = 12;
export const GRID_MARGIN_PX = 16;
export const ROW_HEIGHT_PX = 360 + GRID_MARGIN_PX;

export interface PanelOnWire {
  readonly id: string;
  readonly title: string;
  readonly position: number;
  readonly width: number;
  readonly height: number;
}

interface WorkspaceOnWire {
  readonly panels: readonly PanelOnWire[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isPanelOnWire(value: unknown): value is PanelOnWire {
  return (
    isRecord(value) &&
    typeof value["id"] === "string" &&
    typeof value["title"] === "string" &&
    typeof value["position"] === "number" &&
    typeof value["width"] === "number" &&
    typeof value["height"] === "number"
  );
}

function isWorkspaceOnWire(value: unknown): value is WorkspaceOnWire {
  return isRecord(value) && Array.isArray(value["panels"]) && value["panels"].every(isPanelOnWire);
}

function parseWorkspace(value: unknown): WorkspaceOnWire {
  if (!isWorkspaceOnWire(value)) {
    throw new Error(`unexpected workspace response shape: ${JSON.stringify(value)}`);
  }

  return value;
}

/** value, or throws message - kept out of every test body so a missing value is a setup failure, not a branch inside the test itself. */
export function requireDefined<T>(value: T | null | undefined, message: string): T {
  if (value === null || value === undefined) {
    throw new Error(message);
  }

  return value;
}

/** The one panel titled title, or throws. */
export function requirePanel(panels: readonly PanelOnWire[], title: string): PanelOnWire {
  return requireDefined(
    panels.find((panel) => panel.title === title),
    `expected a panel titled ${title}`,
  );
}

/**
 * locator's bounding box, or throws - `boundingBox()` is `null` for an
 * element that is not currently rendered. Scrolled into view first:
 * `boundingBox()` reports viewport-relative coordinates and does not
 * scroll on its own, and a resize handle dragged toward the bottom of a
 * tall panel can leave the page scrolled with the very panel a later drag
 * targets sitting above the viewport, at a negative `y` a mouse can't
 * click.
 */
export async function requireBoundingBox(
  locator: Locator,
  message: string,
): Promise<{
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}> {
  await locator.scrollIntoViewIfNeeded();

  return requireDefined(await locator.boundingBox(), message);
}

/**
 * Reads the workspace's panels back over a real request that carries the
 * session `signInAsAdmin` already put in the browser's own cookie jar -
 * `page.request` shares that jar (unlike a bare `fetch` from Node), so no
 * page needs to be loaded to ask it, and nothing here runs inside the page
 * itself.
 */
export async function readPanels(page: Page, workspaceId: string): Promise<readonly PanelOnWire[]> {
  const response = await page.request.get(`/api/workspaces/${workspaceId}`);

  return parseWorkspace(await response.json()).panels;
}

/** Adds one table panel over 在庫一覧 (inventory/ListInventoryItems), titled title. */
export async function addPanel(page: Page, title: string): Promise<void> {
  await page.getByRole("button", { name: "パネルを追加" }).click();
  await page.getByRole("combobox", { name: "操作" }).click();
  await page.getByText("在庫一覧").click();
  await page.getByRole("textbox", { name: "パネル名" }).fill(title);
  await page.getByRole("button", { name: "追加" }).click();
  await expect(page.getByText(title, { exact: true })).toBeVisible();
}

/** The `.react-grid-item` that carries title in its own header. */
export function gridItem(page: Page, title: string): Locator {
  return page.locator(".react-grid-item", { has: page.getByText(title, { exact: true }) });
}

/**
 * A point inside title's own header bar, near its left edge - away from
 * the refresh/edit/arrange controls `PanelActions` draws at the header's
 * right (excluded from dragging by `WorkspaceGrid`'s own
 * `draggableCancel`), and read straight off the whole `.react-grid-item`
 * rather than the title text's own span so a text node truncated by
 * `CardHeader`'s own ellipsis never throws this off.
 */
async function headerPoint(
  page: Page,
  title: string,
): Promise<{ readonly x: number; readonly y: number }> {
  const box = await requireBoundingBox(gridItem(page, title), `no grid item found for ${title}`);

  return { x: box.x + 40, y: box.y + 20 };
}

/**
 * `.react-grid-item.cssTransforms`'s own transition (`workspaceGrid.css`):
 * every panel's own position animates over 200ms whenever one panel's
 * resize moves another - so a resize or a drag that reads another panel's
 * position right afterward can read it mid-slide. Longer than the
 * transition itself, the way a person moving the very next panel by hand
 * would not manage to beat it either.
 */
const SETTLE_MS = 300;

/** Drags title's own resize handle by (dx, dy) pixels - `react-grid-layout`'s default single handle, bottom-right (AC-L-102, "resized by its handle"). */
export async function dragResizeHandle(
  page: Page,
  title: string,
  dx: number,
  dy: number,
): Promise<void> {
  const handle = gridItem(page, title).locator(".react-resizable-handle");
  const box = await requireBoundingBox(handle, `no resize handle found for ${title}`);
  const startX = box.x + box.width / 2;
  const startY = box.y + box.height / 2;

  await page.mouse.move(startX, startY);
  await page.mouse.down();
  await page.mouse.move(startX + dx, startY + dy, { steps: 12 });
  await page.mouse.up();
  await page.waitForTimeout(SETTLE_MS);
}

/** Drags title's own header (not a button, so `WorkspaceGrid`'s own `draggableCancel` does not exclude it) to just above aboveTitle's own header (AC-L-102, "dragged to a new place"). */
export async function dragPanelAbove(page: Page, title: string, aboveTitle: string): Promise<void> {
  const { x: startX, y: startY } = await headerPoint(page, title);
  const target = await headerPoint(page, aboveTitle);
  const endX = target.x;
  const endY = target.y - 40;

  await page.mouse.move(startX, startY);
  await page.mouse.down();
  await page.mouse.move(startX, startY - 10, { steps: 5 });
  await page.mouse.move((startX + endX) / 2, (startY + endY) / 2, { steps: 10 });
  await page.mouse.move(endX, endY, { steps: 10 });
  await page.mouse.up();
  await page.waitForTimeout(SETTLE_MS);
}
