import { describe, expect, it } from "vite-plus/test";

import { applyTransform, type Transform } from "./transform";

describe("applyTransform", () => {
  it("groups by a field with three distinct values into three rows", () => {
    const rows = [
      { status: "allocated" },
      { status: "staged" },
      { status: "allocated" },
      { status: "quarantined" },
    ];
    const transform: Transform = { groupBy: "status", aggregate: "count" };

    expect(applyTransform(rows, transform)).toEqual([
      { status: "allocated", count: 2 },
      { status: "staged", count: 1 },
      { status: "quarantined", count: 1 },
    ]);
  });

  it("orders output rows by first-seen order of the group key", () => {
    const rows = [{ status: "c" }, { status: "a" }, { status: "b" }, { status: "a" }];
    const transform: Transform = { groupBy: "status", aggregate: "count" };

    expect(applyTransform(rows, transform)).toEqual([
      { status: "c", count: 1 },
      { status: "a", count: 2 },
      { status: "b", count: 1 },
    ]);
  });

  it("count counts rows per group, ignoring any field", () => {
    const rows = [{ status: "a" }, { status: "a" }, { status: "b" }];
    const transform: Transform = { groupBy: "status", aggregate: "count" };

    expect(applyTransform(rows, transform)).toEqual([
      { status: "a", count: 2 },
      { status: "b", count: 1 },
    ]);
  });

  it("sum reads transform.field and adds it up per group", () => {
    const rows = [
      { status: "a", quantity: 3 },
      { status: "a", quantity: 4 },
      { status: "b", quantity: 10 },
    ];
    const transform: Transform = { groupBy: "status", aggregate: "sum", field: "quantity" };

    expect(applyTransform(rows, transform)).toEqual([
      { status: "a", sum: 7 },
      { status: "b", sum: 10 },
    ]);
  });

  it("avg reads transform.field and averages it per group", () => {
    const rows = [
      { status: "a", quantity: 3 },
      { status: "a", quantity: 5 },
      { status: "b", quantity: 10 },
    ];
    const transform: Transform = { groupBy: "status", aggregate: "avg", field: "quantity" };

    expect(applyTransform(rows, transform)).toEqual([
      { status: "a", avg: 4 },
      { status: "b", avg: 10 },
    ]);
  });

  it("groups a row whose groupBy field is missing under a shared bucket rather than dropping it", () => {
    const rows = [{ status: "a" }, { note: "no status field here" }];
    const transform: Transform = { groupBy: "status", aggregate: "count" };

    // Dropping the second row would make the total quietly disagree with
    // `rows.length`, which is a worse bug than an honest "unknown" bucket.
    expect(applyTransform(rows, transform)).toEqual([
      { status: "a", count: 1 },
      { status: null, count: 1 },
    ]);
  });

  it("groups a row whose groupBy value is null under the same shared bucket as a missing one", () => {
    const rows = [{ status: null }, { note: "missing status" }];
    const transform: Transform = { groupBy: "status", aggregate: "count" };

    expect(applyTransform(rows, transform)).toEqual([{ status: null, count: 2 }]);
  });

  it("groups a row whose groupBy value is not a string (e.g. a number) under the same shared bucket", () => {
    const rows = [{ status: 42 }, { status: null }];
    const transform: Transform = { groupBy: "status", aggregate: "count" };

    expect(applyTransform(rows, transform)).toEqual([{ status: null, count: 2 }]);
  });

  it("skips a non-numeric value under field for sum, rather than producing NaN", () => {
    const rows = [
      { status: "a", quantity: 3 },
      { status: "a", quantity: "not-a-number" },
      { status: "a", quantity: null },
    ];
    const transform: Transform = { groupBy: "status", aggregate: "sum", field: "quantity" };

    expect(applyTransform(rows, transform)).toEqual([{ status: "a", sum: 3 }]);
  });

  it("skips a non-numeric value under field for avg, rather than producing NaN", () => {
    const rows = [
      { status: "a", quantity: 3 },
      { status: "a", quantity: 5 },
      { status: "a", quantity: "not-a-number" },
    ];
    const transform: Transform = { groupBy: "status", aggregate: "avg", field: "quantity" };

    expect(applyTransform(rows, transform)).toEqual([{ status: "a", avg: 4 }]);
  });

  it("sums to 0 for a group whose values were all skipped", () => {
    const rows = [{ status: "a", quantity: "n/a" }];
    const transform: Transform = { groupBy: "status", aggregate: "sum", field: "quantity" };

    expect(applyTransform(rows, transform)).toEqual([{ status: "a", sum: 0 }]);
  });

  it("averages to null for a group whose values were all skipped, rather than 0 or NaN", () => {
    const rows = [{ status: "a", quantity: "n/a" }];
    const transform: Transform = { groupBy: "status", aggregate: "avg", field: "quantity" };

    // 0 would claim the average of nothing is zero; NaN is a bug a person
    // reports. Neither is honest, so a group with no valid values gets null.
    expect(applyTransform(rows, transform)).toEqual([{ status: "a", avg: null }]);
  });

  it("treats sum with field entirely absent as if every value were non-numeric", () => {
    const rows = [{ status: "a" }, { status: "a" }];
    const transform: Transform = { groupBy: "status", aggregate: "sum" };

    expect(applyTransform(rows, transform)).toEqual([{ status: "a", sum: 0 }]);
  });

  it("treats avg with field entirely absent as if every value were non-numeric", () => {
    const rows = [{ status: "a" }, { status: "a" }];
    const transform: Transform = { groupBy: "status", aggregate: "avg" };

    expect(applyTransform(rows, transform)).toEqual([{ status: "a", avg: null }]);
  });

  it("returns an empty array for empty input", () => {
    const transform: Transform = { groupBy: "status", aggregate: "count" };

    expect(applyTransform([], transform)).toEqual([]);
  });
});
