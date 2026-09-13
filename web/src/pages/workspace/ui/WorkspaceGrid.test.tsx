import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vite-plus/test";

import { postInvoke, type WorkspacePanel } from "@/shared/api/client";

import { WorkspaceGrid } from "./WorkspaceGrid";

vi.mock("@/shared/api/client", () => ({
  postInvoke: vi.fn<typeof postInvoke>(),
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
});
