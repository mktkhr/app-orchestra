import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vite-plus/test";

import { getWorkspace, postInvoke } from "@/shared/api/client";

import { WorkspacePage } from "./WorkspacePage";

vi.mock("@/shared/api/client", () => ({
  getWorkspace: vi.fn<typeof getWorkspace>(),
  postInvoke: vi.fn<typeof postInvoke>(),
}));

describe("WorkspacePage", () => {
  it("draws every panel a workspace holds, each from its own /api/invoke call", async () => {
    vi.mocked(getWorkspace).mockResolvedValue({
      id: "ws-1",
      name: "在庫ボード",
      panels: [
        {
          id: "pnl-1",
          workspaceId: "ws-1",
          service: "inventory",
          operationId: "ListInventoryItems",
          args: { status: "quarantined" },
          component: "table",
          title: "検品保留の在庫",
          position: 0,
        },
      ],
    });
    vi.mocked(postInvoke).mockResolvedValue({
      component: "table",
      data: { items: [{ id: "itm-001" }] },
    });

    render(<WorkspacePage workspaceId="ws-1" />);

    expect(await screen.findByRole("heading", { name: "在庫ボード" })).toBeTruthy();
    expect(screen.getByText("検品保留の在庫")).toBeTruthy();
    await waitFor(() => {
      expect(screen.getByText("itm-001")).toBeTruthy();
    });
    expect(getWorkspace).toHaveBeenCalledWith("ws-1");
  });

  it("reports a workspace that failed to load", async () => {
    vi.mocked(getWorkspace).mockRejectedValue(new Error("boom"));

    render(<WorkspacePage workspaceId="ws-missing" />);

    expect(await screen.findByText(/取得に失敗しました/u)).toBeTruthy();
  });

  it("still draws a panel that fails, alongside one that succeeds", async () => {
    vi.mocked(getWorkspace).mockResolvedValue({
      id: "ws-1",
      name: "在庫ボード",
      panels: [
        {
          id: "pnl-ok",
          workspaceId: "ws-1",
          service: "inventory",
          operationId: "ListInventoryItems",
          args: {},
          component: "table",
          title: "検品保留の在庫",
          position: 0,
        },
        {
          id: "pnl-bad",
          workspaceId: "ws-1",
          service: "missing",
          operationId: "Nope",
          args: {},
          component: "table",
          title: "壊れたパネル",
          position: 1,
        },
      ],
    });
    vi.mocked(postInvoke)
      .mockResolvedValueOnce({ component: "table", data: { items: [{ id: "itm-001" }] } })
      .mockRejectedValueOnce(new Error("unreachable"));

    render(<WorkspacePage workspaceId="ws-1" />);

    expect(await screen.findByText("itm-001")).toBeTruthy();
    expect(await screen.findByText(/取得に失敗しました/u)).toBeTruthy();
    expect(screen.getByText("壊れたパネル")).toBeTruthy();
  });
});
