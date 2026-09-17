import { realCases } from "./cases-real.ts";
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
 *
 * `cases-real.ts`'s cases are appended at the end: real-catalogue questions
 * run against the actual dev services rather than a fixture built to
 * exercise every operation (docs/specs/eval.md "Real-catalogue cases"),
 * split into its own file to stay under the 300-line guard.
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
    //
    // accept narrowed to ask alone (2026-09-17, restoring AC-B-105's own
    // "returns kind: ask" under two-stage planning): a guessed
    // status:quarantined is no longer an accepted outcome at all - the
    // enum-guess guard (services/platform/internal/usecase/orchestrator_enum_guess.go)
    // turns every such guess into this same ask before it ever reaches a
    // result, so the wide "asked or guessed" band this case's own doc
    // comment used to describe is gone: this is deterministic 30/30 now,
    // not noise the metric had to route around.
    runs: 30,
    accept: [{ kind: "ask", param: "status" }],
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
    // The inventory case's twin on the other service, and the harder one:
    // 有給 has no near neighbour among みなし労働 / 振替休日 / 代休 / 待機
    // for the model to guess at, so it drops the filter rather than
    // guessing - measured at 7-9/10 reject every time it has been run.
    //
    // Judged on reject like its twin, but be clear about what that buys:
    // with the baseline already at 0.7 and ORCHESTRA_EVAL_TOLERANCE at 0.3,
    // a regression would need a reject rate above 1.0, so this case cannot
    // fail a run. It is not a check; it is a number printed on every run
    // (AC-E-203 prints reject counts whether or not the judged rate held),
    // and a person reading 7/10 there is reading that the defect is still
    // live.
    //
    // accept narrowed to ask alone (2026-09-17, restoring AC-B-105 under
    // two-stage planning): a guessed kind - compensatory, most often
    // measured - is no longer accepted as a result at all, the same
    // tightening as no-enum-value's own twin above. The enum-guess guard
    // turns any such guess into this ask deterministically, so the four
    // guessed-result entries this accept list used to carry - the wide
    // "asked or guessed" set docs/specs/eval.md section 4 called noise -
    // are gone; reject is still the metric this case is judged on, for the
    // same reason as before.
    id: "no-enum-value-attendance",
    question: "有給の勤怠はある？",
    metric: "reject",
    accept: [{ kind: "ask", param: "kind" }],
    reject: [
      { kind: "result", service: "attendance", operationId: "ListAttendanceRecords", args: {} },
    ],
  },
  ...realCases,
];
