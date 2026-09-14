/**
 * `measure()`'s own tests (docs/plans/narrowing.md Task 4, Step 4). A
 * handful of hand-built operations and questions, not the real fixture —
 * these check the recall computation's rules (a tie is not a hit, an answer
 * outside the catalogue is excluded rather than scored zero, several
 * answers means recalled if any one of them is found) against inputs where
 * the right numbers can be worked out by hand.
 */
import { expect, test } from "vite-plus/test";

import type { FixtureOperation } from "./fixture/index.ts";
import { measure, type AxisRecall, type MeasureResult, type RecallAtK } from "./measure.ts";
import type { Axis, Question } from "./corpus/index.ts";

/** A minimal `FixtureOperation`: only `summary` carries text the bigram scorer reads. */
function op(operationId: string, summary: string): FixtureOperation {
  return {
    operationId,
    service: "test",
    serviceDisplayName: "",
    summary,
    description: "",
    displayName: "",
    isSetting: false,
  };
}

function question(id: string, axis: Axis, text: string, answers: readonly string[]): Question {
  return { id, axis, text, answers };
}

function axisRecall(result: MeasureResult, axis: Axis | "overall"): AxisRecall {
  const found = result.axisRecalls.find((entry) => entry.axis === axis);

  if (found === undefined) throw new Error(`no axis recall for ${axis}`);

  return found;
}

function recallAtK(result: MeasureResult, axis: Axis | "overall", k: number): RecallAtK {
  const found = axisRecall(result, axis).recalls.find((entry) => entry.k === k);

  if (found === undefined) throw new Error(`no recall at k=${String(k)} for axis ${axis}`);

  return found;
}

// Two operations share every bigram of "在庫の一覧" exactly (a genuine tie);
// a third shares nothing with any question below.
const CATALOG: readonly FixtureOperation[] = [
  op("listInventoryItems", "在庫の一覧"),
  op("listInventoryLots", "在庫の一覧"),
  op("unrelatedOperation", "無関係な文字列"),
];

test("an answer with no tie is recalled the same way on best and worst", () => {
  const q = question("q1", "A", "在庫の一覧", ["unrelatedOperation"]);
  const catalog = [op("unrelatedOperation", "在庫の一覧"), op("other", "別の文字列")];
  const result = measure(catalog, [q], [10]);
  const recall = recallAtK(result, "A", 10);

  expect(recall.pessimisticHits).toBe(1);
  expect(recall.optimisticHits).toBe(1);
});

test("a tied answer is recalled on best but not on worst, at a K inside the tie", () => {
  const q = question("q1", "A", "在庫の一覧", ["listInventoryLots"]);
  const result = measure(CATALOG, [q], [1]);
  const recall = recallAtK(result, "A", 1);

  // Both listInventoryItems and listInventoryLots score identically against
  // "在庫の一覧"; at K=1 the answer's best rank is 1 (it could win the tie)
  // and its worst rank is 2 (it could lose the tie) — a tie is not a hit.
  expect(recall.optimisticHits).toBe(1);
  expect(recall.pessimisticHits).toBe(0);
});

test("a question is recalled if any of its several answers is found", () => {
  const q = question("q1", "B", "在庫の一覧", ["unrelatedOperation", "listInventoryLots"]);
  const result = measure(CATALOG, [q], [10]);
  const recall = recallAtK(result, "B", 10);

  expect(recall.pessimisticHits).toBe(1);
});

test("a question whose answer is not in the catalogue is excluded, not scored zero", () => {
  const q = question("q1", "A", "在庫の一覧", ["notInThisCatalogue"]);
  const result = measure(CATALOG, [q], [10]);
  const axis = axisRecall(result, "A");

  expect(axis.total).toBe(0);
  expect(axis.excluded).toBe(1);
});

test("a question whose answer shares no bigram with anything is included and scores zero", () => {
  const q = question("q1", "A", "在庫の一覧", ["unrelatedOperation"]);
  const result = measure(CATALOG, [q], [10]);
  const axis = axisRecall(result, "A");
  const recall = recallAtK(result, "A", 10);

  expect(axis.total).toBe(1);
  expect(recall.pessimisticHits).toBe(0);
  expect(recall.optimisticHits).toBe(0);
});

test("overall aggregates every axis's questions", () => {
  const q1 = question("q1", "A", "在庫の一覧", ["listInventoryItems"]);
  const q2 = question("q2", "B", "在庫の一覧", ["listInventoryItems"]);
  const result = measure(CATALOG, [q1, q2], [10]);
  const overall = axisRecall(result, "overall");

  expect(overall.total).toBe(2);
});

test("reports a non-negative per-query wall-clock", () => {
  const q = question("q1", "A", "在庫の一覧", ["listInventoryItems"]);
  const result = measure(CATALOG, [q], [10]);

  expect(result.averageQueryMillis >= 0).toBe(true);
});
