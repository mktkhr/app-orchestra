import { afterEach, describe, expect, it, vi } from "vite-plus/test";

import { getHealth, getWorkspace, postPlan } from "./client";

function stubFetch(status: number, body: unknown): void {
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(() =>
      Promise.resolve(
        new Response(JSON.stringify(body), {
          status,
          headers: { "content-type": "application/json" },
        }),
      ),
    ),
  );
}

describe("getHealth", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the body on success", async () => {
    stubFetch(200, { status: "ok" });

    await expect(getHealth()).resolves.toEqual({ status: "ok" });
  });

  it("throws when the request fails", async () => {
    stubFetch(500, { message: "boom" });

    await expect(getHealth()).rejects.toThrow("GET /api/health failed");
  });
});

describe("postPlan", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the decision on success", async () => {
    stubFetch(200, { kind: "none", message: "該当する操作が見つかりませんでした。" });

    await expect(postPlan({ query: "宇宙船を予約して" })).resolves.toEqual({
      kind: "none",
      message: "該当する操作が見つかりませんでした。",
    });
  });

  it("throws when the request fails", async () => {
    stubFetch(500, { message: "boom" });

    await expect(postPlan({ query: "在庫の一覧を見せて" })).rejects.toThrow(
      "POST /api/plan failed",
    );
  });
});

describe("getWorkspace", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the workspace and its panels on success", async () => {
    const workspace = {
      id: "ws-1",
      name: "在庫ボード",
      panels: [
        {
          id: "pnl-1",
          workspaceId: "ws-1",
          service: "inventory",
          operationId: "ListInventoryItems",
          args: { status: "quarantined" },
          component: "table",
          title: "検品保留の在庫",
          position: 0,
        },
      ],
    };

    stubFetch(200, workspace);

    await expect(getWorkspace("ws-1")).resolves.toEqual(workspace);
  });

  it("throws when the request fails", async () => {
    stubFetch(404, { message: "not found" });

    await expect(getWorkspace("missing")).rejects.toThrow("GET /api/workspaces/{id} failed");
  });
});
