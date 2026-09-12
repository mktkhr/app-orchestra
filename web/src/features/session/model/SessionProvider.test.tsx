import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { deleteSession, getSession, onUnauthorized, postSession } from "@/shared/api/client";

import { useSession } from "./sessionContext";
import { SessionProvider } from "./SessionProvider";

vi.mock("@/shared/api/client", () => ({
  getSession: vi.fn<typeof getSession>(),
  postSession: vi.fn<typeof postSession>(),
  deleteSession: vi.fn<typeof deleteSession>(),
  onUnauthorized: vi.fn<typeof onUnauthorized>(),
}));

/** Renders `useSession()`'s state as text a test can assert on. */
function Probe(): JSX.Element {
  const session = useSession();

  const handleSignIn = (): void => {
    void session.signIn("admin", "secret");
  };

  const handleSignOut = (): void => {
    void session.signOut();
  };

  return (
    <div>
      <span>status:{session.status}</span>
      <span>user:{session.user === null ? "none" : session.user.name}</span>
      <span>error:{session.signInError ?? "none"}</span>
      <button onClick={handleSignIn}>sign in</button>
      <button onClick={handleSignOut}>sign out</button>
    </div>
  );
}

describe("SessionProvider", () => {
  beforeEach(() => {
    vi.mocked(getSession).mockReset();
    vi.mocked(postSession).mockReset();
    vi.mocked(deleteSession).mockReset();
    vi.mocked(onUnauthorized).mockReset();
    vi.mocked(onUnauthorized).mockReturnValue(() => {});
  });

  it("reads the session once at startup", async () => {
    vi.mocked(getSession).mockResolvedValue({ id: "usr-1", name: "admin", role: "admin" });

    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );

    expect(await screen.findByText("user:admin")).toBeTruthy();
    expect(screen.getByText("status:ready")).toBeTruthy();
    expect(getSession).toHaveBeenCalledTimes(1);
  });

  it("starts with no user when the startup read finds nobody signed in", async () => {
    vi.mocked(getSession).mockResolvedValue(null);

    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );

    expect(await screen.findByText("status:ready")).toBeTruthy();
    expect(screen.getByText("user:none")).toBeTruthy();
  });

  it("signs in and reads the session again", async () => {
    const user = userEvent.setup();

    vi.mocked(getSession).mockResolvedValue(null);
    vi.mocked(postSession).mockResolvedValue({ id: "usr-2", name: "たろう", role: "user" });

    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );

    await screen.findByText("status:ready");
    await user.click(screen.getByRole("button", { name: "sign in" }));

    expect(await screen.findByText("user:たろう")).toBeTruthy();
    expect(postSession).toHaveBeenCalledWith({ name: "admin", password: "secret" });
  });

  it("reports a failed sign-in without setting a user", async () => {
    const user = userEvent.setup();

    vi.mocked(getSession).mockResolvedValue(null);
    vi.mocked(postSession).mockRejectedValue(new Error("POST /api/session failed"));

    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );

    await screen.findByText("status:ready");
    await user.click(screen.getByRole("button", { name: "sign in" }));

    expect(await screen.findByText("error:名前またはパスワードが正しくありません。")).toBeTruthy();
    expect(screen.getByText("user:none")).toBeTruthy();
  });

  it("signs out", async () => {
    const user = userEvent.setup();

    vi.mocked(getSession).mockResolvedValue({ id: "usr-1", name: "admin", role: "admin" });
    vi.mocked(deleteSession).mockResolvedValue();

    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );

    await screen.findByText("user:admin");
    await user.click(screen.getByRole("button", { name: "sign out" }));

    await waitFor(() => {
      expect(screen.getByText("user:none")).toBeTruthy();
    });

    expect(deleteSession).toHaveBeenCalledTimes(1);
  });

  it("drops the user when any call anywhere reports 401", async () => {
    vi.mocked(getSession).mockResolvedValue({ id: "usr-1", name: "admin", role: "admin" });

    let notify: (() => void) | undefined;

    vi.mocked(onUnauthorized).mockImplementation((listener) => {
      notify = listener;

      return () => {};
    });

    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );

    await screen.findByText("user:admin");

    expect(notify).toBeDefined();
    notify?.();

    await waitFor(() => {
      expect(screen.getByText("user:none")).toBeTruthy();
    });
  });
});
