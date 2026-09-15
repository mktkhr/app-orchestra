import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";

import { ConversationProvider } from "../model/conversationStore";
import { Conversation } from "./Conversation";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

const QUESTION = "在庫の一覧を見せて";

const SOURCE = {
  service: "inventory",
  serviceDisplayName: "在庫管理",
  operationId: "listInventoryItems",
  args: {},
};

/**
 * 「違いましたか？」 - a `result`'s further shortlist candidates, offered
 * as chips (docs/specs/shortlisting.md, section 4, H5). Split out of
 * `Conversation.test.tsx`, the same way `ConversationChart.test.tsx` is,
 * to stay under its `max-lines` budget.
 */
describe("Conversation, a result with alternatives", () => {
  it("draws the alternatives line and a chip per candidate, and re-asks with the chosen one", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValueOnce({
      kind: "result",
      component: "table",
      data: { items: [] },
      source: SOURCE,
      alternatives: [
        { operationId: "op-2", displayName: "候補2", service: "inventory" },
        { operationId: "op-3", displayName: "候補3", service: "inventory" },
      ],
    });

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: QUESTION }));

    expect(await screen.findByText("違いましたか？")).toBeTruthy();
    expect(screen.getByText("候補2")).toBeTruthy();
    expect(screen.getByText("候補3")).toBeTruthy();

    vi.mocked(postPlan).mockResolvedValueOnce({ kind: "none", message: "結果はありません。" });

    const chosenChip = screen.getByRole("button", { name: "候補3" });

    await user.click(chosenChip);

    expect(await screen.findByText("結果はありません。")).toBeTruthy();
    expect(postPlan).toHaveBeenCalledTimes(2);
    expect(postPlan).toHaveBeenLastCalledWith(
      expect.objectContaining({ query: QUESTION, preferred: "op-3" }),
    );

    // The chip click is a choice, not a second question - the transcript
    // shows "→ 候補3", not the question text a second time
    // (docs/specs/shortlisting.md, section 4, H5).
    expect(await screen.findByText("→ 候補3")).toBeTruthy();
    expect(screen.getAllByText(QUESTION)).toHaveLength(1);

    // The user's own words: the chosen chip stays clearly visible, the
    // other becomes disabled - not both stacked as two live operations
    // under one question.
    expect(screen.queryByRole("button", { name: "候補3" })).toBeNull();
    expect(screen.queryByRole("button", { name: "候補2" })).toBeNull();
    expect(screen.getByText("候補3").closest(".MuiChip-colorPrimary")).toBeTruthy();
    expect(screen.getByText("候補2").closest(".Mui-disabled")).toBeTruthy();

    // A click on either chip now that the turn is answered posts nothing
    // further - the second click the user described as stacking a second
    // operation under the same question is no longer possible. `fireEvent`,
    // not `user.click`: both chips declare `pointer-events: none` once the
    // turn is answered (the disabled one through MUI's own `Mui-disabled`
    // class, the chosen one through having no `onClick` at all), and
    // `user.click` refuses to dispatch through that - which is itself the
    // proof neither chip is reachable by a real click any more.
    fireEvent.click(screen.getByText("候補3"));
    fireEvent.click(screen.getByText("候補2"));

    expect(postPlan).toHaveBeenCalledTimes(2);
  });

  it("activates a chip on Enter, the same as a click", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValueOnce({
      kind: "result",
      component: "table",
      data: { items: [] },
      source: SOURCE,
      alternatives: [{ operationId: "op-2", displayName: "候補2", service: "inventory" }],
    });

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: QUESTION }));
    const label = await screen.findByText("候補2");
    const chip = label.closest<HTMLElement>('[role="button"]');

    expect(chip).toBeTruthy();

    vi.mocked(postPlan).mockResolvedValueOnce({ kind: "none", message: "結果はありません。" });

    chip?.focus();
    await user.keyboard("{Enter}");

    expect(await screen.findByText("結果はありません。")).toBeTruthy();
    expect(postPlan).toHaveBeenLastCalledWith(
      expect.objectContaining({ query: QUESTION, preferred: "op-2" }),
    );
  });

  it("draws no alternatives line when the result has none", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValueOnce({
      kind: "result",
      component: "table",
      data: { items: [] },
      source: SOURCE,
    });

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: QUESTION }));

    expect(await screen.findByText("在庫管理 / listInventoryItems")).toBeTruthy();
    expect(screen.queryByText("違いましたか？")).toBeNull();
  });
});
