import type { PlanOutcome } from "../eval/types.ts";
import type { DialogueScore } from "./score.ts";
import type { Dialogue, TurnRecord } from "./types.ts";

/** "N/M" for a report line's count. */
function fraction(count: number, total: number): string {
  return `${String(count)}/${String(total)}`;
}

/**
 * A short label for what a turn actually did - only printed on a failing
 * turn (report line 6), so a person reading the report sees why it
 * failed without re-reading the jsonl row.
 */
function describeOutcome(outcome: PlanOutcome): string {
  if (outcome.kind === "result" || outcome.kind === "form") {
    const decision = outcome.source ?? outcome.target;
    const operationId = decision?.operationId ?? "?";
    const args = decision?.args ?? outcome.initial ?? {};
    const named = Object.entries(args)
      .map(([key, value]) => `${key}=${String(value)}`)
      .toSorted()
      .join(",");

    return `${outcome.kind}:${operationId}(${named})`;
  }

  if (outcome.kind === "ask") {
    return `ask(${outcome.param ?? "?"})`;
  }

  return outcome.kind;
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

/** One `dNN: turn1:PASS  turn2:FAIL(...)` line per dialogue - report number 6. */
function dialogueLine(dialogue: Dialogue, records: readonly TurnRecord[]): string {
  const parts = records.map((record) => {
    const label = `turn${String(record.turnIndex + 1)}`;

    if (record.pass) return `${label}:PASS`;

    return `${label}:FAIL(${describeOutcome(record.outcome)})`;
  });

  return `${dialogue.id}: ${parts.join("  ")}`;
}

/** Renders the full text report (numbers 1-6) `run.ts` prints and saves. */
export function formatReport(
  dialogues: readonly Dialogue[],
  records: readonly TurnRecord[],
  result: DialogueScore,
): string {
  const grouped = byDialogue(records);

  const lines = [
    `all turns:             ${fraction(result.allTurns.correct, result.allTurns.total)}`,
    `turns 2+:              ${fraction(result.turns2Plus.correct, result.turns2Plus.total)}`,
    `turns 2+ (conditional):${fraction(
      result.conditionalTurns2Plus.correct,
      result.conditionalTurns2Plus.total,
    )}`,
    `dialogues all correct: ${fraction(
      result.dialoguesAllCorrect.correct,
      result.dialoguesAllCorrect.total,
    )}`,
    `latency mean: ${result.latencyMeanMs.toFixed(0)}ms  p50: ${result.latencyP50Ms.toFixed(0)}ms`,
    "",
    ...dialogues.map((dialogue) => dialogueLine(dialogue, grouped.get(dialogue.id) ?? [])),
  ];

  return lines.join("\n");
}
