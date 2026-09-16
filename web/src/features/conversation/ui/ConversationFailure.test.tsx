import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";
import { PlanRequestError } from "@/shared/api/planRequestError";

import { ConversationProvider } from "../model/conversationStore";
import { Conversation } from "./Conversation";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

// Split out of Conversation.test.tsx, which sits at eslint's max-lines.
// Since a service's 4xx answer became a `none` result (2026-09-16), a
// failed POST /api/plan is a platform or service fault, and the platform's
// own message is what the person needs to see.
describe("Conversation, a failed plan request", () => {
  it("shows the platform's own message when the request failed on the server", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockRejectedValue(
      new PlanRequestError("invoking inventory: service unreachable"),
    );

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.type(screen.getByLabelText("質問を入力"), "在庫を見せて");
    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(
      await screen.findByText(
        "質問の送信に失敗しました。時間をおいて試してください。（invoking inventory: service unreachable）",
      ),
    ).toBeTruthy();
  });
});
