import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { addPanel, listWorkspaces, postPlan } from "@/shared/api/client";
import { getCatalog } from "@/shared/api/catalog";

import { ConversationProvider } from "@/features/conversation";

import { ConversationPanel } from "./ConversationPanel";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
  listWorkspaces: vi.fn<typeof listWorkspaces>(),
  addPanel: vi.fn<typeof addPanel>(),
}));

vi.mock("@/shared/api/catalog", () => ({
  getCatalog: vi.fn<typeof getCatalog>(),
}));

describe("ConversationPanel", () => {
  beforeEach(() => {
    vi.mocked(getCatalog).mockReset();
    vi.mocked(addPanel).mockReset();
  });

  it("renders the conversation, offering a save control on a table result", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({
      kind: "result",
      component: "table",
      data: { items: [{ id: "itm-001" }] },
      source: {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "listInventoryItems",
      },
    });
    vi.mocked(listWorkspaces).mockResolvedValue([]);

    render(
      <ConversationProvider>
        <ConversationPanel />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "在庫の一覧を見せて" }));

    expect(await screen.findByText("itm-001")).toBeTruthy();
    expect(screen.getByRole("button", { name: "ワークスペースに保存" })).toBeTruthy();
  });

  it("defaults the save control to the given workspace", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({
      kind: "result",
      component: "table",
      data: { items: [{ id: "itm-001" }] },
      source: {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "listInventoryItems",
      },
    });
    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 0 },
    ]);

    render(
      <ConversationProvider>
        <ConversationPanel defaultWorkspaceId="ws-1" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "在庫の一覧を見せて" }));
    await user.click(await screen.findByRole("button", { name: "ワークスペースに保存" }));

    const picker = await screen.findByRole("combobox", { name: "保存先のワークスペース" });

    expect(picker).toHaveProperty("textContent", "在庫ボード");

    // docs/specs/offering.md, O4: the workspace this conversation is asked
    // from must reach postPlan's own request, not only the save control.
    expect(postPlan).toHaveBeenCalledWith({ query: "在庫の一覧を見せて", workspaceId: "ws-1" });
  });

  it("offers a proposal's own form when asked from a workspace, and places it through onPanelPlaced", async () => {
    const user = userEvent.setup();

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

    const onPanelPlaced = vi.fn<(panel: unknown) => void>();

    render(
      <ConversationProvider>
        <ConversationPanel defaultWorkspaceId="ws-1" onPanelPlaced={onPanelPlaced} />
      </ConversationProvider>,
    );

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
    expect(onPanelPlaced).toHaveBeenCalled();
  });

  it("offers no proposal at all on the chat screen (no workspace, AC-N-104)", async () => {
    const user = userEvent.setup();

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

    render(
      <ConversationProvider>
        <ConversationPanel />
      </ConversationProvider>,
    );

    await user.type(screen.getByLabelText("質問を入力"), "在庫をステータス別に棒グラフで置いて");
    await user.click(screen.getByRole("button", { name: "送信" }));

    await waitFor(() => {
      expect(screen.getByLabelText("質問を入力")).toHaveProperty("disabled", false);
    });
    expect(screen.queryByText("ステータス別の在庫")).toBeNull();
    expect(getCatalog).not.toHaveBeenCalled();
  });
});
