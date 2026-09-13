import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import type { WorkspacePanel } from "@/shared/api/client";
import { patchPanel } from "@/shared/api/panels";

import { useArrangement } from "./useArrangement";

vi.mock("@/shared/api/panels", () => ({
  patchPanel: vi.fn<typeof patchPanel>(),
}));

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

describe("useArrangement", () => {
  beforeEach(() => {
    vi.mocked(patchPanel).mockReset();
    vi.mocked(patchPanel).mockResolvedValue(panel());
  });

  it("PATCHes only the panels a drag actually moved, and shows the result immediately", () => {
    const panels = [panel({ id: "first", position: 0 }), panel({ id: "second", position: 1 })];
    const { result } = renderHook(() => useArrangement("ws-1", panels));

    result.current.onDragStop([
      { i: "second", x: 0, y: 0, w: 6, h: 1 },
      { i: "first", x: 6, y: 0, w: 6, h: 1 },
    ]);

    expect(vi.mocked(patchPanel)).toHaveBeenCalledTimes(2);
    expect(vi.mocked(patchPanel)).toHaveBeenCalledWith("ws-1", "first", { position: 1 });
    expect(vi.mocked(patchPanel)).toHaveBeenCalledWith("ws-1", "second", { position: 0 });
  });

  it("PATCHes no panel when a drag ends without changing anyone's position", () => {
    const panels = [panel({ id: "first", position: 0 }), panel({ id: "second", position: 1 })];
    const { result } = renderHook(() => useArrangement("ws-1", panels));

    result.current.onDragStop([
      { i: "first", x: 0, y: 0, w: 6, h: 1 },
      { i: "second", x: 6, y: 0, w: 6, h: 1 },
    ]);

    expect(vi.mocked(patchPanel)).not.toHaveBeenCalled();
  });

  it("a resize PATCHes exactly the one panel that was resized", () => {
    const panels = [panel({ id: "first", position: 0, width: 6, height: 1 })];
    const { result } = renderHook(() => useArrangement("ws-1", panels));

    result.current.onResizeStop(
      [{ i: "first", x: 0, y: 0, w: 8, h: 2 }],
      { i: "first", x: 0, y: 0, w: 6, h: 1 },
      { i: "first", x: 0, y: 0, w: 8, h: 2 },
    );

    expect(vi.mocked(patchPanel)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(patchPanel)).toHaveBeenCalledWith("ws-1", "first", { width: 8, height: 2 });
  });

  it("shows a change immediately, before the reload that would otherwise carry it", () => {
    const panels = [panel({ id: "first", position: 0, width: 6, height: 1 })];
    const { result, rerender } = renderHook(({ panels: p }) => useArrangement("ws-1", p), {
      initialProps: { panels },
    });

    result.current.onResizeStop(
      [{ i: "first", x: 0, y: 0, w: 8, h: 2 }],
      { i: "first", x: 0, y: 0, w: 6, h: 1 },
      { i: "first", x: 0, y: 0, w: 8, h: 2 },
    );
    rerender({ panels });

    expect(result.current.panels[0]).toMatchObject({ width: 8, height: 2 });
  });

  // AC-L-107: narrowResizeBy PATCHes only narrowHeight, and leaves the
  // panel's own width and height out of the request entirely.
  it("narrowResizeBy PATCHes only narrowHeight, leaving width and height alone", () => {
    const panels = [panel({ id: "first", position: 0, width: 6, height: 3 })];
    const { result } = renderHook(() => useArrangement("ws-1", panels));

    result.current.narrowResizeBy("first", 1);

    expect(vi.mocked(patchPanel)).toHaveBeenCalledExactlyOnceWith("ws-1", "first", {
      narrowHeight: 4,
    });
  });

  it("narrowResizeBy shows the change immediately, and leaves height untouched", () => {
    const panels = [panel({ id: "first", position: 0, width: 6, height: 3 })];
    const { result, rerender } = renderHook(({ panels: p }) => useArrangement("ws-1", p), {
      initialProps: { panels },
    });

    result.current.narrowResizeBy("first", 1);
    rerender({ panels });

    expect(result.current.panels[0]).toMatchObject({ narrowHeight: 4, height: 3 });
  });

  it("narrowResizeBy does nothing for a panel id that is not in the workspace", () => {
    const panels = [panel({ id: "first", position: 0 })];
    const { result } = renderHook(() => useArrangement("ws-1", panels));

    result.current.narrowResizeBy("missing", 1);

    expect(vi.mocked(patchPanel)).not.toHaveBeenCalled();
  });
});
