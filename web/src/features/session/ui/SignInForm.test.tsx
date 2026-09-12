import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import {
  type deleteSession,
  getSession,
  type onUnauthorized,
  postSession,
} from "@/shared/api/client";

import { SessionProvider } from "../model/SessionProvider";
import { SignInForm } from "./SignInForm";

vi.mock("@/shared/api/client", () => ({
  getSession: vi.fn<typeof getSession>(),
  postSession: vi.fn<typeof postSession>(),
  deleteSession: vi.fn<typeof deleteSession>(),
  onUnauthorized: vi.fn<typeof onUnauthorized>(() => () => {}),
}));

describe("SignInForm", () => {
  it("submits the name and password typed into it", async () => {
    const user = userEvent.setup();

    vi.mocked(getSession).mockResolvedValue(null);
    vi.mocked(postSession).mockResolvedValue({ id: "usr-1", name: "admin", role: "admin" });

    render(
      <SessionProvider>
        <SignInForm />
      </SessionProvider>,
    );

    await user.type(screen.getByLabelText("名前"), "admin");
    await user.type(screen.getByLabelText("パスワード"), "dev-only-admin-password");
    await user.click(screen.getByRole("button", { name: "サインイン" }));

    expect(postSession).toHaveBeenCalledWith({
      name: "admin",
      password: "dev-only-admin-password",
    });
  });

  it("masks the password field", () => {
    vi.mocked(getSession).mockResolvedValue(null);

    render(
      <SessionProvider>
        <SignInForm />
      </SessionProvider>,
    );

    expect(screen.getByLabelText("パスワード")).toHaveProperty("type", "password");
  });

  it("shows a human-readable error on a failed sign-in, and keeps the button enabled", async () => {
    const user = userEvent.setup();

    vi.mocked(getSession).mockResolvedValue(null);
    vi.mocked(postSession).mockRejectedValue(new Error("POST /api/session failed"));

    render(
      <SessionProvider>
        <SignInForm />
      </SessionProvider>,
    );

    const button = screen.getByRole("button", { name: "サインイン" });

    expect(button).not.toHaveProperty("disabled", true);

    await user.click(button);

    expect(await screen.findByText("名前またはパスワードが正しくありません。")).toBeTruthy();
    expect(button).not.toHaveProperty("disabled", true);
  });

  it("never disables the submit button merely because the fields are empty", () => {
    vi.mocked(getSession).mockResolvedValue(null);

    render(
      <SessionProvider>
        <SignInForm />
      </SessionProvider>,
    );

    expect(screen.getByRole("button", { name: "サインイン" })).not.toHaveProperty("disabled", true);
  });
});
