import type { Case } from "./types.ts";

/**
 * The eval corpus (docs/specs/eval.md, AC-E-205): at least a filter named by
 * its label, a filter named by a word the enum does not have, a question
 * asking for everything, a create, a question no service answers, a
 * capability question, and a follow-up naming no service.
 *
 * Operation ids are the ones the running services actually serve
 * (`GET /openapi.yaml`), not the ones `services/*\/api/openapi.yaml` writes on
 * disk: oapi-codegen's embedded spec renames every operationId to the Go
 * identifier it generated from it (`listInventoryItems` -> `ListInventoryItems`),
 * and the platform's catalogue is built from what a service serves, never
 * from the source file - the same reason `e2e/src/*.test.ts` already write
 * `ListInventoryItems`/`ListAttendanceRecords` rather than the lower-cased
 * spelling `services/inventory/api/openapi.yaml` declares.
 */
export const cases: readonly Case[] = [
  {
    id: "filter-by-label",
    question: "検品保留の在庫を見せて",
    accept: [
      {
        kind: "result",
        service: "inventory",
        operationId: "ListInventoryItems",
        args: { status: "quarantined" },
      },
    ],
  },
  {
    // The case this suite exists for: qwen3.5-9b-q8 was measured by hand
    // (DECISIONS.md, 2026-09-11) to sometimes drop the filter silently and
    // return every row instead of asking or guessing - the reject outcome
    // below is exactly that silent drop.
    id: "no-enum-value",
    question: "破損した在庫はある？",
    accept: [
      { kind: "ask", param: "status" },
      {
        kind: "result",
        service: "inventory",
        operationId: "ListInventoryItems",
        args: { status: "quarantined" },
      },
    ],
    reject: [{ kind: "result", service: "inventory", operationId: "ListInventoryItems", args: {} }],
  },
  {
    id: "list-everything",
    question: "在庫を全部見せて",
    accept: [{ kind: "result", service: "inventory", operationId: "ListInventoryItems", args: {} }],
  },
  {
    id: "create",
    question: "在庫を登録して。名前はテスト品、数量は5、引当済で",
    accept: [
      {
        kind: "form",
        service: "inventory",
        operationId: "CreateInventoryItem",
        args: { name: "テスト品", quantity: 5, status: "allocated" },
      },
    ],
  },
  {
    id: "unanswerable",
    question: "今日の天気は？",
    accept: [{ kind: "none" }],
  },
  {
    id: "capability",
    question: "何ができるの？",
    accept: [{ kind: "result", service: "platform", operationId: "list_capabilities" }],
  },
  {
    id: "follow-up-stays",
    question: "検品保留のものだけ見せて",
    turns: [
      {
        question: "在庫の一覧を見せて",
        kind: "result",
        service: "inventory",
        operationId: "ListInventoryItems",
        args: {},
      },
    ],
    accept: [
      {
        kind: "result",
        service: "inventory",
        operationId: "ListInventoryItems",
        args: { status: "quarantined" },
      },
    ],
  },
];
