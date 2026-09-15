import { writeFileSync } from "node:fs";

import type { Axis } from "../narrowing/corpus/index.ts";
import { outputPathFor } from "./pass-io.ts";
import { score, type Kind, type QuestionResult } from "./score.ts";

/**
 * The per-wording miss list (docs/plans/wording.md Task 2, Step 2): every
 * row whose pick was not an answer, grouped by axis, with a footer of kind
 * counts and the ids `list_capabilities` was picked for. This is what the
 * record quotes, so it is rendered as plain readable text, not JSON.
 *
 * A "miss" is any row score() marks not correct@1 - a wrong result or
 * form, but also an ask, a none, or an error, since none of those are the
 * answer either.
 */

const AXES: readonly Axis[] = ["A", "B", "C", "D", "E"];
const KINDS: readonly Kind[] = ["result", "ask", "none", "form", "proposal", "error"];

/** What a row picked, for the miss line - the operation id, or a parenthesised note when there was none to name. */
function pickLabel(result: QuestionResult): string {
  if (result.operationId !== undefined) return result.operationId;
  if (result.kind === "ask") return "(asked)";
  if (result.kind === "error") return "(error)";

  return "(none)";
}

function missLine(result: QuestionResult): string {
  const alternatives = (result.alternatives ?? []).join(", ") || "(none)";
  const answers = result.answers.join(", ") || "(none)";

  return [
    result.id,
    result.text,
    `kind=${result.kind}`,
    `pick=${pickLabel(result)}`,
    `alternatives=${alternatives}`,
    `answers=${answers}`,
  ].join(" | ");
}

function axisBlock(axis: Axis, misses: readonly QuestionResult[]): string {
  const rows = misses.filter((r) => r.axis === axis);

  if (rows.length === 0) return `## axis ${axis}\n(none)`;

  return [`## axis ${axis} (${String(rows.length)})`, ...rows.map((r) => missLine(r))].join("\n");
}

function kindCountsLine(misses: readonly QuestionResult[]): string {
  const counts = KINDS.map(
    (kind) => `${kind}=${String(misses.filter((r) => r.kind === kind).length)}`,
  );

  return `kind counts: ${counts.join(" ")}`;
}

function listCapabilitiesLine(misses: readonly QuestionResult[]): string {
  const ids = misses.filter((r) => r.operationId === "list_capabilities").map((r) => r.id);

  return `list_capabilities picked for: ${ids.length > 0 ? ids.join(", ") : "(none)"}`;
}

/** Renders one wording's miss list from its results - pure, so misses.test.ts covers it with fakes. */
export function renderMisses(wording: string, results: readonly QuestionResult[]): string {
  const misses = results.filter((r) => !score(r).correctAt1);
  const header = `# misses: ${wording} (${String(misses.length)} of ${String(results.length)})`;
  const axes = AXES.map((axis) => axisBlock(axis, misses));
  const footer = [kindCountsLine(misses), listCapabilitiesLine(misses)].join("\n");

  return [header, "", ...axes, "", footer].join("\n");
}

/** Writes a wording's miss list to `out/misses-<name>.txt`. */
export function writeMisses(wording: string, results: readonly QuestionResult[]): void {
  writeFileSync(
    outputPathFor(`misses-${wording}`).replace(/\.jsonl$/u, ".txt"),
    renderMisses(wording, results),
  );
}
