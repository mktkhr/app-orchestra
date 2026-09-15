import { expect, test } from "vite-plus/test";

import { renderMisses } from "./misses.ts";
import type { QuestionResult } from "./score.ts";

/**
 * misses.ts's own tests: fake results only, no server, no model
 * (docs/plans/wording.md Task 2, Step 2). renderMisses is pure - the
 * file-writing half (writeMisses) is disk I/O run.ts calls after a live
 * pass, not covered here.
 */

function fakeResult(overrides: Partial<QuestionResult> = {}): QuestionResult {
  return {
    id: "a01",
    axis: "A",
    text: "バーコードの発行状況を見せて",
    answers: ["listInventoryBarcodes"],
    kind: "result",
    operationId: "listInventoryBarcodes",
    latencyMs: 100,
    ...overrides,
  };
}

test("renderMisses omits a correct@1 row entirely", () => {
  const rendered = renderMisses("v1", [fakeResult()]);

  expect(rendered).toContain("(0 of 1)");
});

test("renderMisses lists a wrong pick's id, question, kind, pick, alternatives and answers", () => {
  const rendered = renderMisses("v2-commit", [
    fakeResult({
      id: "b12",
      text: "取引先を新規登録したい",
      answers: ["createSalesCustomer"],
      operationId: "listSalesCustomers",
      alternatives: ["listSalesCustomers", "getSalesCustomer"],
    }),
  ]);

  expect(rendered).toContain("b12");
  expect(rendered).toContain("取引先を新規登録したい");
  expect(rendered).toContain("kind=result");
  expect(rendered).toContain("pick=listSalesCustomers");
  expect(rendered).toContain("alternatives=listSalesCustomers, getSalesCustomer");
  expect(rendered).toContain("answers=createSalesCustomer");
});

test("renderMisses groups misses under their own axis heading", () => {
  const rendered = renderMisses("v1", [fakeResult({ id: "b01", axis: "B", answers: ["other"] })]);

  expect(rendered).toContain("## axis B (1)");
  expect(rendered).toContain("## axis A\n(none)");
});

test("renderMisses' footer counts misses by kind", () => {
  const rendered = renderMisses("v1", [
    fakeResult({ id: "a02", kind: "none", answers: ["other"] }),
    fakeResult({ id: "a03", kind: "ask", answers: ["other"] }),
  ]);

  expect(rendered).toContain("kind counts: result=0 ask=1 none=1 form=0 proposal=0 error=0");
});

test("renderMisses' footer names the ids where list_capabilities was picked", () => {
  const rendered = renderMisses("v1", [
    fakeResult({ id: "a04", operationId: "list_capabilities", answers: ["other"] }),
  ]);

  expect(rendered).toContain("list_capabilities picked for: a04");
});

test("renderMisses' footer says none when list_capabilities was never picked", () => {
  const rendered = renderMisses("v1", [fakeResult({ answers: ["other"] })]);

  expect(rendered).toContain("list_capabilities picked for: (none)");
});
