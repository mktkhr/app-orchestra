import { render, screen, waitFor, type RenderResult } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";
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

import { MainContent } from "./MainContent";
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

// MainContent's own screens, stood up here only so a click on a drawer link
// can be watched changing what "the screen" means - not to re-test their
// content, which MainContent.test.tsx already does.
vi.mock("@/pages/chat", () => ({
  ChatPage: () => <p>chat screen</p>,
}));

vi.mock("@/pages/workspace", () => ({
  WorkspacePage: ({ workspaceId }: { workspaceId: string }) => (
    <p>workspace screen: {workspaceId}</p>
  ),
}));

const noop = (): void => {
  // Drawer's onClose, unused by these assertions.
};

const adminUser: SessionUser = { id: "usr-admin", name: "admin", role: "admin" };
const plainUser: SessionUser = { id: "usr-1", name: "someone", role: "user" };

/**
 * Renders NavigationDrawer under the SessionProvider it now needs to know
 * the signed-in person's role (docs/plans/auth.md Task 5), and a
 * MemoryRouter: its rows are `react-router` `Link`s (docs/plans/routing.md
 * Task 1), which throw outside a router's context.
 */
function renderDrawer(
  user: SessionUser,
  element: ReactElement = <NavigationDrawer open beside onClose={noop} />,
): RenderResult {
  vi.mocked(getSession).mockResolvedValue(user);

  return render(
    <MemoryRouter>
      <SessionProvider>{element}</SessionProvider>
    </MemoryRouter>,
  );
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

  it("renders チャット and a workspace as paths, not the old #hash addresses (docs/specs/routing.md section 3)", async () => {
    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 1 },
    ]);

    renderDrawer(adminUser);

    expect(screen.getByRole("link", { name: "チャット" }).getAttribute("href")).toBe("/");
    expect((await screen.findByRole("link", { name: "在庫ボード" })).getAttribute("href")).toBe(
      "/workspaces/ws-1",
    );
    expect(screen.getByRole("link", { name: "ユーザー管理" }).getAttribute("href")).toBe("/users");
  });

  it("follows a workspace link to that workspace's screen, without a page load", async () => {
    const user = userEvent.setup();

    vi.mocked(listWorkspaces).mockResolvedValue([
      { id: "ws-1", name: "在庫ボード", panelCount: 1 },
    ]);
    vi.mocked(getSession).mockResolvedValue(plainUser);

    render(
      <MemoryRouter initialEntries={["/"]}>
        <SessionProvider>
          <NavigationDrawer open beside onClose={noop} />
          <MainContent />
        </SessionProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByText("chat screen")).toBeTruthy();

    await user.click(await screen.findByRole("link", { name: "在庫ボード" }));

    // A page load would tear down this render (and every mock in it) and
    // start the application over from `main.tsx`; finding the new screen
    // in the same render, with the drawer still mounted beside it, is what
    // a client-side navigation looks like and a page load does not.
    expect(await screen.findByText("workspace screen: ws-1")).toBeTruthy();
    expect(screen.queryByText("chat screen")).toBeNull();
    expect(screen.getByRole("link", { name: "チャット" })).toBeTruthy();
  });
});
