import { moreDialogues } from "./dialogues-more.ts";
import type { Dialogue } from "./types.ts";

/**
 * The dialogue instrument's corpus: twelve conversations, twenty-seven
 * turns, run once each against the real dummy services and the real
 * planner (run.ts). Every turn's `question` is what a person would type
 * next; `accept` is what would answer it, matched with `matchesAny`
 * (`e2e/eval/match.ts`) the same way an eval case's `accept` is.
 *
 * Operation ids and enum values are the ones the running services actually
 * serve, the same reason `e2e/eval/cases.ts`'s own doc comment gives.
 *
 * `dialogues-more.ts` holds d07-d12, appended at the end - split out the
 * same way `e2e/eval/cases.ts` appends `cases-real.ts`, to stay under the
 * 300-line guard (harness/quality/file-length.txt).
 */
export const dialogues: readonly Dialogue[] = [
  {
    id: "d01",
    turns: [
      {
        question: "在庫の一覧を見せて",
        accept: [
          { kind: "result", service: "inventory", operationId: "ListInventoryItems", args: {} },
        ],
      },
      {
        question: "検品保留だけにして",
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
        question: "勤怠の方も見せて",
        accept: [
          {
            kind: "result",
            service: "attendance",
            operationId: "ListAttendanceRecords",
            argsAbsent: ["kind"],
          },
        ],
      },
    ],
  },
  {
    id: "d02",
    turns: [
      {
        question: "在庫を全部見せて",
        accept: [
          { kind: "result", service: "inventory", operationId: "ListInventoryItems", args: {} },
        ],
      },
      {
        question: "勤怠でも同じことして",
        accept: [
          { kind: "result", service: "attendance", operationId: "ListAttendanceRecords", args: {} },
        ],
      },
    ],
  },
  {
    id: "d03",
    turns: [
      {
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
        question: "att-002は？",
        accept: [
          {
            kind: "result",
            service: "attendance",
            operationId: "GetAttendanceRecord",
            args: { id: "att-002" },
          },
        ],
      },
    ],
  },
  {
    id: "d04",
    turns: [
      {
        question: "att-003を見せて",
        accept: [
          {
            kind: "result",
            service: "attendance",
            operationId: "GetAttendanceRecord",
            args: { id: "att-003" },
          },
        ],
      },
      {
        question: "itm-004は？",
        accept: [
          {
            kind: "result",
            service: "inventory",
            operationId: "GetInventoryItem",
            args: { id: "itm-004" },
          },
        ],
      },
    ],
  },
  {
    id: "d05",
    turns: [
      {
        question: "勤怠記録の一覧",
        accept: [
          { kind: "result", service: "attendance", operationId: "ListAttendanceRecords", args: {} },
        ],
      },
      {
        question: "代休のものだけ",
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
        question: "振替休日は？",
        accept: [
          {
            kind: "result",
            service: "attendance",
            operationId: "ListAttendanceRecords",
            args: { kind: "substitute" },
          },
        ],
      },
    ],
  },
  {
    id: "d06",
    turns: [
      {
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
        question: "やっぱり全部",
        accept: [
          { kind: "result", service: "inventory", operationId: "ListInventoryItems", args: {} },
        ],
      },
    ],
  },
  ...moreDialogues,
];
