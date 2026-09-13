import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { getUserPermissions, listOperations, setUserPermissions } from "@/shared/api/users";

import { PermissionGrid } from "./PermissionGrid";

vi.mock("@/shared/api/users", () => ({
  listOperations: vi.fn<typeof listOperations>(),
  getUserPermissions: vi.fn<typeof getUserPermissions>(),
  setUserPermissions: vi.fn<typeof setUserPermissions>(),
}));

describe("PermissionGrid", () => {
  beforeEach(() => {
    vi.mocked(listOperations).mockReset();
    vi.mocked(getUserPermissions).mockReset();
    vi.mocked(setUserPermissions).mockReset();
  });

  it("groups operations under their own service's display name, not its identifier", async () => {
    vi.mocked(listOperations).mockResolvedValue([
      {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "ListInventoryItems",
        summary: "在庫の一覧",
      },
      {
        service: "attendance",
        serviceDisplayName: "勤怠管理",
        operationId: "ListAttendance",
        summary: "出勤の一覧",
      },
    ]);
    vi.mocked(getUserPermissions).mockResolvedValue([]);

    render(<PermissionGrid userId="usr-1" />);

    expect(await screen.findByText("在庫管理")).toBeTruthy();
    expect(screen.getByText("勤怠管理")).toBeTruthy();
    expect(screen.queryByText("inventory")).toBeNull();
    expect(screen.queryByText("attendance")).toBeNull();
    expect(screen.getByText("在庫の一覧")).toBeTruthy();
    expect(screen.getByText("出勤の一覧")).toBeTruthy();
  });

  it("checking a service's box grants every operation it holds", async () => {
    const user = userEvent.setup();

    vi.mocked(listOperations).mockResolvedValue([
      {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "ListInventoryItems",
        summary: "在庫の一覧",
      },
      {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "CreateInventoryItem",
        summary: "在庫の作成",
      },
    ]);
    vi.mocked(getUserPermissions).mockResolvedValue([]);

    render(<PermissionGrid userId="usr-1" />);

    await screen.findByText("在庫の一覧");

    await user.click(screen.getByRole("checkbox", { name: "在庫管理をすべて許可" }));

    expect(screen.getByRole("checkbox", { name: "在庫の一覧" })).toHaveProperty("checked", true);
    expect(screen.getByRole("checkbox", { name: "在庫の作成" })).toHaveProperty("checked", true);
  });

  it("saving replaces the account's permissions with what is checked", async () => {
    const user = userEvent.setup();

    vi.mocked(listOperations).mockResolvedValue([
      {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "ListInventoryItems",
        summary: "在庫の一覧",
      },
    ]);
    vi.mocked(getUserPermissions).mockResolvedValue([]);
    vi.mocked(setUserPermissions).mockResolvedValue();

    render(<PermissionGrid userId="usr-1" />);

    await user.click(await screen.findByRole("checkbox", { name: "在庫の一覧" }));
    await user.click(screen.getByRole("button", { name: "保存する" }));

    expect(await screen.findByText("保存しました")).toBeTruthy();
    expect(setUserPermissions).toHaveBeenCalledWith("usr-1", [
      { service: "inventory", operationId: "ListInventoryItems" },
    ]);
  });
});
