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

  // AC-L-107: a panel's narrow height draws on the narrow breakpoint
  // without touching what it draws as on the wide one.
  it("draws narrowHeight on the narrow breakpoint, and height on the wide one, for the same panel", () => {
    const withNarrowHeight = panel({ id: "pnl-1", height: 3, narrowHeight: 1 });

    const wide = buildPanelLayout([withNarrowHeight], 12, true);
    const narrow = buildPanelLayout([withNarrowHeight], 1, false);

    expect(wide[0]?.h).toBe(3);
    expect(narrow[0]?.h).toBe(1);
  });

  // AC-L-108: no narrow height of its own is as tall as height, on both
  // breakpoints - exactly how every panel saved before this field existed
  // drew.
  it("draws at height on both breakpoints when narrowHeight is absent (AC-L-108)", () => {
    const withoutNarrowHeight = panel({ id: "pnl-1", height: 3 });

    const wide = buildPanelLayout([withoutNarrowHeight], 12, true);
    const narrow = buildPanelLayout([withoutNarrowHeight], 1, false);

    expect(wide[0]?.h).toBe(3);
    expect(narrow[0]?.h).toBe(3);
  });

  it("draws at height on the narrow breakpoint when narrowHeight is null", () => {
    const withNullNarrowHeight = panel({ id: "pnl-1", height: 3, narrowHeight: null });

    const narrow = buildPanelLayout([withNullNarrowHeight], 1, false);

    expect(narrow[0]?.h).toBe(3);
  });

  it("falls back to height on the narrow breakpoint when narrowHeight is not a finite number", () => {
    const invalidNarrowHeight = panel({
      id: "pnl-1",
      height: 3,
      narrowHeight: Number.NaN,
    });

    const narrow = buildPanelLayout([invalidNarrowHeight], 1, false);

    expect(narrow[0]?.h).toBe(3);
  });
});
