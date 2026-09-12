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
    // Judged on reject, not accept (docs/specs/eval.md section 4;
    // DECISIONS.md 2026-09-12 "no-enum-value judged on reject"): whether the
    // model asks or guesses the enum value is not what this case watches,
    // and that split is free to swing. What must not increase is the silent
    // drop this case's reject outcome names.
    metric: "reject",
    // Run at 30, not the corpus default of 10 (measured, DECISIONS.md): at
    // n=10 the reject rate itself swung as widely as accept did (5-9/10
    // across six samples), which is not narrow enough to tell noise from a
    // real regression at any tolerance worth setting. At n=30 three samples
    // held to 16-19/30 (0.53-0.63), a band under half as wide.
    runs: 30,
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
  {
    id: "filter-by-label-allocated",
    question: "引当済の在庫を見せて",
    accept: [
      {
        kind: "result",
        service: "inventory",
        operationId: "ListInventoryItems",
        args: { status: "allocated" },
      },
    ],
  },
  {
    id: "filter-by-label-staged",
    question: "出荷準備完了の在庫を見せて",
    accept: [
      {
        kind: "result",
        service: "inventory",
        operationId: "ListInventoryItems",
        args: { status: "staged" },
      },
    ],
  },
  {
    id: "filter-by-label-consigned",
    question: "預託在庫を見せて",
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
    id: "filter-by-label-deemed",
    question: "みなし労働の勤怠を見せて",
    accept: [
      {
        kind: "result",
        service: "attendance",
        operationId: "ListAttendanceRecords",
        args: { kind: "deemed" },
      },
    ],
  },
  {
    id: "filter-by-label-substitute",
    question: "振替休日の勤怠を見せて",
    accept: [
      {
        kind: "result",
        service: "attendance",
        operationId: "ListAttendanceRecords",
        args: { kind: "substitute" },
      },
    ],
  },
  {
    id: "filter-by-label-compensatory",
    question: "代休の勤怠を見せて",
    accept: [
      {
        kind: "result",
        service: "attendance",
        operationId: "ListAttendanceRecords",
        args: { kind: "compensatory" },
      },
    ],
  },
  {
    id: "filter-by-label-on-call",
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
    id: "list-everything-attendance",
    question: "勤怠を全部見せて",
    accept: [
      { kind: "result", service: "attendance", operationId: "ListAttendanceRecords", args: {} },
    ],
  },
  {
    id: "create-attendance",
    question: "勤怠を登録して。従業員は山田太郎、種別は振替休日、対象日は2026-09-15で",
    accept: [
      {
        kind: "form",
        service: "attendance",
        operationId: "CreateAttendanceRecord",
        args: { employee: "山田太郎", kind: "substitute", date: "2026-09-15" },
      },
    ],
  },
  {
    id: "follow-up-other-service",
    question: "勤怠でも同じことして",
    turns: [
      {
        question: "在庫を全部見せて",
        kind: "result",
        service: "inventory",
        operationId: "ListInventoryItems",
        args: {},
      },
    ],
    accept: [
      { kind: "result", service: "attendance", operationId: "ListAttendanceRecords", args: {} },
    ],
  },
  {
    id: "no-enum-value-attendance",
    question: "有給の勤怠はある？",
    metric: "reject",
    accept: [
      { kind: "ask", param: "kind" },
      {
        kind: "result",
        service: "attendance",
        operationId: "ListAttendanceRecords",
        args: { kind: "deemed" },
      },
      {
        kind: "result",
        service: "attendance",
        operationId: "ListAttendanceRecords",
        args: { kind: "substitute" },
      },
      {
        kind: "result",
        service: "attendance",
        operationId: "ListAttendanceRecords",
        args: { kind: "compensatory" },
      },
      {
        kind: "result",
        service: "attendance",
        operationId: "ListAttendanceRecords",
        args: { kind: "on_call" },
      },
    ],
    reject: [
      { kind: "result", service: "attendance", operationId: "ListAttendanceRecords", args: {} },
    ],
  },
];
