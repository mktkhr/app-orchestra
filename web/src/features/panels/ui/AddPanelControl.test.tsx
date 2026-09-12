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
  operationId: "ListInventoryItems",
  summary: "在庫一覧",
  component: "table",
  schema: {
    type: "object",
    required: [],
    properties: {
      keyword: { type: "string", title: "キーワード" },
    },
  },
  fields: {
    status: { type: "string" },
    quantity: { type: "number" },
  },
};

const chartEntry: CatalogEntry = {
  service: "attendance",
  operationId: "SummarizeAttendance",
  summary: "出勤の集計",
  component: "chart",
  schema: { type: "object", required: [], properties: {} },
  fields: {
    status: { type: "string" },
    count: { type: "number" },
  },
  view: { chart: { category: "status", value: "count", kind: "bar" } },
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

describe("AddPanelControl", () => {
  beforeEach(() => {
    vi.mocked(getCatalog).mockReset();
    vi.mocked(addPanel).mockReset();
  });

  it("lists only the operations the catalogue returned, grouped by service", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([tableEntry, chartEntry]);

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);

    await user.click(await screen.findByRole("combobox", { name: "操作" }));

    expect(await screen.findByText("在庫一覧")).toBeTruthy();
    expect(screen.getByText("出勤の集計")).toBeTruthy();
    expect(screen.getByText("inventory")).toBeTruthy();
    expect(screen.getByText("attendance")).toBeTruthy();
    expect(screen.getAllByRole("option")).toHaveLength(2);
  });

  it("picking an operation shows its arguments, built from schema", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([tableEntry]);

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);
    await pickOperation(user, "在庫一覧");

    expect(await screen.findByLabelText(/キーワード/u)).toBeTruthy();
  });

  it("picking chart shows the axes, offering only fields `fields` describes, and picking table shows none", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([tableEntry]);

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);
    await pickOperation(user, "在庫一覧");

    expect(screen.queryByRole("combobox", { name: "分類の軸" })).toBeNull();

    await user.click(screen.getByRole("combobox", { name: "表示方法" }));
    await user.click(await screen.findByRole("option", { name: "グラフ" }));

    const categoryField = await screen.findByRole("combobox", { name: "分類の軸" });

    await user.click(categoryField);
    const categoryOptions = within(await screen.findByRole("listbox")).getAllByRole("option");

    expect(categoryOptions.map((option) => option.textContent)).toEqual(["status", "quantity"]);
  });

  it("defaults the axes from the catalogue entry's own view", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([chartEntry]);

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);
    await pickOperation(user, "出勤の集計");

    expect(await screen.findByRole("combobox", { name: "分類の軸" })).toHaveProperty(
      "textContent",
      "status",
    );
    expect(screen.getByRole("combobox", { name: "値の軸" })).toHaveProperty("textContent", "count");
    expect(screen.getByRole("combobox", { name: "グラフの種類" })).toHaveProperty(
      "textContent",
      "棒グラフ",
    );
  });

  it("saves the panel with its view (AC-P-102)", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([chartEntry]);
    vi.mocked(addPanel).mockResolvedValue({
      id: "panel-1",
      workspaceId: "ws-1",
      service: chartEntry.service,
      operationId: chartEntry.operationId,
      args: {},
      component: "chart",
      title: chartEntry.summary,
      position: 0,
      view: { chart: { category: "status", value: "count", kind: "bar" } },
    });

    const onAdded = vi.fn<(panel: unknown) => void>();

    render(<AddPanelControl workspaceId="ws-1" onAdded={onAdded} />);
    await openBuilder(user);
    await pickOperation(user, "出勤の集計");

    await user.click(await screen.findByRole("button", { name: "追加" }));

    expect(await screen.findByRole("button", { name: "パネルを追加" })).toBeTruthy();
    expect(addPanel).toHaveBeenCalledWith("ws-1", {
      service: "attendance",
      operationId: "SummarizeAttendance",
      args: {},
      component: "chart",
      title: "出勤の集計",
      view: { chart: { category: "status", value: "count", kind: "bar" } },
    });
    expect(onAdded).toHaveBeenCalled();
  });

  it("posts no transform when none is set - a transform is optional", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([tableEntry]);
    vi.mocked(addPanel).mockResolvedValue({
      id: "panel-2",
      workspaceId: "ws-1",
      service: tableEntry.service,
      operationId: tableEntry.operationId,
      args: {},
      component: "table",
      title: tableEntry.summary,
      position: 0,
    });

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);
    await pickOperation(user, "在庫一覧");

    await user.click(await screen.findByRole("button", { name: "追加" }));

    expect(await screen.findByRole("button", { name: "パネルを追加" })).toBeTruthy();
    expect(addPanel).toHaveBeenCalledWith("ws-1", {
      service: "inventory",
      operationId: "ListInventoryItems",
      args: { keyword: "" },
      component: "table",
      title: "在庫一覧",
    });
  });
  it("says what is still missing instead of ignoring the press", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([tableEntry, chartEntry]);

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);
    await pickOperation(user, "在庫一覧");

    // The save control is live even when the form is not finished - make
    // guard-layout rejects a disabled contained button - so pressing it has
    // to answer rather than do nothing. The name defaults from the summary,
    // so empty it to leave a gap.
    await user.clear(await screen.findByRole("textbox", { name: "パネル名" }));
    await user.click(await screen.findByRole("button", { name: "追加" }));

    expect(await screen.findByText("パネル名を入力してください。")).toBeTruthy();
    expect(addPanel).not.toHaveBeenCalled();
  });
});
