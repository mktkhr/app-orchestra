import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";

import { ConversationProvider } from "../model/conversationStore";
import { Conversation } from "./Conversation";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

// O4 (docs/specs/offering.md): `workspaceId` reaches postPlan's own
// request body unchanged - split out of Conversation.test.tsx to stay
// under its own max-lines budget.
describe("Conversation workspaceId", () => {
  it("forwards workspaceId to postPlan when given one", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({ kind: "none", message: "結果はありません。" });

    render(
      <ConversationProvider>
        <Conversation conversationKey="ws-1" workspaceId="ws-1" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "在庫の一覧を見せて" }));

    expect(await screen.findByText("結果はありません。")).toBeTruthy();
    expect(postPlan).toHaveBeenCalledWith({ query: "在庫の一覧を見せて", workspaceId: "ws-1" });
  });
});
