import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { getCatalog, type CatalogEntry } from "@/shared/api/catalog";
import { patchPanel } from "@/shared/api/panels";

import { EditPanelControl } from "./EditPanelControl";

vi.mock("@/shared/api/catalog", () => ({
  getCatalog: vi.fn<typeof getCatalog>(),
}));

vi.mock("@/shared/api/panels", () => ({
  patchPanel: vi.fn<typeof patchPanel>(),
}));

const chartEntry: CatalogEntry = {
  service: "attendance",
  serviceDisplayName: "勤怠管理",
  operationId: "SummarizeAttendance",
  summary: "出勤の集計",
  displayName: "出勤の集計",
  // "table" (not "chart") so componentOptionsFor offers both - the panel
  // below still saved with "chart", one of the two - and the third test
  // can switch it back to the rule's own answer.
  component: "table",
  schema: {
    type: "object",
    required: [],
    properties: { keyword: { type: "string", title: "キーワード" } },
  },
  fields: { status: { type: "string" }, count: { type: "number" } },
};

const chartPanel = {
  id: "pnl-1",
  workspaceId: "ws-1",
  service: "attendance",
  operationId: "SummarizeAttendance",
  args: { keyword: "現場" },
  component: "chart" as const,
  title: "出勤の状況",
  position: 0,
  view: { chart: { category: "status", value: "count", kind: "bar" as const } },
};

async function openEditor(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(screen.getByRole("button", { name: "編集" }));
}

describe("EditPanelControl", () => {
  beforeEach(() => {
    vi.mocked(getCatalog).mockReset();
    vi.mocked(patchPanel).mockReset();
  });

  it("seeds the form from the panel, and states its operation rather than offering to change it (P13)", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([chartEntry]);

    render(<EditPanelControl workspaceId="ws-1" panel={chartPanel} onSaved={() => {}} />);
    await openEditor(user);

    expect(await screen.findByText(/出勤の集計/u)).toBeTruthy();
    expect(screen.queryByRole("combobox", { name: "操作" })).toBeNull();

    expect(await screen.findByRole("textbox", { name: "パネル名" })).toHaveProperty(
      "value",
      "出勤の状況",
    );
    expect(await screen.findByLabelText(/キーワード/u)).toHaveProperty("value", "現場");
    expect(screen.getByRole("combobox", { name: "分類の軸" })).toHaveProperty(
      "textContent",
      "status",
    );
  });

  it("saves through PATCH with every field the form shows, and calls onSaved with the result (AC-P-108, AC-P-110)", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([chartEntry]);

    const saved = { ...chartPanel, title: "改題後" };

    vi.mocked(patchPanel).mockResolvedValue(saved);

    const onSaved = vi.fn<(panel: unknown) => void>();

    render(<EditPanelControl workspaceId="ws-1" panel={chartPanel} onSaved={onSaved} />);
    await openEditor(user);

    const titleField = await screen.findByRole("textbox", { name: "パネル名" });

    await user.clear(titleField);
    await user.type(titleField, "改題後");
    await user.click(await screen.findByRole("button", { name: "保存" }));

    expect(patchPanel).toHaveBeenCalledWith("ws-1", "pnl-1", {
      title: "改題後",
      args: { keyword: "現場" },
      component: "chart",
      view: { chart: { category: "status", value: "count", kind: "bar" } },
    });
    expect(onSaved).toHaveBeenCalledWith(saved);
  });

  it("removing the chart's axes (switching to table) sends the view as null - remove, not leave alone (section 6a)", async () => {
    const user = userEvent.setup();

    vi.mocked(getCatalog).mockResolvedValue([chartEntry]);
    vi.mocked(patchPanel).mockResolvedValue({ ...chartPanel, component: "table" });

    render(<EditPanelControl workspaceId="ws-1" panel={chartPanel} onSaved={() => {}} />);
    await openEditor(user);

    await user.click(await screen.findByRole("combobox", { name: "表示方法" }));
    await user.click(await screen.findByRole("option", { name: "表" }));
    await user.click(await screen.findByRole("button", { name: "保存" }));

    expect(patchPanel).toHaveBeenCalledWith(
      "ws-1",
      "pnl-1",
      expect.objectContaining({ component: "table", view: null }),
    );
  });
});
