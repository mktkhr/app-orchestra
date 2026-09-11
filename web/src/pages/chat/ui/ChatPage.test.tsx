import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vite-plus/test";

import { getHealth } from "@/shared/api/client";

import { ChatPage } from "./ChatPage";

vi.mock("@/shared/api/client", () => ({
  getHealth: vi.fn<typeof getHealth>(),
}));

describe("ChatPage", () => {
  it("shows the backend status once the health check resolves", async () => {
    vi.mocked(getHealth).mockResolvedValue({ status: "ok" });

    render(<ChatPage />);

    expect(await screen.findByText(/バックエンド応答: ok/u)).toBeTruthy();
  });

  it("shows an error when the health check fails", async () => {
    vi.mocked(getHealth).mockRejectedValue(new Error("network down"));

    render(<ChatPage />);

    expect(await screen.findByText(/バックエンドに接続できません/u)).toBeTruthy();
  });
});
