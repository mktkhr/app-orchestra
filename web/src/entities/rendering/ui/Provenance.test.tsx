import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vite-plus/test";

import { Provenance } from "./Provenance";

describe("Provenance", () => {
  it("shows the service's display name and the operation id", () => {
    render(
      <Provenance
        source={{
          service: "inventory",
          serviceDisplayName: "在庫管理",
          operationId: "listInventoryItems",
        }}
      />,
    );

    expect(screen.getByText("在庫管理 / listInventoryItems")).toBeTruthy();
    expect(screen.queryByText("inventory / listInventoryItems")).toBeNull();
  });

  it("shows no argument expander when there are no arguments", () => {
    render(
      <Provenance
        source={{
          service: "inventory",
          serviceDisplayName: "在庫管理",
          operationId: "listInventoryItems",
        }}
      />,
    );

    expect(screen.queryByText("引数を表示")).toBeNull();
  });

  it("reveals the planner's arguments on expansion", async () => {
    const user = userEvent.setup();

    render(
      <Provenance
        source={{
          service: "inventory",
          serviceDisplayName: "在庫管理",
          operationId: "listInventoryItems",
          args: { status: "allocated" },
        }}
      />,
    );

    expect(screen.queryByText("status=allocated")).toBeNull();

    await user.click(screen.getByText("引数を表示"));

    expect(screen.getByText("status=allocated")).toBeTruthy();
  });
});
