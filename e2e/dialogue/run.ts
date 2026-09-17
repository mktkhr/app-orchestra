import { appendFileSync, mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

import { matchesAny } from "../eval/match.ts";
import { postPlanRaw } from "../eval/plan-client.ts";
import { startEvalPlatform, stopEvalPlatform, type EvalPlatform } from "../eval/services.ts";
import { buildTurns, type DialogueStep } from "./chain.ts";
import { dialogues } from "./dialogues.ts";
import { formatReport } from "./report.ts";
import { score } from "./score.ts";
import type { Dialogue, TurnRecord } from "./types.ts";

/**
 * The dialogue instrument's runner (see this task's own corpus doc
 * comment in dialogues.ts): boots the real dummy services and the real
 * platform (`startEvalPlatform`, the same helper `e2e/eval/run.ts` uses),
 * runs every dialogue once, writes one jsonl row per turn plus a text
 * report to `out/`, and prints the same report to stdout.
 *
 * Every dialogue is run exactly once (docs comment: "the model stack is
 * deterministic" - the same reasoning eval's own `ORCHESTRA_EVAL_N` exists
 * to work around does not apply here, since a dialogue's later turns
 * depend on its earlier ones' real answers and repeating the whole
 * conversation N times would multiply, not average, the noise).
 *
 * Run with `node dialogue/run.ts` (`make eval-dialogue`). Never imported
 * by a *.test.ts file, so `make check` never calls a model through it.
 */

const evalModel = process.env["ORCHESTRA_EVAL_MODEL"] ?? "qwen3.5-9b-q8";
const evalBaseURL = process.env["ORCHESTRA_EVAL_BASE_URL"] ?? "http://localhost:11435/v1";

const outDir = path.join(import.meta.dirname, "out");

/** A filesystem-safe timestamp for this run's output files, so a run never overwrites an earlier one's. */
function timestamp(): string {
  return new Date().toISOString().replaceAll(/[:.]/gu, "-");
}

/**
 * Runs one dialogue's turns in order against baseUrl, chaining each
 * question's `turns` from every earlier question's real outcome
 * (chain.ts's buildTurns) - never from `dialogues.ts`'s own `accept` list,
 * which is only what would count this turn correct, not what a prior turn
 * actually returned.
 */
async function runDialogue(
  baseUrl: string,
  session: EvalPlatform["session"],
  dialogue: Dialogue,
): Promise<readonly TurnRecord[]> {
  const steps: DialogueStep[] = [];
  const records: TurnRecord[] = [];

  for (const [turnIndex, turn] of dialogue.turns.entries()) {
    const requestTurns = buildTurns(steps);
    const start = Date.now();
    // Sequential and deliberate: a dialogue's turn N depends on turn N-1's
    // real answer, so these cannot run concurrently even in principle.
    const { outcome, raw } = await postPlanRaw(baseUrl, session, turn.question, requestTurns);
    const latencyMs = Date.now() - start;
    const pass = matchesAny(outcome, turn.accept);

    records.push({
      dialogueId: dialogue.id,
      turnIndex,
      question: turn.question,
      requestTurns,
      outcome,
      rawResponse: raw,
      expected: turn.accept,
      pass,
      latencyMs,
    });

    console.log(
      `[${dialogue.id}] turn${String(turnIndex + 1)} ${pass ? "PASS" : "FAIL"} ` +
        `${turn.question} (${String(latencyMs)}ms)`,
    );

    steps.push({ question: turn.question, outcome });
  }

  return records;
}

async function main(): Promise<void> {
  const { baseUrl, session } = await startEvalPlatform(evalBaseURL, evalModel);

  try {
    const allRecords: TurnRecord[] = [];

    for (const dialogue of dialogues) {
      // Sequential across dialogues too: this measures one model serving
      // one conversation at a time (see runDialogue's own comment).
      allRecords.push(...(await runDialogue(baseUrl, session, dialogue)));
    }

    mkdirSync(outDir, { recursive: true });

    const stamp = timestamp();
    const jsonlPath = path.join(outDir, `dialogue-${stamp}.jsonl`);
    const reportPath = path.join(outDir, `dialogue-${stamp}.txt`);

    for (const record of allRecords) {
      appendFileSync(jsonlPath, `${JSON.stringify(record)}\n`);
    }

    const result = score(allRecords);
    const reportText = formatReport(dialogues, allRecords, result);

    console.log("");
    console.log(reportText);

    writeFileSync(reportPath, `${reportText}\n`);

    console.log("");
    console.log(`wrote ${jsonlPath}`);
    console.log(`wrote ${reportPath}`);
  } finally {
    await stopEvalPlatform();
  }
}

await main();
