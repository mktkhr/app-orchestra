import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vite-plus/test";

import { useHashRoute } from "./useHashRoute";

function setHash(hash: string): void {
  act(() => {
    window.location.hash = hash;
    // happy-dom (web/vite.config.ts) does not reliably fire `hashchange` on
    // a bare `location.hash` assignment the way a real browser does -
    // dispatch it by hand so the hook's listener sees the same event a
    // person's browser would raise.
    window.dispatchEvent(new Event("hashchange"));
  });
}

describe("useHashRoute", () => {
  afterEach(() => {
    window.location.hash = "";
  });

  it("reads chat for no hash", () => {
    const { result } = renderHook(() => useHashRoute());

    expect(result.current).toEqual({ screen: "chat" });
  });

  it("reads chat for #chat", () => {
    setHash("#chat");

    const { result } = renderHook(() => useHashRoute());

    expect(result.current).toEqual({ screen: "chat" });
  });

  it("reads a workspace id out of #workspace-{id}", () => {
    setHash("#workspace-ws-1");

    const { result } = renderHook(() => useHashRoute());

    expect(result.current).toEqual({ screen: "workspace", workspaceId: "ws-1" });
  });

  it("reads users for #users", () => {
    setHash("#users");

    const { result } = renderHook(() => useHashRoute());

    expect(result.current).toEqual({ screen: "users" });
  });

  it("follows a hash change after mount", () => {
    const { result } = renderHook(() => useHashRoute());

    expect(result.current).toEqual({ screen: "chat" });

    setHash("#workspace-ws-2");

    expect(result.current).toEqual({ screen: "workspace", workspaceId: "ws-2" });
  });
});
