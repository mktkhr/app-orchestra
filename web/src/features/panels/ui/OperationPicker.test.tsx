import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vite-plus/test";

import type { CatalogEntry } from "@/shared/api/catalog";

import { OperationPicker } from "./OperationPicker";

const inventoryEntry: CatalogEntry = {
  service: "inventory",
  serviceDisplayName: "在庫管理",
  operationId: "ListInventoryItems",
  summary: "List stock items, optionally filtered by status.",
  displayName: "在庫一覧",
  component: "table",
  schema: { type: "object", required: [], properties: {} },
  fields: {
    status: { type: "string", title: "ステータス" },
    quantity: { type: "number", title: "数量" },
  },
};

const attendanceEntry: CatalogEntry = {
  service: "attendance",
  serviceDisplayName: "勤怠管理",
  operationId: "SummarizeAttendance",
  summary: "Summarize attendance records by employee.",
  displayName: "出勤の集計",
  component: "chart",
  schema: { type: "object", required: [], properties: {} },
  fields: {
    status: { type: "string", title: "種別" },
    count: { type: "number", title: "件数" },
  },
};

async function openPicker(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(await screen.findByRole("combobox", { name: "操作" }));
}

function renderPicker(): void {
  render(
    <OperationPicker
      entries={[inventoryEntry, attendanceEntry]}
      value={null}
      onChange={() => {}}
      loadError={null}
    />,
  );
}

/**
 * `docs/specs/picking.md` section 3 (K1) and section 4 (K2): what a person
 * can type to find an operation, and what an option shows once they do.
 */
describe("OperationPicker", () => {
  it("finds an operation by the title of a field it returns, though that word appears nowhere in its own name (AC-K-101)", async () => {
    const user = userEvent.setup();

    renderPicker();
    await openPicker(user);
    await user.type(screen.getByRole("combobox", { name: "操作" }), "数量");

    const option = screen.getByRole("option");

    expect(within(option).getByText("在庫一覧")).toBeTruthy();
  });

  it("finds nothing for a word in no operation's name, service, id, summary or fields (AC-K-101)", async () => {
    const user = userEvent.setup();

    renderPicker();
    await openPicker(user);
    await user.type(screen.getByRole("combobox", { name: "操作" }), "そんな単語はない");

    expect(screen.queryAllByRole("option")).toHaveLength(0);
  });

  it("matches case-insensitively on the operation id and the summary", async () => {
    const user = userEvent.setup();

    renderPicker();
    await openPicker(user);
    await user.type(screen.getByRole("combobox", { name: "操作" }), "listinventoryitems");

    expect(screen.getAllByRole("option")).toHaveLength(1);

    await user.clear(screen.getByRole("combobox", { name: "操作" }));
    await user.type(screen.getByRole("combobox", { name: "操作" }), "SUMMARIZE ATTENDANCE");

    expect(screen.getAllByRole("option")).toHaveLength(1);
  });

  it("shows each option's name and its summary underneath (AC-K-102)", async () => {
    const user = userEvent.setup();

    renderPicker();
    await openPicker(user);

    const option = screen.getByRole("option", { name: /在庫一覧/u });

    expect(within(option).getByText("在庫一覧")).toBeTruthy();
    expect(
      within(option).getByText("List stock items, optionally filtered by status."),
    ).toBeTruthy();
  });
});
