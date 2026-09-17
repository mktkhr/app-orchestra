/**
 * Renders the mid measurement's report section (docs/plans/midsizing.md
 * Task 3; docs/specs/midsizing.md section 5). No I/O here - `run-mid.ts`
 * writes the jsonl and log files this is scored from, `pick-log.ts` reads
 * the log, `print-report.ts` wires the two together and prints what this
 * returns.
 */
import type { CountOf, MidScoreboard } from "./score-mid.ts";

/** "32/40" for `{ total: 40, count: 32 }`. */
function ratio(c: CountOf): string {
  return `${String(c.count)}/${String(c.total)}`;
}

function missLine(miss: MidScoreboard["misses"][number]): string {
  return `${miss.id} ${miss.text} → ${miss.kind} ${miss.operationId ?? "(none)"}`;
}

function missesBlock(misses: MidScoreboard["misses"]): string {
  if (misses.length === 0) return "misses: (none)";

  return ["misses:", ...misses.map((miss) => missLine(miss))].join("\n");
}

/** "pick mean 842" when the platform log had at least one `pick_ms` line, or a note explaining why not (docs/plans/midsizing.md Task 3: "else omit and say so"). */
function pickMeanLabel(pickMeanMs: number | undefined): string {
  if (pickMeanMs === undefined) {
    return "pick mean (not available: no pick_ms line found in the platform log)";
  }

  return `pick mean ${pickMeanMs.toFixed(0)}`;
}

/**
 * Renders the mid section: the five numbers (correct@1, false refusal,
 * refused, forced, fabricated), latency, and the misses list (every
 * answerable miss and every forced impossible) - docs/specs/midsizing.md
 * section 5's own sketch.
 */
export function renderMidSection(board: MidScoreboard, pickMeanMs: number | undefined): string {
  const answerableTotal = board.answerable.correctAt1.total;
  const impossibleTotal = board.impossible.refused.total;
  const formsTotal = board.forms.fabricated.total;

  return [
    "## mid (three services, thirty operations, sixty questions)",
    `answerable (${String(answerableTotal)})   correct@1  ${ratio(board.answerable.correctAt1)}   false refusal  ${ratio(board.answerable.falseRefusal)}`,
    `impossible (${String(impossibleTotal)})   refused    ${ratio(board.impossible.refused)}   forced         ${ratio(board.impossible.forced)}`,
    `forms (${String(formsTotal)})        fabricated ${ratio(board.forms.fabricated)}`,
    `latency (ms)      mean ${board.latency.meanMs.toFixed(0)}   p50 ${board.latency.p50Ms.toFixed(0)}   ${pickMeanLabel(pickMeanMs)}`,
    missesBlock(board.misses),
  ].join("\n");
}
