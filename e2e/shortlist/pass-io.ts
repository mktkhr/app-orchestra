import { appendFileSync, existsSync, mkdirSync, readFileSync } from "node:fs";
import path from "node:path";

import { asRecord } from "../src/helpers/wire.ts";
import type { QuestionResult } from "./score.ts";

/**
 * File I/O for one pass's output - shared by the narrowing on/off passes
 * (`on.jsonl` / `off.jsonl`) and the per-wording passes (`on-<name>.jsonl`,
 * docs/plans/wording.md Task 2, Step 1). No test imports this file: it is
 * plain disk I/O, not scoring logic (score.test.ts covers the latter with
 * fakes).
 */

export const outDir = path.join(import.meta.dirname, "out");

/** The output file a pass's rows are appended to - "on", "off", or "on-<wording>". */
export function outputPathFor(name: string): string {
  return path.join(outDir, `${name}.jsonl`);
}

/** Appends one result to a pass's output file, flushing to disk immediately - a killed process loses no completed work. */
export function appendResult(name: string, result: QuestionResult): void {
  mkdirSync(outDir, { recursive: true });
  appendFileSync(outputPathFor(name), `${JSON.stringify(result)}\n`);
}

/** Ids already recorded in a pass's output file, so a resumed run skips what it already has. */
export function alreadyDone(name: string): ReadonlySet<string> {
  const filePath = outputPathFor(name);

  if (!existsSync(filePath)) return new Set();

  const lines = readFileSync(filePath, "utf8").split("\n").filter(Boolean);
  const ids = lines
    .map((line): unknown => JSON.parse(line))
    .map((parsed) => asRecord(parsed)["id"])
    .filter((id): id is string => typeof id === "string");

  return new Set(ids);
}
