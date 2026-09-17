import { expect, test } from "vite-plus/test";

import { scoreMid } from "./score-mid.ts";
import type { QuestionResult } from "./score.ts";

/**
 * score-mid.ts's own tests: fake QuestionResults only, no server, no
 * model (docs/plans/midsizing.md Task 3, AC-M-104).
 */

const CATALOG_IDS = new Set(["listSalesOrders", "getSalesOrder", "createSalesOrder"]);

function answerableRow(overrides: Partial<QuestionResult> = {}): QuestionResult {
  return {
    id: "m01",
    axis: "A",
    text: "受注一覧を見せて",
    answers: ["listSalesOrders"],
    kind: "result",
    operationId: "listSalesOrders",
    latencyMs: 100,
    expect: "answerable",
    ...overrides,
  };
}

function impossibleRow(overrides: Partial<QuestionResult> = {}): QuestionResult {
  return {
    id: "m41",
    axis: "A",
    text: "受注を印刷して",
    answers: [],
    kind: "none",
    latencyMs: 100,
    expect: "impossible",
    ...overrides,
  };
}

test("an answerable result naming its answer is correct@1", () => {
  const board = scoreMid([answerableRow()], CATALOG_IDS);

  expect(board.answerable.correctAt1).toEqual({ total: 1, count: 1 });
});

test("an answerable form naming its answer is correct@1 too", () => {
  const board = scoreMid([answerableRow({ kind: "form" })], CATALOG_IDS);

  expect(board.answerable.correctAt1).toEqual({ total: 1, count: 1 });
});

test("an answerable none is a false refusal, not correct@1", () => {
  const { operationId: _operationId, ...withoutOperationId } = answerableRow();
  const board = scoreMid([{ ...withoutOperationId, kind: "none" }], CATALOG_IDS);

  expect(board.answerable.correctAt1).toEqual({ total: 1, count: 0 });
  expect(board.answerable.falseRefusal).toEqual({ total: 1, count: 1 });
});

test("an answerable list_capabilities is a false refusal", () => {
  const board = scoreMid(
    [answerableRow({ kind: "result", operationId: "list_capabilities" })],
    CATALOG_IDS,
  );

  expect(board.answerable.falseRefusal).toEqual({ total: 1, count: 1 });
});

test("an impossible none is refused", () => {
  const board = scoreMid([impossibleRow()], CATALOG_IDS);

  expect(board.impossible.refused).toEqual({ total: 1, count: 1 });
});

test("an impossible list_capabilities is refused for a capability question", () => {
  const board = scoreMid(
    [impossibleRow({ kind: "result", operationId: "list_capabilities", capability: true })],
    CATALOG_IDS,
  );

  expect(board.impossible.refused).toEqual({ total: 1, count: 1 });
});

test("an impossible list_capabilities is refused for a non-capability question too", () => {
  const board = scoreMid(
    [impossibleRow({ kind: "result", operationId: "list_capabilities" })],
    CATALOG_IDS,
  );

  expect(board.impossible.refused).toEqual({ total: 1, count: 1 });
});

test("an impossible result naming a catalogue operation is forced", () => {
  const board = scoreMid(
    [impossibleRow({ kind: "result", operationId: "listSalesOrders" })],
    CATALOG_IDS,
  );

  expect(board.impossible.forced).toEqual({ total: 1, count: 1 });
});

test("an impossible form naming a catalogue operation is forced too", () => {
  const board = scoreMid(
    [impossibleRow({ kind: "form", operationId: "createSalesOrder" })],
    CATALOG_IDS,
  );

  expect(board.impossible.forced).toEqual({ total: 1, count: 1 });
});

test("an impossible ask is neither refused nor forced", () => {
  const board = scoreMid([impossibleRow({ kind: "ask" })], CATALOG_IDS);

  expect(board.impossible.refused).toEqual({ total: 1, count: 0 });
  expect(board.impossible.forced).toEqual({ total: 1, count: 0 });
});

test("a form whose initial string value appears in the question is not fabricated", () => {
  const row = answerableRow({
    kind: "form",
    operationId: "createSalesOrder",
    text: "取引先を追加、名前は青葉商事",
    answers: ["createSalesOrder"],
    initial: { partnerName: "青葉商事" },
  });
  const board = scoreMid([row], CATALOG_IDS);

  expect(board.forms.fabricated).toEqual({ total: 1, count: 0 });
});

test("a form whose initial string value is absent from the question is fabricated", () => {
  const row = answerableRow({
    kind: "form",
    operationId: "createSalesOrder",
    text: "取引先を追加、名前は青葉商事",
    answers: ["createSalesOrder"],
    initial: { partnerName: "invented商事" },
  });
  const board = scoreMid([row], CATALOG_IDS);

  expect(board.forms.fabricated).toEqual({ total: 1, count: 1 });
});

test("a form's date-shaped initial value is never counted as fabricated", () => {
  const row = answerableRow({
    kind: "form",
    operationId: "createSalesOrder",
    text: "受注を追加して",
    answers: ["createSalesOrder"],
    initial: { dueDate: "2026-09-17" },
  });
  const board = scoreMid([row], CATALOG_IDS);

  expect(board.forms.fabricated).toEqual({ total: 1, count: 0 });
});

test("fabrication is compared case-insensitively for ASCII", () => {
  const row = answerableRow({
    kind: "form",
    operationId: "createSalesOrder",
    text: "SO-0012の内容を変えたい",
    answers: ["createSalesOrder"],
    initial: { id: "so-0012" },
  });
  const board = scoreMid([row], CATALOG_IDS);

  expect(board.forms.fabricated).toEqual({ total: 1, count: 0 });
});

test("a non-form row is never counted toward forms, fabricated or not", () => {
  const board = scoreMid([answerableRow({ kind: "result" })], CATALOG_IDS);

  expect(board.forms.fabricated).toEqual({ total: 0, count: 0 });
});

test("misses list every answerable miss and every forced impossible, nothing else", () => {
  const { operationId: _operationId, ...missedAnswerable } = answerableRow({ id: "m02" });
  const rows = [
    answerableRow({ id: "m01" }),
    { ...missedAnswerable, kind: "none" as const },
    impossibleRow({ id: "m41" }),
    impossibleRow({ id: "m42", kind: "result", operationId: "listSalesOrders" }),
  ];
  const board = scoreMid(rows, CATALOG_IDS);

  expect(board.misses.map((miss) => miss.id)).toEqual(["m02", "m42"]);
});

test("latency is computed across every row regardless of expect", () => {
  const board = scoreMid(
    [answerableRow({ latencyMs: 100 }), impossibleRow({ latencyMs: 300 })],
    CATALOG_IDS,
  );

  expect(board.latency.meanMs).toBe(200);
});
