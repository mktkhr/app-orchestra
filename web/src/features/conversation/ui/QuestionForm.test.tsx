import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { QuestionForm } from "./QuestionForm";

// The 「思考」 switch (platform knobs subproject, decided 2026-09-16): this
// file exercises QuestionForm on its own, with `thinking`/`onThinkingChange`
// as plain props - the value itself living in the conversation store is
// conversationStore.thinking.test.tsx's own concern.
describe("QuestionForm", () => {
  it("labels the thinking switch exactly 「思考」", () => {
    render(
      <QuestionForm
        onSubmit={vi.fn<(query: string) => void>()}
        disabled={false}
        thinking={false}
        onThinkingChange={vi.fn<(value: boolean) => void>()}
      />,
    );

    expect(screen.getByRole("switch", { name: "思考" })).toBeTruthy();
  });

  it("shows the switch on when thinking is true, off when false", () => {
    const { rerender } = render(
      <QuestionForm
        onSubmit={vi.fn<(query: string) => void>()}
        disabled={false}
        thinking={false}
        onThinkingChange={vi.fn<(value: boolean) => void>()}
      />,
    );

    expect(screen.getByRole("switch", { name: "思考" })).toHaveProperty("checked", false);

    rerender(
      <QuestionForm
        onSubmit={vi.fn<(query: string) => void>()}
        disabled={false}
        thinking={true}
        onThinkingChange={vi.fn<(value: boolean) => void>()}
      />,
    );

    expect(screen.getByRole("switch", { name: "思考" })).toHaveProperty("checked", true);
  });

  it("calls onThinkingChange with the new value when the switch is toggled", async () => {
    const user = userEvent.setup();
    const onThinkingChange = vi.fn<(value: boolean) => void>();

    render(
      <QuestionForm
        onSubmit={vi.fn<(query: string) => void>()}
        disabled={false}
        thinking={false}
        onThinkingChange={onThinkingChange}
      />,
    );

    await user.click(screen.getByRole("switch", { name: "思考" }));

    expect(onThinkingChange).toHaveBeenCalledWith(true);
  });

  it("still submits the typed question, unaffected by the switch", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn<(query: string) => void>();

    render(
      <QuestionForm
        onSubmit={onSubmit}
        disabled={false}
        thinking={true}
        onThinkingChange={vi.fn<(value: boolean) => void>()}
      />,
    );

    await user.type(screen.getByLabelText("質問を入力"), "在庫の一覧を見せて");
    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(onSubmit).toHaveBeenCalledWith("在庫の一覧を見せて");
  });
});
