import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vite-plus/test";

import { ResultTable } from "./ResultTable";

function items(count: number): readonly { readonly id: string; readonly name: string }[] {
  return Array.from({ length: count }, (_, index) => ({
    id: `itm-${String(index + 1).padStart(3, "0")}`,
    name: `品目${index + 1}`,
  }));
}

describe("ResultTable", () => {
  it("uses the keys of the first row as columns", () => {
    render(<ResultTable data={{ items: items(1) }} />);

    expect(screen.getByRole("columnheader", { name: "id" })).toBeTruthy();
    expect(screen.getByRole("columnheader", { name: "name" })).toBeTruthy();
  });

  it("renders only the first page of 10 rows out of 30", () => {
    render(<ResultTable data={{ items: items(30) }} />);

    expect(screen.getByText("itm-001")).toBeTruthy();
    expect(screen.getByText("itm-010")).toBeTruthy();
    expect(screen.queryByText("itm-011")).toBeNull();
  });

  it("moves to the next page", async () => {
    const user = userEvent.setup();

    render(<ResultTable data={{ items: items(30) }} />);

    await user.click(screen.getByRole("button", { name: "次のページ" }));

    expect(screen.getByText("itm-011")).toBeTruthy();
    expect(screen.queryByText("itm-001")).toBeNull();
  });

  it("shows no rows message when the array is empty", () => {
    render(<ResultTable data={{ items: [] }} />);

    expect(screen.getByText("結果は0件です。")).toBeTruthy();
  });

  it("expands into a full-screen dialog showing the same rows, and closes back", async () => {
    const user = userEvent.setup();

    render(<ResultTable data={{ items: items(3) }} />);

    expect(screen.queryByRole("dialog")).toBeNull();

    await user.click(screen.getByRole("button", { name: "拡大表示" }));

    const dialog = screen.getByRole("dialog");

    expect(within(dialog).getByText("itm-001")).toBeTruthy();
    expect(within(dialog).getByText("itm-002")).toBeTruthy();

    await user.click(within(dialog).getByRole("button", { name: "閉じる" }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });
});
