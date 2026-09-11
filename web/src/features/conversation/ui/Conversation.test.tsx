import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";

import { Conversation } from "./Conversation";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

describe("Conversation", () => {
  it("shows the three example questions before the first turn", () => {
    render(<Conversation />);

    expect(screen.getByRole("button", { name: "在庫の一覧を見せて" })).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "在庫を登録して。名前はテスト品、数量は5、引当済で" }),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: "破損した在庫はある？" })).toBeTruthy();
  });

  it("submits an example question and appends the question and the answer as turns", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({
      kind: "none",
      message: "該当する操作が見つかりませんでした。",
    });

    render(<Conversation />);

    await user.click(screen.getByRole("button", { name: "在庫の一覧を見せて" }));

    expect(await screen.findByText("在庫の一覧を見せて")).toBeTruthy();
    expect(await screen.findByText("該当する操作が見つかりませんでした。")).toBeTruthy();
    expect(postPlan).toHaveBeenCalledWith({ query: "在庫の一覧を見せて" });
  });

  it("submits a typed question through the form and disables it while pending", async () => {
    const user = userEvent.setup();
    let resolvePlan: (() => void) | undefined;

    vi.mocked(postPlan).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvePlan = () => {
            resolve({ kind: "none", message: "結果はありません。" });
          };
        }),
    );

    render(<Conversation />);

    await user.type(screen.getByLabelText("質問を入力"), "出勤簿を見せて");
    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(screen.getByLabelText("質問を入力")).toHaveProperty("disabled", true);
    expect(await screen.findByText("出勤簿を見せて")).toBeTruthy();

    resolvePlan?.();

    expect(await screen.findByText("結果はありません。")).toBeTruthy();
    expect(screen.getByLabelText("質問を入力")).toHaveProperty("disabled", false);
  });

  it("shows an error and re-enables the form when the request fails", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockRejectedValue(new Error("network down"));

    render(<Conversation />);

    await user.type(screen.getByLabelText("質問を入力"), "壊れた質問");
    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(await screen.findByText(/質問の送信に失敗しました/u)).toBeTruthy();
    expect(screen.getByLabelText("質問を入力")).toHaveProperty("disabled", false);
  });
});
