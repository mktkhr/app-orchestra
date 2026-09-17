import { readFileSync } from "node:fs";

import { asRecord, isRecord } from "../src/helpers/wire.ts";
import type { Axis, QuestionResult } from "./score.ts";

/**
 * Reads a pass's `.jsonl` output file back into `QuestionResult`s - shared
 * by print-report.ts (the plain on/off passes and the per-wording passes,
 * docs/plans/wording.md Task 2, Step 3).
 */

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
  const text = typeof record["text"] === "string" ? record["text"] : "";
  const operationId = typeof record["operationId"] === "string" ? record["operationId"] : undefined;
  const askDegraded = record["askDegraded"] === true ? true : undefined;
  const alternatives = Array.isArray(record["alternatives"])
    ? record["alternatives"].filter((a): a is string => typeof a === "string")
    : undefined;
  const viaValue = record["via"];
  const via = viaValue === "plan" || viaValue === "invoke-500" ? viaValue : undefined;
  const latencyMs = typeof record["latencyMs"] === "number" ? record["latencyMs"] : 0;
  const errorMessage =
    typeof record["errorMessage"] === "string" ? record["errorMessage"] : undefined;
  const errorStatus = typeof record["errorStatus"] === "number" ? record["errorStatus"] : undefined;
  const rawInitial = isRecord(record["initial"]) ? record["initial"] : undefined;
  const initial =
    rawInitial === undefined
      ? undefined
      : Object.fromEntries(
          Object.entries(rawInitial).filter(
            (entry): entry is [string, string] => typeof entry[1] === "string",
          ),
        );
  const expectValue = record["expect"];
  const expect =
    expectValue === "answerable" || expectValue === "impossible" ? expectValue : undefined;
  const capability = record["capability"] === true ? true : undefined;

  return {
    id,
    axis,
    text,
    answers,
    kind,
    ...(operationId !== undefined && { operationId }),
    ...(askDegraded !== undefined && { askDegraded }),
    ...(alternatives !== undefined && { alternatives }),
    ...(via !== undefined && { via }),
    ...(initial !== undefined && { initial }),
    latencyMs,
    ...(errorMessage !== undefined && { errorMessage }),
    ...(errorStatus !== undefined && { errorStatus }),
    ...(expect !== undefined && { expect }),
    ...(capability !== undefined && { capability }),
  };
}

/** Reads a `.jsonl` output file at filePath into its QuestionResults, in file order. */
export function readResults(filePath: string): readonly QuestionResult[] {
  return readFileSync(filePath, "utf8")
    .split("\n")
    .filter(Boolean)
    .map((line) => parseLine(line));
}
