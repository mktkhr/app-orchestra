import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";

import { ConversationProvider } from "../model/conversationStore";
import { Conversation } from "./Conversation";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

/** Reads an array element without a non-null assertion or an in-test conditional. */
function nth<T>(items: readonly T[], index: number): T {
  const item = items[index];

  if (item === undefined) {
    throw new Error(`expected an element at index ${String(index)}`);
  }

  return item;
}

describe("Conversation", () => {
  it("shows the three example questions before the first turn", () => {
    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    expect(screen.getByRole("button", { name: "在庫の一覧を見せて" })).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "在庫を登録して。名前はテスト品、数量は5、引当済で" }),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: "破損した在庫はある？" })).toBeTruthy();
  });

  it("submits an example question and appends the question and the answer as turns", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({
      kind: "none",
      message: "該当する操作が見つかりませんでした。",
    });

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "在庫の一覧を見せて" }));

    expect(await screen.findByText("在庫の一覧を見せて")).toBeTruthy();
    expect(await screen.findByText("該当する操作が見つかりませんでした。")).toBeTruthy();
    expect(postPlan).toHaveBeenCalledWith({ query: "在庫の一覧を見せて" });
  });

  it("submits a typed question through the form and disables it while pending", async () => {
    const user = userEvent.setup();
    let resolvePlan: (() => void) | undefined;

    vi.mocked(postPlan).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvePlan = () => {
            resolve({ kind: "none", message: "結果はありません。" });
          };
        }),
    );

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.type(screen.getByLabelText("質問を入力"), "出勤簿を見せて");
    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(screen.getByLabelText("質問を入力")).toHaveProperty("disabled", true);
    expect(await screen.findByText("出勤簿を見せて")).toBeTruthy();

    resolvePlan?.();

    expect(await screen.findByText("結果はありません。")).toBeTruthy();
    expect(screen.getByLabelText("質問を入力")).toHaveProperty("disabled", false);
  });

  it("shows an error and re-enables the form when the request fails", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockRejectedValue(new Error("network down"));

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.type(screen.getByLabelText("質問を入力"), "壊れた質問");
    await user.click(screen.getByRole("button", { name: "送信" }));

    expect(await screen.findByText(/質問の送信に失敗しました/u)).toBeTruthy();
    expect(screen.getByLabelText("質問を入力")).toHaveProperty("disabled", false);
  });

  it("renders a table result paginated, with its provenance and arguments revealed on expand", async () => {
    const user = userEvent.setup();
    const items = Array.from({ length: 30 }, (_, index) => ({
      id: `itm-${String(index + 1).padStart(3, "0")}`,
    }));

    vi.mocked(postPlan).mockResolvedValue({
      kind: "result",
      component: "table",
      data: { items },
      source: {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "listInventoryItems",
        args: { status: "allocated" },
      },
    });

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "在庫の一覧を見せて" }));

    expect(await screen.findByText("在庫管理 / listInventoryItems")).toBeTruthy();
    expect(await screen.findByText("itm-001")).toBeTruthy();
    expect(screen.queryByText("itm-011")).toBeNull();

    expect(screen.queryByText("status=allocated")).toBeNull();
    await user.click(screen.getByText("引数を表示"));
    expect(screen.getByText("status=allocated")).toBeTruthy();
  });

  it("expands a table result into a full-screen dialog and closes it back", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({
      kind: "result",
      component: "table",
      data: { items: [{ id: "itm-001" }, { id: "itm-002" }] },
      source: {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "listInventoryItems",
      },
    });

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "在庫の一覧を見せて" }));
    expect(await screen.findByText("itm-001")).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "拡大表示" }));

    const dialog = await screen.findByRole("dialog");

    expect(within(dialog).getByText("itm-001")).toBeTruthy();

    await user.click(within(dialog).getByRole("button", { name: "閉じる" }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });

  it("renders an ask result as choices and resolves the picked answer into a new turn", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValueOnce({
      kind: "ask",
      question: "「破損」に近いステータスはどれですか？",
      param: "status",
      options: [
        { value: "allocated", label: "引当済" },
        { value: "staged", label: "出荷準備完了" },
        { value: "quarantined", label: "検品保留" },
        { value: "consigned", label: "預託在庫" },
      ],
    });

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "破損した在庫はある？" }));

    expect(await screen.findByText("「破損」に近いステータスはどれですか？")).toBeTruthy();
    expect(screen.getByRole("button", { name: "検品保留" })).toBeTruthy();

    vi.mocked(postPlan).mockResolvedValueOnce({
      kind: "result",
      component: "table",
      data: { items: [{ id: "itm-001", status: "quarantined" }] },
      source: {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "listInventoryItems",
        args: { status: "quarantined" },
      },
    });

    await user.click(screen.getByRole("button", { name: "検品保留" }));

    expect(postPlan).toHaveBeenLastCalledWith({
      query: "破損した在庫はある？",
      answers: [{ param: "status", value: "quarantined" }],
    });
    expect(await screen.findByText("itm-001")).toBeTruthy();
  });

  it("keeps its turns after unmounting and remounting with the same key", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({ kind: "none", message: "結果はありません。" });

    const { rerender } = render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "在庫の一覧を見せて" }));
    expect(await screen.findByText("結果はありません。")).toBeTruthy();

    // Simulate opening a workspace (unmounting Conversation) and coming back.
    rerender(
      <ConversationProvider>
        <div>elsewhere</div>
      </ConversationProvider>,
    );
    rerender(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
      </ConversationProvider>,
    );

    expect(screen.getByText("在庫の一覧を見せて")).toBeTruthy();
    expect(screen.getByText("結果はありません。")).toBeTruthy();
  });

  it("empties its turns, and only its own, when 新しい会話 is pressed", async () => {
    const user = userEvent.setup();

    vi.mocked(postPlan).mockResolvedValue({ kind: "none", message: "結果はありません。" });

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" />
        <Conversation conversationKey="ws-1" />
      </ConversationProvider>,
    );

    const askButtons = screen.getAllByRole("button", { name: "在庫の一覧を見せて" });

    await user.click(nth(askButtons, 0));
    await user.click(nth(askButtons, 1));
    await screen.findAllByText("結果はありません。");

    const newConversationButtons = screen.getAllByRole("button", { name: "新しい会話" });

    await user.click(nth(newConversationButtons, 0));

    expect(screen.getAllByRole("button", { name: "在庫の一覧を見せて" })).toHaveLength(1);
    expect(screen.getAllByText("結果はありません。")).toHaveLength(1);
  });
});
