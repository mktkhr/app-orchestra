import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { getUserPermissions, listOperations, setUserPermissions } from "@/shared/api/users";

import { usePermissionGrid } from "./usePermissionGrid";

vi.mock("@/shared/api/users", () => ({
  listOperations: vi.fn<typeof listOperations>(),
  getUserPermissions: vi.fn<typeof getUserPermissions>(),
  setUserPermissions: vi.fn<typeof setUserPermissions>(),
}));

describe("usePermissionGrid", () => {
  beforeEach(() => {
    vi.mocked(listOperations).mockReset();
    vi.mocked(getUserPermissions).mockReset();
    vi.mocked(setUserPermissions).mockReset();
  });

  it("groups the catalogue by service and marks what the account already holds", async () => {
    vi.mocked(listOperations).mockResolvedValue([
      {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operationId: "ListInventoryItems",
        summary: "List items",
      },
      { service: "inventory", serviceDisplayName: "在庫管理", operationId: "CreateInventoryItem" },
      { service: "attendance", serviceDisplayName: "勤怠管理", operationId: "ListAttendance" },
    ]);
    vi.mocked(getUserPermissions).mockResolvedValue([
      { service: "inventory", operationId: "ListInventoryItems" },
    ]);

    const { result } = renderHook(() => usePermissionGrid("usr-1"));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.groups).toEqual([
      {
        service: "inventory",
        serviceDisplayName: "在庫管理",
        operations: [
          {
            service: "inventory",
            serviceDisplayName: "在庫管理",
            operationId: "ListInventoryItems",
            summary: "List items",
          },
          {
            service: "inventory",
            serviceDisplayName: "在庫管理",
            operationId: "CreateInventoryItem",
          },
        ],
      },
      {
        service: "attendance",
        serviceDisplayName: "勤怠管理",
        operations: [
          { service: "attendance", serviceDisplayName: "勤怠管理", operationId: "ListAttendance" },
        ],
      },
    ]);
    expect(result.current.granted.has("inventory ListInventoryItems")).toBe(true);
    expect(result.current.granted.has("inventory CreateInventoryItem")).toBe(false);
  });

  it("toggles one operation on and off", async () => {
    vi.mocked(listOperations).mockResolvedValue([
      { service: "inventory", serviceDisplayName: "在庫管理", operationId: "ListInventoryItems" },
    ]);
    vi.mocked(getUserPermissions).mockResolvedValue([]);

    const { result } = renderHook(() => usePermissionGrid("usr-1"));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    act(() => {
      result.current.toggleOperation("inventory", "ListInventoryItems");
    });
    expect(result.current.granted.has("inventory ListInventoryItems")).toBe(true);

    act(() => {
      result.current.toggleOperation("inventory", "ListInventoryItems");
    });
    expect(result.current.granted.has("inventory ListInventoryItems")).toBe(false);
  });

  it("grants every operation a service holds in one gesture, and revokes them all the same way", async () => {
    vi.mocked(listOperations).mockResolvedValue([
      { service: "inventory", serviceDisplayName: "在庫管理", operationId: "ListInventoryItems" },
      { service: "inventory", serviceDisplayName: "在庫管理", operationId: "CreateInventoryItem" },
    ]);
    vi.mocked(getUserPermissions).mockResolvedValue([]);

    const { result } = renderHook(() => usePermissionGrid("usr-1"));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    act(() => {
      result.current.toggleService("inventory");
    });
    expect(result.current.granted.has("inventory ListInventoryItems")).toBe(true);
    expect(result.current.granted.has("inventory CreateInventoryItem")).toBe(true);

    act(() => {
      result.current.toggleService("inventory");
    });
    expect(result.current.granted.has("inventory ListInventoryItems")).toBe(false);
    expect(result.current.granted.has("inventory CreateInventoryItem")).toBe(false);
  });

  it("saves the whole granted set, replacing what the account held", async () => {
    vi.mocked(listOperations).mockResolvedValue([
      { service: "inventory", serviceDisplayName: "在庫管理", operationId: "ListInventoryItems" },
    ]);
    vi.mocked(getUserPermissions).mockResolvedValue([]);
    vi.mocked(setUserPermissions).mockResolvedValue();

    const { result } = renderHook(() => usePermissionGrid("usr-1"));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    act(() => {
      result.current.toggleOperation("inventory", "ListInventoryItems");
    });

    await act(async () => {
      await result.current.save();
    });

    expect(setUserPermissions).toHaveBeenCalledWith("usr-1", [
      { service: "inventory", operationId: "ListInventoryItems" },
    ]);
    expect(result.current.saved).toBe(true);
  });

  it("reports a load failure", async () => {
    vi.mocked(listOperations).mockRejectedValue(new Error("boom"));
    vi.mocked(getUserPermissions).mockResolvedValue([]);

    const { result } = renderHook(() => usePermissionGrid("usr-1"));

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });
  });

  it("reports a save failure", async () => {
    vi.mocked(listOperations).mockResolvedValue([]);
    vi.mocked(getUserPermissions).mockResolvedValue([]);
    vi.mocked(setUserPermissions).mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => usePermissionGrid("usr-1"));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.save();
    });

    expect(result.current.saveError).not.toBeNull();
    expect(result.current.saved).toBe(false);
  });
});
