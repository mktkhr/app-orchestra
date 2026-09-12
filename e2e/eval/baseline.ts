import { readFileSync, writeFileSync } from "node:fs";

import { asRecord } from "../src/helpers/wire.ts";
import type { CaseTally } from "./types.ts";

/**
 * One case's recorded accept and reject rates, and how many runs they were
 * measured over. Both counts are always recorded, regardless of which one a
 * case is judged on (Case.metric) - a case can switch which rate it judges
 * without needing a fresh baseline for the count it had been ignoring.
 */
export interface BaselineEntry {
  readonly total: number;
  readonly accept: number;
  readonly reject: number;
}

/** The recorded baseline (docs/specs/eval.md, E3/AC-E-204): per case, the last accepted run. */
export interface Baseline {
  readonly cases: Readonly<Record<string, BaselineEntry>>;
}

/** value narrowed to a BaselineEntry, or undefined when it does not have that shape. */
function asBaselineEntry(value: unknown): BaselineEntry | undefined {
  const record = asRecord(value);
  const { total, accept, reject } = record;

  return typeof total === "number" && typeof accept === "number" && typeof reject === "number"
    ? { total, accept, reject }
    : undefined;
}

/** value narrowed to a Baseline, or an empty one when the file held something else. */
function asBaseline(value: unknown): Baseline {
  const cases: Record<string, BaselineEntry> = {};

  for (const [id, raw] of Object.entries(asRecord(asRecord(value)["cases"]))) {
    const entry = asBaselineEntry(raw);

    if (entry !== undefined) cases[id] = entry;
  }

  return { cases };
}

/** Reads path's baseline, or an empty one when the file does not exist yet. */
export function loadBaseline(path: string): Baseline {
  try {
    return asBaseline(JSON.parse(readFileSync(path, "utf-8")) as unknown);
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") {
      return { cases: {} };
    }

    throw error;
  }
}

/** Writes tallies to path as the new baseline (AC-E-204: only `make eval-accept` calls this). */
export function writeBaseline(path: string, tallies: readonly CaseTally[]): void {
  const cases: Record<string, BaselineEntry> = {};

  for (const tally of tallies) {
    cases[tally.id] = { total: tally.total, accept: tally.accept, reject: tally.reject };
  }

  writeFileSync(path, `${JSON.stringify({ cases }, null, 2)}\n`, "utf-8");
}
