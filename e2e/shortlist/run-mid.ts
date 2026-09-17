import path from "node:path";

import { midQuestions } from "../narrowing/corpus/mid.ts";
import { signIn, type Session } from "../src/helpers/auth.ts";
import { boot, NARROWING, type Booted } from "./boot.ts";
import { variantSuffix } from "./flags.ts";
import { alreadyDone, appendResult, outDir } from "./pass-io.ts";
import { kindOf, postPlan } from "./plan-request.ts";
import type { QuestionResult } from "./score.ts";

/**
 * `run.ts`'s `--corpus mid` support (docs/plans/midsizing.md Task 3;
 * docs/specs/midsizing.md M5): boots the fixture server on the mid
 * subset's three services and asks the sixty mid questions, one pass per
 * named wording (or one plain "default" pass without `--wording`). Split
 * out of `run.ts` for the same reason `flags.ts` was: `run.ts` is close
 * to the 300-line cap already. NOT imported by any test file: it needs a
 * live platform and, for narrowing, a live embedder and reranker
 * (`run.ts`'s own doc comment).
 */

/** Runs every not-yet-done mid question against one booted platform, appending each result to `outputName`'s file as it completes - the mid analogue of `run.ts`'s `runQuestions`, over `midQuestions()` instead of the shortlist corpus. */
async function runMidQuestions(
  outputName: string,
  baseUrl: string,
  session: Session,
): Promise<void> {
  const done = alreadyDone(outputName);

  for (const question of midQuestions()) {
    if (done.has(question.id)) continue;

    const start = Date.now();
    // Sequential and deliberate, exactly as run.ts's runQuestions: one
    // question at a time, so latency measures the platform's own round
    // trip.
    const response = await postPlan(baseUrl, session, question.text);
    const latencyMs = Date.now() - start;

    if (latencyMs > 5000) {
      console.warn(
        `[mid] over 5s: "${question.text}" (${question.id}) took ${String(latencyMs)}ms`,
      );
    }

    const kind = kindOf(response);
    const result: QuestionResult = {
      id: question.id,
      axis: question.axis,
      text: question.text,
      answers: question.answers,
      kind,
      ...(response.operationId !== undefined && { operationId: response.operationId }),
      ...(response.alternatives !== undefined && { alternatives: response.alternatives }),
      ...(response.via !== undefined && { via: response.via }),
      ...(response.initial !== undefined && { initial: response.initial }),
      latencyMs,
      ...(response.errorMessage !== undefined && { errorMessage: response.errorMessage }),
      ...(response.errorStatus !== undefined && { errorStatus: response.errorStatus }),
      expect: question.expect,
      ...(question.capability !== undefined && { capability: question.capability }),
    };

    appendResult(outputName, result);
    console.log(`[mid] ${question.id} ${question.expect} ${result.kind} ${String(latencyMs)}ms`);
  }
}

/**
 * One mid pass for one named wording (or "default" when `name` is
 * undefined): narrowing on, K=20 by default, the mid fixture, plus
 * whatever of `thinking` / `repeatPenalty` / `stages` was given - the
 * platform's own defaults otherwise (`make eval-mid`'s: two stages,
 * `v6-unmatched-filter`, thinking off, narrowing on -
 * docs/plans/midsizing.md Task 4). `narrowing: "off"` (the full-catalogue
 * Jev trial's own flag, `--narrowing off`) boots with narrowing off
 * instead - `{}` in place of `narrowing: NARROWING`, the same "off" shape
 * `run-shortlist.ts`'s own `runPass("off")` passes `boot()`. `undefined`
 * (the flag not given at all) keeps this byte-identical to every call
 * before `--narrowing` existed - narrowing on. Rows go to
 * `out/mid-<variant>.jsonl`; the platform's own stdout/stderr to
 * `out/mid-<variant>.log`, read back for `pick_ms` by `pick-log.ts` - both
 * names carry a `-narrowing-off` suffix under `--narrowing off` so neither
 * output collides with the narrowing-on pass's own files.
 */
async function runMidPass(
  name: string | undefined,
  thinking?: "on" | "off",
  repeatPenalty?: number,
  stages?: 1 | 2,
  narrowing?: "on" | "off",
): Promise<void> {
  const wordingLabel = name ?? "default";
  const narrowingSuffix = narrowing === "off" ? "-narrowing-off" : "";
  const variant = `${wordingLabel}${variantSuffix(thinking, repeatPenalty, stages)}${narrowingSuffix}`;
  const outputName = `mid-${variant}`;
  const booted: Booted = await boot({
    ...(narrowing === "off" ? {} : { narrowing: NARROWING }),
    fixture: "mid",
    // "default" is a label for the output file, not a wording the platform
    // knows - leave ORCHESTRA_PLANNER_WORDING unset so the platform's own
    // wording.Default() applies (run.ts's runWordingPass does the same).
    ...(name === undefined || name === "default" ? {} : { wording: name }),
    logFile: path.join(outDir, `${outputName}.log`),
    ...(thinking === undefined ? {} : { thinking }),
    ...(repeatPenalty === undefined ? {} : { repeatPenalty }),
    ...(stages === undefined ? {} : { stages }),
  });

  try {
    const session = await signIn(booted.baseUrl, "admin", booted.adminPassword);

    await runMidQuestions(outputName, booted.baseUrl, session);
  } finally {
    await booted.stop();
  }
}

/** `--corpus mid`'s entry point: one pass per name in `names`, or a single "default" pass when `names` is undefined or empty (no `--wording` given). `narrowing` is `--narrowing`'s own value, forwarded to every pass unchanged (see `runMidPass`'s own doc comment). */
export async function runMid(
  names: readonly string[] | undefined,
  thinking?: "on" | "off",
  repeatPenalty?: number,
  stages?: 1 | 2,
  narrowing?: "on" | "off",
): Promise<void> {
  const targets: readonly (string | undefined)[] =
    names === undefined || names.length === 0 ? [undefined] : names;

  for (const name of targets) {
    // Sequential and deliberate, exactly as run.ts's main: one platform
    // boot per wording, one at a time.
    await runMidPass(name, thinking, repeatPenalty, stages, narrowing);
  }
}
