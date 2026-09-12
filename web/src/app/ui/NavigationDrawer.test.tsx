import { render, screen, waitFor, type RenderResult } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { SessionProvider } from "@/features/session";
import {
  createWorkspace,
  deleteWorkspace,
  getSession,
  listWorkspaces,
  type deleteSession,
  type onUnauthorized,
  type postSession,
  type SessionUser,
} from "@/shared/api/client";

import { NavigationDrawer } from "./NavigationDrawer";

vi.mock("@/shared/api/client", () => ({
  listWorkspaces: vi.fn<typeof listWorkspaces>(),
  createWorkspace: vi.fn<typeof createWorkspace>(),
  deleteWorkspace: vi.fn<typeof deleteWorkspace>(),
  getSession: vi.fn<typeof getSession>(),
  postSession: vi.fn<typeof postSession>(),
  deleteSession: vi.fn<typeof deleteSession>(),
  onUnauthorized: vi.fn<typeof onUnauthorized>(() => () => {}),
}));

const noop = (): void => {
  // Drawer's onClose, unused by these assertions.
};

const adminUser: SessionUser = { id: "usr-admin", name: "admin", role: "admin" };
const plainUser: SessionUser = { id: "usr-1", name: "someone", role: "user" };

/** Renders NavigationDrawer under the SessionProvider it now needs to know the signed-in person's role (docs/plans/auth.md Task 5). */
function renderDrawer(
  user: SessionUser,
  element: ReactElement = <NavigationDrawer open beside onClose={noop} />,
): RenderResult {
  vi.mocked(getSession).mockResolvedValue(user);

  return render(<SessionProvider>{element}</SessionProvider>);
}

describe("NavigationDrawer", () => {
  beforeEach(() => {
    // `vi.mock`'s factory mocks are not spies on a real implementation, so
    // `restoreMocks` (web/vite.config.ts) restores nothing for them - clear
    // call history by hand between tests.
    vi.mocked(listWorkspaces).mockReset();
    vi.mocked(createWorkspace).mockReset();
    vi.mocked(deleteWorkspace).mockReset();
    vi.mocked(getSession).mockReset();
  });

  it("lists every workspace the API returns under チャット", async () => {
    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 1 },
      { id: "ws-2", name: "出勤ダッシュボード", panelCount: 0 },
    ]);

    renderDrawer(plainUser);

    expect(screen.getByRole("link", { name: "チャット" })).toBeTruthy();
    expect(await screen.findByRole("link", { name: "在庫ボード" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "出勤ダッシュボード" })).toBeTruthy();
  });

  it("creates a workspace and appends it to the list", async () => {
    const user = userEvent.setup();

    vi.mocked(listWorkspaces).mockResolvedValue([]);
    vi.mocked(createWorkspace).mockResolvedValue({ id: "ws-9", name: "新しいボード" });

    renderDrawer(plainUser);

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

    renderDrawer(plainUser);

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

    renderDrawer(plainUser);

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

  it("shows ユーザー管理 for an admin", async () => {
    vi.mocked(listWorkspaces).mockResolvedValue([]);

    renderDrawer(adminUser);

    expect(await screen.findByRole("link", { name: "ユーザー管理" })).toBeTruthy();
  });

  it("hides ユーザー管理 from a non-admin (docs/specs/auth.md section 7)", async () => {
    vi.mocked(listWorkspaces).mockResolvedValue([]);

    renderDrawer(plainUser);

    await screen.findByRole("link", { name: "チャット" });
    expect(screen.queryByRole("link", { name: "ユーザー管理" })).toBeNull();
  });
});
