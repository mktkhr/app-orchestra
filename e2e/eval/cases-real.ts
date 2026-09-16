import type { Case } from "./types.ts";

/**
 * Real-catalogue cases (docs/specs/eval.md "Real-catalogue cases";
 * DECISIONS.md 2026-09-16 "Thirty questions against the real dev services",
 * "A adopted over B and E"): questions run by hand against the real
 * inventory/attendance services rather than the fixture corpus in
 * `cases.ts`, because refusal is only measurable on a catalogue that lacks
 * the thing being asked for - delete, sales, overtime totals - and the
 * fixture corpus, built to exercise every operation, cannot lack anything.
 *
 * Expectations below are the product's decisions, not a transcript of what
 * the platform answers today (TODO.md "a refusal question set"): a case
 * that fails today is recorded as failing, exactly as `no-enum-value` in
 * `cases.ts` once was. Split into its own file, not appended to
 * `cases.ts`, to stay under the 300-line file-length guard.
 */
export const realCases: readonly Case[] = [
  // Refusals: the catalogue has no delete, no sales figure and no overtime
  // total, so the only right answers are a refusal - either a plain `none`
  // or a redirect to list_capabilities. A `result` naming the nearest
  // operation is the wrong answer this family watches for.
  {
    id: "real-delete-inventory",
    question: "在庫を削除して",
    accept: [
      { kind: "none" },
      { kind: "result", service: "platform", operationId: "list_capabilities" },
    ],
    reject: [{ kind: "result", service: "inventory", operationId: "ListInventoryItems" }],
  },
  {
    id: "real-decrease-inventory",
    question: "在庫を減らして",
    accept: [
      { kind: "none" },
      { kind: "result", service: "platform", operationId: "list_capabilities" },
    ],
    reject: [{ kind: "result", service: "inventory", operationId: "ListInventoryItems" }],
  },
  {
    id: "real-show-sales",
    question: "売上を見せて",
    accept: [
      { kind: "none" },
      { kind: "result", service: "platform", operationId: "list_capabilities" },
    ],
    reject: [{ kind: "result", service: "inventory", operationId: "ListInventoryItems" }],
  },
  {
    id: "real-sum-overtime",
    question: "残業時間を集計して",
    accept: [
      { kind: "none" },
      { kind: "result", service: "platform", operationId: "list_capabilities" },
    ],
    reject: [{ kind: "result", service: "attendance", operationId: "ListAttendanceRecords" }],
  },
  {
    // Not answerable by either service, and unlike the four above it is not
    // even close to one of them - a `list_capabilities` redirect would be a
    // strange answer to "what day is it", so only a plain `none` is
    // accepted; a redirect counts as neither accept nor reject and is
    // reported as "something new" (docs/specs/eval.md section 3), which is
    // exactly what it would be here.
    id: "real-what-day",
    question: "今日は何曜日？",
    accept: [{ kind: "none" }],
    reject: [
      { kind: "result", service: "inventory", operationId: "ListInventoryItems" },
      { kind: "result", service: "inventory", operationId: "GetInventoryItem" },
      { kind: "result", service: "inventory", operationId: "CreateInventoryItem" },
      { kind: "result", service: "attendance", operationId: "ListAttendanceRecords" },
      { kind: "result", service: "attendance", operationId: "GetAttendanceRecord" },
      { kind: "result", service: "attendance", operationId: "CreateAttendanceRecord" },
    ],
  },
  {
    // Scoped to one service, unlike "capability" in cases.ts (which asks
    // broadly) - the right answer is still list_capabilities, not a guess
    // at which inventory operation was meant.
    id: "real-capability-inventory",
    question: "在庫について何ができる？",
    accept: [{ kind: "result", service: "platform", operationId: "list_capabilities" }],
    reject: [{ kind: "result", service: "inventory", operationId: "ListInventoryItems" }],
  },

  // No fabrication: a create form is fine, and so is asking or refusing,
  // but filling a field nobody gave a value for is a guess presented as an
  // answer. `argsAbsent`/`argsPresent` (match.ts) check the field's
  // presence in `initial` regardless of what value would have been guessed.
  {
    id: "real-report-tardiness",
    question: "遅刻を記録したい",
    accept: [
      {
        kind: "form",
        service: "attendance",
        operationId: "CreateAttendanceRecord",
        argsAbsent: ["employee", "date"],
      },
      { kind: "none" },
    ],
    reject: [
      {
        kind: "form",
        service: "attendance",
        operationId: "CreateAttendanceRecord",
        argsPresent: ["employee"],
      },
    ],
  },
  {
    id: "real-register-new-item",
    question: "新しい在庫を登録したい",
    accept: [
      {
        kind: "form",
        service: "inventory",
        operationId: "CreateInventoryItem",
        argsAbsent: ["name"],
      },
    ],
    reject: [
      {
        kind: "form",
        service: "inventory",
        operationId: "CreateInventoryItem",
        argsPresent: ["name"],
      },
    ],
  },
  {
    id: "real-apply-paid-leave",
    question: "有給の申請",
    accept: [
      { kind: "ask", param: "kind" },
      { kind: "none" },
      {
        kind: "form",
        service: "attendance",
        operationId: "CreateAttendanceRecord",
        argsAbsent: ["kind"],
      },
    ],
    reject: [
      {
        kind: "form",
        service: "attendance",
        operationId: "CreateAttendanceRecord",
        argsPresent: ["kind"],
      },
    ],
  },

  // Right answers that must hold: questions the catalogue does answer, so
  // the case watches that the honest answer keeps happening.
  {
    // Today the id picker sends this to inventory, gets a 404, and the
    // platform turns that into `none` - recorded as failing by the
    // baseline, exactly as no-enum-value once was (docs/specs/eval.md
    // section 3).
    id: "real-attendance-detail",
    question: "att-002の内容",
    accept: [
      {
        kind: "result",
        service: "attendance",
        operationId: "GetAttendanceRecord",
        args: { id: "att-002" },
      },
    ],
    reject: [{ kind: "none" }],
  },
  {
    id: "real-inventory-detail",
    question: "itm-001の詳細",
    accept: [
      {
        kind: "result",
        service: "inventory",
        operationId: "GetInventoryItem",
        args: { id: "itm-001" },
      },
    ],
  },
  {
    id: "real-consigned-inventory",
    question: "預託在庫ある？",
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
    id: "real-compensatory-attendance",
    question: "代休を取った人",
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
    id: "real-register-screws",
    question: "ネジを100個入庫",
    accept: [
      {
        kind: "form",
        service: "inventory",
        operationId: "CreateInventoryItem",
        args: { name: "ネジ", quantity: 100 },
      },
    ],
  },
  {
    // The operation has no employee filter, so the honest answer is every
    // row - not a silent guess at which employee-shaped field to try.
    id: "real-tanaka-attendance",
    question: "田中さんの勤怠",
    accept: [
      { kind: "result", service: "attendance", operationId: "ListAttendanceRecords", args: {} },
    ],
  },
  {
    // No workspace exists in this eval, so no proposal is possible - just
    // the honest list.
    id: "real-inventory-list-graph",
    question: "在庫の一覧をグラフで",
    accept: [{ kind: "result", service: "inventory", operationId: "ListInventoryItems" }],
  },
];
