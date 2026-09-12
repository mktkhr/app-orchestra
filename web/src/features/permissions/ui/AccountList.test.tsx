import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { listUsers } from "@/shared/api/users";

import { AccountList } from "./AccountList";

vi.mock("@/shared/api/users", () => ({
  listUsers: vi.fn<typeof listUsers>(),
}));

describe("AccountList", () => {
  beforeEach(() => {
    vi.mocked(listUsers).mockReset();
  });

  it("lists every account with its role", async () => {
    vi.mocked(listUsers).mockResolvedValue([
      { id: "usr-admin", name: "admin", role: "admin" },
      { id: "usr-1", name: "やまだ", role: "user" },
    ]);

    render(<AccountList selectedId={null} onSelect={() => {}} />);

    expect(await screen.findByText("admin")).toBeTruthy();
    expect(screen.getByText("管理者")).toBeTruthy();
    expect(screen.getByText("やまだ")).toBeTruthy();
    expect(screen.getByText("一般")).toBeTruthy();
  });

  it("calls onSelect with the clicked account's id", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn<(id: string) => void>();

    vi.mocked(listUsers).mockResolvedValue([{ id: "usr-1", name: "やまだ", role: "user" }]);

    render(<AccountList selectedId={null} onSelect={onSelect} />);

    await user.click(await screen.findByText("やまだ"));

    expect(onSelect).toHaveBeenCalledWith("usr-1");
  });
});
