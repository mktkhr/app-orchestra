import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { addPanel } from "@/shared/api/client";
import { getCatalog, type CatalogEntry } from "@/shared/api/catalog";

import { AddPanelControl } from "./AddPanelControl";

vi.mock("@/shared/api/client", () => ({
  addPanel: vi.fn<typeof addPanel>(),
}));

vi.mock("@/shared/api/catalog", () => ({
  getCatalog: vi.fn<typeof getCatalog>(),
}));

/**
 * A catalogue entry shaped like `ListInventoryItems`'s own schema: one
 * optional enum parameter (`status`), nobody's business to fill in unless
 * they want to filter.
 */
const listEntry: CatalogEntry = {
  service: "inventory",
  serviceDisplayName: "在庫管理",
  operationId: "ListInventoryItems",
  summary: "在庫一覧",
  displayName: "在庫一覧",
  component: "table",
  schema: {
    type: "object",
    required: [],
    properties: {
      status: {
        type: "string",
        title: "ステータス",
        enum: ["allocated", "staged", "quarantined", "consigned"],
      },
    },
  },
  fields: { status: { type: "string" } },
};

async function openBuilder(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(screen.getByRole("button", { name: "パネルを追加" }));
}

async function pickOperation(
  user: ReturnType<typeof userEvent.setup>,
  summary: string,
): Promise<void> {
  await user.click(await screen.findByRole("combobox", { name: "操作" }));
  await user.click(await screen.findByText(summary));
}

/**
 * A real enum query parameter, left untouched. `useFormValues` seeds it to
 * `""` (its type-appropriate default for a control nobody touched) -
 * before this test's own fix (`usePanelBuilder.ts`'s `compactArgs`), that
 * `""` was posted as the argument's own value and every later refresh of
 * the panel failed `usecase.validateEnumArg` ("" is not a valid value for
 * "status") - a panel a person just built that can never draw
 * (`docs/plans/dashboard.md` Task 7's own journey). `AddPanelControl.test.tsx`'s
 * `tableEntry` fixture never had a real enum, which is why this gap was
 * not caught until Task 7 tried to run the journey end to end.
 */
describe("AddPanelControl, an operation with an optional enum argument", () => {
  beforeEach(() => {
    vi.mocked(getCatalog).mockReset();
    vi.mocked(addPanel).mockReset();
  });

  it("omits an untouched optional enum argument rather than posting it as an empty string", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([listEntry]);
    vi.mocked(addPanel).mockResolvedValue({
      id: "panel-1",
      workspaceId: "ws-1",
      service: listEntry.service,
      operationId: listEntry.operationId,
      args: {},
      component: "table",
      title: listEntry.summary,
      position: 0,
      width: 12,
      height: 1,
    });

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);
    await pickOperation(user, "在庫一覧");

    await user.click(await screen.findByRole("button", { name: "追加" }));

    expect(await screen.findByRole("button", { name: "パネルを追加" })).toBeTruthy();
    expect(addPanel).toHaveBeenCalledWith("ws-1", {
      service: "inventory",
      operationId: "ListInventoryItems",
      args: {},
      component: "table",
      title: "在庫一覧",
    });
  });
});
