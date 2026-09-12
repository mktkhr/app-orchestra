import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { getUserPermissions, listOperations, listUsers } from "@/shared/api/users";

import { UsersPage } from "./UsersPage";

vi.mock("@/shared/api/users", () => ({
  listUsers: vi.fn<typeof listUsers>(),
  listOperations: vi.fn<typeof listOperations>(),
  getUserPermissions: vi.fn<typeof getUserPermissions>(),
}));

describe("UsersPage", () => {
  beforeEach(() => {
    vi.mocked(listUsers).mockReset();
    vi.mocked(listOperations).mockReset();
    vi.mocked(getUserPermissions).mockReset();
  });

  it("shows the permission grid once an account is selected", async () => {
    const user = userEvent.setup();

    vi.mocked(listUsers).mockResolvedValue([{ id: "usr-1", name: "やまだ", role: "user" }]);
    vi.mocked(listOperations).mockResolvedValue([
      { service: "inventory", operationId: "ListInventoryItems", summary: "在庫の一覧" },
    ]);
    vi.mocked(getUserPermissions).mockResolvedValue([]);

    render(<UsersPage />);

    expect(screen.getByText("権限を設定するアカウントを選んでください。")).toBeTruthy();

    await user.click(await screen.findByText("やまだ"));

    expect(await screen.findByText("在庫の一覧")).toBeTruthy();
  });
});
