import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";

import { useConversation } from "./conversationContext";
import { ConversationProvider } from "./conversationStore";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

/** Renders one conversation's turns as text, and buttons to drive it. */
function Probe({ conversationKey }: { readonly conversationKey: string }): JSX.Element {
  const conversation = useConversation(conversationKey);

  return (
    <div>
      <span data-testid={`turns-${conversationKey}`}>
        {conversation.turns
          .map((turn) => (turn.role === "question" ? turn.text : JSON.stringify(turn.result)))
          .join("|")}
      </span>
      <button
        onClick={() => {
          void conversation.ask("質問");
        }}
      >
        ask-{conversationKey}
      </button>
      <button onClick={conversation.newConversation}>new-{conversationKey}</button>
    </div>
  );
}

describe("conversationStore", () => {
  beforeEach(() => {
    vi.mocked(postPlan).mockReset();
  });

  it("keeps a conversation's turns when the component reading it unmounts and remounts", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({ kind: "none", message: "結果はありません。" });

    const { rerender } = render(
      <ConversationProvider>
        <Probe conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "ask-chat" }));
    expect(await screen.findByText("結果はありません。", { exact: false })).toBeTruthy();

    // Simulate navigating away: unmount the reader, then mount it again.
    rerender(
      <ConversationProvider>
        <div>elsewhere</div>
      </ConversationProvider>,
    );
    rerender(
      <ConversationProvider>
        <Probe conversationKey="chat" />
      </ConversationProvider>,
    );

    expect(screen.getByTestId("turns-chat").textContent).toContain("質問");
  });

  it("keeps two conversations separate by key", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({ kind: "none", message: "結果はありません。" });

    render(
      <ConversationProvider>
        <Probe conversationKey="chat" />
        <Probe conversationKey="ws-1" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "ask-chat" }));
    await screen.findByText("質問", { exact: false, selector: "[data-testid='turns-chat']" });

    expect(screen.getByTestId("turns-chat").textContent).toContain("質問");
    expect(screen.getByTestId("turns-ws-1").textContent).toBe("");
  });

  it("empties only the conversation whose control was pressed", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({ kind: "none", message: "結果はありません。" });

    render(
      <ConversationProvider>
        <Probe conversationKey="chat" />
        <Probe conversationKey="ws-1" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "ask-chat" }));
    await user.click(screen.getByRole("button", { name: "ask-ws-1" }));

    expect(screen.getByTestId("turns-chat").textContent).not.toBe("");
    expect(screen.getByTestId("turns-ws-1").textContent).not.toBe("");

    await user.click(screen.getByRole("button", { name: "new-chat" }));

    expect(screen.getByTestId("turns-chat").textContent).toBe("");
    expect(screen.getByTestId("turns-ws-1").textContent).not.toBe("");
  });

  it("throws when used outside a ConversationProvider", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() => render(<Probe conversationKey="chat" />)).toThrow(
      "useConversation must be used inside a ConversationProvider",
    );

    spy.mockRestore();
  });
});
