import { describe, expect, it } from "vite-plus/test";

import type { Turn } from "./turn";
import { toContextTurns } from "./toContextTurns";

describe("toContextTurns", () => {
  it("returns nothing for no turns", () => {
    expect(toContextTurns([])).toEqual([]);
  });

  it("returns nothing for a single question with no answer yet", () => {
    const turns: Turn[] = [{ id: "1", role: "question", text: "検品保留の在庫を見せて" }];

    expect(toContextTurns(turns)).toEqual([]);
  });

  it("pairs a question with the result answer that followed it, from source", () => {
    const turns: Turn[] = [
      { id: "1", role: "question", text: "検品保留の在庫を見せて" },
      {
        id: "2",
        role: "answer",
        result: {
          kind: "result",
          component: "table",
          data: { items: [] },
          source: {
            service: "inventory",
            operationId: "listInventoryItems",
            args: { status: "quarantined" },
          },
        },
      },
    ];

    expect(toContextTurns(turns)).toEqual([
      {
        question: "検品保留の在庫を見せて",
        kind: "result",
        service: "inventory",
        operationId: "listInventoryItems",
        args: { status: "quarantined" },
      },
    ]);
  });

  it("pairs a question with a form answer's target, not its source", () => {
    const turns: Turn[] = [
      { id: "1", role: "question", text: "在庫を更新して" },
      {
        id: "2",
        role: "answer",
        result: {
          kind: "form",
          schema: {},
          initial: {},
          target: { service: "inventory", operationId: "updateInventoryItem", args: { id: "1" } },
        },
      },
    ];

    expect(toContextTurns(turns)).toEqual([
      {
        question: "在庫を更新して",
        kind: "form",
        service: "inventory",
        operationId: "updateInventoryItem",
        args: { id: "1" },
      },
    ]);
  });

  it("carries no service or operationId for a kind: none answer", () => {
    const turns: Turn[] = [
      { id: "1", role: "question", text: "存在しない質問" },
      { id: "2", role: "answer", result: { kind: "none", message: "結果はありません。" } },
    ];

    expect(toContextTurns(turns)).toEqual([{ question: "存在しない質問", kind: "none" }]);
  });

  it("never carries the answer's data", () => {
    const turns: Turn[] = [
      { id: "1", role: "question", text: "検品保留の在庫を見せて" },
      {
        id: "2",
        role: "answer",
        result: {
          kind: "result",
          component: "table",
          data: { items: [{ id: "itm-001" }] },
          source: { service: "inventory", operationId: "listInventoryItems" },
        },
      },
    ];

    const [turn] = toContextTurns(turns);

    expect(turn).not.toHaveProperty("data");
  });

  it("keeps multiple question/answer pairs oldest first", () => {
    const turns: Turn[] = [
      { id: "1", role: "question", text: "検品保留の在庫を見せて" },
      {
        id: "2",
        role: "answer",
        result: {
          kind: "result",
          component: "table",
          data: {},
          source: { service: "inventory", operationId: "listInventoryItems" },
        },
      },
      { id: "3", role: "question", text: "勤怠でも同じことして" },
      {
        id: "4",
        role: "answer",
        result: {
          kind: "result",
          component: "table",
          data: {},
          source: { service: "attendance", operationId: "listRecords" },
        },
      },
    ];

    expect(toContextTurns(turns)).toEqual([
      {
        question: "検品保留の在庫を見せて",
        kind: "result",
        service: "inventory",
        operationId: "listInventoryItems",
      },
      {
        question: "勤怠でも同じことして",
        kind: "result",
        service: "attendance",
        operationId: "listRecords",
      },
    ]);
  });

  it("drops an answer that follows another answer, with no question of its own", () => {
    const turns: Turn[] = [
      { id: "1", role: "question", text: "在庫を更新して" },
      {
        id: "2",
        role: "answer",
        result: {
          kind: "form",
          schema: {},
          initial: {},
          target: { service: "inventory", operationId: "updateInventoryItem" },
        },
      },
      {
        id: "3",
        role: "answer",
        result: {
          kind: "result",
          component: "detail",
          data: { id: "1" },
          source: { service: "inventory", operationId: "updateInventoryItem" },
        },
      },
    ];

    expect(toContextTurns(turns)).toEqual([
      {
        question: "在庫を更新して",
        kind: "form",
        service: "inventory",
        operationId: "updateInventoryItem",
      },
    ]);
  });
});
