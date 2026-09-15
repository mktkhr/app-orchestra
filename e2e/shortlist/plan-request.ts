import { withSession, type Session } from "../src/helpers/auth.ts";
import { asRecord, isRecord } from "../src/helpers/wire.ts";
import type { QuestionResult } from "./score.ts";

/**
 * Posting one question to `/api/plan` and reading the fields run.ts needs
 * out of the response - shared by the narrowing on/off passes and the
 * per-wording passes (docs/plans/wording.md Task 2). No test imports this
 * file: it needs a live platform (run.ts's own doc comment).
 */

/**
 * One raw /api/plan response, read for the fields score.ts needs.
 *
 * `operationId` reads `source.operationId` for a `result` (the operation
 * actually run) or `target.operationId` for a `form` (the operation named
 * without running it - either a real D8 confirm-before-write, or an
 * `ask_user` degraded into one, see score.ts's own QuestionResult
 * comment). `alternatives` is only ever present on a `result`
 * (orchestrator.go's own `call`) - reading it here regardless of kind is
 * harmless, since a `form` never carries the field on the wire.
 */
export interface PlanResponse {
  readonly kind: string;
  readonly operationId?: string;
  readonly alternatives?: readonly string[];
  readonly via?: "plan" | "invoke-500";
  readonly errorMessage?: string;
  readonly errorStatus?: number;
}

/** Parses an /api/plan response body into the fields run.ts needs, by runtime check (no `as`). */
function parsePlanResponse(value: unknown): PlanResponse {
  const record = asRecord(value);
  const kind = typeof record["kind"] === "string" ? record["kind"] : "error";
  const source = isRecord(record["source"]) ? record["source"] : undefined;
  const target = isRecord(record["target"]) ? record["target"] : undefined;
  const operationId =
    (typeof source?.["operationId"] === "string" ? source["operationId"] : undefined) ??
    (typeof target?.["operationId"] === "string" ? target["operationId"] : undefined);
  const rawAlternatives = Array.isArray(record["alternatives"]) ? record["alternatives"] : [];
  const alternatives = rawAlternatives
    .filter((a) => isRecord(a))
    .map((a) => a["operationId"])
    .filter((id): id is string => typeof id === "string");

  return {
    kind,
    ...(operationId !== undefined && { operationId }),
    ...(alternatives.length > 0 && { alternatives }),
    ...(kind === "result" && { via: "plan" }),
  };
}

/**
 * The fixture (e2e/narrowing/serve.ts) serves contracts only, never
 * `/api/invoke` - so `Orchestrator.Plan`'s existing, pre-shortlisting
 * behaviour of invoking a safe operation synchronously (D8,
 * docs/specs/orchestration.md) 404s against it, and `/api/plan` answers
 * with a 500 whose message still names what it tried to invoke:
 * `invoking <service>/<operationId>: service returned an error: ...`.
 * docs/specs/shortlisting.md section 7 excludes fixture invocation from
 * this measurement on purpose ("planning is measured; invoking a fixture
 * operation is not") - this reads the plan a 500 still names back out as
 * the `result` it would have been, so a safe operation's correct@1 is not
 * silently lost to an invocation the fixture was never meant to answer.
 * The result has no alternatives: those never reached the wire.
 */
const INVOKE_FAILURE = /^invoking [^/]+\/(\S+):/u;

function planFromInvokeFailure(message: string): PlanResponse | undefined {
  const match = INVOKE_FAILURE.exec(message);

  return match?.[1] === undefined
    ? undefined
    : { kind: "result", operationId: match[1], via: "invoke-500" };
}

export async function postPlan(
  baseUrl: string,
  session: Session,
  query: string,
): Promise<PlanResponse> {
  const response = await fetch(
    `${baseUrl}/api/plan`,
    withSession(session, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ query, turns: [] }),
    }),
  );

  if (!response.ok) {
    const body = asRecord(await response.json().catch(() => ({})));
    const message = typeof body["message"] === "string" ? body["message"] : "";

    return (
      planFromInvokeFailure(message) ?? {
        kind: "error",
        errorMessage: message,
        errorStatus: response.status,
      }
    );
  }

  return parsePlanResponse(await response.json());
}

export function kindOf(response: PlanResponse): QuestionResult["kind"] {
  const kinds: readonly QuestionResult["kind"][] = [
    "result",
    "ask",
    "none",
    "form",
    "proposal",
    "error",
  ];

  return kinds.find((k) => k === response.kind) ?? "error";
}
