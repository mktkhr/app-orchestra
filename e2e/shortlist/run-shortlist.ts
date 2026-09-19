import path from "node:path";

import { extensionQuestions, questions, type Question } from "../narrowing/corpus/index.ts";
import { signIn, type Session } from "../src/helpers/auth.ts";
import { boot, NARROWING, type Booted } from "./boot.ts";
import { safeOperationIds } from "./catalogue-safety.ts";
import { missesVariant, passOutputName, variantSuffix, wordingOutputName } from "./flags.ts";
import { writeMissesFromOutput } from "./misses.ts";
import { alreadyDone, appendResult, outDir } from "./pass-io.ts";
import { kindOf, postPlan } from "./plan-request.ts";
import type { QuestionResult } from "./score.ts";

/**
 * Runs the 100-question corpus against a running platform, one question at
 * a time, for the shortlist measurement (docs/plans/shortlisting.md Task
 * 4, Step 2 and Step 4; docs/specs/shortlisting.md section 6) and, with
 * `--wording`, for the per-wording measurement (docs/plans/wording.md Task
 * 2, Step 1). NOT imported by any test file: it needs a live platform, a
 * live fixture and, for the "on" pass, a live embedder and reranker. Split
 * out of `run.ts` (docs/plans/midsizing.md Task 3, alongside `run-mid.ts`)
 * to keep `run.ts` a thin dispatcher and every file under both the
 * 300-line and the ten-dependency caps.
 *
 * `runShortlist` is `run.ts`'s entry point for every corpus but `mid`:
 * `--on-only` / `--off-only` resume a half-finished run: each pass writes
 * its own output file and is independent of the other. `--wording
 * a,b,c` runs one narrowing-on, K=20 pass per named wording instead,
 * setting `ORCHESTRA_PLANNER_WORDING=<name>` on the platform for that
 * pass; `v1` is always included first even when not named, so every
 * report has the baseline to compare against - unless `--thinking` or
 * `--repeat-penalty` is also given (see below), in which case only the
 * named wording(s) run, `v1` not forced in. `--wording` and
 * `--on-only`/`--off-only` are mutually exclusive modes: without
 * `--wording`, this behaves exactly as it did before this flag existed.
 *
 * `--thinking on|off` and `--repeat-penalty <float>` set
 * `ORCHESTRA_PLANNER_THINKING` / `ORCHESTRA_PLANNER_REPEAT_PENALTY` on the
 * booted platform for every pass this invocation runs (platform knobs
 * subproject, 2026-09-16). Each carries its own suffix on the output file
 * and miss list - `-nothink` for `--thinking off`, `-rp<value>` for
 * `--repeat-penalty` - so a variant run never overwrites a plain wording
 * pass's files, and multiple variants of the same wording can coexist
 * (e.g. `on-v6-unmatched-filter-nothink.jsonl`,
 * `on-v6-unmatched-filter-nothink-rp1.1.jsonl`). Given with no
 * `--wording`, the pass runs against the platform's own default wording,
 * labelled `default`.
 */

/** Runs three sample questions and prints their raw responses, warning (not failing) if the first exceeds 5s (AC-H-107). */
async function smokeTest(
  baseUrl: string,
  session: Session,
  questionSet: () => readonly Question[],
): Promise<void> {
  const samples = questionSet().slice(0, 3);

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
  questionSet: () => readonly Question[],
): Promise<readonly QuestionResult[]> {
  const safeIds = safeOperationIds();
  const done = alreadyDone(outputName);
  const results: QuestionResult[] = [];

  for (const question of questionSet()) {
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

/** `pass` is `"on"` or `"off"`; `extCorpus` picks `passOutputName`'s `ext-` prefix (`on.jsonl`/`off.jsonl` vs `ext-on.jsonl`/`ext-off.jsonl`, flags.ts). */
async function runPass(
  pass: "on" | "off",
  questionSet: () => readonly Question[],
  extCorpus: boolean,
): Promise<void> {
  const options = pass === "on" ? { narrowing: NARROWING } : {};
  const booted: Booted = await boot(options);
  const outputName = passOutputName(pass, extCorpus);

  try {
    const session = await signIn(booted.baseUrl, "admin", booted.adminPassword);

    if (pass === "on") await smokeTest(booted.baseUrl, session, questionSet);

    await runQuestions(outputName, outputName, booted.baseUrl, session, questionSet);
  } finally {
    await booted.stop();
  }
}

/**
 * One pass for one named wording, optionally overriding thinking, the
 * repeat penalty and/or the staging (docs/plans/staging.md, Task 3):
 * narrowing on, K=20, `ORCHESTRA_PLANNER_WORDING=<name>`
 * (docs/plans/wording.md Task 2, Step 1) plus, when given,
 * `ORCHESTRA_PLANNER_THINKING` / `ORCHESTRA_PLANNER_REPEAT_PENALTY` /
 * `ORCHESTRA_PLANNER_STAGES` (platform knobs subproject, 2026-09-16;
 * staging, docs/plans/staging.md). `narrowing: "off"` (`--narrowing off`,
 * the full-catalogue Jev trial) boots with narrowing off instead and adds
 * a `-narrowing-off` suffix, mirroring `run-mid.ts`'s `runMidPass`; the
 * `on-` prefix stays, since it names this pass kind, not the narrowing.
 * Rows go to
 * `out/on-<name><suffix>.jsonl`; the miss list to
 * `out/misses-<name><suffix>.txt`; the platform's own stdout/stderr -
 * including the `planner truncated by max_tokens` warn line - to
 * `out/on-<name><suffix>.log`.
 */
async function runWordingPass(
  name: string,
  thinking: "on" | "off" | undefined,
  repeatPenalty: number | undefined,
  stages: 1 | 2 | undefined,
  narrowing: "on" | "off" | undefined,
  questionSet: () => readonly Question[],
  extCorpus: boolean,
): Promise<void> {
  const narrowingSuffix = narrowing === "off" ? "-narrowing-off" : "";
  const variant = `${name}${variantSuffix(thinking, repeatPenalty, stages)}${narrowingSuffix}`;
  const outputName = wordingOutputName(variant, extCorpus);
  const booted: Booted = await boot({
    ...(narrowing === "off" ? {} : { narrowing: NARROWING }),
    // "default" is a label for the report, not a wording the platform
    // knows: leave ORCHESTRA_PLANNER_WORDING unset so the platform's own
    // wording.Default() applies (its config rejects any unknown name).
    ...(name === "default" ? {} : { wording: name }),
    logFile: path.join(outDir, `${outputName}.log`),
    ...(thinking === undefined ? {} : { thinking }),
    ...(repeatPenalty === undefined ? {} : { repeatPenalty }),
    ...(stages === undefined ? {} : { stages }),
  });

  try {
    const session = await signIn(booted.baseUrl, "admin", booted.adminPassword);

    await runQuestions(outputName, outputName, booted.baseUrl, session, questionSet);
    // Reads the whole pass's file back - not just this run's freshly
    // appended rows - so a resumed pass's miss list still covers every
    // row, including ones a previous run already wrote (pass-io.ts's
    // alreadyDone). `--corpus ext`'s own miss list carries an `ext-`
    // prefix too (`misses-ext-<variant>.txt`), unlike the 100-question
    // corpus's own (`misses-<variant>.txt`, unprefixed, unchanged) - so it
    // never collides with the 100-question corpus's own miss list for the
    // same wording/variant name.
    writeMissesFromOutput(missesVariant(variant, extCorpus), outputName);
  } finally {
    await booted.stop();
  }
}

/**
 * `run.ts`'s entry point for every corpus but `mid` (`run-mid.ts` owns that
 * one) - see this file's own doc comment for every flag's meaning.
 * `corpus` is `"ext"` for `--corpus ext` (the 50-question axis D/E
 * extension corpus, `extensionQuestions()`) or `undefined` for the plain
 * 100-question corpus (`questions()`) - the only difference between the
 * two is which question set is asked and which output prefix
 * (`ext-`/none) the pass functions above use; every flag composes with
 * `ext` exactly as it does with the 100-question corpus.
 */
export async function runShortlist(
  names: readonly string[] | undefined,
  thinking: "on" | "off" | undefined,
  repeatPenalty: number | undefined,
  stages: 1 | 2 | undefined,
  narrowing?: "on" | "off",
  corpus?: "ext",
): Promise<void> {
  const isVariant =
    thinking !== undefined ||
    repeatPenalty !== undefined ||
    stages !== undefined ||
    narrowing !== undefined;
  const extCorpus = corpus === "ext";
  const questionSet = extCorpus ? extensionQuestions : questions;

  if (names !== undefined) {
    // `v1` is always included first as a baseline, unless a thinking/
    // repeat-penalty/stages variant was asked for - that is a different
    // experiment with its own recorded baseline (see this file's own doc
    // comment), and forcing an extra `v1` pass through it would only cost
    // GPU time nobody asked to spend.
    const targets = isVariant ? names : [...new Set(["v1", ...names])];

    for (const name of targets) {
      // Sequential and deliberate: one platform boot per wording, one at
      // a time (docs/plans/wording.md Task 2, Step 1).
      await runWordingPass(
        name,
        thinking,
        repeatPenalty,
        stages,
        narrowing,
        questionSet,
        extCorpus,
      );
    }

    return;
  }

  if (isVariant) {
    await runWordingPass(
      "default",
      thinking,
      repeatPenalty,
      stages,
      narrowing,
      questionSet,
      extCorpus,
    );

    return;
  }

  const onOnly = process.argv.includes("--on-only");
  const offOnly = process.argv.includes("--off-only");

  if (!offOnly) await runPass("on", questionSet, extCorpus);
  if (!onOnly) await runPass("off", questionSet, extCorpus);
}
