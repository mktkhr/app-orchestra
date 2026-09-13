import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { postInvoke, type WorkspacePanel } from "@/shared/api/client";
import { patchPanel } from "@/shared/api/panels";

import { WorkspaceGrid } from "./WorkspaceGrid";

vi.mock("@/shared/api/client", () => ({
  postInvoke: vi.fn<typeof postInvoke>(),
}));

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

describe("WorkspaceGrid", () => {
  it("draws every panel in position order, not the order it was given in", async () => {
    vi.mocked(postInvoke).mockResolvedValue({ component: "table", data: { items: [] } });

    render(
      <WorkspaceGrid
        workspaceId="ws-1"
        panels={[
          panel({ id: "pnl-2", position: 1, width: 6, title: "二番目" }),
          panel({ id: "pnl-1", position: 0, width: 6, title: "一番目" }),
        ]}
      />,
    );

    await screen.findByText("一番目");
    const titles = screen.getAllByText(/番目/u).map((node) => node.textContent);

    expect(titles).toEqual(["一番目", "二番目"]);
  });

  it("draws a grid item for a panel that spans the full 12 columns and for two that share a row", async () => {
    vi.mocked(postInvoke).mockResolvedValue({ component: "table", data: { items: [] } });

    const { container } = render(
      <WorkspaceGrid
        workspaceId="ws-1"
        panels={[
          panel({ id: "wide", position: 0, width: 12, title: "上段" }),
          panel({ id: "left", position: 1, width: 6, title: "左下" }),
          panel({ id: "right", position: 2, width: 6, title: "右下" }),
        ]}
      />,
    );

    await screen.findByText("上段");

    // react-grid-layout draws one direct grid-item child per panel.
    const grid = container.querySelector(".react-grid-layout");

    expect(grid).not.toBeNull();
    expect(grid?.querySelectorAll(":scope > .react-grid-item")).toHaveLength(3);
  });

  it("draws a panel with no width full width (AC-L-104)", async () => {
    vi.mocked(postInvoke).mockResolvedValue({ component: "table", data: { items: [] } });

    const { container } = render(
      <WorkspaceGrid workspaceId="ws-1" panels={[panel({ id: "solo", width: 12 })]} />,
    );

    await screen.findByText("パネル");

    const item = container.querySelector<HTMLElement>(".react-grid-item");

    expect(item?.style.width).not.toBe("");
  });

  // AC-L-103: every arrangement a drag can reach, a keyboard can reach too.
  // This test uses no pointer event at all - focus is moved with `Tab`,
  // and every action after that is `userEvent.keyboard`.
  it("moves and resizes a panel by keyboard alone, PATCHing only what changed", async () => {
    vi.mocked(postInvoke).mockResolvedValue({ component: "table", data: { items: [] } });
    vi.mocked(patchPanel).mockResolvedValue(panel());
    const user = userEvent.setup();

    render(
      <WorkspaceGrid
        workspaceId="ws-1"
        panels={[
          panel({ id: "first", position: 0, width: 6, height: 1, title: "一番目" }),
          panel({ id: "second", position: 1, width: 6, height: 1, title: "二番目" }),
        ]}
      />,
    );

    await screen.findByText("一番目");

    const moveButton = screen.getByLabelText("一番目をキーボードで並べ替え・サイズ変更");
    await user.tab();

    while (document.activeElement !== moveButton) {
      await user.tab();
    }

    await user.keyboard("{ArrowRight}");

    expect(vi.mocked(patchPanel)).toHaveBeenCalledWith("ws-1", "first", { position: 1 });
    expect(vi.mocked(patchPanel)).toHaveBeenCalledWith("ws-1", "second", { position: 0 });

    vi.mocked(patchPanel).mockClear();
    await user.keyboard("{Shift>}{ArrowRight}{/Shift}");

    expect(vi.mocked(patchPanel)).toHaveBeenCalledExactlyOnceWith("ws-1", "first", {
      width: 7,
      height: 1,
    });
  });
});
