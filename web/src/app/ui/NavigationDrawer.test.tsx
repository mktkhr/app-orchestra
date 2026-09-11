import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { createWorkspace, deleteWorkspace, listWorkspaces } from "@/shared/api/client";

import { NavigationDrawer } from "./NavigationDrawer";

vi.mock("@/shared/api/client", () => ({
  listWorkspaces: vi.fn<typeof listWorkspaces>(),
  createWorkspace: vi.fn<typeof createWorkspace>(),
  deleteWorkspace: vi.fn<typeof deleteWorkspace>(),
}));

const noop = (): void => {
  // Drawer's onClose, unused by these assertions.
};

describe("NavigationDrawer", () => {
  beforeEach(() => {
    // `vi.mock`'s factory mocks are not spies on a real implementation, so
    // `restoreMocks` (web/vite.config.ts) restores nothing for them - clear
    // call history by hand between tests.
    vi.mocked(listWorkspaces).mockReset();
    vi.mocked(createWorkspace).mockReset();
    vi.mocked(deleteWorkspace).mockReset();
  });

  it("lists every workspace the API returns under チャット", async () => {
    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 1 },
      { id: "ws-2", name: "出勤ダッシュボード", panelCount: 0 },
    ]);

    render(<NavigationDrawer open beside onClose={noop} />);

    expect(screen.getByRole("link", { name: "チャット" })).toBeTruthy();
    expect(await screen.findByRole("link", { name: "在庫ボード" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "出勤ダッシュボード" })).toBeTruthy();
  });

  it("creates a workspace and appends it to the list", async () => {
    const user = userEvent.setup();

    vi.mocked(listWorkspaces).mockResolvedValue([]);
    vi.mocked(createWorkspace).mockResolvedValue({ id: "ws-9", name: "新しいボード" });

    render(<NavigationDrawer open beside onClose={noop} />);

    await screen.findByLabelText("新しいワークスペース名");
    await user.type(screen.getByLabelText("新しいワークスペース名"), "新しいボード");
    await user.click(screen.getByRole("button", { name: "ワークスペースを作成" }));

    expect(createWorkspace).toHaveBeenCalledWith({ name: "新しいボード" });
    expect(await screen.findByRole("link", { name: "新しいボード" })).toBeTruthy();
  });

  it("asks for confirmation before deleting a workspace, and removes it once confirmed", async () => {
    const user = userEvent.setup();

    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 1 },
    ]);
    vi.mocked(deleteWorkspace).mockResolvedValue();

    render(<NavigationDrawer open beside onClose={noop} />);

    await screen.findByRole("link", { name: "在庫ボード" });
    await user.click(screen.getByRole("button", { name: "在庫ボードを削除" }));

    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(deleteWorkspace).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "削除する" }));

    expect(deleteWorkspace).toHaveBeenCalledWith("ws-1");
    expect(screen.queryByRole("link", { name: "在庫ボード" })).toBeNull();
  });

  it("does nothing when deletion is cancelled", async () => {
    const user = userEvent.setup();

    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 1 },
    ]);

    render(<NavigationDrawer open beside onClose={noop} />);

    await screen.findByRole("link", { name: "在庫ボード" });
    await user.click(screen.getByRole("button", { name: "在庫ボードを削除" }));
    await screen.findByRole("dialog");
    await user.click(screen.getByRole("button", { name: "キャンセル" }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });

    expect(deleteWorkspace).not.toHaveBeenCalled();
    expect(screen.getByRole("link", { name: "在庫ボード" })).toBeTruthy();
  });
});
