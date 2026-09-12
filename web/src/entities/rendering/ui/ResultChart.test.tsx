import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vite-plus/test";

import { ResultChart } from "./ResultChart";

const ROWS = [
  { status: "allocated", count: 3 },
  { status: "staged", count: 1 },
  { status: "quarantined", count: 2 },
];

describe("ResultChart", () => {
  it("draws three bars for three rows with a category and a value", () => {
    const { container } = render(
      <ResultChart data={ROWS} category="status" value="count" kind="bar" title="件数" />,
    );

    expect(container.querySelectorAll(".MuiBarChart-element")).toHaveLength(3);
  });

  it("draws the same rows as a line when kind is line", () => {
    const { container } = render(
      <ResultChart data={ROWS} category="status" value="count" kind="line" title="件数" />,
    );

    expect(container.querySelectorAll(".MuiLineChart-line")).toHaveLength(1);
    expect(container.querySelectorAll(".MuiLineChart-mark")).toHaveLength(3);
  });

  it("draws the same rows as a pie when kind is pie", () => {
    const { container } = render(
      <ResultChart data={ROWS} category="status" value="count" kind="pie" title="件数" />,
    );

    expect(container.querySelectorAll(".MuiPieChart-arc")).toHaveLength(3);
  });

  it("carries its title as a visible figcaption, the chart's text alternative", () => {
    render(
      <ResultChart
        data={ROWS}
        category="status"
        value="count"
        kind="bar"
        title="ステータス別の件数"
      />,
    );

    expect(screen.getByText("ステータス別の件数").tagName).toBe("FIGCAPTION");
  });

  it("skips a row whose value is not a number, rather than drawing it as NaN", () => {
    const rows = [
      { status: "allocated", count: 3 },
      { status: "staged", count: "not-a-number" },
      { status: "quarantined", count: null },
    ];

    const { container } = render(
      <ResultChart data={rows} category="status" value="count" kind="bar" title="件数" />,
    );

    expect(container.querySelectorAll(".MuiBarChart-element")).toHaveLength(1);
  });

  it("draws a row whose category is not a string under a shared placeholder label", () => {
    const rows = [
      { status: 42, count: 3 },
      { status: null, count: 1 },
    ];

    const { container } = render(
      <ResultChart data={rows} category="status" value="count" kind="bar" title="件数" />,
    );

    // Both rows survive - a non-string category is not a reason to drop a row - and both
    // are labelled with the same shared placeholder, consistently with transform.ts's
    // UNGROUPED bucket rather than a different stringification per row.
    expect(container.querySelectorAll(".MuiBarChart-element")).toHaveLength(2);
    expect(screen.getAllByText("null").length).toBeGreaterThan(0);
  });

  it("shows a message instead of a blank chart when there are no rows", () => {
    render(<ResultChart data={[]} category="status" value="count" kind="bar" title="件数" />);

    expect(screen.getByText("結果は0件です。")).toBeTruthy();
  });

  it("shows the same message when every row is filtered out for a non-numeric value", () => {
    const rows = [{ status: "a", count: "n/a" }];

    render(<ResultChart data={rows} category="status" value="count" kind="bar" title="件数" />);

    expect(screen.getByText("結果は0件です。")).toBeTruthy();
  });
});
