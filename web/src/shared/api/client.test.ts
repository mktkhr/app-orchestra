import { afterEach, describe, expect, it, vi } from "vite-plus/test";

import {
  deleteSession,
  getHealth,
  getSession,
  getWorkspace,
  onUnauthorized,
  postPlan,
  postSession,
} from "./client";

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

describe("getSession", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the signed-in user on success", async () => {
    stubFetch(200, { id: "usr-1", name: "admin", role: "admin" });

    await expect(getSession()).resolves.toEqual({ id: "usr-1", name: "admin", role: "admin" });
  });

  it("resolves to null, not a rejection, when nobody is signed in", async () => {
    stubFetch(401, { message: "unauthorized" });

    await expect(getSession()).resolves.toBeNull();
  });
});

describe("postSession", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the signed-in user on success", async () => {
    stubFetch(200, { id: "usr-1", name: "admin", role: "admin" });

    await expect(postSession({ name: "admin", password: "correct" })).resolves.toEqual({
      id: "usr-1",
      name: "admin",
      role: "admin",
    });
  });

  it("throws on a wrong name or password", async () => {
    stubFetch(401, { message: "wrong" });

    await expect(postSession({ name: "admin", password: "wrong" })).rejects.toThrow(
      "POST /api/session failed",
    );
  });
});

describe("deleteSession", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("resolves on success", async () => {
    stubFetch(204, {});

    await expect(deleteSession()).resolves.toBeUndefined();
  });
});

describe("onUnauthorized", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("notifies every subscriber when any call comes back 401", async () => {
    stubFetch(401, { message: "unauthorized" });

    const listener = vi.fn<() => void>();
    const unsubscribe = onUnauthorized(listener);

    await getSession();

    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    await getSession();

    expect(listener).toHaveBeenCalledTimes(1);
  });

  it("does not notify on a non-401 response", async () => {
    stubFetch(200, { id: "usr-1", name: "admin", role: "admin" });

    const listener = vi.fn<() => void>();
    const unsubscribe = onUnauthorized(listener);

    await getSession();
    unsubscribe();

    expect(listener).not.toHaveBeenCalled();
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
      "POST /api/plan failed: boom",
    );
  });

  it("carries the server's message on the error", async () => {
    stubFetch(500, { message: "invoking inventory: service unreachable" });

    await expect(postPlan({ query: "在庫の一覧を見せて" })).rejects.toMatchObject({
      name: "PlanRequestError",
      serverMessage: "invoking inventory: service unreachable",
    });
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
