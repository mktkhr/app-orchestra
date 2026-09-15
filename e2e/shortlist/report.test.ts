import { expect, test } from "vite-plus/test";

import {
  RECALL_AT_20_ROW,
  STAND_IN_PICKER_ROW,
  renderErrors,
  renderReferenceRows,
  renderReport,
  renderRun,
  type RunReport,
} from "./report.ts";
import { scoreboard, type QuestionResult } from "./score.ts";

/**
 * report.ts's own tests: fake scoreboards only, no server, no model
 * (docs/plans/shortlisting.md Task 4, Step 5).
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

function fakeRunReport(results: readonly QuestionResult[]): RunReport {
  return { board: scoreboard(results), results };
}

test("renderRun's header states the run label and every axis plus overall", () => {
  const rendered = renderRun("narrowing on", scoreboard([fakeResult()]));

  expect(rendered).toContain("## narrowing on");
  expect(rendered).toContain("overall");
});

test.each([
  { field: "correctAt1", label: "correct@1" },
  { field: "correctAtShown", label: "correct@shown" },
])("renderRun states $label's direction", ({ label }) => {
  const rendered = renderRun("narrowing off", scoreboard([fakeResult()]));

  expect(rendered).toContain(`${label} (higher better)`);
});

test("renderRun reports latency and how many requests exceeded 5000ms", () => {
  const rendered = renderRun("narrowing on", scoreboard([fakeResult({ latencyMs: 6000 })]));

  expect(rendered).toContain("over 5000ms: 1");
});

test("renderReferenceRows carries the stand-in picker's overall 83 and per-axis A100/B100/C72/D53/E70", () => {
  expect(STAND_IN_PICKER_ROW).toEqual({ A: 100, B: 100, C: 72, D: 53, E: 70, overall: 83 });
});

test("renderReferenceRows carries the recall@20 +reranker(w)+written row", () => {
  expect(RECALL_AT_20_ROW).toEqual({ A: 96, B: 96, C: 84, D: 93, E: 100, overall: 93 });
});

test("renderReferenceRows' table names both fixed rows", () => {
  const rendered = renderReferenceRows();

  expect(rendered).toContain("pick:e5-large-q8+reranker");
  expect(rendered).toContain("recall@20");
});

test("renderReport renders both runs and the reference rows together", () => {
  const run = fakeRunReport([fakeResult()]);
  const rendered = renderReport({ on: run, off: run });

  expect(rendered).toContain("## narrowing on");
  expect(rendered).toContain("## narrowing off");
  expect(rendered).toContain("## reference (DECISIONS.md, 2026-09-15)");
});

test("renderRun breaks the over-5000ms count down by axis", () => {
  const rendered = renderRun(
    "narrowing on",
    scoreboard([fakeResult({ axis: "B", latencyMs: 6000 })]),
  );

  expect(rendered).toContain("over 5000ms by axis: A=0 B=1 C=0 D=0 E=0");
});

test("renderErrors lists every error row with its question, latency and message", () => {
  const { operationId: _operationId, ...withoutOperationId } = fakeResult();
  const rendered = renderErrors("narrowing on", [
    {
      ...withoutOperationId,
      id: "b23",
      kind: "error",
      text: "取引先を新規登録したい",
      latencyMs: 56000,
      errorStatus: 500,
      errorMessage: "endpoint not found in catalogue: employee/updateExpenseEmployee",
    },
  ]);

  expect(rendered).toContain("## narrowing on errors (1)");
  expect(rendered).toContain("取引先を新規登録したい");
  expect(rendered).toContain("56000ms");
  expect(rendered).toContain("HTTP 500");
  expect(rendered).toContain("endpoint not found in catalogue");
});

test("renderErrors says so when a run has none", () => {
  const rendered = renderErrors("narrowing off", [fakeResult()]);

  expect(rendered).toContain("(none)");
});
