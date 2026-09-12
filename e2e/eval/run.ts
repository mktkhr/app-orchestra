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

// N and tolerance were picked from measurement, not guessed, and re-measured
// twice as the corpus changed (DECISIONS.md has the numbers both times).
// `ORCHESTRA_EVAL_N` is the default every case gets unless it names its own
// `runs`: six of the seven cases held steady at 10/10 or 5/5 across every
// ten-run sample taken, so ten is enough for them. The seventh,
// `no-enum-value`, is judged on `reject` (Case.metric) rather than `accept`
// precisely because its accept/reject split itself would not hold still at
// n=10 - and neither did reject: six ten-run samples read 5-9/10, a band as
// wide as accept's. Tripling its own `runs` to 30 (cases.ts) narrowed it to
// a band of 15-22/30 over nine samples, which is why only that one case
// overrides the default instead of raising it for everyone and paying the
// wall-clock cost on cases that never needed it.
//
// `ORCHESTRA_EVAL_TOLERANCE` stays one number for every case (not per-case):
// 0.3 was sized, in the original measurement, to a two-sample-deviation's
// width for a rate this uncertain around n=10, and it sits just outside the
// 0.23-wide band measured at n=30 above - close enough that the filter
// starting to drop outright still clears it, and far enough that nothing
// smaller can be read out of a single run either way (docs/specs/eval.md
// section 4a).
const defaultRuns = Number(process.env["ORCHESTRA_EVAL_N"] ?? "10");
const tolerance = Number(process.env["ORCHESTRA_EVAL_TOLERANCE"] ?? "0.3");

const acceptMode = process.argv.includes("--accept");

async function runCase(
  baseUrl: string,
  session: Session,
  evalCase: (typeof cases)[number],
): Promise<CaseTally> {
  let accept = 0;
  let reject = 0;
  const total = evalCase.runs ?? defaultRuns;

  for (let i = 0; i < total; i++) {
    // Deliberately sequential (no-await-in-loop is off, see
    // harness/quality/oxlint/policy.ts): this measures one model serving
    // one request at a time, not a load test.
    const outcome = await postPlan(baseUrl, session, evalCase.question, evalCase.turns ?? []);

    if (matchesAny(outcome, evalCase.accept)) accept++;
    if (evalCase.reject !== undefined && matchesAny(outcome, evalCase.reject)) reject++;
  }

  return { id: evalCase.id, total, accept, reject, metric: evalCase.metric ?? "accept" };
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
