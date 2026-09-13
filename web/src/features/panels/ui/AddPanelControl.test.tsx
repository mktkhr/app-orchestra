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
  serviceDisplayName: "勤怠管理",
  operationId: "SummarizeAttendance",
  summary: "出勤の集計",
  displayName: "出勤の集計",
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
  // An option now shows its name and its summary underneath (AC-K-102),
  // so its accessible name is both lines - a `RegExp` matches within that
  // rather than requiring the whole thing.
  await user.click(await screen.findByRole("option", { name: new RegExp(summary, "u") }));
}

describe("AddPanelControl", () => {
  beforeEach(() => {
    vi.mocked(getCatalog).mockReset();
    vi.mocked(addPanel).mockReset();
  });

  it("lists only the operations the catalogue returned, grouped by the service's display name", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([tableEntry, chartEntry]);

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);

    await user.click(await screen.findByRole("combobox", { name: "操作" }));

    // Each option now shows its name and its summary underneath it
    // (AC-K-102) - in these fixtures the same string, so `getAllByText`
    // rather than `getByText`/`findByText`, which throw on more than one
    // match.
    expect((await screen.findAllByText("在庫一覧")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("出勤の集計").length).toBeGreaterThan(0);
    expect(screen.getByText("在庫管理")).toBeTruthy();
    expect(screen.getByText("勤怠管理")).toBeTruthy();
    expect(screen.queryByText("inventory")).toBeNull();
    expect(screen.queryByText("attendance")).toBeNull();
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
      width: 12,
      height: 1,
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
      width: 12,
      height: 1,
    });

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);
    await pickOperation(user, "在庫一覧");

    await user.click(await screen.findByRole("button", { name: "追加" }));

    expect(await screen.findByRole("button", { name: "パネルを追加" })).toBeTruthy();
    // "keyword" is optional and nobody typed into it - `compactArgs`
    // (`usePanelBuilder.ts`) drops it rather than posting it as an empty
    // string, the same way an unfilled optional query parameter has to be
    // omitted for a real enum parameter to invoke without error (see
    // `AddPanelControlArgs.test.tsx`).
    expect(addPanel).toHaveBeenCalledWith("ws-1", {
      service: "inventory",
      operationId: "ListInventoryItems",
      args: {},
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

  it("shows only the picker before an operation is picked, and its arguments, component, name and the transform's switch alone after (AC-K-103)", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([tableEntry]);

    render(<AddPanelControl workspaceId="ws-1" onAdded={() => {}} />);
    await openBuilder(user);

    // Before an operation is picked: the picker, and nothing else.
    expect(screen.getByRole("combobox", { name: "操作" })).toBeTruthy();
    expect(screen.queryByRole("textbox", { name: "パネル名" })).toBeNull();
    expect(screen.queryByRole("combobox", { name: "表示方法" })).toBeNull();
    expect(screen.queryByRole("switch")).toBeNull();

    await pickOperation(user, "在庫一覧");

    // After: its arguments, how to draw it, its name.
    expect(await screen.findByLabelText(/キーワード/u)).toBeTruthy();
    expect(screen.getByRole("combobox", { name: "表示方法" })).toBeTruthy();
    expect(screen.getByRole("textbox", { name: "パネル名" })).toBeTruthy();

    // The transform is on screen as its own switch alone - not its three
    // fields, which apply to nothing until the switch is on.
    expect(screen.getByRole("switch", { name: "集計してから描画する" })).toBeTruthy();
    expect(screen.queryByRole("combobox", { name: "グループ化する項目" })).toBeNull();
    expect(screen.queryByRole("combobox", { name: "集計方法" })).toBeNull();
    expect(screen.queryByRole("combobox", { name: "集計する項目" })).toBeNull();
  });
});
