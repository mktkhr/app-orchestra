import { describe, expect, it } from "vite-plus/test";

import type { WorkspacePanel } from "@/shared/api/client";

import {
  clampHeight,
  clampWidth,
  moveChanges,
  positionChanges,
  resizeChange,
  sizeChange,
} from "./arrangement";

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

describe("positionChanges", () => {
  it("returns nothing when a drag ends back where every panel already was", () => {
    const panels = [panel({ id: "left", position: 0 }), panel({ id: "right", position: 1 })];

    const changes = positionChanges(panels, [
      { i: "left", x: 0, y: 0, w: 6, h: 1 },
      { i: "right", x: 6, y: 0, w: 6, h: 1 },
    ]);

    expect(changes).toEqual([]);
  });

  it("PATCHes only the panels whose position actually changed, not a renumbering of everyone", () => {
    const panels = [
      panel({ id: "first", position: 0 }),
      panel({ id: "second", position: 1 }),
      panel({ id: "third", position: 2 }),
    ];

    // "first" is dragged past "second": "first" and "second" swap places,
    // "third" stays exactly where it already was.
    const changes = positionChanges(panels, [
      { i: "second", x: 0, y: 0, w: 4, h: 1 },
      { i: "first", x: 4, y: 0, w: 4, h: 1 },
      { i: "third", x: 8, y: 0, w: 4, h: 1 },
    ]);

    expect(changes.toSorted((a, b) => a.id.localeCompare(b.id))).toEqual([
      { id: "first", position: 1 },
      { id: "second", position: 0 },
    ]);
  });
});

describe("sizeChange", () => {
  it("reads the one resized panel's own width and height, clamped", () => {
    expect(sizeChange({ i: "pnl-1", x: 0, y: 0, w: 40, h: 0 })).toEqual({
      id: "pnl-1",
      width: 12,
      height: 1,
    });
  });
});

describe("moveChanges", () => {
  const panels = [
    panel({ id: "first", position: 0 }),
    panel({ id: "second", position: 1 }),
    panel({ id: "third", position: 2 }),
  ];

  it("swaps with the previous panel, and no other", () => {
    expect(
      moveChanges(panels, "second", "previous").toSorted((a, b) => a.id.localeCompare(b.id)),
    ).toEqual([
      { id: "first", position: 1 },
      { id: "second", position: 0 },
    ]);
  });

  it("swaps with the next panel, and no other", () => {
    expect(
      moveChanges(panels, "second", "next").toSorted((a, b) => a.id.localeCompare(b.id)),
    ).toEqual([
      { id: "second", position: 2 },
      { id: "third", position: 1 },
    ]);
  });

  it("does nothing at either end of the order", () => {
    expect(moveChanges(panels, "first", "previous")).toEqual([]);
    expect(moveChanges(panels, "third", "next")).toEqual([]);
  });
});

describe("resizeChange", () => {
  it("nudges width and height by the given delta, clamped", () => {
    expect(resizeChange(panel({ width: 12, height: 1 }), 1, 0)).toEqual({
      id: "pnl-1",
      width: 12,
      height: 1,
    });
    expect(resizeChange(panel({ width: 6, height: 2 }), -1, 1)).toEqual({
      id: "pnl-1",
      width: 5,
      height: 3,
    });
    expect(resizeChange(panel({ width: 1, height: 1 }), -1, -1)).toEqual({
      id: "pnl-1",
      width: 1,
      height: 1,
    });
  });
});

describe("clampWidth / clampHeight", () => {
  it("clamps to the grid's own range", () => {
    expect(clampWidth(0)).toBe(1);
    expect(clampWidth(40)).toBe(12);
    expect(clampHeight(0)).toBe(1);
  });
});
