import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { listUsers } from "@/shared/api/users";

import { useAccounts } from "./useAccounts";

vi.mock("@/shared/api/users", () => ({
  listUsers: vi.fn<typeof listUsers>(),
}));

describe("useAccounts", () => {
  beforeEach(() => {
    vi.mocked(listUsers).mockReset();
  });

  it("loads every account once", async () => {
    vi.mocked(listUsers).mockResolvedValue([
      { id: "usr-admin", name: "admin", role: "admin" },
      { id: "usr-1", name: "someone", role: "user" },
    ]);

    const { result } = renderHook(() => useAccounts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.accounts).toHaveLength(2);
    expect(result.current.error).toBeNull();
  });

  it("reports a load failure", async () => {
    vi.mocked(listUsers).mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useAccounts());

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });

    expect(result.current.accounts).toHaveLength(0);
  });
});
