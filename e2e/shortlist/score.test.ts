import { expect, test } from "vite-plus/test";

import { latencyStats, score, scoreboard, tally, type QuestionResult } from "./score.ts";

/**
 * score.ts's own tests: no server, no model - every QuestionResult here is
 * a fake (docs/plans/shortlisting.md Task 4, Step 3).
 */

const base: QuestionResult = {
  id: "a01",
  axis: "A",
  text: "バーコードの発行状況を見せて",
  answers: ["listInventoryBarcodes"],
  kind: "result",
  operationId: "listInventoryBarcodes",
  latencyMs: 100,
};

test.each([
  { name: "a matching result is correct@1", overrides: {}, want: true },
  {
    name: "a non-matching result is not correct@1",
    overrides: { operationId: "getSalesOrder" },
    want: false,
  },
  { name: "an ask is not correct@1", overrides: { kind: "ask" as const }, want: false },
  { name: "a none is not correct@1", overrides: { kind: "none" as const }, want: false },
])("$name", ({ overrides, want }) => {
  expect(score({ ...base, ...overrides }).correctAt1).toBe(want);
});

test.each([
  {
    name: "a form over an unsafe operation matching the answer key is correct@1",
    overrides: { kind: "form" as const, operationId: "createInventoryBarcode" },
    answers: ["createInventoryBarcode"],
    want: true,
  },
  {
    name: "a form over an unsafe operation not matching the answer key is not correct@1",
    overrides: { kind: "form" as const, operationId: "createInventoryBarcode" },
    answers: ["listInventoryBarcodes"],
    want: false,
  },
  {
    name: "a form degraded from an ask over a safe operation is never correct@1, even if its target matches",
    overrides: { kind: "form" as const, operationId: "listInventoryBarcodes", askDegraded: true },
    answers: ["listInventoryBarcodes"],
    want: false,
  },
])("$name", ({ overrides, answers, want }) => {
  expect(score({ ...base, ...overrides, answers }).correctAt1).toBe(want);
});

test.each([
  {
    name: "an ask-degraded form is scored as asked, not as a pick",
    overrides: { kind: "form" as const, operationId: "listInventoryBarcodes", askDegraded: true },
  },
  { name: "a plain ask is scored as asked", overrides: { kind: "ask" as const } },
])("$name", ({ overrides }) => {
  const scored = score({ ...base, ...overrides });

  expect(scored.asked).toBe(true);
});

test("a form over an unsafe operation is not scored as asked", () => {
  const scored = score({ ...base, kind: "form", operationId: "createInventoryBarcode" });

  expect(scored.asked).toBe(false);
});

test.each([
  { name: "correct@1 is also correct@shown", overrides: {}, want: true },
  {
    name: "a wrong result with the answer among alternatives is correct@shown",
    overrides: {
      operationId: "getSalesOrder",
      alternatives: ["listInventoryBarcodes"],
    },
    want: true,
  },
  {
    name: "a wrong result with the answer absent from alternatives is not correct@shown",
    overrides: { operationId: "getSalesOrder", alternatives: ["listSalesOrders"] },
    want: false,
  },
  {
    name: "a wrong result with no alternatives is not correct@shown",
    overrides: { operationId: "getSalesOrder" },
    want: false,
  },
])("$name", ({ overrides, want }) => {
  expect(score({ ...base, ...overrides }).correctAtShown).toBe(want);
});

test.each([
  { kind: "ask" as const, field: "asked" as const },
  { kind: "none" as const, field: "none" as const },
  { kind: "error" as const, field: "error" as const },
])("a $kind result flags only its own kind", ({ kind, field }) => {
  const { operationId: _operationId, ...withoutOperationId } = base;
  const scored = score({ ...withoutOperationId, kind });

  expect(scored[field]).toBe(true);
});

test("tally counts and rates a group of scored questions", () => {
  const scored = [
    score(base),
    score({ ...base, id: "a02", operationId: "getSalesOrder" }),
    score({ ...base, id: "a03", kind: "ask" }),
    score({ ...base, id: "a04", kind: "none" }),
  ];

  expect(tally(scored)).toEqual({
    total: 4,
    correctAt1: 1,
    correctAtShown: 1,
    asked: 1,
    none: 1,
    error: 0,
    correctAt1Rate: 0.25,
    correctAtShownRate: 0.25,
  });
});

test("tally of an empty group reads every rate as 0, not NaN", () => {
  expect(tally([]).correctAt1Rate).toBe(0);
});

test("latencyStats of an odd-length sample reads the middle value as p50", () => {
  const stats = latencyStats([100, 500, 300]);

  expect(stats.p50Ms).toBe(300);
});

test("latencyStats of an even-length sample averages the two middle values", () => {
  const stats = latencyStats([100, 200, 300, 400]);

  expect(stats.p50Ms).toBe(250);
});

test("latencyStats counts requests over 5000ms", () => {
  const stats = latencyStats([100, 5001, 6000, 200]);

  expect(stats.over5000ms).toBe(2);
});

test("latencyStats of an empty sample reads every field as 0", () => {
  expect(latencyStats([])).toEqual({ meanMs: 0, p50Ms: 0, maxMs: 0, over5000ms: 0 });
});

test("scoreboard groups results by axis and reports overall alongside them", () => {
  const board = scoreboard([base, { ...base, id: "b01", axis: "B", operationId: "getSalesOrder" }]);

  expect(board.overall.total).toBe(2);
  expect(board.byAxis["A"]?.correctAt1).toBe(1);
  expect(board.byAxis["B"]?.correctAt1).toBe(0);
});

test("scoreboard reads latency across every result regardless of axis", () => {
  const board = scoreboard([
    { ...base, latencyMs: 100 },
    { ...base, id: "a02", latencyMs: 300 },
  ]);

  expect(board.latency.meanMs).toBe(200);
});
