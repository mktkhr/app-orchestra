import { afterEach, describe, expect, it, vi } from "vite-plus/test";

import { nextTurnId } from "./turnId";

describe("nextTurnId", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uses crypto.randomUUID when the context has it", () => {
    vi.stubGlobal("crypto", { randomUUID: () => "11111111-2222-3333-4444-555555555555" });

    expect(nextTurnId()).toBe("11111111-2222-3333-4444-555555555555");
  });

  it("still returns an id where crypto.randomUUID does not exist", () => {
    // Plain HTTP on anything but localhost - a LAN or Tailscale address -
    // is not a secure context, and the property is simply missing there.
    vi.stubGlobal("crypto", {});

    expect(nextTurnId()).toMatch(/^turn-\d+-\d+$/u);
  });

  it("does not repeat itself without crypto.randomUUID", () => {
    vi.stubGlobal("crypto", {});

    expect(nextTurnId()).not.toBe(nextTurnId());
  });
});
