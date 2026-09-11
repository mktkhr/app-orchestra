import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vite-plus/test";

import { ChatPage } from "./ChatPage";

describe("ChatPage", () => {
  it("renders the title and the conversation's empty-state examples", () => {
    render(<ChatPage />);

    expect(screen.getByRole("heading", { name: "チャット" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "在庫の一覧を見せて" })).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "在庫を登録して。名前はテスト品、数量は5、引当済で" }),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: "破損した在庫はある？" })).toBeTruthy();
  });
});
