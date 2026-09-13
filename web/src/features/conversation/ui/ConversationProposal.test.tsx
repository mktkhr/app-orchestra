import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";

import { ConversationProvider } from "../model/conversationStore";
import { Conversation } from "./Conversation";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

const proposalResult = {
  kind: "proposal" as const,
  panel: {
    service: "inventory",
    operationId: "SummarizeInventory",
    args: {},
    component: "chart" as const,
    title: "ステータス別の在庫",
    view: { chart: { category: "status", value: "count", kind: "bar" as const } },
  },
};

/**
 * A `proposal` answer turn (`docs/specs/proposing.md` section 5,
 * `docs/plans/proposing.md` Task 1) - drawn through the `renderProposal`
 * slot `Conversation` composes with `ProposalControl`
 * (`widgets/conversation`'s own job), or not drawn at all when no slot is
 * given (N4, AC-N-104). Split out of `Conversation.test.tsx`, which is
 * already at its `max-lines` budget - the same reasoning
 * `ConversationChart.test.tsx` gives for its own split.
 */
describe("Conversation, a proposal", () => {
  it("draws a proposal turn through renderProposal, handing it the panel the platform filled in", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue(proposalResult);

    render(
      <ConversationProvider>
        <Conversation
          conversationKey="ws-1"
          renderProposal={({ panel }) => <div>提案: {panel.title}</div>}
        />
      </ConversationProvider>,
    );

    await user.type(screen.getByLabelText("質問を入力"), "在庫をステータス別に棒グラフで置いて");
    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(await screen.findByText("提案: ステータス別の在庫")).toBeTruthy();
  });

  it("draws no proposal at all with no renderProposal given, and does not throw (AC-N-104)", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue(proposalResult);

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.type(screen.getByLabelText("質問を入力"), "在庫をステータス別に棒グラフで置いて");
    await user.click(screen.getByRole("button", { name: "送信" }));

    await waitFor(() => {
      expect(screen.getByLabelText("質問を入力")).toHaveProperty("disabled", false);
    });
    expect(screen.queryByText("ステータス別の在庫")).toBeNull();
    expect(screen.queryByRole("button", { name: "配置" })).toBeNull();
  });
});
