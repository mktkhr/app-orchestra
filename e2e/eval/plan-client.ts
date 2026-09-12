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

/** Parses an /api/plan response body into the fields a case's expected outcomes can name. */
function parsePlanOutcome(value: unknown): PlanOutcome {
  const record = asRecord(value);
  const kind = record["kind"];

  if (typeof kind !== "string") {
    throw new TypeError(`unexpected /api/plan response shape: ${JSON.stringify(value)}`);
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

/** Posts one question (with, optionally, the conversation before it) against a running platform. */
export async function postPlan(
  baseUrl: string,
  session: Session,
  question: string,
  turns: readonly CaseTurn[] = [],
): Promise<PlanOutcome> {
  const response = await fetch(
    `${baseUrl}/api/plan`,
    withSession(session, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ query: question, turns }),
    }),
  );

  return parsePlanOutcome(await response.json());
}
