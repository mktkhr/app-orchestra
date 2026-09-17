import type { Dialogue } from "./types.ts";

/** d07-d12 of the dialogue corpus - see dialogues.ts's own doc comment for why this is split out. */
export const moreDialogues: readonly Dialogue[] = [
  {
    id: "d07",
    turns: [
      {
        question: "在庫の一覧",
        accept: [
          { kind: "result", service: "inventory", operationId: "ListInventoryItems", args: {} },
        ],
      },
      {
        question: "勤怠の一覧も",
        accept: [
          { kind: "result", service: "attendance", operationId: "ListAttendanceRecords", args: {} },
        ],
      },
      {
        question: "さっきの在庫で出荷準備完了のものは？",
        accept: [
          {
            kind: "result",
            service: "inventory",
            operationId: "ListInventoryItems",
            args: { status: "staged" },
          },
        ],
      },
    ],
  },
  {
    id: "d08",
    turns: [
      {
        question: "在庫の一覧",
        accept: [
          { kind: "result", service: "inventory", operationId: "ListInventoryItems", args: {} },
        ],
      },
      {
        question: "今日の天気は？",
        accept: [{ kind: "none" }],
      },
    ],
  },
  {
    id: "d09",
    turns: [
      {
        question: "勤怠の一覧",
        accept: [
          { kind: "result", service: "attendance", operationId: "ListAttendanceRecords", args: {} },
        ],
      },
      {
        question: "在庫を登録したい",
        accept: [{ kind: "form", service: "inventory", operationId: "CreateInventoryItem" }],
      },
    ],
  },
  {
    id: "d10",
    turns: [
      {
        question: "待機の勤怠を見せて",
        accept: [
          {
            kind: "result",
            service: "attendance",
            operationId: "ListAttendanceRecords",
            args: { kind: "on_call" },
          },
        ],
      },
      {
        question: "この人たちの記録を追加したい",
        accept: [
          {
            kind: "form",
            service: "attendance",
            operationId: "CreateAttendanceRecord",
            argsAbsent: ["employee"],
          },
        ],
      },
    ],
  },
  {
    id: "d11",
    turns: [
      {
        question: "在庫の一覧",
        accept: [
          { kind: "result", service: "inventory", operationId: "ListInventoryItems", args: {} },
        ],
      },
      {
        // "詳細を見せて" names no item: GetInventoryItem's own `id` has no
        // default to fill in and no enum to guess from (unlike a status
        // filter), so either shape the planner can honestly reach - a
        // confirm-before-detail form naming the operation, or asking for
        // the id outright (its own `param`, `services/inventory/api/openapi.yaml`'s
        // `getInventoryItem` names it "id") - counts, the same "any of
        // these" accept list an eval case's own ask/guess split uses
        // (e.g. `no-enum-value`, e2e/eval/cases.ts).
        question: "詳細を見せて",
        accept: [
          { kind: "form", service: "inventory", operationId: "GetInventoryItem" },
          { kind: "ask", param: "id" },
        ],
      },
    ],
  },
  {
    id: "d12",
    turns: [
      {
        question: "預託在庫はある？",
        accept: [
          {
            kind: "result",
            service: "inventory",
            operationId: "ListInventoryItems",
            args: { status: "consigned" },
          },
        ],
      },
      {
        question: "勤怠で待機のものは？",
        accept: [
          {
            kind: "result",
            service: "attendance",
            operationId: "ListAttendanceRecords",
            args: { kind: "on_call" },
          },
        ],
      },
    ],
  },
];
