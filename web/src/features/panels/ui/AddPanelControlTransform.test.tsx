import { render, screen, within } from "@testing-library/react";
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

const tableEntry: CatalogEntry = {
  service: "inventory",
  serviceDisplayName: "在庫管理",
  operationId: "ListInventoryItems",
  summary: "在庫一覧",
  displayName: "在庫一覧",
  component: "table",
  schema: { type: "object", required: [], properties: {} },
  fields: {
    status: { type: "string" },
    quantity: { type: "number" },
  },
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

async function selectFromDropdown(
  user: ReturnType<typeof userEvent.setup>,
  fieldName: string,
  optionName: string,
): Promise<void> {
  await user.click(screen.getByRole("combobox", { name: fieldName }));
  await user.click(await screen.findByRole("option", { name: optionName }));
}

/**
 * A chart built on top of a transform (`docs/plans/dashboard.md` Task 7's
 * own journey - "a list operation with a group-by and a bar chart"). Split
 * out of `AddPanelControl.test.tsx`, which is already at its `max-lines`
 * budget.
 *
 * `applyTransform` replaces every row with exactly two keys - the group-by
 * field and the aggregate's own name (`entities/rendering/lib/transform.ts`)
 * - so a chart drawn on top of one can only ever name those two axes, never
 * an arbitrary field of the raw response. Before this test the chart's own
 * axis pickers still offered every raw field regardless: a person could
 * pick `quantity` as the value axis over a `count` transform and save a
 * panel whose chart would silently draw nothing (`ResultChart` skips a
 * non-numeric value). `chartFieldOptionsFor` (`usePanelFields.ts`) is the
 * fix this test pins.
 */
describe("AddPanelControl, a chart built on top of a transform", () => {
  beforeEach(() => {
    vi.mocked(getCatalog).mockReset();
    vi.mocked(addPanel).mockReset();
  });

  it("offers only the group-by field and the aggregate's own name as chart axes once a transform is on", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([tableEntry]);

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);
    await pickOperation(user, "在庫一覧");

    await selectFromDropdown(user, "表示方法", "グラフ");
    await user.click(screen.getByRole("switch", { name: "集計してから描画する" }));
    await selectFromDropdown(user, "グループ化する項目", "status");

    await user.click(screen.getByRole("combobox", { name: "分類の軸" }));
    const categoryOptions = within(await screen.findByRole("listbox")).getAllByRole("option");

    expect(categoryOptions.map((option) => option.textContent)).toEqual(["status", "件数"]);

    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("combobox", { name: "値の軸" }));
    const valueOptions = within(await screen.findByRole("listbox")).getAllByRole("option");

    expect(valueOptions.map((option) => option.textContent)).toEqual(["status", "件数"]);
  });

  it("saves a panel whose chart draws the transform's own output shape", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([tableEntry]);
    vi.mocked(addPanel).mockResolvedValue({
      id: "panel-1",
      workspaceId: "ws-1",
      service: tableEntry.service,
      operationId: tableEntry.operationId,
      args: {},
      component: "chart",
      title: tableEntry.summary,
      position: 0,
      view: {
        transform: { groupBy: "status", aggregate: "count" },
        chart: { category: "status", value: "count", kind: "bar" },
      },
    });

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);
    await pickOperation(user, "在庫一覧");

    await selectFromDropdown(user, "表示方法", "グラフ");
    await user.click(screen.getByRole("switch", { name: "集計してから描画する" }));
    await selectFromDropdown(user, "グループ化する項目", "status");
    await selectFromDropdown(user, "分類の軸", "status");
    await selectFromDropdown(user, "値の軸", "件数");

    await user.click(await screen.findByRole("button", { name: "追加" }));

    expect(await screen.findByRole("button", { name: "パネルを追加" })).toBeTruthy();
    expect(addPanel).toHaveBeenCalledWith("ws-1", {
      service: "inventory",
      operationId: "ListInventoryItems",
      args: {},
      component: "chart",
      title: "在庫一覧",
      view: {
        transform: { groupBy: "status", aggregate: "count" },
        chart: { category: "status", value: "count", kind: "bar" },
      },
    });
  });
});
