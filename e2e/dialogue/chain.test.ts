import { expect, test } from "vite-plus/test";

import type { PlanOutcome } from "../eval/types.ts";
import { buildTurns, type DialogueStep } from "./chain.ts";

/**
 * chain.ts's own tests: no server, no model - every PlanOutcome here is a
 * fake (mirrors e2e/eval/match.test.ts's style).
 */

test("a result turn carries service/operationId/args from source", () => {
  const outcome: PlanOutcome = {
    kind: "result",
    source: { service: "inventory", operationId: "ListInventoryItems", args: { status: "staged" } },
  };
  const steps: readonly DialogueStep[] = [{ question: "出荷準備完了の在庫を見せて", outcome }];

  expect(buildTurns(steps)).toEqual([
    {
      question: "出荷準備完了の在庫を見せて",
      kind: "result",
      service: "inventory",
      operationId: "ListInventoryItems",
      args: { status: "staged" },
    },
  ]);
});

test("a form turn carries service/operationId from target, not source", () => {
  const outcome: PlanOutcome = {
    kind: "form",
    target: { service: "inventory", operationId: "CreateInventoryItem" },
    initial: { status: "allocated" },
  };
  const steps: readonly DialogueStep[] = [{ question: "在庫を登録したい", outcome }];

  // initial is never sent as args - only what the wire's `target` itself
  // carries (form.args, when the operation names them), same as
  // toContextTurns.ts's own decision = source ?? target.
  expect(buildTurns(steps)).toEqual([
    {
      question: "在庫を登録したい",
      kind: "form",
      service: "inventory",
      operationId: "CreateInventoryItem",
    },
  ]);
});

test("an ask or none turn carries kind alone, no service/operationId/args", () => {
  const ask: PlanOutcome = { kind: "ask", param: "status" };
  const none: PlanOutcome = { kind: "none" };

  expect(buildTurns([{ question: "破損した在庫はある？", outcome: ask }])).toEqual([
    { question: "破損した在庫はある？", kind: "ask" },
  ]);
  expect(buildTurns([{ question: "今日の天気は？", outcome: none }])).toEqual([
    { question: "今日の天気は？", kind: "none" },
  ]);
});

test("chains every prior step in order, oldest first", () => {
  const first: PlanOutcome = {
    kind: "result",
    source: { service: "inventory", operationId: "ListInventoryItems", args: {} },
  };
  const second: PlanOutcome = {
    kind: "result",
    source: {
      service: "inventory",
      operationId: "ListInventoryItems",
      args: { status: "quarantined" },
    },
  };
  const steps: readonly DialogueStep[] = [
    { question: "在庫の一覧を見せて", outcome: first },
    { question: "検品保留だけにして", outcome: second },
  ];

  expect(buildTurns(steps)).toEqual([
    {
      question: "在庫の一覧を見せて",
      kind: "result",
      service: "inventory",
      operationId: "ListInventoryItems",
      args: {},
    },
    {
      question: "検品保留だけにして",
      kind: "result",
      service: "inventory",
      operationId: "ListInventoryItems",
      args: { status: "quarantined" },
    },
  ]);
});

test("an empty steps list builds no turns", () => {
  expect(buildTurns([])).toEqual([]);
});
