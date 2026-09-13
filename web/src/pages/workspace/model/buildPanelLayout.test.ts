import { describe, expect, it } from "vite-plus/test";

import type { WorkspacePanel } from "@/shared/api/client";

import { buildPanelLayout } from "./buildPanelLayout";

function panel(overrides: Partial<WorkspacePanel> = {}): WorkspacePanel {
  return {
    id: "pnl-1",
    workspaceId: "ws-1",
    service: "inventory",
    operationId: "ListInventoryItems",
    args: {},
    component: "table",
    title: "パネル",
    position: 0,
    width: 12,
    height: 1,
    ...overrides,
  };
}

describe("buildPanelLayout", () => {
  it("draws widths 12, 6 and 6 as one full-width row and two beside each other", () => {
    const layout = buildPanelLayout(
      [
        panel({ id: "wide", position: 0, width: 12 }),
        panel({ id: "left", position: 1, width: 6 }),
        panel({ id: "right", position: 2, width: 6 }),
      ],
      12,
    );

    expect(layout).toEqual([
      { i: "wide", x: 0, y: 0, w: 12, h: 1, static: true },
      { i: "left", x: 0, y: 1, w: 6, h: 1, static: true },
      { i: "right", x: 6, y: 1, w: 6, h: 1, static: true },
    ]);
  });

  it("draws every panel in position order, not the order it was given in", () => {
    const layout = buildPanelLayout(
      [
        panel({ id: "second", position: 1, width: 12 }),
        panel({ id: "first", position: 0, width: 12 }),
      ],
      12,
    );

    expect(layout.map((item) => item.i)).toEqual(["first", "second"]);
  });

  it("spans the single column on the narrow breakpoint, whatever width was asked for", () => {
    const layout = buildPanelLayout(
      [
        panel({ id: "wide", position: 0, width: 12 }),
        panel({ id: "left", position: 1, width: 6 }),
        panel({ id: "right", position: 2, width: 6 }),
      ],
      1,
    );

    expect(layout).toEqual([
      { i: "wide", x: 0, y: 0, w: 1, h: 1, static: true },
      { i: "left", x: 0, y: 1, w: 1, h: 1, static: true },
      { i: "right", x: 0, y: 2, w: 1, h: 1, static: true },
    ]);
  });

  it("draws a panel with no width full width (AC-L-104)", () => {
    const layout = buildPanelLayout([panel({ id: "default", width: 12 })], 12);

    expect(layout).toEqual([{ i: "default", x: 0, y: 0, w: 12, h: 1, static: true }]);
  });

  it("is static by default, and not static when interactive (Task 2: drag and resize on the wide breakpoint)", () => {
    const panels = [panel({ id: "solo" })];

    expect(buildPanelLayout(panels, 12)[0]?.static).toBe(true);
    expect(buildPanelLayout(panels, 12, false)[0]?.static).toBe(true);
    expect(buildPanelLayout(panels, 12, true)[0]?.static).toBe(false);
  });
  it("falls back to a drawable span when width or height is not a number", () => {
    // A response missing width/height - an older platform, a proxy, anything
    // partial - used to reach react-grid-layout as NaN, which computes a
    // container 16px tall for a 360px panel and draws every element after it
    // underneath, silently. Measured on a real screen; see DECISIONS.md.
    const { width: _width, height: _height, ...missing } = panel({ id: "missing" });
    const layout = buildPanelLayout([missing], 12);

    expect(layout).toEqual([{ i: "missing", x: 0, y: 0, w: 12, h: 1, static: true }]);
  });
});
