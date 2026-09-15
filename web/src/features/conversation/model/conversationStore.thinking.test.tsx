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

const STORAGE_KEY = "orchestra.conversation.thinking";

/**
 * Renders the 「思考」 switch's current value and buttons to toggle it, ask
 * an ordinary question, and re-plan with `preferred` set - one probe per
 * test, so each `ask` call is unambiguous about which path it drove.
 */
function Probe(): JSX.Element {
  const conversation = useConversation("chat");

  return (
    <div>
      <span data-testid="thinking">{String(conversation.thinking)}</span>
      <span data-testid="turns">
        {conversation.turns
          .map((turn) => (turn.role === "answer" ? JSON.stringify(turn.result) : turn.text))
          .join("|")}
      </span>
      <button
        onClick={() => {
          conversation.setThinking(!conversation.thinking);
        }}
      >
        toggle
      </button>
      <button
        onClick={() => {
          void conversation.ask("在庫の一覧を見せて");
        }}
      >
        ask
      </button>
      <button
        onClick={() => {
          void conversation.ask("在庫の一覧を見せて", { preferred: "op-2", label: "候補2" });
        }}
      >
        ask-preferred
      </button>
    </div>
  );
}

// docs/specs/shortlisting.md's "platform knobs" subproject, decided
// 2026-09-16: the 「思考」 switch's state lives in the store, not any one
// component, is included explicitly on every postPlan call this client
// makes, and is persisted to localStorage under one key.
describe("conversationStore, thinking", () => {
  beforeEach(() => {
    vi.mocked(postPlan).mockReset();
    globalThis.localStorage.clear();
  });

  it("defaults to off when nothing is stored", () => {
    render(
      <ConversationProvider>
        <Probe />
      </ConversationProvider>,
    );

    expect(screen.getByTestId("thinking").textContent).toBe("false");
  });

  it("restores a persisted true value on mount", () => {
    globalThis.localStorage.setItem(STORAGE_KEY, "true");

    render(
      <ConversationProvider>
        <Probe />
      </ConversationProvider>,
    );

    expect(screen.getByTestId("thinking").textContent).toBe("true");
  });

  it("persists a toggle to localStorage", async () => {
    const user = userEvent.setup();

    render(
      <ConversationProvider>
        <Probe />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "toggle" }));

    expect(screen.getByTestId("thinking").textContent).toBe("true");
    expect(globalThis.localStorage.getItem(STORAGE_KEY)).toBe("true");
  });

  it("sends thinking: false explicitly on an ordinary question by default", async () => {
    const user = userEvent.setup();
    vi.mocked(postPlan).mockResolvedValue({ kind: "none", message: "結果はありません。" });

    render(
      <ConversationProvider>
        <Probe />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "ask" }));
    await screen.findByText("結果はありません。", {
      exact: false,
      selector: "[data-testid='turns']",
    });

    expect(postPlan).toHaveBeenCalledWith(
      expect.objectContaining({ query: "在庫の一覧を見せて", thinking: false }),
    );
  });

  it("sends thinking: true once the switch is toggled on, for both an ordinary question and a preferred re-plan", async () => {
    const user = userEvent.setup();
    vi.mocked(postPlan).mockResolvedValue({ kind: "none", message: "結果はありません。" });

    render(
      <ConversationProvider>
        <Probe />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "toggle" }));
    await user.click(screen.getByRole("button", { name: "ask" }));
    await screen.findByText("結果はありません。", {
      exact: false,
      selector: "[data-testid='turns']",
    });

    expect(postPlan).toHaveBeenLastCalledWith(
      expect.objectContaining({ query: "在庫の一覧を見せて", thinking: true }),
    );

    vi.mocked(postPlan).mockResolvedValueOnce({ kind: "none", message: "結果はありません。" });
    await user.click(screen.getByRole("button", { name: "ask-preferred" }));

    await waitFor(() => {
      expect(postPlan).toHaveBeenCalledTimes(2);
    });

    expect(postPlan).toHaveBeenLastCalledWith(
      expect.objectContaining({ query: "在庫の一覧を見せて", preferred: "op-2", thinking: true }),
    );
  });
});
