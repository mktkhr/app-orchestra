import type { TurnRecord } from "./types.ts";

/** A count against its total - "N/M" pairs everywhere the report prints a rate. */
export interface CountFraction {
  readonly correct: number;
  readonly total: number;
}

/** The dialogue instrument's own report numbers, computed from a flat list of every turn run. */
export interface DialogueScore {
  /** 1: every turn, across every dialogue. */
  readonly allTurns: CountFraction;
  /** 2: only turns that follow at least one earlier turn in their dialogue. */
  readonly turns2Plus: CountFraction;
  /**
   * 3: of the turns counted in `turns2Plus`, only the ones whose every
   * earlier turn in the same dialogue also passed - whether the platform
   * stays correct once the conversation is already on track, separated
   * from a dialogue that was already off the rails by the time this turn
   * was asked.
   */
  readonly conditionalTurns2Plus: CountFraction;
  /** 4: dialogues where every one of their turns passed, against the dialogue count. */
  readonly dialoguesAllCorrect: CountFraction;
  readonly latencyMeanMs: number;
  readonly latencyP50Ms: number;
}

/** records grouped by dialogueId, each group sorted by turnIndex ascending. */
function byDialogue(records: readonly TurnRecord[]): ReadonlyMap<string, readonly TurnRecord[]> {
  const map = new Map<string, TurnRecord[]>();

  for (const record of records) {
    const group = map.get(record.dialogueId) ?? [];

    group.push(record);
    map.set(record.dialogueId, group);
  }

  for (const group of map.values()) {
    group.sort((a, b) => a.turnIndex - b.turnIndex);
  }

  return map;
}

/** The p-th percentile (0..1) of sorted, nearest-rank on a 0-based index. 0 when sorted is empty. */
function percentile(sorted: readonly number[], p: number): number {
  if (sorted.length === 0) return 0;

  const index = Math.min(sorted.length - 1, Math.floor(p * sorted.length));

  return sorted[index] ?? 0;
}

function countFraction(records: readonly TurnRecord[]): CountFraction {
  return { correct: records.filter((record) => record.pass).length, total: records.length };
}

/**
 * Turns a flat list of every turn run (`run.ts`, one dialogue after
 * another) into the report's numbers. Pure: no I/O, no model, so this is
 * what score.test.ts exercises with fakes and what `make check` runs
 * without a server.
 */
export function score(records: readonly TurnRecord[]): DialogueScore {
  const allTurns = countFraction(records);
  const laterTurns = records.filter((record) => record.turnIndex >= 1);
  const turns2Plus = countFraction(laterTurns);

  const grouped = byDialogue(records);
  let conditionalCorrect = 0;
  let conditionalTotal = 0;
  let dialoguesAllCorrect = 0;

  for (const group of grouped.values()) {
    if (group.every((record) => record.pass)) dialoguesAllCorrect += 1;

    let priorAllCorrect = true;

    for (const record of group) {
      if (record.turnIndex >= 1 && priorAllCorrect) {
        conditionalTotal += 1;
        if (record.pass) conditionalCorrect += 1;
      }

      if (!record.pass) priorAllCorrect = false;
    }
  }

  const latencies = records.map((record) => record.latencyMs).toSorted((a, b) => a - b);
  const latencyMeanMs =
    latencies.length === 0
      ? 0
      : latencies.reduce((sum, value) => sum + value, 0) / latencies.length;

  return {
    allTurns,
    turns2Plus,
    conditionalTurns2Plus: { correct: conditionalCorrect, total: conditionalTotal },
    dialoguesAllCorrect: { correct: dialoguesAllCorrect, total: grouped.size },
    latencyMeanMs,
    latencyP50Ms: percentile(latencies, 0.5),
  };
}
