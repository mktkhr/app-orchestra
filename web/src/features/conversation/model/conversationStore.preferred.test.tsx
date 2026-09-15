import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";

import { useConversation } from "./conversationContext";
import { ConversationProvider } from "./conversationStore";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

/** Renders one conversation's turns as text, and a button to ask it. */
function Probe({ conversationKey }: { readonly conversationKey: string }): JSX.Element {
  const conversation = useConversation(conversationKey);

  return (
    <div>
      <span data-testid={`turns-${conversationKey}`}>
        {conversation.turns
          .map((turn) => (turn.role === "answer" ? JSON.stringify(turn.result) : turn.text))
          .join("|")}
      </span>
      <button
        onClick={() => {
          void conversation.ask("質問");
        }}
      >
        ask-{conversationKey}
      </button>
    </div>
  );
}

/**
 * Asks "chat" with `preferred: "op-2"` and `label: "候補2"` on click -
 * `PreferredProbe`'s own tests below. Renders the turns as `role:text` so a
 * test can tell a `choice` turn (label) apart from a `question` turn (text)
 * without reaching into the store directly.
 */
function PreferredProbe(): JSX.Element {
  const conversation = useConversation("chat");

  return (
    <div>
      <span data-testid="turns-chat">
        {conversation.turns
          .map((turn) =>
            turn.role === "answer" ? JSON.stringify(turn.result) : `${turn.role}:${turn.text}`,
          )
          .join("|")}
      </span>
      <button
        onClick={() => {
          void conversation.ask("質問", { preferred: "op-2", label: "候補2" });
        }}
      >
        ask-preferred
      </button>
    </div>
  );
}

// docs/specs/shortlisting.md, section 4: `preferred` follows `workspaceId`'s
// own path onto `postPlan`'s body, and a result's `alternatives` ride along
// on its turn unchanged. Split out of `conversationStore.test.tsx`, which is
// already at its `max-lines` budget (`harness/quality/eslint`).
describe("conversationStore, preferred and alternatives", () => {
  beforeEach(() => {
    vi.mocked(postPlan).mockReset();
  });

  it("posts preferred when ask is given one", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({ kind: "none", message: "結果はありません。" });

    render(
      <ConversationProvider>
        <PreferredProbe />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "ask-preferred" }));
    await waitFor(() => {
      expect(postPlan).toHaveBeenCalled();
    });

    expect(postPlan).toHaveBeenCalledWith({ query: "質問", preferred: "op-2", thinking: false });
  });

  it("appends a choice turn, not a second question turn, when ask is given preferred and label", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({ kind: "none", message: "結果はありません。" });

    render(
      <ConversationProvider>
        <PreferredProbe />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "ask-preferred" }));
    await waitFor(() => {
      expect(postPlan).toHaveBeenCalled();
    });

    expect(screen.getByTestId("turns-chat").textContent).toBe(
      'choice:質問|{"kind":"none","message":"結果はありません。"}',
    );
  });

  it("keeps alternatives on a result turn, and leaves them off a turn whose response has none", async () => {
    const user = userEvent.setup();
    const source = {
      service: "inventory",
      serviceDisplayName: "在庫管理",
      operationId: "listInventoryItems",
      args: {},
    };

    vi.mocked(postPlan).mockResolvedValueOnce({
      kind: "result",
      component: "table",
      data: { items: [] },
      source,
      alternatives: [{ operationId: "op-2", displayName: "候補2", service: "inventory" }],
    });
    vi.mocked(postPlan).mockResolvedValueOnce({
      kind: "result",
      component: "table",
      data: { items: [] },
      source,
    });

    render(
      <ConversationProvider>
        <Probe conversationKey="chat" />
        <Probe conversationKey="chat-2" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "ask-chat" }));
    await screen.findByText("op-2", {
      exact: false,
      selector: "[data-testid='turns-chat']",
    });

    await user.click(screen.getByRole("button", { name: "ask-chat-2" }));
    await screen.findByText("listInventoryItems", {
      exact: false,
      selector: "[data-testid='turns-chat-2']",
    });

    expect(screen.getByTestId("turns-chat").textContent).toContain("op-2");
    expect(screen.getByTestId("turns-chat-2").textContent).not.toContain("alternatives");
  });
});
