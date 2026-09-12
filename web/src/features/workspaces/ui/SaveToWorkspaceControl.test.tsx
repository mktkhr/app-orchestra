import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { addPanel, createWorkspace, listWorkspaces } from "@/shared/api/client";

import { SaveToWorkspaceControl } from "./SaveToWorkspaceControl";

vi.mock("@/shared/api/client", () => ({
  listWorkspaces: vi.fn<typeof listWorkspaces>(),
  createWorkspace: vi.fn<typeof createWorkspace>(),
  addPanel: vi.fn<typeof addPanel>(),
}));

const source = {
  service: "inventory",
  operationId: "listInventoryItems",
  args: { status: "allocated" },
};

describe("SaveToWorkspaceControl", () => {
  it("offers the control on a result, and posts the result's own provenance as a panel", async () => {
    const user = userEvent.setup();

    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 0 },
    ]);
    vi.mocked(addPanel).mockResolvedValue({
      id: "panel-1",
      workspaceId: "ws-1",
      service: source.service,
      operationId: source.operationId,
      args: source.args,
      component: "table",
      title: "在庫の一覧を見せて",
      position: 0,
    });

    render(
      <SaveToWorkspaceControl
        source={source}
        component="table"
        defaultTitle="在庫の一覧を見せて"
      />,
    );

    await user.click(screen.getByRole("button", { name: "ワークスペースに保存" }));

    expect(await screen.findByLabelText("タイトル")).toHaveProperty("value", "在庫の一覧を見せて");
    expect(await screen.findByRole("combobox", { name: "保存先のワークスペース" })).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(await screen.findByText("「在庫ボード」に保存しました")).toBeTruthy();
    expect(addPanel).toHaveBeenCalledWith("ws-1", {
      service: "inventory",
      operationId: "listInventoryItems",
      args: { status: "allocated" },
      component: "table",
      title: "在庫の一覧を見せて",
    });
  });

  it("offers to make a workspace on the spot when none exist yet", async () => {
    const user = userEvent.setup();

    vi.mocked(listWorkspaces).mockResolvedValue([]);
    vi.mocked(createWorkspace).mockResolvedValue({ id: "ws-2", name: "新規ワークスペース" });
    vi.mocked(addPanel).mockResolvedValue({
      id: "panel-2",
      workspaceId: "ws-2",
      service: source.service,
      operationId: source.operationId,
      args: source.args,
      component: "table",
      title: "在庫の一覧を見せて",
      position: 0,
    });

    render(
      <SaveToWorkspaceControl
        source={source}
        component="table"
        defaultTitle="在庫の一覧を見せて"
      />,
    );

    await user.click(screen.getByRole("button", { name: "ワークスペースに保存" }));
    expect(await screen.findByLabelText("新しいワークスペース名")).toBeTruthy();

    await user.type(screen.getByLabelText("新しいワークスペース名"), "新規ワークスペース");
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(await screen.findByText("「新規ワークスペース」に保存しました")).toBeTruthy();
    expect(createWorkspace).toHaveBeenCalledWith({ name: "新規ワークスペース" });
    expect(addPanel).toHaveBeenCalledWith("ws-2", {
      service: "inventory",
      operationId: "listInventoryItems",
      args: { status: "allocated" },
      component: "table",
      title: "在庫の一覧を見せて",
    });
  });

  it("shows an error and stays open when saving fails", async () => {
    const user = userEvent.setup();

    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 0 },
    ]);
    vi.mocked(addPanel).mockRejectedValue(new Error("network down"));

    render(
      <SaveToWorkspaceControl
        source={source}
        component="table"
        defaultTitle="在庫の一覧を見せて"
      />,
    );

    await user.click(screen.getByRole("button", { name: "ワークスペースに保存" }));
    await screen.findByRole("combobox", { name: "保存先のワークスペース" });
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(await screen.findByText(/送信に失敗しました/u)).toBeTruthy();
    expect(screen.queryByText(/に保存しました/u)).toBeNull();
  });

  it("carries the result's own view onto the saved panel (AC-P-105)", async () => {
    const user = userEvent.setup();
    const view = { chart: { category: "status", value: "count", kind: "bar" as const } };

    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 0 },
    ]);
    vi.mocked(addPanel).mockResolvedValue({
      id: "panel-1",
      workspaceId: "ws-1",
      service: source.service,
      operationId: source.operationId,
      args: source.args,
      component: "chart",
      title: "ステータス別の件数",
      position: 0,
      view,
    });

    render(
      <SaveToWorkspaceControl
        source={source}
        component="chart"
        view={view}
        defaultTitle="ステータス別の件数"
      />,
    );

    await user.click(screen.getByRole("button", { name: "ワークスペースに保存" }));
    await screen.findByRole("combobox", { name: "保存先のワークスペース" });
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(await screen.findByText("「在庫ボード」に保存しました")).toBeTruthy();
    expect(addPanel).toHaveBeenCalledWith("ws-1", {
      service: "inventory",
      operationId: "listInventoryItems",
      args: { status: "allocated" },
      component: "chart",
      title: "ステータス別の件数",
      view,
    });
  });

  it("defaults the picker to the given workspace instead of the first one loaded", async () => {
    const user = userEvent.setup();

    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 0 },
      { id: "ws-2", name: "出勤ダッシュボード", panelCount: 0 },
    ]);

    render(
      <SaveToWorkspaceControl
        source={source}
        component="table"
        defaultTitle="在庫の一覧を見せて"
        defaultWorkspaceId="ws-2"
      />,
    );

    await user.click(screen.getByRole("button", { name: "ワークスペースに保存" }));

    const picker = await screen.findByRole("combobox", { name: "保存先のワークスペース" });

    expect(picker).toHaveProperty("textContent", "出勤ダッシュボード");
  });
});
