import { readFileSync } from "node:fs";
import path from "node:path";

import { asRecord } from "../src/helpers/wire.ts";
import { renderReport } from "./report.ts";
import { scoreboard, type Axis, type QuestionResult } from "./score.ts";

/**
 * Prints the shortlist report from the two output files run.ts wrote
 * (docs/plans/shortlisting.md Task 4, Step 5). `make eval-shortlist` runs
 * `run.ts` then this - kept separate so a report can be reprinted from an
 * already-finished run without re-running any question.
 */

const outDir = path.join(import.meta.dirname, "out");

/** Parses one line of a pass's output file into a QuestionResult, by runtime check (no `as`). */
function parseLine(line: string): QuestionResult {
  const record = asRecord(JSON.parse(line));
  const id = typeof record["id"] === "string" ? record["id"] : "";
  const axisValue = record["axis"];
  const axis: Axis =
    axisValue === "B" || axisValue === "C" || axisValue === "D" || axisValue === "E"
      ? axisValue
      : "A";
  const answers = Array.isArray(record["answers"])
    ? record["answers"].filter((a): a is string => typeof a === "string")
    : [];
  const kindValue = record["kind"];
  const kind: QuestionResult["kind"] =
    kindValue === "ask" ||
    kindValue === "none" ||
    kindValue === "form" ||
    kindValue === "proposal" ||
    kindValue === "error"
      ? kindValue
      : "result";
  const operationId = typeof record["operationId"] === "string" ? record["operationId"] : undefined;
  const alternatives = Array.isArray(record["alternatives"])
    ? record["alternatives"].filter((a): a is string => typeof a === "string")
    : undefined;
  const latencyMs = typeof record["latencyMs"] === "number" ? record["latencyMs"] : 0;

  return {
    id,
    axis,
    answers,
    kind,
    ...(operationId !== undefined && { operationId }),
    ...(alternatives !== undefined && { alternatives }),
    latencyMs,
  };
}

function readPass(pass: "on" | "off"): readonly QuestionResult[] {
  const filePath = path.join(outDir, `${pass}.jsonl`);

  return readFileSync(filePath, "utf8")
    .split("\n")
    .filter(Boolean)
    .map((line) => parseLine(line));
}

console.log(renderReport({ on: scoreboard(readPass("on")), off: scoreboard(readPass("off")) }));
