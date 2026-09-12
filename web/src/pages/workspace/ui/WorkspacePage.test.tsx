import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { addPanel, getWorkspace, postInvoke } from "@/shared/api/client";
import { getCatalog } from "@/shared/api/catalog";

import { ConversationProvider } from "@/features/conversation";

import { WorkspacePage } from "./WorkspacePage";

vi.mock("@/shared/api/client", () => ({
  getWorkspace: vi.fn<typeof getWorkspace>(),
  postInvoke: vi.fn<typeof postInvoke>(),
  addPanel: vi.fn<typeof addPanel>(),
}));

vi.mock("@/shared/api/catalog", () => ({
  getCatalog: vi.fn<typeof getCatalog>(),
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

    render(
      <ConversationProvider>
        <WorkspacePage workspaceId="ws-1" />
      </ConversationProvider>,
    );

    expect(await screen.findByRole("heading", { name: "在庫ボード" })).toBeTruthy();
    expect(screen.getByText("検品保留の在庫")).toBeTruthy();
    await waitFor(() => {
      expect(screen.getByText("itm-001")).toBeTruthy();
    });
    expect(getWorkspace).toHaveBeenCalledWith("ws-1");
  });

  it("reports a workspace that failed to load", async () => {
    vi.mocked(getWorkspace).mockRejectedValue(new Error("boom"));

    render(
      <ConversationProvider>
        <WorkspacePage workspaceId="ws-missing" />
      </ConversationProvider>,
    );

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

    render(
      <ConversationProvider>
        <WorkspacePage workspaceId="ws-1" />
      </ConversationProvider>,
    );

    expect(await screen.findByText("itm-001")).toBeTruthy();
    expect(await screen.findByText(/取得に失敗しました/u)).toBeTruthy();
    expect(screen.getByText("壊れたパネル")).toBeTruthy();
  });

  it("shows a panel added through the control, with no question asked (AC-P-102)", async () => {
    const user = userEvent.setup();

    vi.mocked(getWorkspace).mockResolvedValue({ id: "ws-1", name: "在庫ボード", panels: [] });
    vi.mocked(getCatalog).mockResolvedValue([
      {
        service: "inventory",
        operationId: "ListInventoryItems",
        summary: "在庫一覧",
        component: "table",
        schema: { type: "object", required: [], properties: {} },
        fields: { status: { type: "string" } },
      },
    ]);
    vi.mocked(addPanel).mockResolvedValue({
      id: "pnl-new",
      workspaceId: "ws-1",
      service: "inventory",
      operationId: "ListInventoryItems",
      args: {},
      component: "table",
      title: "在庫一覧",
      position: 0,
    });
    vi.mocked(postInvoke).mockResolvedValue({
      component: "table",
      data: { items: [{ id: "itm-9" }] },
    });

    render(
      <ConversationProvider>
        <WorkspacePage workspaceId="ws-1" />
      </ConversationProvider>,
    );

    await screen.findByRole("heading", { name: "在庫ボード" });
    await user.click(screen.getByRole("button", { name: "パネルを追加" }));
    await user.click(await screen.findByRole("combobox", { name: "操作" }));
    await user.click(await screen.findByText("在庫一覧"));
    await user.click(await screen.findByRole("button", { name: "追加" }));

    expect(await screen.findByRole("button", { name: "パネルを追加" })).toBeTruthy();
    expect(addPanel).toHaveBeenCalledWith("ws-1", {
      service: "inventory",
      operationId: "ListInventoryItems",
      args: {},
      component: "table",
      title: "在庫一覧",
    });
    await waitFor(() => {
      expect(screen.getByText("itm-9")).toBeTruthy();
    });
  });
});
