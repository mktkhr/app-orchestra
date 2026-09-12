import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import {
  deleteSession,
  getSession,
  type onUnauthorized,
  type postSession,
} from "@/shared/api/client";

import { SessionProvider } from "../model/SessionProvider";
import { SignedInUser } from "./SignedInUser";

vi.mock("@/shared/api/client", () => ({
  getSession: vi.fn<typeof getSession>(),
  postSession: vi.fn<typeof postSession>(),
  deleteSession: vi.fn<typeof deleteSession>(),
  onUnauthorized: vi.fn<typeof onUnauthorized>(() => () => {}),
}));

describe("SignedInUser", () => {
  it("shows the signed-in person's name", async () => {
    vi.mocked(getSession).mockResolvedValue({ id: "usr-1", name: "たろう", role: "user" });

    render(
      <SessionProvider>
        <SignedInUser />
      </SessionProvider>,
    );

    expect(await screen.findByText("たろう")).toBeTruthy();
  });

  it("signs out on click", async () => {
    const user = userEvent.setup();

    vi.mocked(getSession).mockResolvedValue({ id: "usr-1", name: "たろう", role: "user" });
    vi.mocked(deleteSession).mockResolvedValue();

    render(
      <SessionProvider>
        <SignedInUser />
      </SessionProvider>,
    );

    await screen.findByText("たろう");
    await user.click(screen.getByRole("button", { name: "サインアウト" }));

    expect(deleteSession).toHaveBeenCalledTimes(1);
  });

  it("renders nothing while nobody is signed in", () => {
    vi.mocked(getSession).mockResolvedValue(null);

    const { container } = render(
      <SessionProvider>
        <SignedInUser />
      </SessionProvider>,
    );

    // Before and after the startup read resolves, `user` is null either
    // way, so this holds without waiting for it.
    expect(container.textContent).toBe("");
  });
});
