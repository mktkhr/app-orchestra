import { expect, test } from "vite-plus/test";

import { catalogOf } from "./fixture/index.ts";
import { buildIndex, narrow, rankRangeOf } from "./lexical.ts";

/**
 * The lexical baseline's own tests (docs/plans/narrowing.md Task 3). They
 * assert the properties Task 4 will lean on — the index is stateless and
 * fast — rather than a recall number, which is Task 4's measurement to make.
 */

const CATALOG_1000 = catalogOf(5);
const INDEX_1000 = buildIndex(CATALOG_1000);

/** Every `listXOne覧` operation id, for the "beats every other list" check. */
function everyListOperationId(): readonly string[] {
  return CATALOG_1000.filter((op) => op.summary.endsWith("の一覧")).map((op) => op.operationId);
}

/** The top result for a query, run against a freshly built index. */
function topResultFor(question: string): string {
  return narrow(buildIndex(CATALOG_1000), question, 1)[0]?.operationId ?? "";
}

/** Milliseconds `narrow` takes to score the whole 1000-operation catalogue. */
function scoringDurationMs(question: string): number {
  const start = performance.now();

  narrow(INDEX_1000, question, 1000);

  return performance.now() - start;
}

/** A snapshot of an index's contents, to check a run of queries left it untouched. */
function indexSnapshot(index: ReturnType<typeof buildIndex>): readonly [string, number][] {
  return [...index.entries()].map(([id, entry]) => [id, entry.bigrams.size]);
}

test("an exact noun match out-ranks a shared verb", () => {
  const winner = topResultFor("在庫ロット");
  const otherListOperations = everyListOperationId().filter((id) => id !== winner);

  expect(winner).toBe("listInventoryLots");
  expect(otherListOperations.includes("listInventoryLots")).toBe(false);
});

test("scoring 1000 operations takes under 5ms", () => {
  // Warm up once so JIT compilation is not what gets measured.
  scoringDurationMs("在庫ロットを見せて");

  expect(scoringDurationMs("在庫ロットを見せて")).toBeLessThan(5);
});

test("the index is built once and holds no state between queries", () => {
  const before = indexSnapshot(INDEX_1000);

  narrow(INDEX_1000, "在庫ロットを見せて", 10);
  narrow(INDEX_1000, "注文を一覧", 10);
  narrow(INDEX_1000, "先月の残業時間", 10);

  expect(indexSnapshot(INDEX_1000)).toEqual(before);
});

/** How many candidates `narrow` offers for a question, asking for ten. */
function candidateCountFor(question: string): number {
  return narrow(INDEX_1000, question, 10).length;
}

/** The rank range of one operation, as a plain pair, or [0, 0] when absent. */
function rankPairFor(question: string, operationId: string): readonly [number, number] {
  const range = rankRangeOf(INDEX_1000, question, operationId);

  return range === undefined ? [0, 0] : [range.best, range.worst];
}

test("an operation sharing no bigram with the question is not a candidate", () => {
  // 休みたい is an axis-D question (docs/specs/narrowing.md section 4): the
  // fixture writes 有給休暇, never 休みたい. The baseline cannot answer it, and
  // says so by returning nothing - rather than ten operations picked by
  // whatever order the catalogue happens to be in.
  expect(candidateCountFor("休みたい")).toBe(0);
});

test("a question the fixture does write still returns candidates", () => {
  expect(candidateCountFor("在庫ロットを見せて")).toBe(10);
});

test("a rank range reports the tie it sits inside", () => {
  // Axis B: 注文 is 受注 in sales and 発注 in purchasing, and both display as
  // 注文一覧. Neither can be ranked ahead of the other, and the range says so.
  expect(rankPairFor("注文を一覧", "listSalesOrders")).toEqual([1, 2]);
  expect(rankPairFor("注文を一覧", "listPurchasingOrders")).toEqual([1, 2]);
});

test("an absent operation has no rank range", () => {
  expect(rankPairFor("休みたい", "listAttendancePaidLeaves")).toEqual([0, 0]);
});
