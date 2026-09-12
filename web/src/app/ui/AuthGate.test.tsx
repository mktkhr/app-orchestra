import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import {
  deleteSession,
  getSession,
  listWorkspaces,
  type onUnauthorized,
  postSession,
} from "@/shared/api/client";
import { SessionProvider } from "@/features/session";

import { AuthGate } from "./AuthGate";

vi.mock("@/shared/api/client", () => ({
  getSession: vi.fn<typeof getSession>(),
  postSession: vi.fn<typeof postSession>(),
  deleteSession: vi.fn<typeof deleteSession>(),
  onUnauthorized: vi.fn<typeof onUnauthorized>(() => () => {}),
  listWorkspaces: vi.fn<typeof listWorkspaces>(),
}));

/**
 * docs/plans/auth.md Task 4, Step 1: "with a scripted API answering 401,
 * the shell shows the sign-in screen; signing in shows the chat."
 */
describe("AuthGate", () => {
  it("shows the sign-in screen, and nothing of the chat, when the API answers 401", async () => {
    vi.mocked(getSession).mockResolvedValue(null);

    render(
      <SessionProvider>
        <AuthGate />
      </SessionProvider>,
    );

    expect(await screen.findByRole("heading", { name: "サインイン" })).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "チャット" })).toBeNull();
    expect(screen.queryByRole("button", { name: "サインアウト" })).toBeNull();
  });

  it("shows the chat once signing in succeeds", async () => {
    const user = userEvent.setup();

    vi.mocked(getSession).mockResolvedValue(null);
    vi.mocked(postSession).mockResolvedValue({ id: "usr-1", name: "admin", role: "admin" });
    vi.mocked(listWorkspaces).mockResolvedValue([]);

    render(
      <SessionProvider>
        <AuthGate />
      </SessionProvider>,
    );

    await screen.findByRole("heading", { name: "サインイン" });
    await user.type(screen.getByLabelText("名前"), "admin");
    await user.type(screen.getByLabelText("パスワード"), "dev-only-admin-password");
    await user.click(screen.getByRole("button", { name: "サインイン" }));

    expect(await screen.findByRole("heading", { name: "チャット" })).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "サインイン" })).toBeNull();
  });

  it("shows the signed-in person's name in the bar, once signed in", async () => {
    vi.mocked(getSession).mockResolvedValue({ id: "usr-1", name: "admin", role: "admin" });
    vi.mocked(listWorkspaces).mockResolvedValue([]);

    render(
      <SessionProvider>
        <AuthGate />
      </SessionProvider>,
    );

    expect(await screen.findByText("admin")).toBeTruthy();
    expect(screen.getByRole("button", { name: "サインアウト" })).toBeTruthy();
  });

  it("returns to the sign-in screen on sign-out", async () => {
    const user = userEvent.setup();

    vi.mocked(getSession).mockResolvedValue({ id: "usr-1", name: "admin", role: "admin" });
    vi.mocked(deleteSession).mockResolvedValue();
    vi.mocked(listWorkspaces).mockResolvedValue([]);

    render(
      <SessionProvider>
        <AuthGate />
      </SessionProvider>,
    );

    await screen.findByRole("heading", { name: "チャット" });
    await user.click(screen.getByRole("button", { name: "サインアウト" }));

    expect(await screen.findByRole("heading", { name: "サインイン" })).toBeTruthy();
  });
});
