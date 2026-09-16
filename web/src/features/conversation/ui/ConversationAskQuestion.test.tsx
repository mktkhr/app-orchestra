import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";

import { ConversationProvider } from "../model/conversationStore";
import { Conversation } from "./Conversation";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

// A safe operation's parameter with no enum comes back as `kind: "ask"`
// with only `question` - no `param`, no `options` - a plain question the
// person answers by typing their next message rather than picking a chip.
describe("Conversation, a question-only ask", () => {
  it("renders as a plain bubble, with no button to answer it, and carries it as a turn once answered", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValueOnce({
      kind: "ask",
      question: "新しい名前は何ですか？",
    });

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.type(screen.getByLabelText("質問を入力"), "在庫の名前を変更して");
    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(await screen.findByText("新しい名前は何ですか？")).toBeTruthy();
    expect(screen.getByText("質問")).toBeTruthy();
    expect(screen.queryByText("ask")).toBeNull();

    vi.mocked(postPlan).mockResolvedValueOnce({ kind: "none", message: "更新しました。" });

    await user.type(screen.getByLabelText("質問を入力"), "テスト品です");
    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(await screen.findByText("更新しました。")).toBeTruthy();
    expect(postPlan).toHaveBeenLastCalledWith({
      query: "テスト品です",
      turns: [{ question: "在庫の名前を変更して", kind: "ask" }],
      thinking: false,
    });
  });
});
