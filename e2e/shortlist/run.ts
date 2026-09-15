import { appendFileSync, existsSync, mkdirSync, readFileSync } from "node:fs";
import path from "node:path";

import { questions } from "../narrowing/corpus/index.ts";
import { signIn, withSession, type Session } from "../src/helpers/auth.ts";
import { asRecord, isRecord } from "../src/helpers/wire.ts";
import { boot, type Booted } from "./boot.ts";
import type { QuestionResult } from "./score.ts";

/**
 * Runs the 100-question corpus against a running platform, one question at
 * a time, for the shortlist measurement (docs/plans/shortlisting.md Task
 * 4, Step 2 and Step 4; docs/specs/shortlisting.md section 6). NOT
 * imported by any test file: it needs a live platform, a live fixture and,
 * for the "on" pass, a live embedder and reranker.
 *
 * Run with `node shortlist/run.ts` (invoked by `make eval-shortlist`).
 * `--on-only` / `--off-only` resume a half-finished run: each pass writes
 * its own output file and is independent of the other.
 */

const NARROWING = { embedModel: "e5-large-q8", rerankModel: "bge-reranker-v2-m3-q8", k: 20 };

const outDir = path.join(import.meta.dirname, "out");

function outputPathFor(pass: "on" | "off"): string {
  return path.join(outDir, `${pass}.jsonl`);
}

/** One raw /api/plan response, read for the fields score.ts needs. */
interface PlanResponse {
  readonly kind: string;
  readonly operationId?: string;
  readonly alternatives?: readonly string[];
}

/** Parses an /api/plan response body into the fields run.ts needs, by runtime check (no `as`). */
function parsePlanResponse(value: unknown): PlanResponse {
  const record = asRecord(value);
  const kind = typeof record["kind"] === "string" ? record["kind"] : "error";
  const source = isRecord(record["source"]) ? record["source"] : undefined;
  const operationId =
    typeof source?.["operationId"] === "string" ? source["operationId"] : undefined;
  const rawAlternatives = Array.isArray(record["alternatives"]) ? record["alternatives"] : [];
  const alternatives = rawAlternatives
    .filter((a) => isRecord(a))
    .map((a) => a["operationId"])
    .filter((id): id is string => typeof id === "string");

  return {
    kind,
    ...(operationId !== undefined && { operationId }),
    ...(alternatives.length > 0 && { alternatives }),
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

  return match?.[1] === undefined ? undefined : { kind: "result", operationId: match[1] };
}

async function postPlan(baseUrl: string, session: Session, query: string): Promise<PlanResponse> {
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

    return planFromInvokeFailure(message) ?? { kind: "error" };
  }

  return parsePlanResponse(await response.json());
}

/** Appends one result to a run's output file, flushing to disk immediately - a killed process loses no completed work. */
function appendResult(pass: "on" | "off", result: QuestionResult): void {
  appendFileSync(outputPathFor(pass), `${JSON.stringify(result)}\n`);
}

/** Ids already recorded in a pass's output file, so a resumed run skips what it already has. */
function alreadyDone(pass: "on" | "off"): ReadonlySet<string> {
  const filePath = outputPathFor(pass);

  if (!existsSync(filePath)) return new Set();

  const lines = readFileSync(filePath, "utf8").split("\n").filter(Boolean);
  const ids = lines
    .map((line): unknown => JSON.parse(line))
    .map((parsed) => asRecord(parsed)["id"])
    .filter((id): id is string => typeof id === "string");

  return new Set(ids);
}

/** Runs three sample questions and prints their raw responses, warning (not failing) if the first exceeds 5s (AC-H-107). */
async function smokeTest(baseUrl: string, session: Session): Promise<void> {
  const samples = questions().slice(0, 3);

  console.log("shortlist smoke test: 3 sample questions, raw /api/plan responses");

  for (const [index, question] of samples.entries()) {
    const start = Date.now();
    // Sequential: one question at a time is the point (see runPass).
    const response = await postPlan(baseUrl, session, question.text);
    const elapsedMs = Date.now() - start;

    console.log(
      `smoke[${String(index)}] ${question.text} -> ${JSON.stringify(response)} (${String(elapsedMs)}ms)`,
    );

    if (index === 0 && elapsedMs > 5000) {
      console.warn(
        `smoke test warning: first response took ${String(elapsedMs)}ms (>5s) - this is a model load, H4 is not met yet; continuing anyway`,
      );
    }
  }
}

async function runPass(pass: "on" | "off"): Promise<void> {
  mkdirSync(outDir, { recursive: true });

  const options = pass === "on" ? { narrowing: NARROWING } : {};
  const booted: Booted = await boot(options);

  try {
    const session = await signIn(booted.baseUrl, "admin", booted.adminPassword);

    if (pass === "on") {
      await smokeTest(booted.baseUrl, session);
    }

    const done = alreadyDone(pass);

    for (const question of questions()) {
      if (done.has(question.id)) continue;

      const start = Date.now();
      // Sequential and deliberate: one question against one model at a
      // time, so latency measures the platform's own round trip rather
      // than queueing behind other requests (docs/specs/shortlisting.md
      // section 6).
      const response = await postPlan(booted.baseUrl, session, question.text);
      const latencyMs = Date.now() - start;

      if (latencyMs > 5000) {
        console.warn(
          `[${pass}] over 5s: "${question.text}" (${question.id}) took ${String(latencyMs)}ms`,
        );
      }

      const result: QuestionResult = {
        id: question.id,
        axis: question.axis,
        answers: question.answers,
        kind: kindOf(response),
        ...(response.operationId !== undefined && { operationId: response.operationId }),
        ...(response.alternatives !== undefined && { alternatives: response.alternatives }),
        latencyMs,
      };

      appendResult(pass, result);
      console.log(
        `[${pass}] ${question.id} ${question.axis} ${result.kind} ${String(latencyMs)}ms`,
      );
    }
  } finally {
    await booted.stop();
  }
}

function kindOf(response: PlanResponse): QuestionResult["kind"] {
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

async function main(): Promise<void> {
  const onOnly = process.argv.includes("--on-only");
  const offOnly = process.argv.includes("--off-only");

  if (!offOnly) await runPass("on");
  if (!onOnly) await runPass("off");
}

await main();
