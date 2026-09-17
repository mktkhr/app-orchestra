import type { Session } from "../src/helpers/auth.ts";
import { withSession } from "../src/helpers/auth.ts";
import { asRecord, isRecord } from "../src/helpers/wire.ts";
import type { CaseTurn, PlanOutcome } from "./types.ts";

/** value narrowed to a Record<string, unknown>, or undefined when it was not one. */
function asRecordOrUndefined(value: unknown): Record<string, unknown> | undefined {
  return isRecord(value) ? value : undefined;
}

/** value narrowed to a string, or undefined when it was not one. */
function asStringOrUndefined(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

/**
 * One of PlanOutcome's Source-shaped fields (source/target), read off an
 * unknown JSON value. Built with conditional spreads rather than assigning
 * `service`/`operationId`/`args` straight from the maybe-undefined readers:
 * `exactOptionalPropertyTypes` (tsconfig.base.json) treats an optional
 * property assigned `undefined` differently from one left out entirely, and
 * PlanOutcome's fields mean the latter - "the wire response did not carry
 * this", not "it carried an explicit null".
 */
function asSource(value: unknown): PlanOutcome["source"] {
  const record = asRecordOrUndefined(value);

  if (record === undefined) return undefined;

  const service = asStringOrUndefined(record["service"]);
  const operationId = asStringOrUndefined(record["operationId"]);
  const args = asRecordOrUndefined(record["args"]);

  return {
    ...(service !== undefined && { service }),
    ...(operationId !== undefined && { operationId }),
    ...(args !== undefined && { args }),
  };
}

/**
 * Parses an /api/plan response body into the fields a case's expected
 * outcomes can name.
 *
 * A body with no `kind` is a body the platform refused to answer with: an
 * error, because the model named an operation the catalogue does not have,
 * or an argument outside a parameter's enum, or anything else the request
 * never got past. That is a run, and `docs/specs/eval.md` section 3 already
 * says what to do with one: "a run that matches neither is counted apart -
 * it is the model doing something new, and that is worth seeing on its
 * own."
 *
 * It used to throw, which ended the whole suite on the first such run. Two
 * models were scored "could not be measured" that way, each after one bad
 * answer inside case two of eighteen - lfm25-8b-a1b-q8 invented an
 * operation, spark-x25-4b-q8 invented an enum value - and sixteen cases
 * that would have said what else those models can and cannot do were never
 * run (`DECISIONS.md`, 2026-09-14). A comparison that stops at the first
 * mistake compares nothing.
 *
 * `kind: "error"` matches no case's `accept` or `reject`, since no case
 * names it, so such a run lands in the counted-apart column by
 * construction rather than by a second rule.
 */
function parsePlanOutcome(value: unknown): PlanOutcome {
  const record = asRecord(value);
  const kind = record["kind"];

  if (typeof kind !== "string") {
    return { kind: "error" };
  }

  const source = asSource(record["source"]);
  const target = asSource(record["target"]);
  const initial = asRecordOrUndefined(record["initial"]);
  const param = asStringOrUndefined(record["param"]);

  return {
    kind,
    ...(source !== undefined && { source }),
    ...(target !== undefined && { target }),
    ...(initial !== undefined && { initial }),
    ...(param !== undefined && { param }),
  };
}

/** postPlanRaw's result: the outcome eval's own cases match against, plus the response body it was parsed from. */
export interface PlanResponse {
  readonly outcome: PlanOutcome;
  readonly raw: unknown;
}

/**
 * Posts one question (with, optionally, the conversation before it)
 * against a running platform, returning both the parsed outcome and the
 * raw response body. `postPlan` (below) is this with only `outcome` kept -
 * eval's own cases never needed the raw body, `e2e/dialogue`'s output rows
 * do (they record the exact request/response of every turn).
 */
export async function postPlanRaw(
  baseUrl: string,
  session: Session,
  question: string,
  turns: readonly CaseTurn[] = [],
): Promise<PlanResponse> {
  const response = await fetch(
    `${baseUrl}/api/plan`,
    withSession(session, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ query: question, turns }),
    }),
  );

  const raw: unknown = await response.json();

  return { outcome: parsePlanOutcome(raw), raw };
}

/** Posts one question (with, optionally, the conversation before it) against a running platform. */
export async function postPlan(
  baseUrl: string,
  session: Session,
  question: string,
  turns: readonly CaseTurn[] = [],
): Promise<PlanOutcome> {
  return (await postPlanRaw(baseUrl, session, question, turns)).outcome;
}
