import { questions } from "../narrowing/corpus/index.ts";
import { signIn, type Session } from "../src/helpers/auth.ts";
import { boot, type Booted } from "./boot.ts";
import { safeOperationIds } from "./catalogue-safety.ts";
import { writeMisses } from "./misses.ts";
import { readResults } from "./parse-result.ts";
import { alreadyDone, appendResult, outputPathFor } from "./pass-io.ts";
import { kindOf, postPlan } from "./plan-request.ts";
import type { QuestionResult } from "./score.ts";

/**
 * Runs the 100-question corpus against a running platform, one question at
 * a time, for the shortlist measurement (docs/plans/shortlisting.md Task
 * 4, Step 2 and Step 4; docs/specs/shortlisting.md section 6) and, with
 * `--wording`, for the per-wording measurement (docs/plans/wording.md Task
 * 2, Step 1). NOT imported by any test file: it needs a live platform, a
 * live fixture and, for the "on" pass, a live embedder and reranker.
 *
 * Run with `node shortlist/run.ts` (invoked by `make eval-shortlist`).
 * `--on-only` / `--off-only` resume a half-finished run: each pass writes
 * its own output file and is independent of the other. `--wording
 * a,b,c` runs one narrowing-on, K=20 pass per named wording instead,
 * setting `ORCHESTRA_PLANNER_WORDING=<name>` on the platform for that
 * pass; `v1` is always included first even when not named, so every
 * report has the baseline to compare against. `--wording` and
 * `--on-only`/`--off-only` are mutually exclusive modes: without
 * `--wording`, run.ts behaves exactly as it did before this flag existed.
 */

const NARROWING = { embedModel: "e5-large-q8", rerankModel: "bge-reranker-v2-m3-q8", k: 20 };

/** Runs three sample questions and prints their raw responses, warning (not failing) if the first exceeds 5s (AC-H-107). */
async function smokeTest(baseUrl: string, session: Session): Promise<void> {
  const samples = questions().slice(0, 3);

  console.log("shortlist smoke test: 3 sample questions, raw /api/plan responses");

  for (const [index, question] of samples.entries()) {
    const start = Date.now();
    // Sequential: one question at a time is the point (see runQuestions).
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

/**
 * Runs every not-yet-done question in the corpus against one booted
 * platform, appending each result to `outputName`'s file as it completes.
 * Shared by the narrowing on/off passes and the per-wording passes: the
 * only difference between them is which platform `boot()` started and
 * which output file the rows go to.
 */
async function runQuestions(
  label: string,
  outputName: string,
  baseUrl: string,
  session: Session,
): Promise<readonly QuestionResult[]> {
  const safeIds = safeOperationIds();
  const done = alreadyDone(outputName);
  const results: QuestionResult[] = [];

  for (const question of questions()) {
    if (done.has(question.id)) continue;

    const start = Date.now();
    // Sequential and deliberate: one question against one model at a
    // time, so latency measures the platform's own round trip rather
    // than queueing behind other requests (docs/specs/shortlisting.md
    // section 6).
    const response = await postPlan(baseUrl, session, question.text);
    const latencyMs = Date.now() - start;

    if (latencyMs > 5000) {
      console.warn(
        `[${label}] over 5s: "${question.text}" (${question.id}) took ${String(latencyMs)}ms`,
      );
    }

    const kind = kindOf(response);
    // An ask_user over a safe operation with no enum for its parameter
    // degrades into this same form shape (score.ts's own QuestionResult
    // comment; orchestrator.go's `ask`) - scored as `asked`, not as a
    // pick, by score.ts. Not a platform bug to fix here - worth a follow
    // up item in this repository's own memory files, left for the
    // session that owns them (this task stops short of DECISIONS.md /
    // STATE.md / TODO.md).
    const askDegraded =
      kind === "form" && response.operationId !== undefined && safeIds.has(response.operationId);

    const result: QuestionResult = {
      id: question.id,
      axis: question.axis,
      text: question.text,
      answers: question.answers,
      kind,
      ...(response.operationId !== undefined && { operationId: response.operationId }),
      ...(askDegraded && { askDegraded }),
      ...(response.alternatives !== undefined && { alternatives: response.alternatives }),
      ...(response.via !== undefined && { via: response.via }),
      latencyMs,
      ...(response.errorMessage !== undefined && { errorMessage: response.errorMessage }),
      ...(response.errorStatus !== undefined && { errorStatus: response.errorStatus }),
    };

    appendResult(outputName, result);
    results.push(result);
    console.log(`[${label}] ${question.id} ${question.axis} ${result.kind} ${String(latencyMs)}ms`);
  }

  return results;
}

async function runPass(pass: "on" | "off"): Promise<void> {
  const options = pass === "on" ? { narrowing: NARROWING } : {};
  const booted: Booted = await boot(options);

  try {
    const session = await signIn(booted.baseUrl, "admin", booted.adminPassword);

    if (pass === "on") await smokeTest(booted.baseUrl, session);

    await runQuestions(pass, pass, booted.baseUrl, session);
  } finally {
    await booted.stop();
  }
}

/** One pass for one named wording: narrowing on, K=20, `ORCHESTRA_PLANNER_WORDING=<name>` (docs/plans/wording.md Task 2, Step 1). Rows go to `out/on-<name>.jsonl`; the miss list to `out/misses-<name>.txt`. */
async function runWordingPass(name: string): Promise<void> {
  const outputName = `on-${name}`;
  const booted: Booted = await boot({ narrowing: NARROWING, wording: name });

  try {
    const session = await signIn(booted.baseUrl, "admin", booted.adminPassword);

    await runQuestions(outputName, outputName, booted.baseUrl, session);
    // Read the whole pass's file back - not just this run's freshly
    // appended rows - so a resumed pass's miss list still covers every
    // row, including ones a previous run already wrote (pass-io.ts's
    // alreadyDone).
    writeMisses(name, readResults(outputPathFor(outputName)));
  } finally {
    await booted.stop();
  }
}

/** `--wording a,b,c`'s names, deduplicated in first-seen order, with `v1` always first even when not named. */
function wordingArg(): readonly string[] | undefined {
  const flagIndex = process.argv.indexOf("--wording");

  if (flagIndex === -1) return undefined;

  const raw = process.argv[flagIndex + 1] ?? "";
  const named = raw
    .split(",")
    .map((n) => n.trim())
    .filter((n) => n.length > 0);
  const ordered = ["v1", ...named];

  return [...new Set(ordered)];
}

async function main(): Promise<void> {
  const wording = wordingArg();

  if (wording !== undefined) {
    for (const name of wording) {
      // Sequential and deliberate: one platform boot per wording, one at
      // a time (docs/plans/wording.md Task 2, Step 1).
      await runWordingPass(name);
    }

    return;
  }

  const onOnly = process.argv.includes("--on-only");
  const offOnly = process.argv.includes("--off-only");

  if (!offOnly) await runPass("on");
  if (!onOnly) await runPass("off");
}

await main();
