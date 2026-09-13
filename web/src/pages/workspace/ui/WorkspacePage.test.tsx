import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { addPanel, getWorkspace, postInvoke, postPlan } from "@/shared/api/client";
import { getCatalog } from "@/shared/api/catalog";

import { ConversationProvider } from "@/features/conversation";

import { WorkspacePage } from "./WorkspacePage";

vi.mock("@/shared/api/client", () => ({
  getWorkspace: vi.fn<typeof getWorkspace>(),
  postInvoke: vi.fn<typeof postInvoke>(),
  addPanel: vi.fn<typeof addPanel>(),
  postPlan: vi.fn<typeof postPlan>(),
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
          width: 12,
          height: 1,
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
          width: 12,
          height: 1,
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
          width: 12,
          height: 1,
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
        serviceDisplayName: "在庫管理",
        operationId: "ListInventoryItems",
        summary: "在庫一覧",
        displayName: "在庫一覧",
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
      width: 12,
      height: 1,
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
    // An option now shows its name and its summary underneath (AC-K-102),
    // so its accessible name is both lines - a `RegExp` matches within
    // that rather than requiring the whole thing.
    await user.click(await screen.findByRole("option", { name: /在庫一覧/u }));
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

  it("places a proposal asked for in the workspace's own chat, and shows it in the grid", async () => {
    const user = userEvent.setup();

    vi.mocked(getWorkspace).mockResolvedValue({ id: "ws-1", name: "在庫ボード", panels: [] });
    vi.mocked(postPlan).mockResolvedValue({
      kind: "proposal",
      panel: {
        service: "inventory",
        operationId: "SummarizeInventory",
        args: {},
        component: "chart",
        title: "ステータス別の在庫",
        view: { chart: { category: "status", value: "count", kind: "bar" } },
      },
    });
    vi.mocked(getCatalog).mockResolvedValue([
      {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "SummarizeInventory",
        summary: "在庫の集計",
        displayName: "在庫の集計",
        component: "table",
        schema: { type: "object", required: [], properties: {} },
        fields: { status: { type: "string" }, count: { type: "number" } },
      },
    ]);
    vi.mocked(addPanel).mockResolvedValue({
      id: "pnl-new",
      workspaceId: "ws-1",
      service: "inventory",
      operationId: "SummarizeInventory",
      args: {},
      component: "chart",
      title: "ステータス別の在庫",
      position: 0,
      width: 12,
      height: 1,
      view: { chart: { category: "status", value: "count", kind: "bar" } },
    });
    vi.mocked(postInvoke).mockResolvedValue({
      component: "chart",
      data: { items: [{ status: "allocated", count: 3 }] },
    });

    render(
      <ConversationProvider>
        <WorkspacePage workspaceId="ws-1" />
      </ConversationProvider>,
    );

    await screen.findByRole("heading", { name: "在庫ボード" });
    await user.type(screen.getByLabelText("質問を入力"), "在庫をステータス別に棒グラフで置いて");
    await user.click(screen.getByRole("button", { name: "送信" }));

    await user.click(await screen.findByRole("button", { name: "配置" }));

    expect(addPanel).toHaveBeenCalledWith("ws-1", {
      service: "inventory",
      operationId: "SummarizeInventory",
      args: {},
      component: "chart",
      title: "ステータス別の在庫",
      view: { chart: { category: "status", value: "count", kind: "bar" } },
    });
    await waitFor(() => {
      expect(screen.getAllByText("ステータス別の在庫").length).toBeGreaterThan(0);
    });
  });
});
