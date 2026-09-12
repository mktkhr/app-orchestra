import path from "node:path";

import type { Session } from "../src/helpers/auth.ts";
import { loadBaseline, writeBaseline } from "./baseline.ts";
import { cases } from "./cases.ts";
import { matchesAny } from "./match.ts";
import { postPlan } from "./plan-client.ts";
import { report } from "./report.ts";
import { startEvalPlatform, stopEvalPlatform } from "./services.ts";
import type { CaseTally } from "./types.ts";

/**
 * The evaluation suite's runner (docs/specs/eval.md; AC-E-201, AC-E-204).
 *
 * `services.ts` starts both dummy services and the platform from their
 * built binaries with the real tool-calling planner (`ORCHESTRA_EVAL_MODEL`)
 * rather than the stub every other e2e suite uses. `make check` never runs
 * this file (AC-E-202); only `make eval` and `make eval-accept` do.
 *
 * Run with `node e2e/eval/run.ts` (make eval) or `node e2e/eval/run.ts
 * --accept` (make eval-accept, AC-E-204: the only thing that rewrites
 * baseline.json).
 */

const baselinePath = path.join(import.meta.dirname, "baseline.json");

const evalModel = process.env["ORCHESTRA_EVAL_MODEL"] ?? "qwen3.5-9b-q8";
const evalBaseURL = process.env["ORCHESTRA_EVAL_BASE_URL"] ?? "http://localhost:11435/v1";

// N and tolerance were picked from measurement, not guessed: ten runs each
// of every case in this corpus against qwen3.5-9b-q8 (2026-09-12) held
// steady at 5/5 for six of the seven, and the ambiguous-filter case swung
// between 2/10 and 4/10 reject across two ten-run samples - noise a
// smaller N could not tell apart from an actual regression. 0.3 keeps that
// swing (a two-sample-standard-deviation's width, for a rate this uncertain
// around n=10) from failing a run that changed nothing; a real regression -
// the filter starting to drop outright, not merely wobble - reads as a
// larger drop than that.
const runsPerCase = Number(process.env["ORCHESTRA_EVAL_N"] ?? "10");
const tolerance = Number(process.env["ORCHESTRA_EVAL_TOLERANCE"] ?? "0.3");

const acceptMode = process.argv.includes("--accept");

async function runCase(
  baseUrl: string,
  session: Session,
  evalCase: (typeof cases)[number],
): Promise<CaseTally> {
  let accept = 0;
  let reject = 0;

  for (let i = 0; i < runsPerCase; i++) {
    // Deliberately sequential (no-await-in-loop is off, see
    // harness/quality/oxlint/policy.ts): this measures one model serving
    // one request at a time, not a load test.
    const outcome = await postPlan(baseUrl, session, evalCase.question, evalCase.turns ?? []);

    if (matchesAny(outcome, evalCase.accept)) accept++;
    if (evalCase.reject !== undefined && matchesAny(outcome, evalCase.reject)) reject++;
  }

  return { id: evalCase.id, total: runsPerCase, accept, reject };
}

async function main(): Promise<void> {
  const { baseUrl, session } = await startEvalPlatform(evalBaseURL, evalModel);

  try {
    const tallies: CaseTally[] = [];

    for (const evalCase of cases) {
      // Deliberately sequential; see runCase's own comment.
      tallies.push(await runCase(baseUrl, session, evalCase));
    }

    if (acceptMode) {
      writeBaseline(baselinePath, tallies);
      console.log(`baseline written: ${baselinePath}`);

      return;
    }

    const baseline = loadBaseline(baselinePath);
    const regressions = report(tallies, baseline, tolerance);

    if (regressions.length > 0) {
      process.exitCode = 1;
    }
  } finally {
    await stopEvalPlatform();
  }
}

await main();
