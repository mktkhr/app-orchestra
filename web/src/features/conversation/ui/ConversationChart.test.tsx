import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vite-plus/test";

import { postPlan } from "@/shared/api/client";

import { ConversationProvider } from "../model/conversationStore";
import { Conversation } from "./Conversation";
import type { SaveControlSlotProps } from "./TurnList";

vi.mock("@/shared/api/client", () => ({
  postPlan: vi.fn<typeof postPlan>(),
}));

/**
 * A chart result from the chat (AC-P-105, `docs/plans/dashboard.md` Task
 * 7): a chat answer whose endpoint declared `x-ui-hint.chart` draws with no
 * view configured by the person, and saving it - through the
 * `renderSaveControl` slot `Conversation` composes with
 * `SaveToWorkspaceControl` - carries the contract's own axes onto the new
 * panel. Split out of `Conversation.test.tsx`, which is already at its
 * `max-lines` budget (`harness/quality/eslint`).
 */
describe("Conversation, a chart result", () => {
  it("draws a chart result with no view configured, and offers the axes to the save slot", async () => {
    const user = userEvent.setup();
    const view = { chart: { category: "status", value: "count", kind: "bar" as const } };
    const source = {
      service: "inventory",
      serviceDisplayName: "在庫管理",
      operationId: "countByStatus",
      args: {},
    };

    vi.mocked(postPlan).mockResolvedValue({
      kind: "result",
      component: "chart",
      data: { items: [{ status: "allocated", count: 3 }] },
      source,
      view,
    });

    const renderSaveControl = vi.fn<(props: SaveControlSlotProps) => ReactNode>(() => "save-slot");

    render(
      <ConversationProvider>
        <Conversation conversationKey="chat" renderSaveControl={renderSaveControl} />
      </ConversationProvider>,
    );

    await user.click(screen.getByRole("button", { name: "在庫の一覧を見せて" }));

    expect(await screen.findByText("save-slot")).toBeTruthy();
    expect(renderSaveControl).toHaveBeenCalledWith(
      expect.objectContaining({ source, component: "chart", view }),
    );
  });
});
