import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vite-plus/test";

import { PanelCardShell } from "./PanelCardShell";

describe("PanelCardShell", () => {
  it("shows the title and draws whatever it is given", () => {
    render(
      <PanelCardShell title="検品保留の在庫">
        <p>中身</p>
      </PanelCardShell>,
    );

    expect(screen.getByText("検品保留の在庫")).toBeTruthy();
    expect(screen.getByText("中身")).toBeTruthy();
  });

  it("draws the given action alongside the title", () => {
    render(
      <PanelCardShell title="検品保留の在庫" action={<button type="button">更新</button>}>
        <p>中身</p>
      </PanelCardShell>,
    );

    expect(screen.getByRole("button", { name: "更新" })).toBeTruthy();
  });
});
