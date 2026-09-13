import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { addPanel } from "@/shared/api/client";
import { getCatalog, type CatalogEntry } from "@/shared/api/catalog";
import type { ProposedPanel } from "@/shared/api/panels";

import { ProposalControl } from "./ProposalControl";

vi.mock("@/shared/api/client", () => ({
  addPanel: vi.fn<typeof addPanel>(),
}));

vi.mock("@/shared/api/catalog", () => ({
  getCatalog: vi.fn<typeof getCatalog>(),
}));

const chartEntry: CatalogEntry = {
  service: "inventory",
  serviceDisplayName: "在庫管理",
  operationId: "SummarizeInventory",
  summary: "在庫の集計",
  displayName: "在庫の集計",
  component: "table",
  schema: { type: "object", required: [], properties: {} },
  fields: { status: { type: "string" }, count: { type: "number" } },
};

const proposedPanel: ProposedPanel = {
  service: "inventory",
  operationId: "SummarizeInventory",
  args: {},
  component: "chart",
  title: "ステータス別の在庫",
  view: { chart: { category: "status", value: "count", kind: "bar" } },
};

describe("ProposalControl", () => {
  beforeEach(() => {
    vi.mocked(getCatalog).mockReset();
    vi.mocked(addPanel).mockReset();
  });

  it("draws the builder's own form, filled in from the proposal, with its operation stated rather than offered", async () => {
    vi.mocked(getCatalog).mockResolvedValue([chartEntry]);

    render(<ProposalControl workspaceId="ws-1" panel={proposedPanel} onPlaced={() => {}} />);

    expect(await screen.findByText(/在庫の集計/u)).toBeTruthy();
    expect(screen.queryByRole("combobox", { name: "操作" })).toBeNull();
    expect(await screen.findByRole("textbox", { name: "パネル名" })).toHaveProperty(
      "value",
      "ステータス別の在庫",
    );
    expect(screen.getByRole("combobox", { name: "分類の軸" })).toHaveProperty(
      "textContent",
      "status",
    );
    expect(screen.getByRole("button", { name: "配置" })).toBeTruthy();
  });

  it("places the proposal as it was filled in, and shows a confirmation once placed", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([chartEntry]);
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

    const onPlaced = vi.fn<(panel: unknown) => void>();

    render(<ProposalControl workspaceId="ws-1" panel={proposedPanel} onPlaced={onPlaced} />);

    await user.click(await screen.findByRole("button", { name: "配置" }));

    expect(addPanel).toHaveBeenCalledWith("ws-1", {
      service: "inventory",
      operationId: "SummarizeInventory",
      args: {},
      component: "chart",
      title: "ステータス別の在庫",
      view: { chart: { category: "status", value: "count", kind: "bar" } },
    });
    expect(onPlaced).toHaveBeenCalled();
    expect(await screen.findByText("「ステータス別の在庫」を配置しました。")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "配置" })).toBeNull();
  });

  it("places the edited value when a field is changed before pressing 配置 (AC-N-103)", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([chartEntry]);
    vi.mocked(addPanel).mockResolvedValue({
      id: "pnl-new",
      workspaceId: "ws-1",
      service: "inventory",
      operationId: "SummarizeInventory",
      args: {},
      component: "chart",
      title: "在庫の状況",
      position: 0,
      width: 12,
      height: 1,
      view: { chart: { category: "status", value: "count", kind: "bar" } },
    });

    render(<ProposalControl workspaceId="ws-1" panel={proposedPanel} onPlaced={() => {}} />);

    const titleField = await screen.findByRole("textbox", { name: "パネル名" });

    await user.clear(titleField);
    await user.type(titleField, "在庫の状況");
    await user.click(await screen.findByRole("button", { name: "配置" }));

    expect(addPanel).toHaveBeenCalledWith("ws-1", expect.objectContaining({ title: "在庫の状況" }));
  });

  it("shows the catalogue's own load error rather than the form", async () => {
    vi.mocked(getCatalog).mockRejectedValue(new Error("boom"));

    render(<ProposalControl workspaceId="ws-1" panel={proposedPanel} onPlaced={() => {}} />);

    expect(await screen.findByText("カタログの取得に失敗しました。")).toBeTruthy();
  });
});
