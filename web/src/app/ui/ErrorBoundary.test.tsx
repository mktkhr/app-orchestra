import { render, screen } from "@testing-library/react";
import type { JSX } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { ErrorBoundary } from "./ErrorBoundary";

function Throws(): JSX.Element {
  throw new Error("crypto.randomUUID is not a function");
}

describe("ErrorBoundary", () => {
  beforeEach(() => {
    // React reports a caught error through console.error. Silence it so a
    // passing test does not print a stack trace.
    vi.spyOn(console, "error").mockImplementation(() => {
      // swallowed
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders its children when nothing throws", () => {
    render(
      <ErrorBoundary>
        <p>そのまま</p>
      </ErrorBoundary>,
    );

    expect(screen.getByText("そのまま")).toBeTruthy();
  });

  it("names the failure instead of leaving the page blank", () => {
    render(
      <ErrorBoundary>
        <Throws />
      </ErrorBoundary>,
    );

    expect(screen.getByText("画面の描画に失敗しました")).toBeTruthy();
    expect(screen.getByText("crypto.randomUUID is not a function")).toBeTruthy();
  });
});
