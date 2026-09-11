import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { listWorkspaces, postPlan } from "@/shared/api/client";

import { ConversationPanel } from "./ConversationPanel";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
  listWorkspaces: vi.fn<typeof listWorkspaces>(),
}));

describe("ConversationPanel", () => {
  it("renders the conversation, offering a save control on a table result", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({
      kind: "result",
      component: "table",
      data: { items: [{ id: "itm-001" }] },
      source: { service: "inventory", operationId: "listInventoryItems" },
    });
    vi.mocked(listWorkspaces).mockResolvedValue([]);

    render(<ConversationPanel />);

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
      source: { service: "inventory", operationId: "listInventoryItems" },
    });
    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 0 },
    ]);

    render(<ConversationPanel defaultWorkspaceId="ws-1" />);

    await user.click(screen.getByRole("button", { name: "在庫の一覧を見せて" }));
    await user.click(await screen.findByRole("button", { name: "ワークスペースに保存" }));

    const picker = await screen.findByRole("combobox", { name: "保存先のワークスペース" });

    expect(picker).toHaveProperty("textContent", "在庫ボード");
  });
});
