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
 * returns the ids that regressed on the rate they are judged on
 * (`tally.metric`, Case.metric) by more than tolerance - what `run.ts` exits
 * non-zero over (AC-E-201). An `"accept"` case (the default) regresses when
 * its accept rate drops below baseline; a `"reject"` case regresses the
 * other way round, when its reject rate *rises* above baseline - the case
 * that watches a specific wrong outcome cares that it did not become more
 * common, not that some other rate held. A rate that moved the good
 * direction is reported too, per section 4 ("improvements are as worth
 * seeing as regressions"), and a recorded reject outcome appearing at all is
 * reported whether or not the judged rate held (AC-E-203) - that is every
 * reject count already printed on the line, unconditionally.
 */
export function report(
  tallies: readonly CaseTally[],
  baseline: Baseline,
  tolerance: number,
): readonly string[] {
  const regressions: string[] = [];

  for (const tally of tallies) {
    const { metric } = tally;
    const acceptRate = rate(tally.accept, tally.total);
    const rejectRate = rate(tally.reject, tally.total);
    const judgedRate = metric === "accept" ? acceptRate : rejectRate;
    const baselineEntry = baseline.cases[tally.id];
    const metricNote = metric === "reject" ? " (judged: reject)" : "";
    let trailer = "";

    if (baselineEntry === undefined) {
      trailer = "  (no baseline recorded)";
    } else {
      const baselineJudgedCount = metric === "accept" ? baselineEntry.accept : baselineEntry.reject;
      const baselineJudgedRate = rate(baselineJudgedCount, baselineEntry.total);
      const delta = judgedRate - baselineJudgedRate;
      const regressed = metric === "accept" ? delta < -tolerance : delta > tolerance;
      const improved = metric === "accept" ? delta > tolerance : delta < -tolerance;
      const baselineFraction = fraction(baselineJudgedCount, baselineEntry.total);

      if (regressed) {
        trailer = `  ← REGRESSION, baseline ${baselineFraction} ${metric}`;
        regressions.push(tally.id);
      } else if (improved) {
        trailer = `  ← improved, baseline ${baselineFraction} ${metric}`;
      }
    }

    console.log(
      `${tally.id.padEnd(20)} ${fraction(tally.accept, tally.total).padEnd(8)} accept   ` +
        `${fraction(tally.reject, tally.total).padEnd(8)} reject${metricNote}${trailer}`,
    );
  }

  return regressions;
}
