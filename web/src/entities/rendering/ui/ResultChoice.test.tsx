import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { postPlan, type PlanResult } from "@/shared/api/client";

import { ResultChoice } from "./ResultChoice";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

const question = "「破損」に近いステータスはどれですか？";
const options = [
  { value: "allocated", label: "引当済" },
  { value: "staged", label: "出荷準備完了" },
  { value: "quarantined", label: "検品保留" },
  { value: "consigned", label: "預託在庫" },
];

describe("ResultChoice", () => {
  it("shows the question and all four Japanese option labels", () => {
    render(
      <ResultChoice
        question={question}
        param="status"
        options={options}
        originalQuery="破損した在庫はある？"
        onAnswered={() => {}}
      />,
    );

    expect(screen.getByText(question)).toBeTruthy();

    for (const option of options) {
      expect(screen.getByRole("button", { name: option.label })).toBeTruthy();
    }
  });

  it("re-posts the original question with the chosen answer and reports the result", async () => {
    const user = userEvent.setup();
    const onAnswered = vi.fn<(result: PlanResult) => void>();

    vi.mocked(postPlan).mockResolvedValue({
      kind: "result",
      component: "table",
      data: { items: [{ id: "itm-001", status: "quarantined" }] },
      source: {
        service: "inventory",
        operationId: "listInventoryItems",
        args: { status: "quarantined" },
      },
    });

    render(
      <ResultChoice
        question={question}
        param="status"
        options={options}
        originalQuery="破損した在庫はある？"
        onAnswered={onAnswered}
      />,
    );

    await user.click(screen.getByRole("button", { name: "検品保留" }));

    expect(postPlan).toHaveBeenCalledWith({
      query: "破損した在庫はある？",
      answers: [{ param: "status", value: "quarantined" }],
    });

    expect(onAnswered).toHaveBeenCalledWith(
      expect.objectContaining({ kind: "result", component: "table" }),
    );
  });

  it("ignores a second click while the first answer is still in flight", async () => {
    const user = userEvent.setup();
    let resolvePlan: ((result: PlanResult) => void) | undefined;

    // vite.config.ts's `restoreMocks` clears a spy's implementation between
    // tests but not a `vi.fn()`'s own call history, so the previous test's
    // one call would otherwise still be on this mock's tally when this
    // test asserts a call count rather than a call's shape.
    vi.mocked(postPlan).mockClear();
    vi.mocked(postPlan).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvePlan = resolve;
        }),
    );

    render(
      <ResultChoice
        question={question}
        param="status"
        options={options}
        originalQuery="破損した在庫はある？"
        onAnswered={() => {}}
      />,
    );

    await user.click(screen.getByRole("button", { name: "検品保留" }));
    fireEvent.click(screen.getByRole("button", { name: "検品保留" }));
    fireEvent.click(screen.getByRole("button", { name: "引当済" }));

    resolvePlan?.({ kind: "none", message: "結果はありません。" });

    expect(postPlan).toHaveBeenCalledTimes(1);
  });

  it("shows an error when the request fails", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockRejectedValue(new Error("network down"));

    render(
      <ResultChoice
        question={question}
        param="status"
        options={options}
        originalQuery="破損した在庫はある？"
        onAnswered={() => {}}
      />,
    );

    await user.click(screen.getByRole("button", { name: "検品保留" }));

    expect(await screen.findByText(/送信に失敗しました/u)).toBeTruthy();
  });
});
