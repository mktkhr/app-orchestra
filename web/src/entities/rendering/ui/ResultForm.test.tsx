import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { postInvoke, type PlanResult } from "@/shared/api/client";

import { ResultForm } from "./ResultForm";

vi.mock("@/shared/api/client", () => ({
  postInvoke: vi.fn<typeof postInvoke>(),
}));

const schema = {
  type: "object",
  required: ["name", "status", "quantity"],
  properties: {
    name: { type: "string", title: "名前" },
    quantity: { type: "integer", title: "数量" },
    status: {
      type: "string",
      enum: ["allocated", "staged", "quarantined", "consigned"],
      enumLabels: {
        allocated: "引当済",
        staged: "出荷準備完了",
        quarantined: "検品保留",
        consigned: "預託在庫",
      },
      title: "ステータス",
    },
  },
};

const target = { service: "inventory", operationId: "CreateInventoryItem" };
const initial = { name: "テスト品", quantity: 5, status: "allocated" };

describe("ResultForm", () => {
  it("shows the status select with Japanese labels, 引当済 already selected", () => {
    render(<ResultForm schema={schema} initial={initial} target={target} onSubmitted={() => {}} />);

    const select = screen.getByRole("combobox", { name: /ステータス/u });

    expect(select.textContent).toBe("引当済");
    expect(screen.getByLabelText(/名前/u)).toHaveProperty("value", "テスト品");
    expect(screen.getByLabelText(/数量/u)).toHaveProperty("value", "5");
  });

  it("submits the edited values to /api/invoke and reports a detail result", async () => {
    const user = userEvent.setup();
    const onSubmitted = vi.fn<(result: PlanResult) => void>();

    vi.mocked(postInvoke).mockResolvedValue({
      component: "detail",
      data: { name: "編集後の品名", quantity: 7, status: "allocated" },
    });

    render(
      <ResultForm schema={schema} initial={initial} target={target} onSubmitted={onSubmitted} />,
    );

    const nameField = screen.getByLabelText(/名前/u);

    await user.clear(nameField);
    await user.type(nameField, "編集後の品名");
    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(postInvoke).toHaveBeenCalledWith({
      service: "inventory",
      operationId: "CreateInventoryItem",
      args: { name: "編集後の品名", quantity: 5, status: "allocated" },
    });

    expect(onSubmitted).toHaveBeenCalledWith({
      kind: "result",
      component: "detail",
      data: { name: "編集後の品名", quantity: 7, status: "allocated" },
      source: {
        service: "inventory",
        operationId: "CreateInventoryItem",
        args: { name: "編集後の品名", quantity: 5, status: "allocated" },
      },
    });
  });

  it("shows an error and re-enables the button when the invoke call fails", async () => {
    const user = userEvent.setup();

    vi.mocked(postInvoke).mockRejectedValue(new Error("network down"));

    render(<ResultForm schema={schema} initial={initial} target={target} onSubmitted={() => {}} />);

    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(await screen.findByText(/送信に失敗しました/u)).toBeTruthy();
    expect(screen.getByRole("button", { name: "送信" })).toHaveProperty("disabled", false);
  });
});
