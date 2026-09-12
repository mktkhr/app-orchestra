import type { Baseline } from "./baseline.ts";
import type { CaseTally } from "./types.ts";

/** accept/total as a fraction, or 0 when total is 0 (a case that never ran). */
function rate(accept: number, total: number): number {
  return total === 0 ? 0 : accept / total;
}

/** "N/M" for a report line's accept or reject column. */
function fraction(count: number, total: number): string {
  return `${String(count)}/${String(total)}`;
}

/**
 * Prints one report line per tally (docs/specs/eval.md section 4) and
 * returns the ids whose accept rate dropped below its baseline by more than
 * tolerance - what `run.ts` exits non-zero over (AC-E-201). A rate that rose
 * is reported too, per section 4 ("improvements are as worth seeing as
 * regressions"), and a recorded reject outcome appearing at all is reported
 * whether or not the accept rate held (AC-E-203) - that is every reject
 * count already printed on the line, unconditionally.
 */
export function report(
  tallies: readonly CaseTally[],
  baseline: Baseline,
  tolerance: number,
): readonly string[] {
  const regressions: string[] = [];

  for (const tally of tallies) {
    const acceptRate = rate(tally.accept, tally.total);
    const baselineEntry = baseline.cases[tally.id];
    let trailer = "";

    if (baselineEntry === undefined) {
      trailer = "  (no baseline recorded)";
    } else {
      const baselineRate = rate(baselineEntry.accept, baselineEntry.total);
      const delta = acceptRate - baselineRate;

      if (delta < -tolerance) {
        trailer = `  ← REGRESSION, baseline ${fraction(baselineEntry.accept, baselineEntry.total)} accept`;
        regressions.push(tally.id);
      } else if (delta > tolerance) {
        trailer = `  ← improved, baseline ${fraction(baselineEntry.accept, baselineEntry.total)} accept`;
      }
    }

    console.log(
      `${tally.id.padEnd(20)} ${fraction(tally.accept, tally.total).padEnd(8)} accept   ` +
        `${fraction(tally.reject, tally.total).padEnd(8)} reject${trailer}`,
    );
  }

  return regressions;
}
