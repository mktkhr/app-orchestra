import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vite-plus/test";

import type { Fields } from "../model/rows";
import { ResultDetail } from "./ResultDetail";

describe("ResultDetail", () => {
  it("renders each property as a label/value pair, translating an enum value to its Japanese label", () => {
    const fields: Fields = {
      status: {
        type: "string",
        enum: ["allocated", "staged"],
        enumLabels: { allocated: "引当済", staged: "出荷準備完了" },
      },
    };

    render(
      <ResultDetail
        data={{ name: "テスト品", quantity: 5, status: "allocated" }}
        fields={fields}
      />,
    );

    expect(screen.getByText("name")).toBeTruthy();
    expect(screen.getByText("テスト品")).toBeTruthy();
    expect(screen.getByText("quantity")).toBeTruthy();
    expect(screen.getByText("5")).toBeTruthy();
    expect(screen.getByText("status")).toBeTruthy();
    expect(screen.getByText("引当済")).toBeTruthy();
    expect(screen.queryByText("allocated")).toBeNull();
  });

  it("falls back to the raw key and value when there is no field schema", () => {
    render(<ResultDetail data={{ id: "itm-001" }} />);

    expect(screen.getByText("id")).toBeTruthy();
    expect(screen.getByText("itm-001")).toBeTruthy();
  });

  it("shows a message when the object has no properties", () => {
    render(<ResultDetail data={{}} />);

    expect(screen.getByText("表示できる項目がありません。")).toBeTruthy();
  });
});
