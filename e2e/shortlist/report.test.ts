import { expect, test } from "vite-plus/test";

import {
  RECALL_AT_20_ROW,
  STAND_IN_PICKER_ROW,
  renderReferenceRows,
  renderReport,
  renderRun,
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
    answers: ["listInventoryBarcodes"],
    kind: "result",
    operationId: "listInventoryBarcodes",
    latencyMs: 100,
    ...overrides,
  };
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
  const board = scoreboard([fakeResult()]);
  const rendered = renderReport({ on: board, off: board });

  expect(rendered).toContain("## narrowing on");
  expect(rendered).toContain("## narrowing off");
  expect(rendered).toContain("## reference (DECISIONS.md, 2026-09-15)");
});
