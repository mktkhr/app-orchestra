import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";

import { useConversation } from "./conversationContext";
import { ConversationProvider } from "./conversationStore";
import type { Turn } from "./turn";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

/**
 * Renders one turn's `role`, and - for an `alternatives` turn - its
 * `chosen` field, one line per turn, so a test can assert the exact turn
 * shape the store produced without reaching into the store directly.
 */
function describeTurn(turn: Turn): string {
  return turn.role === "alternatives" ? `alternatives:${turn.chosen ?? "unanswered"}` : turn.role;
}

function Probe(): JSX.Element {
  const conversation = useConversation("chat");

  return (
    <div>
      <span data-testid="roles">{conversation.turns.map(describeTurn).join("|")}</span>
      <button
        onClick={() => {
          void conversation.ask("在庫の一覧を見せて");
        }}
      >
        ask
      </button>
      {conversation.turns
        .filter((turn) => turn.role === "alternatives")
        .flatMap((turn) =>
          turn.alternatives.map((alternative) => (
            <button
              key={alternative.operationId}
              onClick={() => {
                void conversation.ask("在庫の一覧を見せて", {
                  preferred: alternative.operationId,
                  label: alternative.displayName,
                  alternativesTurnId: turn.id,
                });
              }}
            >
              choose-{alternative.operationId}
            </button>
          )),
        )}
    </div>
  );
}

const SOURCE = {
  service: "inventory",
  serviceDisplayName: "在庫管理",
  operationId: "listInventoryItems",
  args: {},
};

/**
 * The store's own turn shape for a `result` carrying `alternatives`
 * (docs/specs/shortlisting.md, section 4, H5). Split out of
 * `conversationStore.preferred.test.tsx`, which already asserts `preferred`
 * and `alternatives` ride onto the right requests and turns, to cover the
 * shape and the `chosen` field the `alternatives` turn's own render (
 * `AlternativesRow`) depends on.
 */
describe("conversationStore, the alternatives turn", () => {
  beforeEach(() => {
    vi.mocked(postPlan).mockReset();
  });

  it("appends question, answer, alternatives for a result that carries alternatives", async () => {
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
        <Probe />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "ask" }));

    await waitFor(() => {
      expect(screen.getByTestId("roles").textContent).toBe(
        "question|answer|alternatives:unanswered",
      );
    });
  });

  it("appends no alternatives turn for a result with none", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValueOnce({
      kind: "result",
      component: "table",
      data: { items: [] },
      source: SOURCE,
    });

    render(
      <ConversationProvider>
        <Probe />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "ask" }));

    await waitFor(() => {
      expect(screen.getByTestId("roles").textContent).toBe("question|answer");
    });
  });

  it("posts preferred, appends a choice turn, and marks the alternatives turn chosen", async () => {
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
        <Probe />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "ask" }));
    await screen.findByRole("button", { name: "choose-op-2" });

    vi.mocked(postPlan).mockResolvedValueOnce({ kind: "none", message: "結果はありません。" });

    await user.click(screen.getByRole("button", { name: "choose-op-2" }));

    await waitFor(() => {
      expect(screen.getByTestId("roles").textContent).toBe(
        "question|answer|alternatives:op-2|choice|answer",
      );
    });

    expect(postPlan).toHaveBeenLastCalledWith(
      expect.objectContaining({ query: "在庫の一覧を見せて", preferred: "op-2" }),
    );
  });
});
