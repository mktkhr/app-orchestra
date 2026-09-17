/**
 * Pure scoring for the mid measurement (docs/plans/midsizing.md Task 3;
 * docs/specs/midsizing.md M4, section 5, AC-M-104). No I/O here -
 * `run-mid.ts` gathers `QuestionResult`s from a running platform against
 * `midQuestions()`; this file only turns those results into the five
 * numbers `make eval-mid` prints, so it is what `score-mid.test.ts`
 * exercises with fakes and what `make check` runs without a server or a
 * model.
 *
 * `scoreMid`'s rules, verbatim from the plan:
 *
 * - An answerable row is correct@1 iff `kind ∈ {result, form}` and its
 *   `operationId` is one of the question's `answers`.
 * - A false refusal (answerable only) is `kind == none`, or
 *   `operationId == list_capabilities`.
 * - An impossible row is refused iff `kind == none`, or
 *   `operationId == list_capabilities` - for any impossible question, not
 *   only a `capability` one: a capability question's `list_capabilities`
 *   is the right answer, a non-capability one's is still a refusal.
 * - An impossible row is forced iff `kind ∈ {result, form}` and its
 *   `operationId` is one of the thirty catalogue operations.
 * - A form (any row, either half) is fabricated when any of its
 *   `initial` string values - not date-shaped `\d{4}-\d{2}-\d{2}`, not an
 *   enum member of the target operation's own request body schema - is
 *   absent from the question text, compared case-insensitively for ASCII.
 *   (The enum exclusion was missing from the first `make eval-mid` run,
 *   Task 3 review: 8 of 11 counted "fabrications" were `status: "active"`
 *   / `"pending"`, `openapi-document.ts`'s shared `FixtureStatus` enum on
 *   every create/update operation's request body - not invented strings.)
 */
import type { MidOperationInfo } from "../narrowing/fixture/index.ts";
import { latencyStats, type Kind, type LatencyStats, type QuestionResult } from "./score.ts";

/** A date-shaped `initial` value (`YYYY-MM-DD`) is never counted as fabricated - the plan excludes it by construction, not by comparing it against the question text. */
const DATE_SHAPED = /^\d{4}-\d{2}-\d{2}$/u;

/** One counted-and-total pair, e.g. `{ total: 40, count: 32 }` for 32/40 correct@1. */
export interface CountOf {
  readonly total: number;
  readonly count: number;
}

/** `operationId -> its own enum values`, so the fabrication check can tell a schema-declared enum member from an invented string - `fixture/mid.ts`'s `midOperationInfo()` builds this from `midCatalog()`'s own operations. */
export type MidCatalogue = ReadonlyMap<string, MidOperationInfo>;

export interface MidScoreboard {
  readonly answerable: { readonly correctAt1: CountOf; readonly falseRefusal: CountOf };
  readonly impossible: { readonly refused: CountOf; readonly forced: CountOf };
  readonly forms: { readonly fabricated: CountOf };
  readonly latency: LatencyStats;
  readonly misses: readonly MidMiss[];
}

/** One row worth quoting in the report's misses list - an answerable miss, or a forced impossible. */
export interface MidMiss {
  readonly id: string;
  readonly text: string;
  readonly kind: Kind;
  readonly operationId?: string;
}

/** A row the platform actually picked from: a `result` (it ran the operation) or a `form` (named the operation without running it). */
function isPick(row: QuestionResult): boolean {
  return row.kind === "result" || row.kind === "form";
}

/** True for a row the platform refused: a plain `none`, or a `list_capabilities` pick - the same test for a false refusal (answerable) and a correct refusal (impossible), read oppositely by the caller. */
function isRefusal(row: QuestionResult): boolean {
  return row.kind === "none" || row.operationId === "list_capabilities";
}

/** correct@1 for an answerable row: a pick naming one of its answers. */
function isCorrectAt1(row: QuestionResult): boolean {
  return isPick(row) && row.answers.includes(row.operationId ?? "");
}

/** forced for an impossible row: a pick naming one of the thirty catalogue operations. */
function isForced(row: QuestionResult, catalogue: MidCatalogue): boolean {
  return isPick(row) && row.operationId !== undefined && catalogue.has(row.operationId);
}

/** True when value, lowercased, appears in text, lowercased - ASCII case-insensitivity; non-ASCII text is compared as-is either side. */
function appearsIn(text: string, value: string): boolean {
  return text.toLowerCase().includes(value.toLowerCase());
}

/** True when value is one of the target operation's own request body enum values - a row with no operationId, or one the catalogue does not know, has none. */
function isEnumValue(row: QuestionResult, catalogue: MidCatalogue, value: string): boolean {
  const info = row.operationId === undefined ? undefined : catalogue.get(row.operationId);

  return info !== undefined && info.enumValues.has(value);
}

/** A `form` row is fabricated when any of its `initial` string values is neither date-shaped, nor an enum member of the operation it targets, nor present in its own question text. */
function isFabricated(row: QuestionResult, catalogue: MidCatalogue): boolean {
  const values = Object.values(row.initial ?? {});

  return values.some(
    (value) =>
      !DATE_SHAPED.test(value) &&
      !isEnumValue(row, catalogue, value) &&
      !appearsIn(row.text, value),
  );
}

function countOf(
  rows: readonly QuestionResult[],
  predicate: (row: QuestionResult) => boolean,
): CountOf {
  return { total: rows.length, count: rows.filter((row) => predicate(row)).length };
}

function toMiss(row: QuestionResult): MidMiss {
  return {
    id: row.id,
    text: row.text,
    kind: row.kind,
    ...(row.operationId !== undefined && { operationId: row.operationId }),
  };
}

/** Scores a full mid run (any mix of answerable/impossible rows) into the five numbers plus latency and a misses list, given the thirty catalogue operations and their own enum values (`fixture/mid.ts`'s `midOperationInfo()`). */
export function scoreMid(rows: readonly QuestionResult[], catalogue: MidCatalogue): MidScoreboard {
  const answerableRows = rows.filter((row) => row.expect === "answerable");
  const impossibleRows = rows.filter((row) => row.expect === "impossible");
  const formRows = rows.filter((row) => row.kind === "form");

  const answerableMisses = answerableRows.filter((row) => !isCorrectAt1(row));
  const forcedImpossibles = impossibleRows.filter((row) => isForced(row, catalogue));

  return {
    answerable: {
      correctAt1: countOf(answerableRows, isCorrectAt1),
      falseRefusal: countOf(answerableRows, isRefusal),
    },
    impossible: {
      refused: countOf(impossibleRows, isRefusal),
      forced: countOf(impossibleRows, (row) => isForced(row, catalogue)),
    },
    forms: { fabricated: countOf(formRows, (row) => isFabricated(row, catalogue)) },
    latency: latencyStats(rows.map((row) => row.latencyMs)),
    misses: [...answerableMisses, ...forcedImpossibles].map((row) => toMiss(row)),
  };
}
