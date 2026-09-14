import { expect, test } from "vite-plus/test";

import { catalogOf } from "./fixture/index.ts";
import { buildIndex, narrow } from "./lexical.ts";

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
